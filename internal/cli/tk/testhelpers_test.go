package tk

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
)

// newRunRepo makes a temp repo holding an empty tickets area, with
// TICKETS_DIR unset and cwd inside the repo, so commands find the area by
// walking up as they would for a real user.
func newRunRepo(t *testing.T) string {
	t.Helper()
	t.Setenv(ticket.TicketsDirEnv, "")
	os.Unsetenv(ticket.TicketsDirEnv)
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(repo, ".tickets"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(chdir(t, repo))
	return repo
}

// startRun records runID the way `loop ledger` does for a new run: a
// ledger naming .tickets/<runID>, plus that folder with its run.json.
func startRun(t *testing.T, repo, runID string) string {
	t.Helper()
	saveRunLedger(t, repo, runID)
	dir := filepath.Join(repo, ".tickets", runID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ticket.WriteRunSummary(dir, ticket.RunSummary{RunID: runID}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func saveRunLedger(t *testing.T, repo, runID string) {
	t.Helper()
	ticketsDir := ".tickets/" + runID
	if err := (&ledger.RunState{RunID: runID, TicketsDir: &ticketsDir}).Save(repo); err != nil {
		t.Fatal(err)
	}
}

func assertNotExist(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s exists (stat err %v), want nothing created", path, err)
	}
}

func ticketPathForTest(t *testing.T, dir, id string) string {
	t.Helper()
	path, err := ticket.Resolve(dir, id)
	if err != nil {
		t.Fatalf("ticket.Resolve(%q): %v", id, err)
	}
	return path
}

func loadForTest(t *testing.T, path string) *ticket.Ticket {
	t.Helper()
	tk, err := ticket.Load(path)
	if err != nil {
		t.Fatalf("ticket.Load(%q): %v", path, err)
	}
	return tk
}

// ticketFieldForTest builds an Extra field with the leading-space
// convention Parse/Bytes rely on (see internal/ticket.Field's doc).
func ticketFieldForTest(key, value string) ticket.Field {
	return ticket.Field{Key: key, Value: " " + value}
}

// writeRawTicket writes content verbatim as filename in dir — for
// hand-constructed malformed fixtures (missing/non-bracket fields, no
// frontmatter at all) that runCreate can't produce, since it always
// writes well-formed frontmatter.
func writeRawTicket(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o644); err != nil {
		t.Fatalf("write ticket %s: %v", filename, err)
	}
}

// setDeps hand-writes a ticket's deps array directly via the ticket
// package (dep/undep commands are a later ticket's scope), used here only
// to set up fixtures for ready/blocked/ls tests. depIDs are written
// verbatim, including ids that don't resolve to any ticket file — the
// dangling-dependency fixtures ready/blocked need.
func setDeps(t *testing.T, dir, id string, depIDs ...string) {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	tk := loadForTest(t, path)
	tk.Deps = depIDs
	if err := tk.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
}
