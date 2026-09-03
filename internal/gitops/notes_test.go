package gitops

import (
	"reflect"
	"strings"
	"testing"
)

// sampleShowOutput is the exact shape a real `tk show` prints (captured
// against this repo's own goa-jatp ticket): frontmatter, body, an
// "## Acceptance Criteria" section, "## Notes" with a timestamp marker
// followed by key:value lines, then a further "## Blocking" section.
const sampleShowOutput = `---
id: goa-jatp
status: in_progress
deps: [goa-fxh3]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U6A
---
# Git/branch mechanics

Some description.

## Acceptance Criteria

- a
- b

## Notes

**2026-09-03T07:32:05Z**

branch: feature/git-branch-mechanics
base: chore/project-scaffolding-and-command-tree
claim_sha: 3fd3f22dc00165f8388793fb800d262edc88d302

## Blocking

- goa-5zwn [open] Commit/PR mechanics
`

func TestNotesSection_ExtractsBetweenNotesHeadingAndNextHeading(t *testing.T) {
	section := notesSection(sampleShowOutput)
	for _, want := range []string{"**2026-09-03T07:32:05Z**", "branch: feature/git-branch-mechanics"} {
		if !strings.Contains(section, want) {
			t.Errorf("notesSection missing %q in %q", want, section)
		}
	}
	if strings.Contains(section, "goa-5zwn") {
		t.Errorf("notesSection leaked past the next heading: %q", section)
	}
}

func TestNotesSection_NoNotesHeading_ReturnsEmpty(t *testing.T) {
	if got := notesSection("# Title\n\nNo notes here.\n"); got != "" {
		t.Errorf("notesSection = %q, want empty", got)
	}
}

func TestNotesSection_NotesIsTheLastSection_ReturnsToEndOfString(t *testing.T) {
	input := "## Notes\n\n**t**\n\nbranch: x\n"
	got := notesSection(input)
	if !strings.Contains(got, "branch: x") {
		t.Errorf("notesSection = %q, missing trailing content", got)
	}
}

func TestParseKeyValueNote_AllKVLines_ReturnsFields(t *testing.T) {
	got := parseKeyValueNote("branch: feat/x\nclaim_sha: abc123")
	want := map[string]string{"branch": "feat/x", "claim_sha": "abc123"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("parseKeyValueNote = %v, want %v", got, want)
	}
}

// TestParseKeyValueNote_ProseLine_ReturnsNil is the machine-readable-note
// guard: a note qualifies as data only if EVERY non-blank line is
// `key: value` — one prose line among otherwise-valid ones is enough to
// disqualify the whole note, so it never gets misread as data.
func TestParseKeyValueNote_ProseLine_ReturnsNil(t *testing.T) {
	got := parseKeyValueNote("Reconciliation: PR 42 merged externally; closing.\nThis ticket was auto-closed by the reconciliation loop.")
	if got != nil {
		t.Errorf("parseKeyValueNote = %v, want nil", got)
	}
}

func TestParseKeyValueNote_Empty_ReturnsNil(t *testing.T) {
	if got := parseKeyValueNote("   \n  \n"); got != nil {
		t.Errorf("parseKeyValueNote = %v, want nil", got)
	}
}

// TestNoteFieldsForTicket_LaterNoteFieldsOverrideEarlier mirrors
// note_fields_for_ticket's contract: fields merge oldest to newest, so a
// later note's fields extend or override an earlier one's (a claim-time
// branch: note, then a ship-time note that adds pr:/sha:).
func TestNoteFieldsForTicket_LaterNoteFieldsOverrideEarlier(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "Note override test")
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x")
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: https://example.com/pr/9\nsha: deadbeef")

	fields, err := noteFieldsForTicket(repoRoot, ticketID)
	if err != nil {
		t.Fatalf("noteFieldsForTicket: %v", err)
	}
	want := map[string]string{"branch": "feat/x", "pr": "https://example.com/pr/9", "sha": "deadbeef"}
	if !reflect.DeepEqual(fields, want) {
		t.Errorf("fields = %v, want %v", fields, want)
	}
}

func TestNoteFieldsForTicket_NoNotesYet_ReturnsEmptyMap(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "No notes yet")

	fields, err := noteFieldsForTicket(repoRoot, ticketID)
	if err != nil {
		t.Fatalf("noteFieldsForTicket: %v", err)
	}
	if len(fields) != 0 {
		t.Errorf("fields = %v, want empty", fields)
	}
}
