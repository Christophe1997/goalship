// Package tickettest builds ticket fixtures in-process so tests never need
// the bash tk binary to set up state. Every helper finds the tickets
// directory the way production code does (ticket.ResolveTicketsDir), so a
// test that sets TICKETS_DIR keeps exercising the override.
package tickettest

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// Create writes a new open task titled title and returns its ID.
func Create(t testing.TB, repoRoot, title string) string {
	t.Helper()
	dir := ticket.ResolveTicketsDir(repoRoot)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("tickettest: mkdir %s: %v", dir, err)
	}
	id, err := ticket.GenerateID(dir)
	if err != nil {
		t.Fatalf("tickettest: generate id: %v", err)
	}
	tk := &ticket.Ticket{
		ID:       id,
		Status:   "open",
		Created:  time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		Type:     "task",
		Priority: 2,
		Body:     "# " + title + "\n\n",
	}
	if err := tk.Save(filepath.Join(dir, id+".md")); err != nil {
		t.Fatalf("tickettest: save %s: %v", id, err)
	}
	return id
}

// SetStatus sets ticket id to status ("open", "in_progress", or "closed").
func SetStatus(t testing.TB, repoRoot, id, status string) {
	t.Helper()
	update(t, repoRoot, id, func(tk *ticket.Ticket) { tk.Status = status })
}

// AddNote appends a timestamped note to ticket id.
func AddNote(t testing.TB, repoRoot, id, text string) {
	t.Helper()
	update(t, repoRoot, id, func(tk *ticket.Ticket) { tk.AddNote(text) })
}

// Dep makes ticket id depend on depID; repeating the call adds nothing.
func Dep(t testing.TB, repoRoot, id, depID string) {
	t.Helper()
	depPath, err := ticket.Resolve(ticket.ResolveTicketsDir(repoRoot), depID)
	if err != nil {
		t.Fatalf("tickettest: resolve dependency %s: %v", depID, err)
	}
	resolvedDep := strings.TrimSuffix(filepath.Base(depPath), ".md")
	update(t, repoRoot, id, func(tk *ticket.Ticket) {
		if !slices.Contains(tk.Deps, resolvedDep) {
			tk.Deps = append(tk.Deps, resolvedDep)
		}
	})
}

func update(t testing.TB, repoRoot, id string, mutate func(*ticket.Ticket)) {
	t.Helper()
	if _, err := ticket.Update(ticket.ResolveTicketsDir(repoRoot), id, mutate); err != nil {
		t.Fatalf("tickettest: update %s: %v", id, err)
	}
}
