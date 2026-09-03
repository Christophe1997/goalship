package tk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

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
