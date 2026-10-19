package runfolder

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
)

func newRepo(t *testing.T) string {
	t.Helper()
	t.Setenv(ticket.TicketsDirEnv, "")
	return t.TempDir()
}

func saveLedger(t *testing.T, repoRoot, runID string, ticketsDir *string) {
	t.Helper()
	if err := (&ledger.RunState{RunID: runID, TicketsDir: ticketsDir}).Save(repoRoot); err != nil {
		t.Fatal(err)
	}
}

func makeRunFolder(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := ticket.WriteRunSummary(dir, ticket.RunSummary{RunID: filepath.Base(dir)}); err != nil {
		t.Fatal(err)
	}
}

func ptr(s string) *string { return &s }

func TestResolve_LedgerTicketsDir(t *testing.T) {
	repo := newRepo(t)
	saveLedger(t, repo, "r1", ptr(".tickets/r1"))

	got, err := Resolve(repo, "r1", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(repo, ".tickets", "r1"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolve_AE4_LegacyLedgerUsesTicketsArea(t *testing.T) {
	repo := newRepo(t)
	saveLedger(t, repo, "old", nil)

	got, err := Resolve(repo, "old", false)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(repo, ".tickets"); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestResolve_AE2_TicketsDirEnvWins(t *testing.T) {
	repo := newRepo(t)
	saveLedger(t, repo, "r1", ptr(".tickets/r1"))
	override := filepath.Join(t.TempDir(), "elsewhere")
	t.Setenv(ticket.TicketsDirEnv, override)

	for _, runID := range []string{"r1", "unknown"} {
		got, err := Resolve(repo, runID, false)
		if err != nil {
			t.Fatalf("%s: %v", runID, err)
		}
		if got != override {
			t.Fatalf("%s: got %q, want %q", runID, got, override)
		}
	}
}

func TestResolve_LedgerlessRunFolder(t *testing.T) {
	repo := newRepo(t)
	dir := filepath.Join(repo, ".tickets", "fresh")
	makeRunFolder(t, dir)

	got, err := Resolve(repo, "fresh", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}

	if _, err := Resolve(repo, "fresh", false); err == nil {
		t.Fatal("ledger required: want error, got nil")
	}
}

func TestResolve_UnknownRunRefusedWithoutCreatingFiles(t *testing.T) {
	repo := newRepo(t)

	for _, allow := range []bool{true, false} {
		if _, err := Resolve(repo, "typo", allow); err == nil {
			t.Fatalf("allowLedgerless=%v: want error, got nil", allow)
		}
	}
	entries, err := os.ReadDir(repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("repo should stay empty, has %v", entries)
	}
}

func TestResolve_RefusesAliasingRunIDs(t *testing.T) {
	repo := newRepo(t)
	saveLedger(t, repo, "..", ptr(".tickets"))

	for _, runID := range []string{"", ".", ".."} {
		if _, err := Resolve(repo, runID, true); err == nil {
			t.Fatalf("run id %q: want error, got nil", runID)
		}
	}
}

func TestResolve_TraversalRunIDStaysInsideArea(t *testing.T) {
	repo := newRepo(t)
	dir := filepath.Join(repo, ".tickets", ".._x")
	makeRunFolder(t, dir)

	got, err := Resolve(repo, "../x", true)
	if err != nil {
		t.Fatal(err)
	}
	if got != dir {
		t.Fatalf("got %q, want %q", got, dir)
	}
}

func TestAll_AreaThenSortedRunFolders(t *testing.T) {
	repo := newRepo(t)
	area := filepath.Join(repo, ".tickets")
	makeRunFolder(t, filepath.Join(area, "b"))
	makeRunFolder(t, filepath.Join(area, "a"))
	if err := os.MkdirAll(filepath.Join(area, "not-a-run"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := All(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{area, filepath.Join(area, "a"), filepath.Join(area, "b")}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAll_TicketsDirEnv(t *testing.T) {
	repo := newRepo(t)
	makeRunFolder(t, filepath.Join(repo, ".tickets", "ignored"))
	override := t.TempDir()
	makeRunFolder(t, filepath.Join(override, "r1"))
	t.Setenv(ticket.TicketsDirEnv, override)

	got, err := All(repo)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{override, filepath.Join(override, "r1")}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
