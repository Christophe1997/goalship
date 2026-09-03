package ticket

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestScanAwkTicket_ParsesScalarsArraysAndTitle(t *testing.T) {
	data := []byte("---\n" +
		"id: goa-x\n" +
		"status: open\n" +
		"deps: [goa-a, goa-b]\n" +
		"links: []\n" +
		"created: 2026-01-01T00:00:00Z\n" +
		"type: feature\n" +
		"priority: 2\n" +
		"assignee: chris\n" +
		"---\n" +
		"# A Title\n\nBody text.\n")

	got := ScanAwkTicket(data)

	if got.ID != "goa-x" {
		t.Errorf("ID = %q, want goa-x", got.ID)
	}
	if got.Status != "open" {
		t.Errorf("Status = %q, want open", got.Status)
	}
	if want := []string{"goa-a", "goa-b"}; !reflect.DeepEqual(got.Deps, want) {
		t.Errorf("Deps = %v, want %v", got.Deps, want)
	}
	if got.Title != "A Title" {
		t.Errorf("Title = %q, want %q", got.Title, "A Title")
	}

	wantFields := []AwkField{
		{Key: "id", Value: "goa-x"},
		{Key: "status", Value: "open"},
		{Key: "deps", Value: []any{"goa-a", "goa-b"}},
		{Key: "links", Value: []any{}},
		{Key: "created", Value: "2026-01-01T00:00:00Z"},
		{Key: "type", Value: "feature"},
		{Key: "priority", Value: "2"}, // string, not int: matches bash's unconditional quoting
		{Key: "assignee", Value: "chris"},
	}
	if !reflect.DeepEqual(got.Fields, wantFields) {
		t.Errorf("Fields = %#v, want %#v", got.Fields, wantFields)
	}
}

func TestScanAwkTicket_TogglesOnEveryDashLine(t *testing.T) {
	// bash's awk toggles in_front on *every* "---" line, not just the
	// first two: a body-level horizontal rule re-enters "frontmatter"
	// mode. cmd_query would then treat a "key: value"-shaped body line
	// between two such rules as a field.
	data := []byte("---\nid: goa-x\n---\nBody.\n---\nnote: leaked\n---\nMore body.\n")

	got := ScanAwkTicket(data)

	if got.ID != "goa-x" {
		t.Fatalf("ID = %q, want goa-x", got.ID)
	}
	found := false
	for _, f := range got.Fields {
		if f.Key == "note" {
			found = true
			if f.Value != "leaked" {
				t.Errorf("note field value = %v, want leaked", f.Value)
			}
		}
	}
	if !found {
		t.Errorf("Fields = %#v, want a \"note\" field captured from the re-toggled region", got.Fields)
	}
}

func TestScanAwkTicket_SkipsFilesWithNoFrontmatter(t *testing.T) {
	got := ScanAwkTicket([]byte("just some text\nno dashes here\n"))
	if len(got.Fields) != 0 {
		t.Errorf("Fields = %#v, want none", got.Fields)
	}
	if got.ID != "" {
		t.Errorf("ID = %q, want empty", got.ID)
	}
}

func TestQuery_IdentityPreservesFileFieldOrder(t *testing.T) {
	dir := t.TempDir()
	data := "---\nid: goa-x\nstatus: open\ndeps: []\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: feature\npriority: 3\nassignee: chris\nexternal-ref: U1\n---\n# T\n"
	if err := os.WriteFile(filepath.Join(dir, "goa-x.md"), []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := Query(dir, ".")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1", len(lines))
	}
	want := `{"id":"goa-x","status":"open","deps":[],"links":[],"created":"2026-01-01T00:00:00Z","type":"feature","priority":"3","assignee":"chris","external-ref":"U1"}`
	if got := string(lines[0]); got != want {
		t.Errorf("Query(\".\") =\n%s\nwant\n%s", got, want)
	}
}

func TestQuery_SelectFiltersByField(t *testing.T) {
	dir := t.TempDir()
	writeMini := func(name, id, status string) {
		data := "---\nid: " + id + "\nstatus: " + status + "\ndeps: []\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 2\n---\n# T\n"
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeMini("a.md", "id-a", "open")
	writeMini("b.md", "id-b", "in_progress")

	lines, err := Query(dir, `select(.status=="in_progress")`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(lines) != 1 {
		t.Fatalf("len(lines) = %d, want 1: %v", len(lines), lines)
	}
	if !strings.Contains(string(lines[0]), `"id":"id-b"`) {
		t.Errorf("lines[0] = %s, want id-b", lines[0])
	}
}

// TestQuery_ParityWithBashTk compares this package's Query against the
// real installed bash tk 0.3.2 binary on this repo's own .tickets/ (read
// only — query never mutates), for the three filter shapes goalship's
// own loop_runner.py sends. Skips if the binary isn't on PATH.
func TestQuery_ParityWithBashTk(t *testing.T) {
	tkPath, err := exec.LookPath("tk")
	if err != nil {
		t.Skip("bash tk not installed; skipping parity test")
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ticketsDir := filepath.Join(repoRoot, ".tickets")
	if _, err := os.Stat(ticketsDir); err != nil {
		t.Skipf("no .tickets dir at %s; skipping parity test", ticketsDir)
	}

	for _, filter := range []string{".", `select(.status=="in_progress")`, `select(.id=="goa-g7ei")`} {
		t.Run(filter, func(t *testing.T) {
			cmd := exec.Command(tkPath, "query", filter)
			cmd.Dir = repoRoot
			bashOut, err := cmd.Output()
			if err != nil {
				t.Fatalf("bash tk query %q: %v", filter, err)
			}

			lines, err := Query(ticketsDir, filter)
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			var goOut strings.Builder
			for _, l := range lines {
				goOut.Write(l)
				goOut.WriteByte('\n')
			}

			if goOut.String() != string(bashOut) {
				t.Errorf("Query(%q) mismatch\n--- bash tk ---\n%s\n--- go ---\n%s", filter, bashOut, goOut.String())
			}
		})
	}
}
