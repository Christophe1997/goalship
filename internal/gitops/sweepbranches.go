package gitops

import (
	"fmt"
	"strings"
)

// SweepCandidate outcomes — the CLI's JSON wire values, matching this
// package's existing named-outcome-constant convention (see reconcile.go's
// Outcome* constants).
const (
	SweepOutcomeWouldDelete = "would-delete"
	SweepOutcomeDeleted     = "deleted"
	SweepOutcomeFailed      = "failed"
)

// SweepCandidate is one goalship-managed ticket branch SweepBranches judged
// eligible for cleanup: it is still on origin, its ticket's recorded PR has
// merged, and it is not currently any open PR's base or head.
type SweepCandidate struct {
	TicketID string
	Branch   string
	PRRef    string
	Outcome  string // one of the SweepOutcome* constants
	Error    string // set only when Outcome == SweepOutcomeFailed
}

// SweepBranches finds every ticket branch that is still on origin, has a
// merged PR, and isn't any currently open PR's base or head, and either
// reports (execute == false) or deletes (execute == true) each one from
// origin. The open-PR graph must validate before any ticket is evaluated: a
// malformed/truncated graph makes the whole eligibility check unsafe, not
// just one candidate, so a ListOpenPRs/ValidateOpenPRGraph failure is a hard
// error for the whole call, returned before any candidate is considered.
// Each delete is re-verified against a fresh open-PR listing, so a PR opened
// onto a candidate since the initial snapshot blocks its delete. A
// per-branch delete failure in execute mode is not hard: it's recorded on
// that candidate (Outcome "failed") and the batch continues. A hard error
// after some branches were already deleted names them, since no candidate
// list is returned alongside it.
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

	referenced := referencedBranches(prs)

	candidates := make([]SweepCandidate, 0, len(tickets))
	var deleted []string
	for _, t := range tickets {
		id, _ := t["id"].(string)
		if id == "" {
			continue
		}
		fields, err := noteFieldsForTicket(repoRoot, id)
		if err != nil {
			return nil, abortedSweepError(deleted, err)
		}
		branch := fields["branch"]
		prRef := fields["pr"]
		if branch == "" || prRef == "" {
			continue
		}
		if _, inUse := referenced[branch]; inUse {
			continue
		}
		onOrigin, err := remoteBranchExists(repoRoot, branch)
		if err != nil {
			return nil, abortedSweepError(deleted, err)
		}
		if !onOrigin {
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
			if err := deleteUnreferencedBranch(repoRoot, hostTool, branch); err != nil {
				candidate.Outcome = SweepOutcomeFailed
				candidate.Error = err.Error()
			} else {
				candidate.Outcome = SweepOutcomeDeleted
				deleted = append(deleted, branch)
			}
		}
		candidates = append(candidates, candidate)
	}

	return candidates, nil
}

// abortedSweepError makes a hard mid-sweep error carry the branches already
// deleted: the caller gets no candidate list back on error, so without this
// the operator would have no record that anything was removed.
func abortedSweepError(deleted []string, err error) error {
	if len(deleted) == 0 {
		return err
	}
	return fmt.Errorf("gitops: sweep aborted after deleting %s: %w", strings.Join(deleted, ", "), err)
}

// remoteBranchExists reports whether origin holds branch, asking about that
// one ref rather than listing origin's branches. ls-remote treats its pattern
// as a glob matched against ref tails, so the output is compared for the
// exact ref instead of trusting that any output means a match. Its argument
// always starts with "refs/heads/", so unlike deleteRemoteBranch it needs no
// rejectFlagLikeRef guard.
func remoteBranchExists(repoRoot, branch string) (bool, error) {
	ref := "refs/heads/" + branch
	out, err := run(repoRoot, "git", "ls-remote", "--heads", "origin", ref)
	if err != nil {
		return false, err
	}
	for line := range strings.SplitSeq(out, "\n") {
		if _, lineRef, _ := strings.Cut(line, "\t"); lineRef == ref {
			return true, nil
		}
	}
	return false, nil
}

// referencedBranches maps every branch some open PR uses as its base, or as
// its head, to the first such PR's number: a branch can carry a merged PR in
// its history and still be a different, currently-open PR's live head (e.g.
// reused after its own PR merged). A cross-repo PR's head is not a branch
// here, but its base still is.
func referencedBranches(prs []OpenPR) map[string]int {
	referenced := make(map[string]int, len(prs)*2)
	for _, pr := range prs {
		branches := []string{pr.Base}
		if !pr.CrossRepo {
			branches = append(branches, pr.Branch)
		}
		for _, branch := range branches {
			if _, seen := referenced[branch]; !seen {
				referenced[branch] = pr.Number
			}
		}
	}
	return referenced
}

// deleteUnreferencedBranch deletes branch only after a fresh open-PR listing
// shows nothing references it. The initial snapshot can be minutes old by
// the time a delete is reached, and the host silently closes a PR whose
// base branch disappears. This narrows that window but cannot close it: no
// host offers an atomic delete-unless-referenced.
func deleteUnreferencedBranch(repoRoot, hostTool, branch string) error {
	prs, err := ListOpenPRs(repoRoot, hostTool)
	if err != nil {
		return fmt.Errorf("gitops: re-listing open PRs before deleting %q: %w", branch, err)
	}
	if err := ValidateOpenPRGraph(prs); err != nil {
		return fmt.Errorf("gitops: re-listed open PRs before deleting %q are unsafe: %w", branch, err)
	}
	if number, inUse := referencedBranches(prs)[branch]; inUse {
		return fmt.Errorf("gitops: branch %q is now referenced by open PR #%d", branch, number)
	}
	return deleteRemoteBranch(repoRoot, branch)
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
