package gitops

import "fmt"

// Reconciliation outcomes — mirrors reconciliation.py's ReconciliationAction
// outcome strings exactly (loop_runner.py's callers switch on these literal
// values, so they must match byte-for-byte).
const (
	OutcomeClosedMerged           = "closed_merged"
	OutcomeClosedShipNoteOrphaned = "closed_ship_note_orphaned"
	OutcomeFailedClosedUnmerged   = "failed_closed_unmerged"
	OutcomeNoRecoverableState     = "no_recoverable_state"
	OutcomeRetryPRCreation        = "retry_pr_creation"
	OutcomeRetargetBaseMerged     = "retarget_base_merged"
	OutcomeBlockedStaleBase       = "blocked_stale_base"
	OutcomePRStateUnresolved      = "pr_state_unresolved"
)

// ReconciliationAction reports one in-progress ticket reconcile touched.
// PRRef is only ever populated for closed_ship_note_orphaned and
// retarget_base_merged — for the latter it is the ticket's OWN pr (not
// base's), so a caller can retarget it directly without re-deriving it via
// tk show. Mirrors reconciliation.py's ReconciliationAction.
type ReconciliationAction struct {
	TicketID string
	Outcome  string
	Detail   string
	PRRef    string
}

// ReconciliationReport is Reconcile's result. AuthFailure is "" when absent
// (a plain Go zero value, not *string) — the CLI print layer converts that
// to JSON null, mirroring this codebase's layering elsewhere. Mirrors
// reconciliation.py's ReconciliationReport.
type ReconciliationReport struct {
	Actions     []ReconciliationAction
	AuthFailure string
}

// tkTicketClose, tkTicketReopen, and tkTicketAddNote are thin `tk`
// subprocess wrappers — mirrors reconciliation.py's tk_close, tk_reopen,
// and tk_add_note. Named distinctly from helpers_test.go's identically
// themed but differently-signatured (*testing.T-first) fixture helpers of
// almost the same name, which those tests keep using directly; Go has no
// overloading, so this package can't hold both under the exact same name.
func tkTicketClose(repoRoot, ticketID string) error {
	_, err := run(repoRoot, "tk", "close", ticketID)
	return err
}

func tkTicketReopen(repoRoot, ticketID string) error {
	_, err := run(repoRoot, "tk", "reopen", ticketID)
	return err
}

func tkTicketAddNote(repoRoot, ticketID, text string) error {
	_, err := run(repoRoot, "tk", "add-note", ticketID, text)
	return err
}

// findTicketByBranch returns the ticket whose merged note fields carry
// branch, scanning every ticket regardless of status — a stacked base is
// normally already closed by the time its own PR has merged, so limiting
// this to in-progress tickets would make retarget_base_merged unreachable.
// Mirrors reconciliation.py's find_ticket_by_branch.
func findTicketByBranch(repoRoot, branch string) (ticketID string, fields map[string]string, found bool, err error) {
	tickets, err := tkQuery(repoRoot, ".")
	if err != nil {
		return "", nil, false, err
	}
	for _, t := range tickets {
		id, _ := t["id"].(string)
		if id == "" {
			continue
		}
		f, err := noteFieldsForTicket(repoRoot, id)
		if err != nil {
			return "", nil, false, err
		}
		if f["branch"] == branch {
			return id, f, true, nil
		}
	}
	return "", nil, false, nil
}

// reconcileStackedBase checks whether a ticket's open PR, stacked on base,
// has had base's own PR resolve out from under it. Returns a nil action
// (not an error) both when base's PR is still open and when its lookup
// itself failed — a lookup failure on the BASE's PR falls through to the
// caller's ship-note-orphan check rather than becoming pr_state_unresolved,
// which is reserved for a failed lookup on the ticket's OWN PR. Mirrors
// reconciliation.py's _reconcile_stacked_base.
func reconcileStackedBase(repoRoot, hostTool, ticketID, base, prRef string) (*ReconciliationAction, error) {
	_, baseFields, found, err := findTicketByBranch(repoRoot, base)
	if err != nil {
		return nil, err
	}
	var baseState string
	if found {
		if basePR := baseFields["pr"]; basePR != "" {
			baseState, _ = PRState(repoRoot, hostTool, basePR)
		}
	}
	switch baseState {
	case "merged":
		return &ReconciliationAction{TicketID: ticketID, Outcome: OutcomeRetargetBaseMerged, Detail: base, PRRef: prRef}, nil
	case "closed":
		note := fmt.Sprintf("Reconciliation: base %s closed without merging; blocked.", base)
		if err := tkTicketAddNote(repoRoot, ticketID, note); err != nil {
			return nil, err
		}
		return &ReconciliationAction{TicketID: ticketID, Outcome: OutcomeBlockedStaleBase, Detail: base}, nil
	default:
		return nil, nil
	}
}

