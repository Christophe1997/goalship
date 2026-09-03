package loop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/gitops"
	"github.com/Christophe1997/goalship/internal/ledger"
)

// claimTestSetup creates a repo with one ticket and returns the repo root
// and that ticket's ID, ready for a claim call against runID "r1".
func claimTestSetup(t *testing.T) (repoRoot, ticketID string) {
	t.Helper()
	repoRoot = newLoopTestRepo(t)
	ticketID = tkCreate(t, repoRoot, "Claimable ticket")
	return repoRoot, ticketID
}

func saveRunState(t *testing.T, repoRoot string, state *ledger.RunState) {
	t.Helper()
	if err := state.Save(repoRoot); err != nil {
		t.Fatalf("save run state: %v", err)
	}
}

func execClaimExpectError(t *testing.T, args []string) error {
	t.Helper()
	cmd := NewClaimCmd()
	cmd.SetOut(&noopWriter{})
	cmd.SetErr(&noopWriter{})
	cmd.SetArgs(args)
	return cmd.Execute()
}

type noopWriter struct{}

func (*noopWriter) Write(p []byte) (int, error) { return len(p), nil }

func TestClaimCmd_RefusesWhenReviewStatePending(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	saveRunState(t, repoRoot, &ledger.RunState{RunID: "r1", ReviewState: ledger.ReviewStatePending})

	err := execClaimExpectError(t, []string{repoRoot, ticketID, "feat/x", "main", "main", "--run-id", "r1"})
	if err == nil {
		t.Fatal("claim: expected an error for a pending review_state, got nil")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Errorf("error = %q, want it to mention \"not approved\"", err)
	}

	// The gate must run before any git operation: a refused claim must
	// never create the branch it was asked to claim.
	if exists, _ := gitops.LocalBranchExists(repoRoot, "feat/x"); exists {
		t.Error("branch \"feat/x\" was created despite the claim being refused")
	}
}

func TestClaimCmd_RefusesWhenReviewStateRejected(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	saveRunState(t, repoRoot, &ledger.RunState{RunID: "r1", ReviewState: ledger.ReviewStateRejected})

	err := execClaimExpectError(t, []string{repoRoot, ticketID, "feat/x", "main", "main", "--run-id", "r1"})
	if err == nil {
		t.Fatal("claim: expected an error for a rejected review_state, got nil")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Errorf("error = %q, want it to mention \"not approved\"", err)
	}
}

// TestClaimCmd_RefusesWhenReviewStateFieldAbsent covers a ledger file
// written before review_state existed at all (not merely a fresh/missing
// ledger) — ParseRunState must default the absent key to pending the same
// way LoadRunState's own missing-file branch does.
func TestClaimCmd_RefusesWhenReviewStateFieldAbsent(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	path := ledger.ResolveLedgerPath(repoRoot, "r1")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir ledger dir: %v", err)
	}
	rawLedger := `{
  "run_id": "r1",
  "shipped_count": 0,
  "consecutive_failures": 0,
  "claimed_ticket_ids": [],
  "goal": "",
  "ticket_mode": null,
  "terminal_state": null,
  "trunk_branch": null
}`
	if err := os.WriteFile(path, []byte(rawLedger), 0o644); err != nil {
		t.Fatalf("write raw ledger: %v", err)
	}

	err := execClaimExpectError(t, []string{repoRoot, ticketID, "feat/x", "main", "main", "--run-id", "r1"})
	if err == nil {
		t.Fatal("claim: expected an error for a ledger with no review_state key, got nil")
	}
	if !strings.Contains(err.Error(), "not approved") {
		t.Errorf("error = %q, want it to mention \"not approved\"", err)
	}
}

func TestClaimCmd_RefusesWhenTicketNotInApprovedSet(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	saveRunState(t, repoRoot, &ledger.RunState{
		RunID: "r1", ReviewState: ledger.ReviewStateApproved,
		ApprovedTicketIDs: []string{"some-other-ticket"},
	})

	err := execClaimExpectError(t, []string{repoRoot, ticketID, "feat/x", "main", "main", "--run-id", "r1"})
	if err == nil {
		t.Fatal("claim: expected an error when the ticket isn't in approved_ticket_ids, got nil")
	}
	if !strings.Contains(err.Error(), "not in its approved_ticket_ids") {
		t.Errorf("error = %q, want it to mention approved_ticket_ids", err)
	}
}

