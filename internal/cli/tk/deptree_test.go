package tk

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// writeTestTicketWithTitle writes a minimal ticket whose body's first
// "# " line is title — dep tree/cycle read status/title/deps only.
func writeTestTicketWithTitle(t *testing.T, dir, id, title string, deps []string) {
	t.Helper()
	tk := &ticket.Ticket{
		ID: id, Status: "open", Created: "2026-01-01T00:00:00Z", Type: "task", Priority: 2,
		Deps: deps, Body: "\n# " + title + "\n",
	}
	if err := tk.Save(filepath.Join(dir, id+".md")); err != nil {
		t.Fatalf("Save %s: %v", id, err)
	}
}

func TestDepTree_SimpleChain(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "Root", []string{"goa-b"})
	writeTestTicketWithTitle(t, dir, "goa-b", "Middle", []string{"goa-c"})
	writeTestTicketWithTitle(t, dir, "goa-c", "Leaf", nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepTreeCmd(), []string{"goa-a"})
	if err != nil {
		t.Fatalf("dep tree: %v", err)
	}
	want := "goa-a [open] Root\n" +
		"└── goa-b [open] Middle\n" +
		"    └── goa-c [open] Leaf\n"
	if out != want {
		t.Errorf("output =\n%s\nwant\n%s", out, want)
	}
}

func TestDepTree_DedupsSharedDependency(t *testing.T) {
	dir := t.TempDir()
	// a depends on b and c; both b and c depend on d.
	writeTestTicketWithTitle(t, dir, "goa-a", "Root", []string{"goa-b", "goa-c"})
	writeTestTicketWithTitle(t, dir, "goa-b", "B", []string{"goa-d"})
	writeTestTicketWithTitle(t, dir, "goa-c", "C", []string{"goa-d"})
	writeTestTicketWithTitle(t, dir, "goa-d", "D", nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepTreeCmd(), []string{"goa-a"})
	if err != nil {
		t.Fatalf("dep tree: %v", err)
	}
	// d appears once total (non-full mode dedup), attached under
	// whichever parent gives it the deepest max_depth. b and c are
	// siblings at equal subtree depth, so tie-break is by ID.
	if strings.Count(out, "goa-d") != 1 {
		t.Errorf("output =\n%s\nwant exactly one \"goa-d\" line (dedup)", out)
	}
}

func TestDepTree_FullModeShowsEveryOccurrence(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "Root", []string{"goa-b", "goa-c"})
	writeTestTicketWithTitle(t, dir, "goa-b", "B", []string{"goa-d"})
	writeTestTicketWithTitle(t, dir, "goa-c", "C", []string{"goa-d"})
	writeTestTicketWithTitle(t, dir, "goa-d", "D", nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepTreeCmd(), []string{"--full", "goa-a"})
	if err != nil {
		t.Fatalf("dep tree --full: %v", err)
	}
	if strings.Count(out, "goa-d") != 2 {
		t.Errorf("output =\n%s\nwant two \"goa-d\" lines (--full disables dedup)", out)
	}
}

func TestDepTree_CycleDoesNotInfiniteLoop(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "A", []string{"goa-b"})
	writeTestTicketWithTitle(t, dir, "goa-b", "B", []string{"goa-a"})
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepTreeCmd(), []string{"goa-a"})
	if err != nil {
		t.Fatalf("dep tree: %v", err)
	}
	want := "goa-a [open] A\n└── goa-b [open] B\n"
	if out != want {
		t.Errorf("output =\n%s\nwant\n%s", out, want)
	}
}

func TestDepTree_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "A", nil)
	t.Setenv("TICKETS_DIR", dir)

	_, err := execCmd(t, newDepTreeCmd(), []string{"zzzz"})
	if err == nil || err.Error() != "Error: ticket zzzz not found" {
		t.Errorf("err = %v, want \"Error: ticket zzzz not found\" (no quotes: dep tree's own resolver, not ticket_path's)", err)
	}
}

func TestDepTree_AmbiguousPattern(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-abcd", "A", nil)
	writeTestTicketWithTitle(t, dir, "goa-abce", "B", nil)
	t.Setenv("TICKETS_DIR", dir)

	_, err := execCmd(t, newDepTreeCmd(), []string{"abc"})
	if err == nil || err.Error() != "Error: ambiguous ID abc" {
		t.Errorf("err = %v, want \"Error: ambiguous ID abc\"", err)
	}
}

func TestDepTree_MissingArgUsage(t *testing.T) {
	_, err := execCmd(t, newDepTreeCmd(), nil)
	if err == nil || err.Error() != "Usage: ticket dep tree [--full] <id>" {
		t.Errorf("err = %v, want usage text", err)
	}
}
