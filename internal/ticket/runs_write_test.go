package ticket

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteRunSummary_UnchangedSummaryLeavesFileUntouched(t *testing.T) {
	dir := t.TempDir()
	s := RunSummary{RunID: "r1", Goal: "g", StartedAt: "2026-10-19T10:00:00Z"}
	if err := WriteRunSummary(dir, s); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, RunSummaryFile)
	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}

	s.StartedAt = "2030-01-01T00:00:00Z"
	if err := WriteRunSummary(dir, s); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("run.json mtime = %v, want untouched %v", info.ModTime(), old)
	}
}
