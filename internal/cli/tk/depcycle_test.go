package tk

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func writeTestTicketWithStatus(t *testing.T, dir, id, status, title string, deps []string) {
	t.Helper()
	tk := &ticket.Ticket{
		ID: id, Status: status, Created: "2026-01-01T00:00:00Z", Type: "task", Priority: 2,
		Deps: deps, Body: "\n# " + title + "\n",
	}
	if err := tk.Save(filepath.Join(dir, id+".md")); err != nil {
		t.Fatalf("Save %s: %v", id, err)
	}
}

func TestDepCycle_NoCycles(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "A", []string{"goa-b"})
	writeTestTicketWithTitle(t, dir, "goa-b", "B", nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepCycleCmd(), nil)
	if err != nil {
		t.Fatalf("dep cycle: %v", err)
	}
	if want := "No dependency cycles found\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestDepCycle_DetectsTwoNodeCycle(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "A", []string{"goa-b"})
	writeTestTicketWithTitle(t, dir, "goa-b", "B", []string{"goa-a"})
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepCycleCmd(), nil)
	if err != nil {
		t.Fatalf("dep cycle: %v", err)
	}
	if !strings.HasPrefix(out, "Cycle 1: ") {
		t.Errorf("output = %q, want a \"Cycle 1: \" header", out)
	}
	if !strings.Contains(out, "goa-a") || !strings.Contains(out, "[open] A") ||
		!strings.Contains(out, "goa-b") || !strings.Contains(out, "[open] B") {
		t.Errorf("output = %q, want member lines for both goa-a and goa-b", out)
	}
	if strings.Count(out, "Cycle ") != 1 {
		t.Errorf("output = %q, want exactly one cycle reported (a->b->a and b->a->b are the same cycle)", out)
	}
}

func TestDepCycle_ClosedTicketsExcluded(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "A", []string{"goa-b"})
	writeTestTicketWithStatus(t, dir, "goa-b", "closed", "B", []string{"goa-a"})
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepCycleCmd(), nil)
	if err != nil {
		t.Fatalf("dep cycle: %v", err)
	}
	// goa-b is closed, so its edge back to goa-a is excluded from the
	// graph entirely — no cycle to find.
	if want := "No dependency cycles found\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestDepCycle_ThreeNodeCycle(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithTitle(t, dir, "goa-a", "A", []string{"goa-b"})
	writeTestTicketWithTitle(t, dir, "goa-b", "B", []string{"goa-c"})
	writeTestTicketWithTitle(t, dir, "goa-c", "C", []string{"goa-a"})
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, newDepCycleCmd(), nil)
	if err != nil {
		t.Fatalf("dep cycle: %v", err)
	}
	for _, id := range []string{"goa-a", "goa-b", "goa-c"} {
		if !strings.Contains(out, id) {
			t.Errorf("output = %q, want it to mention %s", out, id)
		}
	}
	if strings.Count(out, "Cycle ") != 1 {
		t.Errorf("output = %q, want exactly one cycle", out)
	}
}
