package tk

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// ticketInfo is the subset of a ticket's fields the listing/show commands
// read, mirroring the fields bash tk's awk scripts pull out of frontmatter
// plus the body's first "# " line as title.
type ticketInfo struct {
	id       string
	status   string
	title    string
	assignee string
	parent   string
	deps     []string
	links    []string
	tags     []string
	priority int
	modTime  time.Time
}

// loadTicketInfos reads every *.md file in ticketsDir (already sorted by
// filename — os.ReadDir's documented guarantee, matching bash tk's own
// sorted glob expansion) into a ticketInfo, skipping any file that fails
// to parse.
//
// This is a known divergence from bash tk, not a match for it: awk's
// field-by-field regex scan tolerates a ticket missing a field it wants
// (e.g. no "links:" line — yaml_field/add_link_to_file already treat that
// as absent, defaulting to "[]") by leaving that one field blank, while
// ticket.Parse rejects the whole file when a required field is missing (or
// a key repeats, or an array value isn't "[...]" shaped) — so that ticket
// disappears from ls/ready/blocked/closed entirely instead of appearing
// with partial data. Silent omission from `ready` in particular is
// indistinguishable from "genuinely blocked" to a caller. Not fixed here
// (out of this ticket's scope); flagged as a follow-up in the ticket
// report.
func loadTicketInfos(ticketsDir string) ([]ticketInfo, error) {
	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		return nil, err
	}

	infos := make([]ticketInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		path := filepath.Join(ticketsDir, e.Name())
		t, err := ticket.Load(path)
		if err != nil {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		infos = append(infos, ticketInfo{
			id:       t.ID,
			status:   t.Status,
			title:    firstTitle(t.Body),
			assignee: extraValue(t, "assignee"),
			parent:   extraValue(t, "parent"),
			deps:     t.Deps,
			links:    t.Links,
			tags:     splitTagList(extraValue(t, "tags")),
			priority: t.Priority,
			modTime:  fi.ModTime(),
		})
	}
	return infos, nil
}

// firstTitle returns the text after the body's first "# " line — bash
// tk's `!in_front && /^# / && title == "" { title = substr($0, 3) }`.
func firstTitle(body string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "# ") {
			return line[2:]
		}
	}
	return ""
}

// extraValue reads a Field's value with the single leading space that
// ": " (frontmatter key/value separator) always produces stripped —
// mirroring awk's FS=": " field split, which consumes that one space.
func extraValue(t *ticket.Ticket, key string) string {
	for _, f := range t.Extra {
		if f.Key == key {
			return strings.TrimPrefix(f.Value, " ")
		}
	}
	return ""
}

// splitTagList mirrors bash tk's tag parsing: `gsub(/[\[\] ]/, "", tags)`
// strips brackets and ALL spaces (not just around commas, unlike deps)
// before splitting on ",".
func splitTagList(raw string) []string {
	stripped := strings.Map(func(r rune) rune {
		if r == '[' || r == ']' || r == ' ' {
			return -1
		}
		return r
	}, raw)
	if stripped == "" {
		return nil
	}
	return strings.Split(stripped, ",")
}

func hasTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

// byPriorityThenID is bash tk's ready/blocked sort: ascending priority,
// ties broken by ascending id — a full ordering, unlike ls/closed which
// keep file-listing order.
func byPriorityThenID(infos []ticketInfo) {
	sort.Slice(infos, func(i, j int) bool {
		if infos[i].priority != infos[j].priority {
			return infos[i].priority < infos[j].priority
		}
		return infos[i].id < infos[j].id
	})
}

func isoNow() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}
