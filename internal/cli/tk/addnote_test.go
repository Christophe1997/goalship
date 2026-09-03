package tk

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRunAddNote_AppendsNotesHeadingWhenMissing(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	if _, err := runAddNote(dir, id, "hello world"); err != nil {
		t.Fatalf("runAddNote: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !regexp.MustCompile(`\n## Notes\n\n\*\*[^*]+\*\*\n\nhello world\n$`).Match(got) {
		t.Fatalf("unexpected note block, got: %q", got)
	}
}

func TestRunAddNote_SecondNoteDoesNotDuplicateHeading(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	if _, err := runAddNote(dir, id, "first"); err != nil {
		t.Fatalf("runAddNote 1: %v", err)
	}
	if _, err := runAddNote(dir, id, "second"); err != nil {
		t.Fatalf("runAddNote 2: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if n := strings.Count(string(got), "## Notes"); n != 1 {
		t.Fatalf("## Notes heading count = %d, want 1; content:\n%s", n, got)
	}
	if !strings.Contains(string(got), "first") || !strings.Contains(string(got), "second") {
		t.Fatalf("both notes not present:\n%s", got)
	}
}

func TestRunAddNote_ResolvesPartialID(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	got, err := runAddNote(dir, id[len(id)-4:], "note text")
	if err != nil {
		t.Fatalf("runAddNote: %v", err)
	}
	if got != id {
		t.Errorf("resolved id = %q, want %q", got, id)
	}
}

// Transcribed verbatim from reconciliation.py's _KV_LINE_RE/_NOTES_HEADING_RE/
// _NOTE_MARKER_RE/_NEXT_HEADING_RE, and its _notes_section/tk_show_notes/
// _parse_key_value_note logic — acceptance criterion 2 requires add-note's
// output to parse successfully against those actual patterns, not a
// hand-eyeballed approximation of them.
var (
	kvLineRe       = regexp.MustCompile(`^([a-zA-Z_]+):\s*(.+)$`)
	notesHeadingRe = regexp.MustCompile(`(?m)^## Notes\s*$`)
	nextHeadingRe  = regexp.MustCompile(`(?m)^## \S`)
	noteMarkerRe   = regexp.MustCompile(`(?m)^\*\*[^*]+\*\*\s*$`)
)

func notesSection(showOutput string) string {
	loc := notesHeadingRe.FindStringIndex(showOutput)
	if loc == nil {
		return ""
	}
	rest := showOutput[loc[1]:]
	if nextLoc := nextHeadingRe.FindStringIndex(rest); nextLoc != nil {
		return rest[:nextLoc[0]]
	}
	return rest
}

func splitNotes(section string) []string {
	markers := noteMarkerRe.FindAllStringIndex(section, -1)
	notes := make([]string, 0, len(markers))
	for i, m := range markers {
		start := m[1]
		end := len(section)
		if i+1 < len(markers) {
			end = markers[i+1][0]
		}
		notes = append(notes, strings.TrimSpace(section[start:end]))
	}
	return notes
}

func parseKeyValueNote(noteText string) map[string]string {
	var lines []string
	for _, l := range strings.Split(noteText, "\n") {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
	}
	if len(lines) == 0 {
		return map[string]string{}
	}
	fields := map[string]string{}
	for _, l := range lines {
		m := kvLineRe.FindStringSubmatch(l)
		if m == nil {
			return map[string]string{}
		}
		fields[m[1]] = strings.TrimSpace(m[2])
	}
	return fields
}

func TestRunAddNote_ParsesAgainstReconciliationRegexes(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	if _, err := runAddNote(dir, id, "a prose note about something happening"); err != nil {
		t.Fatalf("runAddNote 1: %v", err)
	}
	if _, err := runAddNote(dir, id, "branch: chore/x\npr: https://example.com/pull/1\nsha: deadbeef"); err != nil {
		t.Fatalf("runAddNote 2: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	section := notesSection(string(raw))
	if section == "" {
		t.Fatalf("_NOTES_HEADING_RE found no heading in:\n%s", raw)
	}

	notes := splitNotes(section)
	if len(notes) != 2 {
		t.Fatalf("_NOTE_MARKER_RE found %d notes, want 2; section:\n%q", len(notes), section)
	}
	if notes[0] != "a prose note about something happening" {
		t.Errorf("note 0 = %q, want the prose text", notes[0])
	}
	if notes[1] != "branch: chore/x\npr: https://example.com/pull/1\nsha: deadbeef" {
		t.Errorf("note 1 = %q, want the kv text", notes[1])
	}

	if fields := parseKeyValueNote(notes[0]); len(fields) != 0 {
		t.Errorf("prose note kv-parsed as %v, want empty (not mistaken for data)", fields)
	}
	fields := parseKeyValueNote(notes[1])
	want := map[string]string{"branch": "chore/x", "pr": "https://example.com/pull/1", "sha": "deadbeef"}
	if len(fields) != len(want) {
		t.Fatalf("kv fields = %v, want %v", fields, want)
	}
	for k, v := range want {
		if fields[k] != v {
			t.Errorf("kv field %q = %q, want %q", k, fields[k], v)
		}
	}
}

func TestNewAddNoteCmd_TextArg(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	cmd := NewAddNoteCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{id, "multi", "word", "note"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "multi word note") {
		t.Errorf("note text not found in file:\n%s", got)
	}
	if !strings.Contains(out.String(), "Note added to "+id) {
		t.Errorf("stdout = %q, want it to mention %q", out.String(), id)
	}
}

func TestNewAddNoteCmd_StdinPipe(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	origStdinTTY := stdinIsTTY
	stdinIsTTY = func() bool { return false }
	defer func() { stdinIsTTY = origStdinTTY }()

	cmd := NewAddNoteCmd()
	cmd.SetIn(strings.NewReader("piped note text\n"))
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(got), "\n\npiped note text\n") {
		t.Errorf("piped note text not found (trailing newline should be trimmed like $(cat)):\n%s", got)
	}
}

func TestNewAddNoteCmd_NoTextNoPipeErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	origStdinTTY := stdinIsTTY
	stdinIsTTY = func() bool { return true }
	defer func() { stdinIsTTY = origStdinTTY }()

	cmd := NewAddNoteCmd()
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute: want error when no text arg and stdin is a tty, got nil")
	}
}
