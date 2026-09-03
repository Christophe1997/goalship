package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReset_ResetsToCleanBaseAndSwitchesBranch(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/will-fail", "origin/main")
	writeFile(t, filepath.Join(repoRoot, "half-done.txt"), "broken\n")

	if err := Reset(repoRoot, "main"); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	current := trimmed(runOK(t, repoRoot, "git", "branch", "--show-current"))
	if current != "main" {
		t.Errorf("current branch = %q, want %q", current, "main")
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "half-done.txt")); !os.IsNotExist(err) {
		t.Errorf("half-done.txt still exists after Reset")
	}
}

// TestReset_NeverDeletesTheUntrackedTicketsDir: .tickets/ is never tracked
// or git-ignored by this tool — a plain `git clean -fd` on abort would wipe
// out the entire ticket store the moment any gate ever fails.
func TestReset_NeverDeletesTheUntrackedTicketsDir(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/will-fail-2", "origin/main")
	mustMkdirAll(t, filepath.Join(repoRoot, ".tickets"))
	writeFile(t, filepath.Join(repoRoot, ".tickets", "T-1.md"), "# T-1\n")
	writeFile(t, filepath.Join(repoRoot, "half-done.txt"), "broken\n")

	if err := Reset(repoRoot, "main"); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".tickets", "T-1.md")); err != nil {
		t.Errorf(".tickets/T-1.md was swept by Reset: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "half-done.txt")); !os.IsNotExist(err) {
		t.Errorf("half-done.txt still exists after Reset")
	}
}

func TestReset_NeverDeletesTheBranchItself(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/kept-around", "origin/main")
	runOK(t, repoRoot, "git", "checkout", "main")

	if err := Reset(repoRoot, "main"); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	branches := runOK(t, repoRoot, "git", "branch", "--format=%(refname:short)")
	found := false
	for _, line := range strings.Split(branches, "\n") {
		if trimmed(line) == "feat/kept-around" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Reset deleted feat/kept-around; branches = %q", branches)
	}
}
