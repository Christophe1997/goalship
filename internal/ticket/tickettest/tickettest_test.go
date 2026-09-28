package tickettest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func TestCreate_LoadsStrictlyAndListsInQueryAsOpenTask(t *testing.T) {
	repoRoot := t.TempDir()

	id := Create(t, repoRoot, "Ship the thing")

	loaded, err := ticket.Load(filepath.Join(repoRoot, ".tickets", id+".md"))
	if err != nil {
		t.Fatalf("strict Load of fixture: %v", err)
	}
	if loaded.ID != id || loaded.Status != "open" || loaded.Type != "task" {
		t.Errorf("fixture = id %q status %q type %q, want id %q open task", loaded.ID, loaded.Status, loaded.Type, id)
	}
	if !strings.Contains(loaded.Body, "# Ship the thing") {
		t.Errorf("body = %q, want the title as a heading", loaded.Body)
	}

	lines, err := ticket.Query(filepath.Join(repoRoot, ".tickets"), ".")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("Query returned %d tickets, want 1", len(lines))
	}
	var got map[string]any
	if err := json.Unmarshal(lines[0], &got); err != nil {
		t.Fatalf("decode query line: %v", err)
	}
	if got["id"] != id || got["status"] != "open" {
		t.Errorf("query row = %v, want id %q status open", got, id)
	}
}

func TestCreate_BackToBackCallsGetDistinctIDs(t *testing.T) {
	repoRoot := t.TempDir()

	seen := make(map[string]bool)
	for i := range 25 {
		id := Create(t, repoRoot, "same title")
		if seen[id] {
			t.Fatalf("duplicate id %q on call %d", id, i)
		}
		seen[id] = true
	}
}

func TestSetStatus_AppliesEachValidStatus(t *testing.T) {
	repoRoot := t.TempDir()
	id := Create(t, repoRoot, "T")

	for _, status := range []string{"in_progress", "closed", "open"} {
		SetStatus(t, repoRoot, id, status)

		loaded, err := ticket.Load(filepath.Join(repoRoot, ".tickets", id+".md"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if loaded.Status != status {
			t.Errorf("Status = %q, want %q", loaded.Status, status)
		}
	}
}

func TestAddNote_AppendsNoteUnderNotesHeading(t *testing.T) {
	repoRoot := t.TempDir()
	id := Create(t, repoRoot, "T")

	AddNote(t, repoRoot, id, "branch: feat/x\npr: PR1")

	loaded, err := ticket.Load(filepath.Join(repoRoot, ".tickets", id+".md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !strings.Contains(loaded.Body, "## Notes\n") || !strings.Contains(loaded.Body, "branch: feat/x\npr: PR1\n") {
		t.Errorf("body missing the notes heading or note text:\n%s", loaded.Body)
	}
}

func TestDep_AddsDependencyOnceAndIgnoresRepeat(t *testing.T) {
	repoRoot := t.TempDir()
	base := Create(t, repoRoot, "base")
	dependent := Create(t, repoRoot, "dependent")

	Dep(t, repoRoot, dependent, base)
	Dep(t, repoRoot, dependent, base)

	loaded, err := ticket.Load(filepath.Join(repoRoot, ".tickets", dependent+".md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Deps) != 1 || loaded.Deps[0] != base {
		t.Errorf("Deps = %v, want exactly [%s]", loaded.Deps, base)
	}
}

func TestCreate_HonorsTicketsDirEnvAndCreatesMissingDirectory(t *testing.T) {
	repoRoot := t.TempDir()
	override := filepath.Join(t.TempDir(), "not", "yet", "there")
	t.Setenv(ticket.TicketsDirEnv, override)

	id := Create(t, repoRoot, "outside the repo")

	if _, err := os.Stat(filepath.Join(override, id+".md")); err != nil {
		t.Errorf("ticket not in the TICKETS_DIR override: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".tickets")); !os.IsNotExist(err) {
		t.Errorf("repoRoot/.tickets exists (err=%v), want the override used instead", err)
	}
}
