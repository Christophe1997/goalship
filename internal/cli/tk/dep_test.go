package tk

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func writeTestTicket(t *testing.T, dir, id string, deps, links []string) string {
	t.Helper()
	tk := &ticket.Ticket{
		ID: id, Status: "open", Created: "2026-01-01T00:00:00Z", Type: "task", Priority: 2,
		Deps: deps, Links: links, Body: "\n# " + id + "\n",
	}
	path := filepath.Join(dir, id+".md")
	if err := tk.Save(path); err != nil {
		t.Fatalf("Save %s: %v", id, err)
	}
	return path
}

func execCmd(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestDep_AddsDependency(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	writeTestTicket(t, dir, "goa-b", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	cmd := NewDepCmd()
	out, err := execCmd(t, cmd, []string{"goa-a", "goa-b"})
	if err != nil {
		t.Fatalf("dep: %v", err)
	}
	if want := "Added dependency: goa-a -> goa-b\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}

	loaded, err := ticket.Load(filepath.Join(dir, "goa-a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Deps) != 1 || loaded.Deps[0] != "goa-b" {
		t.Errorf("Deps = %v, want [goa-b]", loaded.Deps)
	}
}

func TestDep_AlreadyExists_ExitsZero(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", []string{"goa-b"}, nil)
	writeTestTicket(t, dir, "goa-b", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewDepCmd(), []string{"goa-a", "goa-b"})
	if err != nil {
		t.Fatalf("dep: %v", err)
	}
	if want := "Dependency already exists\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

// TestDep_NonexistentDependencyID covers the ticket's acceptance
// criterion directly: bash tk's cmd_dep resolves dep-id via ticket_path()
// too ("Verify dependency exists and resolve to full ID") — despite the
// ticket description's general "no referential-integrity check" framing,
// a dep-id that doesn't resolve to any real ticket DOES error, exactly
// like an unresolvable primary ID would.
func TestDep_NonexistentDependencyID(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	_, err := execCmd(t, NewDepCmd(), []string{"goa-a", "goa-zzzz"})
	if err == nil {
		t.Fatal("dep: want error for nonexistent dependency ID, got nil")
	}
	if want := "Error: ticket 'goa-zzzz' not found"; err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
}

func TestDep_TooFewArgs(t *testing.T) {
	_, err := execCmd(t, NewDepCmd(), []string{"goa-a"})
	if !errors.Is(err, depUsageErr) {
		t.Errorf("err = %v, want depUsageErr", err)
	}
}

func TestUndep_RemovesDependency(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", []string{"goa-b", "goa-c"}, nil)
	writeTestTicket(t, dir, "goa-b", nil, nil)
	writeTestTicket(t, dir, "goa-c", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewUndepCmd(), []string{"goa-a", "goa-b"})
	if err != nil {
		t.Fatalf("undep: %v", err)
	}
	if want := "Removed dependency: goa-a -/-> goa-b\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}

	loaded, err := ticket.Load(filepath.Join(dir, "goa-a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.Deps) != 1 || loaded.Deps[0] != "goa-c" {
		t.Errorf("Deps = %v, want [goa-c]", loaded.Deps)
	}
}

func TestUndep_NotInList(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	writeTestTicket(t, dir, "goa-b", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	_, err := execCmd(t, NewUndepCmd(), []string{"goa-a", "goa-b"})
	if err == nil || err.Error() != "Dependency not found" {
		t.Errorf("err = %v, want \"Dependency not found\"", err)
	}
}

// TestUndep_NonexistentDependencyID covers the acceptance criterion for
// undep the same way TestDep_NonexistentDependencyID does for dep.
func TestUndep_NonexistentDependencyID(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	_, err := execCmd(t, NewUndepCmd(), []string{"goa-a", "goa-zzzz"})
	if err == nil {
		t.Fatal("undep: want error for nonexistent dependency ID, got nil")
	}
	if want := "Error: ticket 'goa-zzzz' not found"; err.Error() != want {
		t.Errorf("err = %q, want %q", err.Error(), want)
	}
}

func TestDep_PartialIDResolvesToFullID(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-abcd", nil, nil)
	writeTestTicket(t, dir, "goa-efgh", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewDepCmd(), []string{"abcd", "efgh"})
	if err != nil {
		t.Fatalf("dep: %v", err)
	}
	if want := "Added dependency: goa-abcd -> goa-efgh\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestFindTicketsDir_MissingErrorsWithBashText(t *testing.T) {
	empty := t.TempDir()
	t.Chdir(empty)
	t.Setenv("TICKETS_DIR", "")
	os.Unsetenv("TICKETS_DIR")

	_, err := findTicketsDir()
	if err == nil {
		t.Fatal("want error, got nil")
	}
}