// Reconcile cross-checks every in-progress ticket against git/PR state
// before the next `tk ready` pick. Always queries tk directly for the
// in-progress set (not a run-state ledger) — no ledger dependency to fall
// back from in the first place. Mirrors reconciliation.py's reconcile.
func Reconcile(repoRoot string) (*ReconciliationReport, error) {
	inProgress, err := tkQuery(repoRoot, `select(.status=="in_progress")`)
	if err != nil {
		return nil, err
	}

	type ticketFields struct {
		id     string
		fields map[string]string
	}
	tickets := make([]ticketFields, 0, len(inProgress))
	needsHostLookup := false
	for _, t := range inProgress {
		id, _ := t["id"].(string)
		fields, err := noteFieldsForTicket(repoRoot, id)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, ticketFields{id: id, fields: fields})
		if fields["pr"] != "" || fields["branch"] != "" {
			needsHostLookup = true
		}
	}

	var hostTool string
	if needsHostLookup {
		hostTool = detectHostTool(gitRemoteURL(repoRoot))
		if hostTool == "" || !hostToolAuthenticated(hostTool) {
			// A credential that keeps failing routes to a preflight-class
			// stop instead of retrying per ticket without limit — no
			// tickets are processed in this path.
			failedTool := hostTool
			if failedTool == "" {
				failedTool = "gh/glab"
			}
			return &ReconciliationReport{AuthFailure: failedTool}, nil
		}
	}

	var actions []ReconciliationAction
	for _, t := range tickets {
		ticketID, fields := t.id, t.fields
		prRef := fields["pr"]
		branch := fields["branch"]
		base := fields["base"]
		sha := fields["sha"]

		if prRef == "" {
			if branch != "" {
				actions = append(actions, ReconciliationAction{TicketID: ticketID, Outcome: OutcomeRetryPRCreation, Detail: branch})
			} else {
				actions = append(actions, ReconciliationAction{TicketID: ticketID, Outcome: OutcomeNoRecoverableState})
			}
			continue
		}

		state, _ := PRState(repoRoot, hostTool, prRef)
		switch state {
		case "merged":
			note := fmt.Sprintf("Reconciliation: PR %s merged externally; closing.", prRef)
			if err := tkTicketAddNote(repoRoot, ticketID, note); err != nil {
				return nil, err
			}
			if err := tkTicketClose(repoRoot, ticketID); err != nil {
				return nil, err
			}
			actions = append(actions, ReconciliationAction{TicketID: ticketID, Outcome: OutcomeClosedMerged, Detail: prRef})
		case "closed":
			note := fmt.Sprintf("Reconciliation: PR %s closed without merging; left open.", prRef)
			if err := tkTicketAddNote(repoRoot, ticketID, note); err != nil {
				return nil, err
			}
			if err := tkTicketReopen(repoRoot, ticketID); err != nil {
				return nil, err
			}
			actions = append(actions, ReconciliationAction{TicketID: ticketID, Outcome: OutcomeFailedClosedUnmerged, Detail: prRef})
		case "open":
			var action *ReconciliationAction
			if base != "" {
				action, err = reconcileStackedBase(repoRoot, hostTool, ticketID, base, prRef)
				if err != nil {
					return nil, err
				}
			}
			switch {
			case action != nil:
				actions = append(actions, *action)
			case sha != "":
				// sha only appears alongside pr once record_ship_note has
				// already run, meaning a crash happened between
				// record_ship_note and cmd_ship's follow-up tk_close —
				// finish the close the crash interrupted.
				if err := tkTicketClose(repoRoot, ticketID); err != nil {
					return nil, err
				}
				actions = append(actions, ReconciliationAction{TicketID: ticketID, Outcome: OutcomeClosedShipNoteOrphaned, Detail: branch, PRRef: prRef})
			}
		default:
			actions = append(actions, ReconciliationAction{TicketID: ticketID, Outcome: OutcomePRStateUnresolved, Detail: prRef})
		}
	}

	return &ReconciliationReport{Actions: actions}, nil
}
