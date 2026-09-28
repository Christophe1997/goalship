package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket/tickettest"
)

// newTestRepo initializes a git repo with a bare "origin" remote and one
// commit on main — the same fixture shape branching.py's own
// BranchingTestCase.setUp uses, so the collision/remote-ref tests below
// exercise the identical scenario the Python suite does.
func newTestRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	bareDir := filepath.Join(root, "origin.git")
	repoRoot := filepath.Join(root, "work")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("mkdir repoRoot: %v", err)
	}

	runOK(t, root, "git", "init", "-q", "--bare", bareDir)
	runOK(t, repoRoot, "git", "init", "-q")
	runOK(t, repoRoot, "git", "config", "user.email", "test@example.com")
	runOK(t, repoRoot, "git", "config", "user.name", "Test")
	writeFile(t, filepath.Join(repoRoot, "README.md"), "placeholder\n")
	runOK(t, repoRoot, "git", "add", "README.md")
	runOK(t, repoRoot, "git", "commit", "-q", "-m", "init")
	runOK(t, repoRoot, "git", "branch", "-m", "main")
	runOK(t, repoRoot, "git", "remote", "add", "origin", bareDir)
	runOK(t, repoRoot, "git", "push", "-q", "-u", "origin", "main")
	runOK(t, repoRoot, "git", "fetch", "-q", "origin")
	return repoRoot
}

func runOK(t *testing.T, dir string, argv ...string) string {
	t.Helper()
	out, err := run(dir, argv...)
	if err != nil {
		t.Fatalf("setup %v: %v", argv, err)
	}
	return out
}

func createBranch(t *testing.T, repoRoot, branch, base string) {
	t.Helper()
	runOK(t, repoRoot, "git", "checkout", "-b", branch, base)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func commitAll(t *testing.T, repoRoot, message string) string {
	t.Helper()
	runOK(t, repoRoot, "git", "add", "-A")
	runOK(t, repoRoot, "git", "commit", "-q", "-m", message)
	sha, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA after commit: %v", err)
	}
	return sha
}

func trimmed(s string) string {
	return strings.TrimSpace(s)
}

// tkCreate and its siblings build ticket fixtures in-process; the names
// mirror the `goalship tk` commands they stand in for.
func tkCreate(t *testing.T, repoRoot, title string) string {
	t.Helper()
	return tickettest.Create(t, repoRoot, title)
}

func tkAddNote(t *testing.T, repoRoot, ticketID, text string) {
	t.Helper()
	tickettest.AddNote(t, repoRoot, ticketID, text)
}

func tkDep(t *testing.T, repoRoot, ticketID, depID string) {
	t.Helper()
	tickettest.Dep(t, repoRoot, ticketID, depID)
}

func tkClose(t *testing.T, repoRoot, ticketID string) {
	t.Helper()
	tickettest.SetStatus(t, repoRoot, ticketID, "closed")
}

// tkStart sets ticketID to in_progress — reconcile only ever looks at
// in-progress tickets, so every reconcile fixture needs this after tkCreate
// (which leaves a ticket "open").
func tkStart(t *testing.T, repoRoot, ticketID string) {
	t.Helper()
	tickettest.SetStatus(t, repoRoot, ticketID, "in_progress")
}
