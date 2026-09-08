package ledger

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureExcluded_CreatesExcludeFileAndInfoDirWhenMissing(t *testing.T) {
	repoRoot := t.TempDir() // no .git at all yet

	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	if strings.TrimSpace(string(data)) != "/.goalship/" {
		t.Errorf("exclude contents = %q, want %q", data, "/.goalship/\n")
	}
}

func TestEnsureExcluded_IdempotentOnRepeatedCalls(t *testing.T) {
	repoRoot := t.TempDir()

	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded 1: %v", err)
	}
	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded 2: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	if n := strings.Count(string(data), "/.goalship/"); n != 1 {
		t.Errorf("entry appears %d times, want 1; contents:\n%s", n, data)
	}
}

func TestEnsureExcluded_AppendsAfterExistingContentWithoutTrailingNewline(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "info"), 0o755); err != nil {
		t.Fatalf("mkdir .git/info: %v", err)
	}
	excludePath := filepath.Join(repoRoot, ".git", "info", "exclude")
	if err := os.WriteFile(excludePath, []byte("*.log"), 0o644); err != nil {
		t.Fatalf("seed exclude: %v", err)
	}

	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded: %v", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	want := "*.log\n/.goalship/\n"
	if string(data) != want {
		t.Errorf("exclude contents = %q, want %q", data, want)
	}
}

func TestEnsureExcluded_PreservesExistingContentEndingInNewline(t *testing.T) {
	repoRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repoRoot, ".git", "info"), 0o755); err != nil {
		t.Fatalf("mkdir .git/info: %v", err)
	}
	excludePath := filepath.Join(repoRoot, ".git", "info", "exclude")
	if err := os.WriteFile(excludePath, []byte("*.log\n"), 0o644); err != nil {
		t.Fatalf("seed exclude: %v", err)
	}

	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded: %v", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude: %v", err)
	}
	want := "*.log\n/.goalship/\n"
	if string(data) != want {
		t.Errorf("exclude contents = %q, want %q", data, want)
	}
}
