package loop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDirtyCmd_CleanRepo_PrintsEmptyArray(t *testing.T) {
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewDirtyCmd(), []string{repoRoot})

	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if len(got) != 0 {
		t.Errorf("dirty paths = %v, want empty", got)
	}
}

func TestDirtyCmd_ReportsUntrackedAndModifiedFiles(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	if err := os.WriteFile(filepath.Join(repoRoot, "untracked.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write untracked.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "README.md"), []byte("modified\n"), 0o644); err != nil {
		t.Fatalf("modify README.md: %v", err)
	}

	out := execCmd(t, NewDirtyCmd(), []string{repoRoot})

	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	want := map[string]bool{"untracked.txt": true, "README.md": true}
	if len(got) != len(want) {
		t.Fatalf("dirty paths = %v, want %v", got, want)
	}
	for _, p := range got {
		if !want[p] {
			t.Errorf("unexpected dirty path %q", p)
		}
	}
}

func TestDirtyCmd_ExcludesGoalshipAndTicketsDirs(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	if err := os.MkdirAll(filepath.Join(repoRoot, ".goalship"), 0o755); err != nil {
		t.Fatalf("mkdir .goalship: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".goalship", "run1.json"), []byte("{}"), 0o644); err != nil {
		t.Fatalf("write .goalship/run1.json: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repoRoot, ".tickets"), 0o755); err != nil {
		t.Fatalf("mkdir .tickets: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, ".tickets", "t1.md"), []byte("---\n---\n"), 0o644); err != nil {
		t.Fatalf("write .tickets/t1.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "real-change.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatalf("write real-change.txt: %v", err)
	}

	out := execCmd(t, NewDirtyCmd(), []string{repoRoot})

	var got []string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if len(got) != 1 || got[0] != "real-change.txt" {
		t.Errorf("dirty paths = %v, want only [real-change.txt]", got)
	}
}
