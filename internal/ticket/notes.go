package ticket

import (
	"fmt"
	"regexp"
	"time"
)

// notesHeadingPresent mirrors bash tk's `grep -q '^## Notes'` (a prefix
// match, not reconciliation.py's own stricter _NOTES_HEADING_RE) — this
// only decides whether AddNote must insert a fresh heading.
var notesHeadingPresent = regexp.MustCompile(`(?m)^## Notes`)

// AddNote appends a "## Notes" heading (if absent) then a timestamped
// "**<UTC timestamp>**\n\n<text>\n" block to the ticket body — the single
// primitive `tk add-note` and `loop ship`'s closing note both build on, so
// the heading/timestamp-formatting subtlety lives in one place. The exact
// spacing (a single "\n## Notes\n", no trailing blank line) is
// load-bearing: it's what keeps the block parseable by reconciliation.py's
// _NOTES_HEADING_RE/_NOTE_MARKER_RE (see cli/tk's addnote_test.go). Does
// not save — callers persist via Ticket.Save.
func (t *Ticket) AddNote(text string) {
	if !notesHeadingPresent.MatchString(t.Body) {
		t.Body += "\n## Notes\n"
	}
	t.Body += fmt.Sprintf("\n**%s**\n\n%s\n", time.Now().UTC().Format("2006-01-02T15:04:05Z"), text)
}
