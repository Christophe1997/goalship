package loop

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/gitops"
)

// withFakeHostTool mirrors internal/gitops/prstate_test.go's helper of the
// same name: a different package's test file, so it can't import that
// unexported test helper directly — the underlying gh/glab argv contract
// itself is covered exhaustively in internal/gitops's own pr_test.go; this
// package's tests only need to prove the CLI wiring (argv parsing, stdout
// shape) is correct.
func withFakeHostTool(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestCommitCmd_CommitsAndPrintsHeadSHA(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "impl.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write impl.go: %v", err)
	}

	out := execCmd(t, NewCommitCmd(), []string{repoRoot, "feat: add impl"})

	wantSHA, err := gitops.HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if strings.TrimSpace(out) != wantSHA {
		t.Errorf("output = %q, want head sha %q", out, wantSHA)
	}
}

func TestCommitCmd_NeverStagesTicketsDirChanges(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	if err := os.MkdirAll(filepath.Join(repoRoot, ".tickets"), 0o755); err != nil {
		t.Fatalf("mkdir .tickets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".tickets", "T-1.md"), []byte("pending\n"), 0o644); err != nil {
		t.Fatalf("write ticket: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "impl.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write impl.go: %v", err)
	}

	execCmd(t, NewCommitCmd(), []string{repoRoot, "feat: add impl"})

	status := runLoopGit(t, repoRoot, "status", "--porcelain", ".tickets")
	if strings.TrimSpace(status) == "" {
		t.Errorf(".tickets/ was staged by loop commit; want it left untouched")
	}
}

func TestHeadSHACmd_PrintsRefSHA(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	want, err := gitops.HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	out := execCmd(t, NewHeadSHACmd(), []string{repoRoot, "main"})
	if strings.TrimSpace(out) != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestPushCmd_PushesBranchAndSetsUpstream(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	bareDir := filepath.Join(t.TempDir(), "origin.git")
	runLoopGit(t, repoRoot, "init", "-q", "--bare", bareDir)
	runLoopGit(t, repoRoot, "remote", "add", "origin", bareDir)

	out := execCmd(t, NewPushCmd(), []string{repoRoot, "main"})
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}

	runLoopGit(t, repoRoot, "fetch", "-q", "origin")
	local := strings.TrimSpace(runLoopGit(t, repoRoot, "rev-parse", "main"))
	remote := strings.TrimSpace(runLoopGit(t, repoRoot, "rev-parse", "origin/main"))
	if local != remote {
		t.Errorf("origin/main = %q, want %q", remote, local)
	}

	upstream := strings.TrimSpace(runLoopGit(t, repoRoot, "rev-parse", "--abbrev-ref", "main@{upstream}"))
	if upstream != "origin/main" {
		t.Errorf("upstream = %q, want origin/main", upstream)
	}
}

func TestFindPRCmd_PrintsURLWhenFound(t *testing.T) {
	withFakeHostTool(t, "gh", `echo '[{"url": "https://github.com/o/r/pull/1"}]'`)
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewFindPRCmd(), []string{repoRoot, "gh", "feat/x"})
	if strings.TrimSpace(out) != "https://github.com/o/r/pull/1" {
		t.Errorf("output = %q, want the PR URL", out)
	}
}

func TestFindPRCmd_PrintsNothingWhenNoneFound(t *testing.T) {
	withFakeHostTool(t, "gh", `echo '[]'`)
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewFindPRCmd(), []string{repoRoot, "gh", "feat/x"})
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestCreatePRCmd_PrintsCreatedURL(t *testing.T) {
	withFakeHostTool(t, "gh", `echo "https://github.com/o/r/pull/2"`)
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewCreatePRCmd(), []string{repoRoot, "gh", "feat/x", "main", "Title", "Body"})
	if strings.TrimSpace(out) != "https://github.com/o/r/pull/2" {
		t.Errorf("output = %q, want the created PR URL", out)
	}
}

func TestCreatePRCmd_HostFailure_ReturnsError(t *testing.T) {
	withFakeHostTool(t, "gh", `exit 1`)
	repoRoot := newLoopTestRepo(t)

	cmd := NewCreatePRCmd()
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{repoRoot, "gh", "feat/x", "main", "Title", "Body"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute: want an error when gh pr create fails, got nil")
	}
}

func TestRetargetPRCmd_PrintsNothingOnSuccess(t *testing.T) {
	withFakeHostTool(t, "gh", `exit 0`)
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewRetargetPRCmd(), []string{repoRoot, "gh", "42", "main"})
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}
}

func TestShipCmd_WritesClosingNoteAndClosesTicket(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "Ticket to ship")

	out := execCmd(t, NewShipCmd(), []string{repoRoot, ticketID, "feat/x", "https://github.com/o/r/pull/3", "deadbeef"})
	if out != "" {
		t.Errorf("output = %q, want empty", out)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".tickets", ticketID+".md"))
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "status: closed") {
		t.Errorf("ticket status not closed:\n%s", content)
	}
	for _, want := range []string{"branch: feat/x", "pr: https://github.com/o/r/pull/3", "sha: deadbeef"} {
		if !strings.Contains(content, want) {
			t.Errorf("closing note missing %q:\n%s", want, content)
		}
	}
}

func TestShipCmd_UnknownTicketID_Errors(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	if err := os.MkdirAll(filepath.Join(repoRoot, ".tickets"), 0o755); err != nil {
		t.Fatalf("mkdir .tickets: %v", err)
	}

	cmd := NewShipCmd()
	cmd.SetOut(&strings.Builder{})
	cmd.SetErr(&strings.Builder{})
	cmd.SetArgs([]string{repoRoot, "no-such-ticket", "feat/x", "https://example.com/pull/1", "deadbeef"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute: want an error for an unresolvable ticket id, got nil")
	}
}
