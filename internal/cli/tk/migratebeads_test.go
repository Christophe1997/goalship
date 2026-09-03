package tk

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

const sampleBeadsJSONL = `{"id":"bd-1","status":"open","title":"Fix thing","description":"desc here","issue_type":"bug","priority":1,"assignee":"chris","created_at":"2026-01-01T00:00:00Z","dependencies":[{"type":"blocks","depends_on_id":"bd-2"},{"type":"related","depends_on_id":"bd-3"}]}
{"id":"bd-4","title":"Minimal issue"}
{"id":"bd-5","title":"With notes","description":"d2","design":"design text","acceptance_criteria":"ac text","notes":"note text","external_ref":"gh-99","dependencies":[{"type":"parent-child","depends_on_id":"bd-1"}]}
`

func TestMigrateBeads_ImportsAndSavesTickets(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".beads", "issues.jsonl"), []byte(sampleBeadsJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("TICKETS_DIR", filepath.Join(dir, ".tickets"))

	out, err := execCmd(t, NewMigrateBeadsCmd(), nil)
	if err != nil {
		t.Fatalf("migrate-beads: %v", err)
	}
	want := "Migrated: bd-1\nMigrated: bd-4\nMigrated: bd-5\nMigrated 3 tickets from beads\n"
	if out != want {
		t.Errorf("output = %q, want %q", out, want)
	}

	bd1, err := ticket.Load(filepath.Join(dir, ".tickets", "bd-1.md"))
	if err != nil {
		t.Fatalf("load bd-1: %v", err)
	}
	if bd1.Status != "open" || bd1.Type != "bug" || bd1.Priority != 1 {
		t.Errorf("bd-1 core fields = %+v, want status=open type=bug priority=1", bd1)
	}
	if len(bd1.Deps) != 1 || bd1.Deps[0] != "bd-2" {
		t.Errorf("bd-1.Deps = %v, want [bd-2]", bd1.Deps)
	}
	if len(bd1.Links) != 1 || bd1.Links[0] != "bd-3" {
		t.Errorf("bd-1.Links = %v, want [bd-3]", bd1.Links)
	}

	bd4, err := ticket.Load(filepath.Join(dir, ".tickets", "bd-4.md"))
	if err != nil {
		t.Fatalf("load bd-4: %v", err)
	}
	if bd4.Status != "open" || bd4.Type != "task" || bd4.Priority != 2 || bd4.Created != "" {
		t.Errorf("bd-4 defaults = %+v, want status=open type=task priority=2 created=\"\"", bd4)
	}

	bd5, err := ticket.Load(filepath.Join(dir, ".tickets", "bd-5.md"))
	if err != nil {
		t.Fatalf("load bd-5: %v", err)
	}
	var hasParent, hasExternalRef bool
	for _, f := range bd5.Extra {
		if f.Key == "parent" && f.Value == " bd-1" {
			hasParent = true
		}
		if f.Key == "external-ref" && f.Value == " gh-99" {
			hasExternalRef = true
		}
	}
	if !hasParent {
		t.Errorf("bd-5.Extra = %+v, want a parent: bd-1 field", bd5.Extra)
	}
	if !hasExternalRef {
		t.Errorf("bd-5.Extra = %+v, want an external-ref: gh-99 field", bd5.Extra)
	}
}

func TestMigrateBeads_MissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	_, err := execCmd(t, NewMigrateBeadsCmd(), nil)
	if err == nil || err.Error() != "Error: .beads/issues.jsonl not found" {
		t.Errorf("err = %v, want \"Error: .beads/issues.jsonl not found\"", err)
	}
}

// TestMigrateBeads_ParityWithBashPipeline compares this package's
// migrate-beads output against bash tk's actual mechanism — the same jq
// program run through real jq, split into files with the same awk
// script cmd_migrate_beads uses — rather than invoking the `tk` binary's
// migrate-beads subcommand directly (a mutating command this test suite
// deliberately never shells out to). Skips if jq/awk aren't on PATH.
func TestMigrateBeads_ParityWithBashPipeline(t *testing.T) {
	jqPath, err := exec.LookPath("jq")
	if err != nil {
		t.Skip("jq not installed; skipping parity test")
	}
	awkPath, err := exec.LookPath("awk")
	if err != nil {
		t.Skip("awk not installed; skipping parity test")
	}

	bashDir := t.TempDir()
	progPath := filepath.Join(bashDir, "prog.jq")
	if err := os.WriteFile(progPath, []byte(ticket.MigrateBeadsProgram), 0o644); err != nil {
		t.Fatal(err)
	}
	jsonlPath := filepath.Join(bashDir, "issues.jsonl")
	if err := os.WriteFile(jsonlPath, []byte(sampleBeadsJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	splitAwkPath := filepath.Join(bashDir, "split.awk")
	splitAwkSrc := `/^<<<FILE:.*>>>$/ {
    if (file) close(file)
    id = substr($0, 9, length($0) - 11)
    file = dir "/" id ".md"
    count++
    next
}
file { print > file }
`
	if err := os.WriteFile(splitAwkPath, []byte(splitAwkSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	bashTicketsDir := filepath.Join(bashDir, "tickets")
	if err := os.MkdirAll(bashTicketsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	jqCmd := exec.Command(jqPath, "-r", "-f", progPath, jsonlPath)
	jqOut, err := jqCmd.Output()
	if err != nil {
		t.Fatalf("jq: %v", err)
	}
	awkCmd := exec.Command(awkPath, "-v", "dir="+bashTicketsDir, "-f", splitAwkPath)
	awkCmd.Stdin = bytes.NewReader(jqOut)
	if err := awkCmd.Run(); err != nil {
		t.Fatalf("awk: %v", err)
	}

	goDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(goDir, ".beads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(goDir, ".beads", "issues.jsonl"), []byte(sampleBeadsJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(goDir)
	t.Setenv("TICKETS_DIR", filepath.Join(goDir, ".tickets"))
	if _, err := execCmd(t, NewMigrateBeadsCmd(), nil); err != nil {
		t.Fatalf("migrate-beads: %v", err)
	}

	for _, id := range []string{"bd-1", "bd-4", "bd-5"} {
		bashBytes, err := os.ReadFile(filepath.Join(bashTicketsDir, id+".md"))
		if err != nil {
			t.Fatalf("read bash output for %s: %v", id, err)
		}
		goBytes, err := os.ReadFile(filepath.Join(goDir, ".tickets", id+".md"))
		if err != nil {
			t.Fatalf("read go output for %s: %v", id, err)
		}
		if string(goBytes) != string(bashBytes) {
			t.Errorf("%s mismatch\n--- bash pipeline ---\n%q\n--- go ---\n%q", id, bashBytes, goBytes)
		}
	}
}