func TestClaimCmd_ApprovedAndInSet_Succeeds(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	saveRunState(t, repoRoot, &ledger.RunState{
		RunID: "r1", ReviewState: ledger.ReviewStateApproved,
		ApprovedTicketIDs: []string{ticketID},
	})

	out := execCmd(t, NewClaimCmd(), []string{repoRoot, ticketID, "feat/x", "main", "main", "--run-id", "r1"})
	if out != "" {
		t.Errorf("stdout = %q, want empty on success (matches the Python original)", out)
	}

	current := strings.TrimSpace(runLoopGit(t, repoRoot, "branch", "--show-current"))
	if current != "feat/x" {
		t.Errorf("current branch = %q, want %q", current, "feat/x")
	}

	body := ticketBody(t, repoRoot, ticketID)
	if !strings.Contains(body, "branch: feat/x") {
		t.Errorf("note missing \"branch: feat/x\":\n%s", body)
	}
	if strings.Contains(body, "base:") {
		t.Errorf("note should omit \"base:\" when base-ref equals trunk-branch:\n%s", body)
	}
	if !strings.Contains(body, "claim_sha:") {
		t.Errorf("note missing \"claim_sha:\":\n%s", body)
	}
}

func TestClaimCmd_BaseRefDiffersFromTrunk_NoteRecordsBase(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	runLoopGit(t, repoRoot, "checkout", "-b", "feat/dep")
	runLoopGit(t, repoRoot, "checkout", "main")
	saveRunState(t, repoRoot, &ledger.RunState{
		RunID: "r1", ReviewState: ledger.ReviewStateApproved,
		ApprovedTicketIDs: []string{ticketID},
	})

	execCmd(t, NewClaimCmd(), []string{repoRoot, ticketID, "feat/x", "feat/dep", "main", "--run-id", "r1"})

	body := ticketBody(t, repoRoot, ticketID)
	if !strings.Contains(body, "base: feat/dep") {
		t.Errorf("note missing \"base: feat/dep\" when base-ref != trunk-branch:\n%s", body)
	}
}

// TestClaimCmd_CrashRecovery_ChecksOutExistingBranchAndWritesNote simulates
// a prior claim that created the branch but crashed before writing the
// claim note: retrying must succeed, must not error on "branch already
// exists", must not create a second/duplicate branch, and must still write
// the note.
func TestClaimCmd_CrashRecovery_ChecksOutExistingBranchAndWritesNote(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	saveRunState(t, repoRoot, &ledger.RunState{
		RunID: "r1", ReviewState: ledger.ReviewStateApproved,
		ApprovedTicketIDs: []string{ticketID},
	})

	// Simulate the crash: branch exists (as create_branch would have left
	// it), but no claim note was ever written.
	runLoopGit(t, repoRoot, "checkout", "-b", "feat/x", "main")
	runLoopGit(t, repoRoot, "checkout", "main")
	if strings.Contains(ticketBody(t, repoRoot, ticketID), "branch:") {
		t.Fatal("test setup: ticket already has a branch note before the crash-recovery claim")
	}

	out := execCmd(t, NewClaimCmd(), []string{repoRoot, ticketID, "feat/x", "main", "main", "--run-id", "r1"})
	if out != "" {
		t.Errorf("stdout = %q, want empty on success", out)
	}

	current := strings.TrimSpace(runLoopGit(t, repoRoot, "branch", "--show-current"))
	if current != "feat/x" {
		t.Errorf("current branch = %q, want %q (checked out, not recreated)", current, "feat/x")
	}
	branches := runLoopGit(t, repoRoot, "branch", "--format=%(refname:short)")
	if n := strings.Count(branches, "feat/x"); n != 1 {
		t.Errorf("branch \"feat/x\" appears %d times in %q, want exactly 1 (no duplicate)", n, branches)
	}

	body := ticketBody(t, repoRoot, ticketID)
	if !strings.Contains(body, "branch: feat/x") {
		t.Errorf("note not written on crash-recovery retry:\n%s", body)
	}
}

func TestClaimCmd_MissingRunID_Errors(t *testing.T) {
	repoRoot, ticketID := claimTestSetup(t)
	err := execClaimExpectError(t, []string{repoRoot, ticketID, "feat/x", "main", "main"})
	if err == nil {
		t.Fatal("claim: expected an error when --run-id is omitted, got nil")
	}
}

func ticketBody(t *testing.T, repoRoot, ticketID string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot, ".tickets", ticketID+".md"))
	if err != nil {
		t.Fatalf("read ticket file for %q: %v", ticketID, err)
	}
	return string(data)
}
