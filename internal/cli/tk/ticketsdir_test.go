package tk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"

	"github.com/Christophe1997/goalship/internal/ticket"
	"github.com/Christophe1997/goalship/internal/ticket/tickettest"
)

func TestLocateTicketsDir_EnvVarWins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)

	got, ok := locateTicketsDir()
	if !ok {
		t.Fatal("locateTicketsDir: want ok=true")
	}
	if got != dir {
		t.Errorf("locateTicketsDir = %q, want %q", got, dir)
	}
}

func TestLocateTicketsDir_WalksUpFromCwd(t *testing.T) {
	t.Setenv("TICKETS_DIR", "")
	os.Unsetenv("TICKETS_DIR")
	repoDir := t.TempDir()
	ticketsDir := filepath.Join(repoDir, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	nested := filepath.Join(repoDir, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	restore := chdir(t, nested)
	defer restore()

	got, ok := locateTicketsDir()
	if !ok {
		t.Fatal("locateTicketsDir: want ok=true")
	}
	wantAbs, _ := filepath.EvalSymlinks(ticketsDir)
	gotAbs, _ := filepath.EvalSymlinks(got)
	if gotAbs != wantAbs {
		t.Errorf("locateTicketsDir = %q, want %q", got, ticketsDir)
	}
}

func TestFindTicketsDir_NotFoundErrors(t *testing.T) {
	t.Setenv("TICKETS_DIR", "")
	os.Unsetenv("TICKETS_DIR")
	restore := chdir(t, t.TempDir())
	defer restore()

	if _, err := findTicketsDir(); err == nil {
		t.Fatal("findTicketsDir: want error when no .tickets exists, got nil")
	}
}

func TestFindOrInitTicketsDir_NotFoundFallsBackToDotTickets(t *testing.T) {
	t.Setenv("TICKETS_DIR", "")
	os.Unsetenv("TICKETS_DIR")
	restore := chdir(t, t.TempDir())
	defer restore()

	got, err := findOrInitTicketsDir()
	if err != nil {
		t.Fatalf("findOrInitTicketsDir: %v", err)
	}
	if got != ".tickets" {
		t.Errorf("findOrInitTicketsDir = %q, want %q", got, ".tickets")
	}
}

func TestEveryTkCommandRegistersRunID(t *testing.T) {
	var walk func(cmd *cobra.Command)
	walk = func(cmd *cobra.Command) {
		for _, sub := range cmd.Commands() {
			if sub.Flags().Lookup("run-id") == nil {
				t.Errorf("%q does not register --run-id", sub.CommandPath())
			}
			walk(sub)
		}
	}
	walk(NewCmd())
}

func TestRunID_AE2_TicketsDirOverrideWins(t *testing.T) {
	repo := newRunRepo(t)
	startRun(t, repo, "A")
	tickettest.CreateInRun(t, repo, "A", "In run A")
	override := t.TempDir()
	t.Setenv(ticket.TicketsDirEnv, override)
	id := tickettest.Create(t, repo, "In override")

	for _, runID := range []string{"A", "unknown"} {
		out, err := execCmd(t, NewLsCmd(), []string{"--run-id", runID})
		if err != nil {
			t.Fatalf("ls --run-id %s: %v", runID, err)
		}
		if want := id + " [open] - In override\n"; out != want {
			t.Errorf("ls --run-id %s = %q, want %q", runID, out, want)
		}
	}
}

// chdir changes the process cwd for the duration of a test and restores it
// after. Package-private helper (not t.Chdir) so tests build under the
// module's go.mod floor (1.23) regardless of the local toolchain's version.
func chdir(t *testing.T, dir string) func() {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	return func() {
		if err := os.Chdir(old); err != nil {
			t.Fatal(err)
		}
	}
}
