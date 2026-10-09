package gitops

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/Christophe1997/goalship/internal/ticket"
)

var (
	notesHeadingRE = regexp.MustCompile(`(?m)^## Notes\s*$`)
	nextHeadingRE  = regexp.MustCompile(`(?m)^## \S`)
	noteMarkerRE   = regexp.MustCompile(`(?m)^\*\*[^*]+\*\*\s*$`)
	kvLineRE       = regexp.MustCompile(`^([a-zA-Z_]+):\s*(.+)$`)
)

// jqString renders s as a jq string literal so an agent-supplied value can be
// spliced into a filter without rewriting it. JSON string syntax is a subset
// of jq's, and json.Marshal escapes the backslash that would start a `\(...)`
// interpolation.
func jqString(s string) string {
	b, _ := json.Marshal(s) // a string always marshals
	return string(b)
}

// queryTickets runs jqFilter over every ticket in the repo's tickets
// directory and decodes each match — mirrors reconciliation.py's tk_query.
func queryTickets(repoRoot, jqFilter string) ([]map[string]any, error) {
	lines, err := ticket.Query(ticket.ResolveTicketsDir(repoRoot), jqFilter)
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	for _, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal(line, &obj); err != nil {
			return nil, fmt.Errorf("gitops: parse ticket query output: %w", err)
		}
		results = append(results, obj)
	}
	return results, nil
}

// stringSlice extracts a []string from a decoded JSON field (e.g. a
// ticket's "deps" array), skipping any non-string element rather than
// failing outright.
func stringSlice(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, e := range raw {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// notesSection extracts the raw text of a ticket file's "## Notes" section,
// up through (not including) the next "## " heading — mirrors
// reconciliation.py's _notes_section.
func notesSection(ticketText string) string {
	loc := notesHeadingRE.FindStringIndex(ticketText)
	if loc == nil {
		return ""
	}
	rest := ticketText[loc[1]:]
	if nextLoc := nextHeadingRE.FindStringIndex(rest); nextLoc != nil {
		return rest[:nextLoc[0]]
	}
	return rest
}

// readTicketFile is the seam note reads go through, reassigned in tests to
// fail a chosen read.
var readTicketFile = os.ReadFile

// readNotes returns the raw text of each note on ticketID, oldest first. It
// reads the ticket file as written rather than through strict ticket.Load:
// the query that lists tickets tolerates a malformed one, and `tk show`
// printed it regardless, so one hand-edited ticket must not abort a whole
// reconcile or sweep.
func readNotes(repoRoot, ticketID string) ([]string, error) {
	path, err := ticket.Resolve(ticket.ResolveTicketsDir(repoRoot), ticketID)
	if err != nil {
		return nil, err
	}
	raw, err := readTicketFile(path)
	if err != nil {
		return nil, err
	}
	section := notesSection(string(raw))
	markers := noteMarkerRE.FindAllStringIndex(section, -1)
	notes := make([]string, 0, len(markers))
	for i, m := range markers {
		start := m[1]
		end := len(section)
		if i+1 < len(markers) {
			end = markers[i+1][0]
		}
		notes = append(notes, strings.TrimSpace(section[start:end]))
	}
	return notes, nil
}

// parseKeyValueNote parses a note's body as key/value fields, but only if
// every non-blank line matches `key: value` — a prose reconciliation note
// never gets misread as data. Mirrors _parse_key_value_note.
func parseKeyValueNote(noteText string) map[string]string {
	var lines []string
	for _, line := range strings.Split(noteText, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return nil
	}
	fields := make(map[string]string, len(lines))
	for _, line := range lines {
		m := kvLineRE.FindStringSubmatch(line)
		if m == nil {
			return nil
		}
		fields[m[1]] = strings.TrimSpace(m[2])
	}
	return fields
}

// noteFieldsForTicket merges key/value fields across all of ticketID's
// structured notes, oldest to newest, so a later note's fields extend or
// override an earlier one's (a claim-time branch: note, then a ship-time
// note that adds pr:/sha:).
func noteFieldsForTicket(repoRoot, ticketID string) (map[string]string, error) {
	notes, err := readNotes(repoRoot, ticketID)
	if err != nil {
		return nil, err
	}
	fields := make(map[string]string)
	for _, note := range notes {
		for k, v := range parseKeyValueNote(note) {
			fields[k] = v
		}
	}
	return fields, nil
}
