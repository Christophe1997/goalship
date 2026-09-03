package gitops

import "testing"

func TestRunBranch_EmptyTicketList_ReturnsEmpty(t *testing.T) {
	repoRoot := newTestRepo(t)
	branch, err := RunBranch(repoRoot, nil)
	if err != nil {
		t.Fatalf("RunBranch: %v", err)
	}
	if branch != "" {
		t.Errorf("branch = %q, want empty", branch)
	}
}

func TestRunBranch_ClaimedTicketWithNoBranchNote_ReturnsEmpty(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "Claimed but not yet branched")

	branch, err := RunBranch(repoRoot, []string{ticketID})
	if err != nil {
		t.Fatalf("RunBranch: %v", err)
	}
	if branch != "" {
		t.Errorf("branch = %q, want empty", branch)
	}
}

func TestRunBranch_ReturnsClaimedTicketsBranch(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "Claimed and branched")
	tkAddNote(t, repoRoot, ticketID, "branch: feat/shared-goal")

	branch, err := RunBranch(repoRoot, []string{ticketID})
	if err != nil {
		t.Fatalf("RunBranch: %v", err)
	}
	if branch != "feat/shared-goal" {
		t.Errorf("branch = %q, want %q", branch, "feat/shared-goal")
	}
}

// TestRunBranch_FindsItRegardlessOfWhichTicketCarriesIt: ticket 2..N of a
// commit-mode run reuse ticket 1's branch, discovered from whichever
// claimed ticket happens to carry the branch note.
func TestRunBranch_FindsItRegardlessOfWhichTicketCarriesIt(t *testing.T) {
	repoRoot := newTestRepo(t)
	first := tkCreate(t, repoRoot, "First ticket, no branch note yet")
	second := tkCreate(t, repoRoot, "Second ticket, already branched")
	tkAddNote(t, repoRoot, second, "branch: feat/shared-goal")

	branch, err := RunBranch(repoRoot, []string{first, second})
	if err != nil {
		t.Fatalf("RunBranch: %v", err)
	}
	if branch != "feat/shared-goal" {
		t.Errorf("branch = %q, want %q", branch, "feat/shared-goal")
	}
}
