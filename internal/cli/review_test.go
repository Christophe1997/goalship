package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ledger"
)

// chdir changes the process cwd for the duration of a test and restores it
// after — mirrors internal/cli/tk/ticketsdir_test.go's own helper of the
// same name (package-private, not t.Chdir, for the same reason noted
// there).
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}
}

// newReviewTestRepo creates a repo root with just a ".git" marker (findRepoRoot
// only checks for its existence, not real git plumbing — internal/gitops's
// own suite already covers real git behavior) and chdirs the test into a
// nested subdirectory of it, so review-status's own repo-root discovery is
// exercised from somewhere other than the root itself.
func newReviewTestRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	if err := os.Mkdir(filepath.Join(repoRoot, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	nested := filepath.Join(repoRoot, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested dir: %v", err)
	}
	t.Cleanup(chdir(t, nested))
	return repoRoot
}

func execReviewStatus(t *testing.T, args []string) (string, error) {
	t.Helper()
	cmd := NewReviewStatusCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func decodeReviewStatusJSON(t *testing.T, out string) reviewStatusResult {
	t.Helper()
	var got reviewStatusResult
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	return got
}

func TestReviewStatusCmd_PendingRun_NoActionableRejection(t *testing.T) {
	repoRoot := newReviewTestRepo(t)
	state := &ledger.RunState{RunID: "run-pending", ReviewState: ledger.ReviewStatePending}
	if err := state.Save(repoRoot); err != nil {
		t.Fatalf("save fixture ledger: %v", err)
	}

	out, err := execReviewStatus(t, []string{"run-pending"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := decodeReviewStatusJSON(t, out)
	if got.ReviewState != ledger.ReviewStatePending {
		t.Errorf("review_state = %q, want %q", got.ReviewState, ledger.ReviewStatePending)
	}
	if got.ReviewNotes != "" {
		t.Errorf("review_notes = %q, want empty (no actionable rejection)", got.ReviewNotes)
	}
}

func TestReviewStatusCmd_RejectedRun_NotesVerbatim(t *testing.T) {
	repoRoot := newReviewTestRepo(t)
	notes := "line one\nline two — unicode: café ✅\nline three, trailing spaces:   "
	state := &ledger.RunState{
		RunID:           "run-rejected",
		ReviewState:     "rejected",
		ReviewNotes:     notes,
		ReviewUpdatedAt: "2026-09-03T12:00:00Z",
	}
	if err := state.Save(repoRoot); err != nil {
		t.Fatalf("save fixture ledger: %v", err)
	}

	out, err := execReviewStatus(t, []string{"run-rejected"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	got := decodeReviewStatusJSON(t, out)
	if got.ReviewState != "rejected" {
		t.Errorf("review_state = %q, want %q", got.ReviewState, "rejected")
	}
	if got.ReviewNotes != notes {
		t.Errorf("review_notes = %q, want %q byte-for-byte", got.ReviewNotes, notes)
	}
	if got.ReviewUpdatedAt != "2026-09-03T12:00:00Z" {
		t.Errorf("review_updated_at = %q, want %q", got.ReviewUpdatedAt, "2026-09-03T12:00:00Z")
	}
}

func TestReviewStatusCmd_ApprovedRun_ReportsApprovedExplicitly(t *testing.T) {
	repoRoot := newReviewTestRepo(t)
	state := &ledger.RunState{RunID: "run-approved", ReviewState: "approved"}
	if err := state.Save(repoRoot); err != nil {
		t.Fatalf("save fixture ledger: %v", err)
	}

	out, err := execReviewStatus(t, []string{"run-approved"})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, `"approved"`) {
		t.Errorf("output = %q, want the literal string %q", out, "approved")
	}
	got := decodeReviewStatusJSON(t, out)
	if got.ReviewState != "approved" {
		t.Errorf("review_state = %q, want %q", got.ReviewState, "approved")
	}
}

func TestReviewStatusCmd_NoLedgerFile_Errors(t *testing.T) {
	newReviewTestRepo(t) // a real repo root, but no ledger ever written

	_, err := execReviewStatus(t, []string{"no-such-run"})
	if err == nil {
		t.Fatal("execute: want an error for a run-id with no ledger file, got nil")
	}
}

func TestReviewStatusCmd_NoGitRepo_ErrorsClearly(t *testing.T) {
	t.Cleanup(chdir(t, t.TempDir()))

	_, err := execReviewStatus(t, []string{"whatever"})
	if err == nil {
		t.Fatal("execute: want an error when no git repository is found, got nil")
	}
}

// execReview runs `goalship review <args>` and returns combined
// stdout+stderr and the command's error. Only the two refusal paths below
// belong in this package's test suite — a run that actually reaches
// reviewserver.Run would bind a listener and block on ctx.Done(), which
// has no place in a fast unit-test suite.
func execReview(t *testing.T, args []string) (string, error) {
	t.Helper()
	cmd := NewReviewCmd()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return buf.String(), err
}

func TestReviewCmd_MissingRunID_RefusesWithClearError(t *testing.T) {
	newReviewTestRepo(t) // a real repo root, but no ledger ever written

	_, err := execReview(t, []string{"no-such-run"})
	if err == nil {
		t.Fatal("execute: want an error for a run-id with no ledger file, got nil")
	}
}

func TestReviewCmd_NoGitRepo_RefusesWithClearError(t *testing.T) {
	t.Cleanup(chdir(t, t.TempDir()))

	_, err := execReview(t, []string{"whatever"})
	if err == nil {
		t.Fatal("execute: want an error when no git repository is found, got nil")
	}
}

func TestReviewCmd_AlreadyApprovedRun_RefusesWithClearError(t *testing.T) {
	repoRoot := newReviewTestRepo(t)
	state := &ledger.RunState{RunID: "run-approved", ReviewState: ledger.ReviewStateApproved}
	if err := state.Save(repoRoot); err != nil {
		t.Fatalf("save fixture ledger: %v", err)
	}

	_, err := execReview(t, []string{"run-approved"})
	if err == nil {
		t.Fatal("execute: want an error for an already-approved run, got nil")
	}
	if !strings.Contains(err.Error(), "run-approved") || !strings.Contains(err.Error(), "approved") {
		t.Errorf("error = %q, want it to name the run id and mention it's already approved", err.Error())
	}
}
