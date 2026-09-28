package gitops

// SweepCandidate outcomes — the CLI's JSON wire values, matching this
// package's existing named-outcome-constant convention (see reconcile.go's
// Outcome* constants).
const (
	SweepOutcomeWouldDelete = "would-delete"
	SweepOutcomeDeleted     = "deleted"
	SweepOutcomeFailed      = "failed"
)

// SweepCandidate is one goalship-managed ticket branch SweepBranches judged
// eligible for cleanup: its ticket's recorded PR has merged, and the branch
// is not currently any open PR's base or head.
type SweepCandidate struct {
	TicketID string
	Branch   string
	PRRef    string
	Outcome  string // one of the SweepOutcome* constants
	Error    string // set only when Outcome == SweepOutcomeFailed
}

// SweepBranches finds every ticket branch with a merged PR that isn't any
// currently open PR's base or head, and either reports (execute == false)
// or deletes (execute == true) each one from origin. The open-PR graph
// must validate before any ticket is evaluated: a malformed/truncated
// graph makes the whole eligibility check unsafe, not just one candidate,
// so a ListOpenPRs/ValidateOpenPRGraph failure is a hard error for the
// whole call, returned before any candidate is considered. A per-branch
// delete failure in execute mode is not hard: it's recorded on that
// candidate (Outcome "failed") and the batch continues.
func SweepBranches(repoRoot, hostTool string, execute bool) ([]SweepCandidate, error) {
	tickets, err := tkQuery(repoRoot, ".")
	if err != nil {
		return nil, err
	}

	prs, err := ListOpenPRs(repoRoot, hostTool)
	if err != nil {
		return nil, err
	}
	if err := ValidateOpenPRGraph(prs); err != nil {
		return nil, err
	}

	// A branch that's any open PR's base OR head is excluded: it can carry
	// a merged PR in its history and still be a different, currently-open
	// PR's live head (e.g. reused after its own PR merged). A cross-repo
	// PR's head is not a branch here, but its base still is.
	baseOrHead := make(map[string]bool, len(prs)*2)
	for _, pr := range prs {
		baseOrHead[pr.Base] = true
		if !pr.CrossRepo {
			baseOrHead[pr.Branch] = true
		}
	}

	candidates := make([]SweepCandidate, 0, len(tickets))
	for _, t := range tickets {
		id, _ := t["id"].(string)
		if id == "" {
			continue
		}
		fields, err := noteFieldsForTicket(repoRoot, id)
		if err != nil {
			return nil, err
		}
		branch := fields["branch"]
		prRef := fields["pr"]
		if branch == "" || prRef == "" || baseOrHead[branch] {
			continue
		}
		if state, ok := PRState(repoRoot, hostTool, prRef); !ok || state != "merged" {
			continue
		}

		candidate := SweepCandidate{TicketID: id, Branch: branch, PRRef: prRef}
		switch {
		case !execute:
			candidate.Outcome = SweepOutcomeWouldDelete
		default:
			if err := deleteRemoteBranch(repoRoot, branch); err != nil {
				candidate.Outcome = SweepOutcomeFailed
				candidate.Error = err.Error()
			} else {
				candidate.Outcome = SweepOutcomeDeleted
			}
		}
		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// deleteRemoteBranch removes branch from origin. Never force (there's
// nothing to force about a delete); mirrors PushBranch's rejectFlagLikeRef
// guard since branch comes from ticket-note data, not a literal.
func deleteRemoteBranch(repoRoot, branch string) error {
	if err := rejectFlagLikeRef("branch", branch); err != nil {
		return err
	}
	_, err := run(repoRoot, "git", "push", "origin", "--delete", branch)
	return err
}
