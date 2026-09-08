package loop

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// tkStart and tkAddNote mirror tkCreate (branch_test.go): thin wrappers
// around the real installed `tk` binary for building fixtures. This
// package can't reach internal/gitops's own identically-purposed test
// helpers — different package, unexported.
func tkStart(t *testing.T, repoRoot, ticketID string) {
	t.Helper()
	cmd := exec.Command("tk", "start", ticketID)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tk start: %v: %s", err, out)
	}
}

func tkAddNote(t *testing.T, repoRoot, ticketID, text string) {
	t.Helper()
	cmd := exec.Command("tk", "add-note", ticketID, text)
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("tk add-note: %v: %s", err, out)
	}
}

// pathWithoutHostTools returns a PATH with a real `tk` (symlinked into its
// own directory, since tk and gh share a bin directory on this machine)
// plus git's directory, but no gh/glab anywhere on it.
func pathWithoutHostTools(t *testing.T) string {
	t.Helper()
	tkPath, err := exec.LookPath("tk")
	if err != nil {
		t.Fatalf("LookPath tk: %v", err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath git: %v", err)
	}
	dir := t.TempDir()
	if err := os.Symlink(tkPath, filepath.Join(dir, "tk")); err != nil {
		t.Fatalf("symlink tk: %v", err)
	}
	return strings.Join([]string{dir, filepath.Dir(gitPath), "/bin"}, string(os.PathListSeparator))
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

	out := execCmd(t, NewReconcileCmd(), []string{repoRoot})

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

	out := execCmd(t, NewReconcileCmd(), []string{repoRoot})

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
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x")

	t.Setenv("PATH", pathWithoutHostTools(t))

	out := execCmd(t, NewReconcileCmd(), []string{repoRoot})

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

func TestReconcileCmd_WrongArgCount_Errors(t *testing.T) {
	cmd := NewReconcileCmd()
	cmd.SetOut(new(strings.Builder))
	cmd.SetErr(new(strings.Builder))
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Fatal("execute: expected an error for a missing repo-root arg, got nil")
	}
}
