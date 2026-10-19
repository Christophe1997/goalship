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

// TestReset_TicketsDirEnvOverride_NeverDeletesOtherInRepoDir pins that once
// TICKETS_DIR points at a different in-repo directory (#25's own deferred
// P0 finding), Reset's clean exclusion follows it there instead of staying
// hardcoded to ".tickets" and sweeping the override dir as untracked cruft.
func TestReset_TicketsDirEnvOverride_NeverDeletesOtherInRepoDir(t *testing.T) {
	repoRoot := newTestRepo(t)
	t.Setenv("TICKETS_DIR", filepath.Join(repoRoot, "custom-tickets"))
	createBranch(t, repoRoot, "feat/will-fail-3", "origin/main")
	mustMkdirAll(t, filepath.Join(repoRoot, "custom-tickets"))
	writeFile(t, filepath.Join(repoRoot, "custom-tickets", "T-1.md"), "# T-1\n")
	writeFile(t, filepath.Join(repoRoot, "half-done.txt"), "broken\n")

	if err := Reset(repoRoot, "main"); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, "custom-tickets", "T-1.md")); err != nil {
		t.Errorf("custom-tickets/T-1.md was swept by Reset (TICKETS_DIR override): %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, "half-done.txt")); !os.IsNotExist(err) {
		t.Errorf("half-done.txt still exists after Reset")
	}
}

// TestReset_KeepsEveryRunFolderAndLooseTickets: the clean exclusion covers
// the whole tickets area, not just one run's folder, so aborting run A's
// ticket never deletes run B's untracked folder or legacy loose tickets.
func TestReset_KeepsEveryRunFolderAndLooseTickets(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/will-fail-4", "origin/main")
	keep := []string{
		filepath.Join(repoRoot, ".tickets", "run-a", "A-1.md"),
		filepath.Join(repoRoot, ".tickets", "run-b", "B-1.md"),
		filepath.Join(repoRoot, ".tickets", "L-1.md"),
	}
	for _, path := range keep {
		mustMkdirAll(t, filepath.Dir(path))
		writeFile(t, path, "# ticket\n")
	}
	writeFile(t, filepath.Join(repoRoot, "half-done.txt"), "broken\n")

	if err := Reset(repoRoot, "main"); err != nil {
		t.Fatalf("Reset: %v", err)
	}

	for _, path := range keep {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("%s was swept by Reset: %v", path, err)
		}
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
