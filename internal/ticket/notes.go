package ticket

import (
	"fmt"
	"regexp"
	"time"
)

// notesHeadingPresent mirrors bash tk's `grep -q '^## Notes'` (a prefix
// match, not the trailing-\s*$ anchor reconciliation.py's own heading
// regex uses — this only decides whether to insert a fresh heading).
var notesHeadingPresent = regexp.MustCompile(`(?m)^## Notes`)

// AppendNote returns body with a timestamped note block appended: a
// "## Notes" heading first, if body doesn't already have one, then a
// "**<timestamp>**\n\n<note>\n" block. Extracted from internal/cli/tk's
// runAddNote (cmd_add_note's Go core) so any caller outside that package —
// internal/cli/loop's claim command included — can produce the exact same
// on-disk shape without duplicating it. The spacing here is load-bearing:
// it's what keeps the block parseable by reconciliation.py's
// _NOTES_HEADING_RE/_NOTE_MARKER_RE (see internal/cli/tk/addnote_test.go).
func AppendNote(body, note string) string {
	if !notesHeadingPresent.MatchString(body) {
		body += "\n## Notes\n"
	}
	timestamp := time.Now().UTC().Format("2006-01-02T15:04:05Z")
	return body + fmt.Sprintf("\n**%s**\n\n%s\n", timestamp, note)
}
