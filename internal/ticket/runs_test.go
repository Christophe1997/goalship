package ticket

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRunSummary_RoundTrips(t *testing.T) {
	dir := t.TempDir()
	outcome := "exhausted"
	want := RunSummary{RunID: "r1", Goal: "ship <things> & more", StartedAt: "2026-10-19T10:00:00Z", Outcome: &outcome}

	if err := WriteRunSummary(dir, want); err != nil {
		t.Fatalf("WriteRunSummary: %v", err)
	}
	got, err := ReadRunSummary(dir)
	if err != nil {
		t.Fatalf("ReadRunSummary: %v", err)
	}
	if !reflect.DeepEqual(*got, want) {
		t.Errorf("round-trip = %+v, want %+v", *got, want)
	}
}

func TestRunSummary_NullOutcomeOnDisk(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRunSummary(dir, RunSummary{RunID: "r1", Goal: "g", StartedAt: "2026-10-19T10:00:00Z"}); err != nil {
		t.Fatalf("WriteRunSummary: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(dir, RunSummaryFile))
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "run_id": "r1",
  "goal": "g",
  "started_at": "2026-10-19T10:00:00Z",
  "outcome": null
}
`
	if string(data) != want {
		t.Errorf("run.json = %s, want %s", data, want)
	}
}

func TestWriteRunSummary_RewriteKeepsStartedAt(t *testing.T) {
	dir := t.TempDir()
	if err := WriteRunSummary(dir, RunSummary{RunID: "r1", Goal: "g", StartedAt: "2026-10-19T10:00:00Z"}); err != nil {
		t.Fatalf("first write: %v", err)
	}
	outcome := "capped"
	if err := WriteRunSummary(dir, RunSummary{RunID: "r1", Goal: "g2", StartedAt: "2030-01-01T00:00:00Z", Outcome: &outcome}); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	got, err := ReadRunSummary(dir)
	if err != nil {
		t.Fatalf("ReadRunSummary: %v", err)
	}
	if got.StartedAt != "2026-10-19T10:00:00Z" || got.Goal != "g2" || got.Outcome == nil || *got.Outcome != "capped" {
		t.Errorf("after rewrite = %+v, want original started_at, goal g2, outcome capped", *got)
	}
}

func TestReadRunSummary_MissingIsNotExist(t *testing.T) {
	if _, err := ReadRunSummary(t.TempDir()); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("ReadRunSummary on empty dir: err = %v, want os.ErrNotExist", err)
	}
}

func TestListRunDirs_OnlySubdirsWithSummarySorted(t *testing.T) {
	area := t.TempDir()
	for _, run := range []string{"zeta", "alpha"} {
		dir := filepath.Join(area, run)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := WriteRunSummary(dir, RunSummary{RunID: run}); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(area, "plain"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTicketFile(t, area, "goa-loose.md")

	got, err := ListRunDirs(area)
	if err != nil {
		t.Fatalf("ListRunDirs: %v", err)
	}
	want := []string{filepath.Join(area, "alpha"), filepath.Join(area, "zeta")}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListRunDirs = %v, want %v", got, want)
	}
}

func TestListRunDirs_MissingAreaIsEmpty(t *testing.T) {
	got, err := ListRunDirs(filepath.Join(t.TempDir(), "absent"))
	if err != nil || len(got) != 0 {
		t.Errorf("ListRunDirs(absent) = (%v, %v), want (empty, nil)", got, err)
	}
}

func TestRunSummary_CorruptFileErrorNamesRecovery(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, RunSummaryFile), []byte("<<<<<<< HEAD\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, readErr := ReadRunSummary(dir)
	writeErr := WriteRunSummary(dir, RunSummary{RunID: "r1"})
	for name, err := range map[string]error{"ReadRunSummary": readErr, "WriteRunSummary": writeErr} {
		if err == nil || !strings.Contains(err.Error(), "fix or delete it") {
			t.Errorf("%s error = %v, want one naming the recovery", name, err)
		}
	}
}
