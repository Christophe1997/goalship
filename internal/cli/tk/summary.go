package tk

import (
	"fmt"
	"io"
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
// sorted glob expansion) into a ticketInfo via ticket.ParseTolerant, which
// defaults a missing/malformed field the way bash tk's own awk-based list
// readers do rather than rejecting the whole file — so ls/ready/blocked/
// closed degrade a hand-edited ticket gracefully instead of dropping it
// (goa-ioe4): a ticket silently missing from `ready` reads identically to
// one genuinely blocked by open deps, to a caller with no other signal.
//
// warn receives one line per file that had anything defaulted (naming
// what), plus one line per file skipped outright (missing id, or no
// frontmatter delimiters at all — nothing sane to default for either).
// Pass io.Discard when a caller doesn't want that surfaced (show's use of
// this same helper is for cross-referencing OTHER tickets' derived
// relationship sections, not the primary ticket.Load it already did
// strictly for the target id).
func loadTicketInfos(ticketsDir string, warn io.Writer) ([]ticketInfo, error) {
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
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(warn, "tk: skipping %s: %v\n", e.Name(), err)
			continue
		}
		t, warnings, err := ticket.ParseTolerant(data)
		if err != nil {
			fmt.Fprintf(warn, "tk: skipping %s: %v\n", e.Name(), err)
			continue
		}
		if len(warnings) > 0 {
			fmt.Fprintf(warn, "tk: %s: %s\n", e.Name(), strings.Join(warnings, "; "))
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
