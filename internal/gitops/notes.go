package gitops

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

var (
	notesHeadingRE = regexp.MustCompile(`(?m)^## Notes\s*$`)
	nextHeadingRE  = regexp.MustCompile(`(?m)^## \S`)
	noteMarkerRE   = regexp.MustCompile(`(?m)^\*\*[^*]+\*\*\s*$`)
	kvLineRE       = regexp.MustCompile(`^([a-zA-Z_]+):\s*(.+)$`)
)

// tkQuery runs `tk query <jqFilter>` against the real installed `tk`
// binary and parses its newline-delimited JSON output — mirrors
// reconciliation.py's tk_query. Ticket-graph query capability doesn't yet
// exist as a Go-native package in this repo, so this shells out directly
// rather than depending on not-yet-landed work.
func tkQuery(repoRoot, jqFilter string) ([]map[string]any, error) {
	out, err := run(repoRoot, "tk", "query", jqFilter)
	if err != nil {
		return nil, err
	}
	var results []map[string]any
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			return nil, fmt.Errorf("gitops: parse tk query output: %w", err)
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

// notesSection extracts the raw text of a `tk show` transcript's
// "## Notes" section, up through (not including) the next "## " heading —
// mirrors reconciliation.py's _notes_section.
func notesSection(showOutput string) string {
	loc := notesHeadingRE.FindStringIndex(showOutput)
	if loc == nil {
		return ""
	}
	rest := showOutput[loc[1]:]
	if nextLoc := nextHeadingRE.FindStringIndex(rest); nextLoc != nil {
		return rest[:nextLoc[0]]
	}
	return rest
}

// tkShowNotes returns the raw text of each note on ticketID, oldest first.
func tkShowNotes(repoRoot, ticketID string) ([]string, error) {
	out, err := run(repoRoot, "tk", "show", ticketID)
	if err != nil {
		return nil, err
	}
	section := notesSection(out)
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
	notes, err := tkShowNotes(repoRoot, ticketID)
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
