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

	"github.com/Christophe1997/goalship/internal/runfolder"
	"github.com/Christophe1997/goalship/internal/ticket"
)

// Create writes a new open task titled title and returns its ID.
func Create(t testing.TB, repoRoot, title string) string {
	t.Helper()
	area := ticket.ResolveTicketsDir(repoRoot)
	return create(t, area, area, title)
}

// CreateInRun writes a new open task titled title into runID's folder
// inside repoRoot's tickets area and returns its ID. It does not write the
// folder's run.json.
func CreateInRun(t testing.TB, repoRoot, runID, title string) string {
	t.Helper()
	area := ticket.ResolveTicketsDir(repoRoot)
	dir, err := runfolder.RunDir(area, runID)
	if err != nil {
		t.Fatalf("tickettest: run dir: %v", err)
	}
	return create(t, area, dir, title)
}

func create(t testing.TB, areaDir, dir, title string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("tickettest: mkdir %s: %v", dir, err)
	}
	id, err := ticket.GenerateID(areaDir, dir)
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
	SetStatusIn(t, ticket.ResolveTicketsDir(repoRoot), id, status)
}

// SetStatusIn is SetStatus for a ticket in dir.
func SetStatusIn(t testing.TB, dir, id, status string) {
	t.Helper()
	update(t, dir, id, func(tk *ticket.Ticket) { tk.Status = status })
}

// AddNote appends a timestamped note to ticket id.
func AddNote(t testing.TB, repoRoot, id, text string) {
	t.Helper()
	AddNoteIn(t, ticket.ResolveTicketsDir(repoRoot), id, text)
}

// AddNoteIn is AddNote for a ticket in dir.
func AddNoteIn(t testing.TB, dir, id, text string) {
	t.Helper()
	update(t, dir, id, func(tk *ticket.Ticket) { tk.AddNote(text) })
}

// Dep makes ticket id depend on depID; repeating the call adds nothing.
func Dep(t testing.TB, repoRoot, id, depID string) {
	t.Helper()
	DepIn(t, ticket.ResolveTicketsDir(repoRoot), id, depID)
}

// DepIn is Dep for tickets in dir.
func DepIn(t testing.TB, dir, id, depID string) {
	t.Helper()
	depPath, err := ticket.Resolve(dir, depID)
	if err != nil {
		t.Fatalf("tickettest: resolve dependency %s: %v", depID, err)
	}
	resolvedDep := strings.TrimSuffix(filepath.Base(depPath), ".md")
	update(t, dir, id, func(tk *ticket.Ticket) {
		if !slices.Contains(tk.Deps, resolvedDep) {
			tk.Deps = append(tk.Deps, resolvedDep)
		}
	})
}

func update(t testing.TB, dir, id string, mutate func(*ticket.Ticket)) {
	t.Helper()
	if _, err := ticket.Update(dir, id, mutate); err != nil {
		t.Fatalf("tickettest: update %s: %v", id, err)
	}
}
