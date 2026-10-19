package loop

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket/tickettest"
)

// tkStart and tkAddNote mirror tkCreate (branch_test.go): fixture builders
// over internal/ticket/tickettest. This package can't reach
// internal/gitops's own identically-purposed test helpers — different
// package, unexported.
func tkStart(t *testing.T, repoRoot, ticketID string) {
	t.Helper()
	tickettest.SetStatus(t, repoRoot, ticketID, "in_progress")
}

func tkAddNote(t *testing.T, repoRoot, ticketID, text string) {
	t.Helper()
	tickettest.AddNote(t, repoRoot, ticketID, text)
}

// pathWithoutHostTools returns a PATH holding only git — no gh/glab
// anywhere on it.
func pathWithoutHostTools(t *testing.T) string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath git: %v", err)
	}
	// A dir holding only git: on Ubuntu runners gh shares /usr/bin with git.
	dir := t.TempDir()
	if err := os.Symlink(gitPath, filepath.Join(dir, "git")); err != nil {
		t.Fatalf("symlink git: %v", err)
	}
	return dir
}

func TestReconcileCmd_JSONShape_ClosedMerged(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "shipped")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	withFakeHostTool(t, "gh", `case "$1" in
  auth) exit 0 ;;
  pr) echo MERGED ;;
esac`)

	newLegacyRun(t, repoRoot, "r1")
	out := execCmd(t, NewReconcileCmd(), []string{repoRoot, "--run-id", "r1"})

	var got reconcileResultJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got.AuthFailure != nil {
		t.Errorf("auth_failure = %v, want null", *got.AuthFailure)
	}
	want := []reconcileActionJSON{{TicketID: ticketID, Outcome: "closed_merged", Detail: "PR1"}}
	if len(got.Actions) != 1 || got.Actions[0] != want[0] {
		t.Errorf("actions = %+v, want %+v", got.Actions, want)
	}
	if !strings.Contains(out, `"ticket_id"`) || !strings.Contains(out, `"pr_ref"`) {
		t.Errorf("output %q missing expected snake_case keys", out)
	}
}

func TestReconcileCmd_JSONShape_EmptyActionsIsEmptyArrayNotNull(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	tkCreate(t, repoRoot, "not in progress") // initializes .tickets/; left "open", so excluded from reconcile

	newLegacyRun(t, repoRoot, "r1")
	out := execCmd(t, NewReconcileCmd(), []string{repoRoot, "--run-id", "r1"})

	var got reconcileResultJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got.Actions == nil {
		t.Errorf("actions = nil, want an explicit empty array")
	}
	if len(got.Actions) != 0 {
		t.Errorf("actions = %+v, want none", got.Actions)
	}
	if got.AuthFailure != nil {
		t.Errorf("auth_failure = %v, want null", *got.AuthFailure)
	}
	if !strings.Contains(out, `"actions"`) {
		t.Errorf("output %q missing expected key %q", out, "actions")
	}
}

func TestReconcileCmd_JSONShape_AuthFailure(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "needs a host tool")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")

	t.Setenv("PATH", pathWithoutHostTools(t))

	newLegacyRun(t, repoRoot, "r1")
	out := execCmd(t, NewReconcileCmd(), []string{repoRoot, "--run-id", "r1"})

	var got reconcileResultJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if got.AuthFailure == nil || *got.AuthFailure != "gh/glab" {
		t.Errorf("auth_failure = %v, want %q", got.AuthFailure, "gh/glab")
	}
	if len(got.Actions) != 0 {
		t.Errorf("actions = %+v, want none", got.Actions)
	}
}

func TestReconcileCmd_RunFolder_ReportsOnlyThatRunsTickets(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	dirA := newRun(t, repoRoot, "run-a")
	dirB := newRun(t, repoRoot, "run-b")
	ticketA := tkCreateInRun(t, repoRoot, "run-a", "run A work")
	tickettest.SetStatusIn(t, dirA, ticketA, "in_progress")
	tickettest.AddNoteIn(t, dirA, ticketA, "branch: feat/a")
	ticketB := tkCreateInRun(t, repoRoot, "run-b", "run B work")
	tickettest.SetStatusIn(t, dirB, ticketB, "in_progress")
	tickettest.AddNoteIn(t, dirB, ticketB, "branch: feat/b")
	loose := tkCreate(t, repoRoot, "loose work")
	tkStart(t, repoRoot, loose)
	tkAddNote(t, repoRoot, loose, "branch: feat/loose")

	out := execCmd(t, NewReconcileCmd(), []string{repoRoot, "--run-id", "run-a"})

	var got reconcileResultJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	want := reconcileActionJSON{TicketID: ticketA, Outcome: "retry_pr_creation", Detail: "feat/a"}
	if len(got.Actions) != 1 || got.Actions[0] != want {
		t.Errorf("actions = %+v, want only %+v", got.Actions, want)
	}
}

func TestReconcileCmd_WrongArgCount_Errors(t *testing.T) {
	cmd := NewReconcileCmd()
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("execute: expected an error for a missing repo-root arg, got nil")
	}
}
