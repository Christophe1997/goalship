package loop

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/gitops"
)

// newLoopTestRepo initializes a plain git repo (one commit on main) for
// exercising these commands' argv wiring — the underlying git/tk plumbing
// itself is covered exhaustively in internal/gitops's own test suite.
func newLoopTestRepo(t *testing.T) string {
	t.Helper()
	repoRoot := t.TempDir()
	runLoopGit(t, repoRoot, "init", "-q")
	runLoopGit(t, repoRoot, "config", "user.email", "test@example.com")
	runLoopGit(t, repoRoot, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	runLoopGit(t, repoRoot, "add", "README.md")
	runLoopGit(t, repoRoot, "commit", "-q", "-m", "init")
	runLoopGit(t, repoRoot, "branch", "-m", "main")
	return repoRoot
}

func runLoopGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func tkCreate(t *testing.T, repoRoot, title string) string {
	t.Helper()
	cmd := exec.Command("tk", "create", title, "-t", "task")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("tk create: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func execCmd(t *testing.T, cmd *cobra.Command, args []string) string {
	t.Helper()
	buf := &bytes.Buffer{}
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute %v: %v", args, err)
	}
	return buf.String()
}

func TestBranchNameCmd_PrintsPlainBranchName(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	out := execCmd(t, NewBranchNameCmd(), []string{repoRoot, "feat", "Add thing"})
	if strings.TrimSpace(out) != "feat/add-thing" {
		t.Errorf("output = %q, want %q", out, "feat/add-thing\n")
	}
}

func TestBranchNameCmd_TooFewArgs_Errors(t *testing.T) {
	cmd := NewBranchNameCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"/tmp", "feat"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("execute: expected an error for too few args, got nil")
	}
}

func TestResolveBaseCmd_NoDependencies_PrintsTrunk(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "Standalone ticket")

	out := execCmd(t, NewResolveBaseCmd(), []string{repoRoot, ticketID, "main"})
	if strings.TrimSpace(out) != "main" {
		t.Errorf("output = %q, want %q", out, "main\n")
	}
}

func TestResolveBaseCmd_AcceptsOptionalHostTool(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "Standalone ticket with host tool arg")

	out := execCmd(t, NewResolveBaseCmd(), []string{repoRoot, ticketID, "main", "gh"})
	if strings.TrimSpace(out) != "main" {
		t.Errorf("output = %q, want %q", out, "main\n")
	}
}

func TestCommitLandedCmd_PrintsYesOrNo(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	runLoopGit(t, repoRoot, "checkout", "-b", "feat/x")
	claimSHA, err := gitops.HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	out := execCmd(t, NewCommitLandedCmd(), []string{repoRoot, "feat/x", claimSHA})
	if strings.TrimSpace(out) != "no" {
		t.Errorf("output = %q, want %q", out, "no")
	}

	if err := os.WriteFile(filepath.Join(repoRoot, "f.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write f.txt: %v", err)
	}
	runLoopGit(t, repoRoot, "add", "-A")
	runLoopGit(t, repoRoot, "commit", "-q", "-m", "feat: add f")

	out = execCmd(t, NewCommitLandedCmd(), []string{repoRoot, "feat/x", claimSHA})
	if strings.TrimSpace(out) != "yes" {
		t.Errorf("output = %q, want %q", out, "yes")
	}
}

func TestRunBranchCmd_NoTicketIDs_PrintsNothing(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	out := execCmd(t, NewRunBranchCmd(), []string{repoRoot})
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestResetCmd_ResetsWorkingTreeToBaseRef(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	runLoopGit(t, repoRoot, "checkout", "-b", "feat/x")
	if err := os.WriteFile(filepath.Join(repoRoot, "dirty.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write dirty.txt: %v", err)
	}

	out := execCmd(t, NewResetCmd(), []string{repoRoot, "main"})
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "dirty.txt")); !os.IsNotExist(err) {
		t.Errorf("dirty.txt still exists after reset")
	}
}
