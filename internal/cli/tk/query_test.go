package tk

import (
	"strings"
	"testing"
)

func TestQueryCmd_DefaultFilterIsIdentity(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewQueryCmd(), nil)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if want := `{"id":"goa-a","status":"open","deps":[],"links":[],"created":"2026-01-01T00:00:00Z","type":"task","priority":"2"}` + "\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestQueryCmd_ExplicitFilter(t *testing.T) {
	dir := t.TempDir()
	writeTestTicketWithStatus(t, dir, "goa-a", "open", "A", nil)
	writeTestTicketWithStatus(t, dir, "goa-b", "in_progress", "B", nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewQueryCmd(), []string{`select(.status=="in_progress")`})
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if want := `"id":"goa-b"`; !strings.Contains(out, want) {
		t.Errorf("output = %q, want it to contain %q", out, want)
	}
}
