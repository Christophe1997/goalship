package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCommitAll_ReturnsNewHeadSHA(t *testing.T) {
	repoRoot := newTestRepo(t)
	writeFile(t, filepath.Join(repoRoot, "impl.go"), "package main\n")

	sha, err := CommitAll(repoRoot, "feat: add impl")
	if err != nil {
		t.Fatalf("CommitAll: %v", err)
	}
	headSHA, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if sha != headSHA {
		t.Errorf("CommitAll = %q, want head sha %q", sha, headSHA)
	}
}

func TestCommitAll_NeverStagesUntrackedTicketsDirChanges(t *testing.T) {
	repoRoot := newTestRepo(t)
	mustMkdirAll(t, filepath.Join(repoRoot, ".tickets"))
	writeFile(t, filepath.Join(repoRoot, ".tickets", "T-1.md"), "pending ticket edit\n")
	writeFile(t, filepath.Join(repoRoot, "impl.go"), "package main\n")

	if _, err := CommitAll(repoRoot, "feat: add impl"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	stat := runOK(t, repoRoot, "git", "show", "--stat", "HEAD")
	if !strings.Contains(stat, "impl.go") {
		t.Errorf("HEAD commit does not include impl.go:\n%s", stat)
	}
	if strings.Contains(stat, ".tickets") {
		t.Errorf("HEAD commit unexpectedly includes .tickets/:\n%s", stat)
	}

	status := runOK(t, repoRoot, "git", "status", "--porcelain", ".tickets")
	if strings.TrimSpace(status) == "" {
		t.Errorf(".tickets/ shows clean after CommitAll; T-1.md should remain untracked")
	}
}

func TestCommitAll_NeverStagesPendingEditsToAnAlreadyTrackedTicket(t *testing.T) {
	repoRoot := newTestRepo(t)
	mustMkdirAll(t, filepath.Join(repoRoot, ".tickets"))
	writeFile(t, filepath.Join(repoRoot, ".tickets", "T-1.md"), "original\n")
	runOK(t, repoRoot, "git", "add", ".tickets/T-1.md")
	runOK(t, repoRoot, "git", "commit", "-q", "-m", "chore: track ticket")

	writeFile(t, filepath.Join(repoRoot, ".tickets", "T-1.md"), "edited\n")
	writeFile(t, filepath.Join(repoRoot, "impl.go"), "package main\n")

	if _, err := CommitAll(repoRoot, "feat: add impl 2"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	status := runOK(t, repoRoot, "git", "status", "--porcelain", ".tickets")
	if strings.TrimSpace(status) == "" {
		t.Errorf(".tickets/T-1.md's pending edit was staged/committed; want it left dirty")
	}
}

// TestCommitAll_OnlyTicketsChanges_FailsRatherThanCommittingThem pins
// commit_all's check=True behavior: when the negative pathspec excludes
// every pending change, `git add` stages nothing and `git commit` exits
// non-zero ("nothing to commit") rather than silently succeeding — the
// same failure branching.py's own check=True subprocess calls surface.
// The load-bearing assertion is that HEAD never moves: a future
// `--allow-empty` or swallowed error would otherwise let a commit
// carrying only .tickets/ content land unnoticed.
func TestCommitAll_OnlyTicketsChanges_FailsRatherThanCommittingThem(t *testing.T) {
	repoRoot := newTestRepo(t)
	mustMkdirAll(t, filepath.Join(repoRoot, ".tickets"))
	writeFile(t, filepath.Join(repoRoot, ".tickets", "T-1.md"), "only change\n")

	before, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA before: %v", err)
	}

	if _, err := CommitAll(repoRoot, "feat: nothing real"); err == nil {
		t.Fatal("CommitAll: want an error when the only pending change is under .tickets/")
	}

	after, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA after: %v", err)
	}
	if after != before {
		t.Errorf("HEAD moved (%q -> %q); a .tickets/-only change must not produce a commit", before, after)
	}
}

func TestCommitAll_ExcludesLedgerDirViaGitInfoExclude(t *testing.T) {
	repoRoot := newTestRepo(t)
	writeFile(t, filepath.Join(repoRoot, "impl.go"), "package main\n")

	if _, err := CommitAll(repoRoot, "feat: x"); err != nil {
		t.Fatalf("CommitAll: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	if !strings.Contains(string(data), "/.goalship/") {
		t.Errorf("exclude file = %q, want it to contain /.goalship/", data)
	}
}

func TestPushBranch_PushesAndSetsUpstreamTracking(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/pushme", "main")
	writeFile(t, filepath.Join(repoRoot, "f.txt"), "x\n")
	localSHA := commitAll(t, repoRoot, "feat: f")

	if err := PushBranch(repoRoot, "feat/pushme"); err != nil {
		t.Fatalf("PushBranch: %v", err)
	}

	runOK(t, repoRoot, "git", "fetch", "-q", "origin")
	remoteSHA, err := HeadSHA(repoRoot, "origin/feat/pushme")
	if err != nil {
		t.Fatalf("HeadSHA origin/feat/pushme: %v", err)
	}
	if remoteSHA != localSHA {
		t.Errorf("origin/feat/pushme = %q, want %q", remoteSHA, localSHA)
	}

	upstream := trimmed(runOK(t, repoRoot, "git", "rev-parse", "--abbrev-ref", "feat/pushme@{upstream}"))
	if upstream != "origin/feat/pushme" {
		t.Errorf("upstream = %q, want origin/feat/pushme", upstream)
	}
}
