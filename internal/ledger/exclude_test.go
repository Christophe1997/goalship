package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureExcluded_CreatesFileAndDirWhenAbsent(t *testing.T) {
	repoRoot := t.TempDir()

	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}
	if got, want := string(data), "/.goalship/\n"; got != want {
		t.Errorf("exclude file content = %q, want %q", got, want)
	}
}

func TestEnsureExcluded_IdempotentOnRepeatedCalls(t *testing.T) {
	repoRoot := t.TempDir()

	for i := 0; i < 3; i++ {
		if err := EnsureExcluded(repoRoot); err != nil {
			t.Fatalf("EnsureExcluded call %d: %v", i, err)
		}
	}

	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}
	if got, want := string(data), "/.goalship/\n"; got != want {
		t.Errorf("exclude file content after repeated calls = %q, want %q (entry must appear exactly once)", got, want)
	}
}

func TestEnsureExcluded_AppendsAfterExistingContentWithTrailingNewline(t *testing.T) {
	repoRoot := t.TempDir()
	excludeDir := filepath.Join(repoRoot, ".git", "info")
	if err := os.MkdirAll(excludeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	excludePath := filepath.Join(excludeDir, "exclude")
	if err := os.WriteFile(excludePath, []byte("*.log\n"), 0o644); err != nil {
		t.Fatalf("write existing exclude: %v", err)
	}

	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded: %v", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}
	if got, want := string(data), "*.log\n/.goalship/\n"; got != want {
		t.Errorf("exclude file content = %q, want %q", got, want)
	}
}

func TestEnsureExcluded_AppendsAfterExistingContentWithoutTrailingNewline(t *testing.T) {
	repoRoot := t.TempDir()
	excludeDir := filepath.Join(repoRoot, ".git", "info")
	if err := os.MkdirAll(excludeDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	excludePath := filepath.Join(excludeDir, "exclude")
	if err := os.WriteFile(excludePath, []byte("*.log"), 0o644); err != nil {
		t.Fatalf("write existing exclude: %v", err)
	}

	if err := EnsureExcluded(repoRoot); err != nil {
		t.Fatalf("EnsureExcluded: %v", err)
	}

	data, err := os.ReadFile(excludePath)
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}
	if got, want := string(data), "*.log\n/.goalship/\n"; got != want {
		t.Errorf("exclude file content = %q, want %q", got, want)
	}
}
