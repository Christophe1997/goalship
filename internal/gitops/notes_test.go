package gitops

import (
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
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

func ticketIDs(t *testing.T, repoRoot string) []string {
	t.Helper()
	tickets, err := queryTickets(repoRoot, ".")
	if err != nil {
		t.Fatalf("queryTickets: %v", err)
	}
	var ids []string
	for _, tk := range tickets {
		id, _ := tk["id"].(string)
		ids = append(ids, id)
	}
	return ids
}

func TestQueryTickets_HonorsTicketsDirEnvOutsideRepoRoot(t *testing.T) {
	repoRoot := newTestRepo(t)
	t.Setenv(ticket.TicketsDirEnv, filepath.Join(t.TempDir(), "elsewhere"))
	ticketID := tkCreate(t, repoRoot, "Lives outside the repo")

	if got := ticketIDs(t, repoRoot); !reflect.DeepEqual(got, []string{ticketID}) {
		t.Errorf("ticket ids = %v, want [%s] from the TICKETS_DIR override", got, ticketID)
	}
}

func TestQueryTickets_DoesNotSearchParentDirectories(t *testing.T) {
	parent := t.TempDir()
	repoRoot := filepath.Join(parent, "work")
	mustMkdirAll(t, repoRoot)
	parentTicket := tkCreate(t, parent, "Only the parent has a .tickets directory")

	tickets, err := queryTickets(repoRoot, ".")
	for _, tk := range tickets {
		if tk["id"] == parentTicket {
			t.Fatalf("queryTickets found %s in a parent directory of the repo root", parentTicket)
		}
	}
	if err == nil && len(tickets) != 0 {
		t.Errorf("tickets = %v, want none from a repo root without its own .tickets", tickets)
	}
}

func TestNoteFieldsForTicket_HonorsTicketsDirEnv(t *testing.T) {
	repoRoot := newTestRepo(t)
	t.Setenv(ticket.TicketsDirEnv, filepath.Join(t.TempDir(), "elsewhere"))
	ticketID := tkCreate(t, repoRoot, "Lives outside the repo")
	tkAddNote(t, repoRoot, ticketID, "branch: feat/elsewhere")

	fields, err := noteFieldsForTicket(repoRoot, ticketID)
	if err != nil {
		t.Fatalf("noteFieldsForTicket: %v", err)
	}
	if fields["branch"] != "feat/elsewhere" {
		t.Errorf("fields = %v, want branch feat/elsewhere read from the TICKETS_DIR override", fields)
	}
}

// writeMalformedTicket writes a ticket that strict ticket.Load rejects (a
// duplicated status key) but that the query still lists and whose notes tk
// show would still print — the shape one hand-edited file can take.
func writeMalformedTicket(t *testing.T, repoRoot, id string) {
	t.Helper()
	dir := ticket.ResolveTicketsDir(repoRoot)
	mustMkdirAll(t, dir)
	writeFile(t, filepath.Join(dir, id+".md"), "---\nid: "+id+"\nstatus: open\nstatus: in_progress\ndeps: []\n---\n# Malformed\n\n## Notes\n\n**2026-09-28T00:00:00Z**\n\nbranch: feat/malformed\npr: PR9\n")
	if _, err := ticket.Load(filepath.Join(dir, id+".md")); err == nil {
		t.Fatal("fixture is not malformed: strict Load accepted it")
	}
}

func TestNoteFieldsForTicket_MalformedFrontmatter_StillReadsNotes(t *testing.T) {
	repoRoot := newTestRepo(t)
	writeMalformedTicket(t, repoRoot, "bad-1")
	healthy := tkCreate(t, repoRoot, "Healthy neighbour")
	tkAddNote(t, repoRoot, healthy, "branch: feat/healthy")

	if got := ticketIDs(t, repoRoot); !containsString(got, "bad-1") {
		t.Fatalf("query ids = %v, want the malformed ticket still listed", got)
	}
	fields, err := noteFieldsForTicket(repoRoot, "bad-1")
	if err != nil {
		t.Fatalf("noteFieldsForTicket on a malformed ticket: %v", err)
	}
	if fields["branch"] != "feat/malformed" || fields["pr"] != "PR9" {
		t.Errorf("fields = %v, want branch feat/malformed and pr PR9", fields)
	}
	if other, err := noteFieldsForTicket(repoRoot, healthy); err != nil || other["branch"] != "feat/healthy" {
		t.Errorf("healthy ticket fields = %v, err = %v; one malformed neighbour must not break it", other, err)
	}
}

func TestNoteFieldsForTicket_UnknownTicket_ErrorsWithErrNotFound(t *testing.T) {
	repoRoot := newTestRepo(t)
	tkCreate(t, repoRoot, "Some ticket")

	_, err := noteFieldsForTicket(repoRoot, "no-such-ticket")
	if !errors.Is(err, ticket.ErrNotFound) {
		t.Errorf("err = %v, want ticket.ErrNotFound", err)
	}
}

func TestNoteFieldsForTicket_AmbiguousTicket_ErrorsWithErrAmbiguous(t *testing.T) {
	repoRoot := newTestRepo(t)
	first := tkCreate(t, repoRoot, "First")
	tkCreate(t, repoRoot, "Second")
	sharedPrefix := first[:strings.Index(first, "-")+1]

	_, err := noteFieldsForTicket(repoRoot, sharedPrefix)
	if !errors.Is(err, ticket.ErrAmbiguous) {
		t.Errorf("err = %v, want ticket.ErrAmbiguous for the shared prefix %q", err, sharedPrefix)
	}
}

func containsString(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}
