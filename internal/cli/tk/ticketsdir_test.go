package tk

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveTicketsDir_EnvVarWins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)

	got, err := resolveTicketsDir(false)
	if err != nil {
		t.Fatalf("resolveTicketsDir: %v", err)
	}
	if got != dir {
		t.Errorf("resolveTicketsDir = %q, want %q", got, dir)
	}
}

func TestResolveTicketsDir_WalksUpFromCwd(t *testing.T) {
	t.Setenv("TICKETS_DIR", "")
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

	got, err := resolveTicketsDir(false)
	if err != nil {
		t.Fatalf("resolveTicketsDir: %v", err)
	}
	wantAbs, _ := filepath.EvalSymlinks(ticketsDir)
	gotAbs, _ := filepath.EvalSymlinks(got)
	if gotAbs != wantAbs {
		t.Errorf("resolveTicketsDir = %q, want %q", got, ticketsDir)
	}
}

func TestResolveTicketsDir_NotFoundErrorsWithoutAllowCreate(t *testing.T) {
	t.Setenv("TICKETS_DIR", "")
	restore := chdir(t, t.TempDir())
	defer restore()

	if _, err := resolveTicketsDir(false); err == nil {
		t.Fatal("resolveTicketsDir: want error when no .tickets exists, got nil")
	}
}

func TestResolveTicketsDir_NotFoundFallsBackToDotTicketsWithAllowCreate(t *testing.T) {
	t.Setenv("TICKETS_DIR", "")
	restore := chdir(t, t.TempDir())
	defer restore()

	got, err := resolveTicketsDir(true)
	if err != nil {
		t.Fatalf("resolveTicketsDir: %v", err)
	}
	if got != ".tickets" {
		t.Errorf("resolveTicketsDir = %q, want %q", got, ".tickets")
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
