// sections.go supports goa-4ufc's structured body editing (PATCH
// /api/tickets/:id's title/description/acceptance-criteria fields):
// title/description/"## " sections live inside Ticket.Body as free-form
// markdown, not dedicated struct fields (see store.go's own doc on Body
// being opaque to this package otherwise).
//
// Reads (ParseSections) and writes (SetTitle/SetDescription/SetSection) are
// implemented as targeted byte-range splicing on the raw Body string, not
// parse-everything-then-re-render: this is what keeps every section NOT
// being edited byte-identical (mirrors store.go's Extra-field byte
// preservation for frontmatter, R8, now applied to the body). Both sides
// share the same span-finding helpers below so a read and a write can never
// disagree about where a block starts and ends.

package ticket

import (
	"regexp"
	"strings"
)

// titleLineRe matches a ticket's title line; sectionLineRe matches a "## "
// section heading. Both anchor on (?m) line boundaries, not "the whole
// body," since a heading can appear anywhere.
var (
	titleLineRe   = regexp.MustCompile(`(?m)^# (.+)$`)
	sectionLineRe = regexp.MustCompile(`(?m)^## (.+)$`)
)

// consumeNewline advances past exactly one "\n" at pos, if present — used
// to skip a matched line's own terminator without also swallowing a blank
// line that belongs to the next block.
func consumeNewline(s string, pos int) int {
	if pos < len(s) && s[pos] == '\n' {
		return pos + 1
	}
	return pos
}

// titleSpan locates the first "# " line (only the first counts — mirrors
// query.go's ScanAwkTicket titleSet precedent: a later "# " line, inside a
// section or otherwise, is ordinary text). start/end bracket the line
// itself, trailing newline included, nothing else; text is its captured
// content. found is false when body has no title line yet.
func titleSpan(body string) (start, end int, text string, found bool) {
	loc := titleLineRe.FindStringSubmatchIndex(body)
	if loc == nil {
		return 0, 0, "", false
	}
	return loc[0], consumeNewline(body, loc[1]), body[loc[2]:loc[3]], true
}

// descriptionSpan locates the description block: everything from just
// after the title line (or byte 0, when body has no title yet) up to the
// first "## " line at or after that point, or EOF. Searching only within
// body[start:] (rather than the whole body) keeps end >= start always, even
// if a "## "-shaped line happens to precede the title in a hand-edited
// file — ParseSections and SetDescription both call this so they can never
// disagree on the range.
func descriptionSpan(body string) (start, end int) {
	start = 0
	if _, titleEnd, _, found := titleSpan(body); found {
		start = titleEnd
	}
	end = len(body)
	if loc := sectionLineRe.FindStringIndex(body[start:]); loc != nil {
		end = start + loc[0]
	}
	return start, end
}

// sectionSpan is one "## " heading's location: headingStart begins the
// heading line itself (so a replace can overwrite the whole block),
// contentStart begins just after it, end is the next heading's start (or
// EOF).
type sectionSpan struct {
	name         string
	headingStart int
	contentStart int
	end          int
}

// sectionSpans returns every "## " section's span, in file order.
func sectionSpans(body string) []sectionSpan {
	locs := sectionLineRe.FindAllStringSubmatchIndex(body, -1)
	spans := make([]sectionSpan, len(locs))
	for i, loc := range locs {
		end := len(body)
		if i+1 < len(locs) {
			end = locs[i+1][0]
		}
		spans[i] = sectionSpan{
			name:         body[loc[2]:loc[3]],
			headingStart: loc[0],
			contentStart: consumeNewline(body, loc[1]),
			end:          end,
		}
	}
	return spans
}

// ParseSections extracts a ticket body's title, description, and named
// "## " sections, in the order they first appear. Values are trimmed of
// surrounding blank lines — the whitespace that separates blocks in
// buildCreateBody's own layout (see create.go), not meaningful content.
func ParseSections(body string) (title, description string, sections map[string]string, order []string) {
	if _, _, text, found := titleSpan(body); found {
		title = text
	}

	dStart, dEnd := descriptionSpan(body)
	description = strings.TrimSpace(body[dStart:dEnd])

	sections = make(map[string]string)
	for _, sp := range sectionSpans(body) {
		if _, exists := sections[sp.name]; !exists {
			order = append(order, sp.name)
		}
		sections[sp.name] = strings.TrimSpace(body[sp.contentStart:sp.end])
	}
	return title, description, sections, order
}

// SetTitle replaces body's title line in place — the same byte range
// titleSpan reads it from, nothing else touched — or inserts one at the
// very start (buildCreateBody's ordering: title first) when body has none
// yet.
func SetTitle(body, title string) string {
	start, end, _, found := titleSpan(body)
	line := "# " + title + "\n"
	if !found {
		return line + body
	}
	return body[:start] + line + body[end:]
}

// SetDescription replaces body's description block — descriptionSpan's own
// range — with newDescription rendered in buildCreateBody's spacing
// convention ("# %s\n\n" leaves one blank line even with no description;
// "%s\n\n" appends a description plus its own trailing blank line). A
// round-trip with the exact value ParseSections returned reproduces the
// original bytes, since that's the same convention already on disk.
func SetDescription(body, description string) string {
	start, end := descriptionSpan(body)
	block := "\n"
	if description != "" {
		block = "\n" + description + "\n\n"
	}
	return body[:start] + block + body[end:]
}

// SetSection replaces the named "## " section's full range (heading plus
// content) in place, following buildCreateBody's "## %s\n\n%s\n\n"
// convention, or appends a new one at the end when body has no section by
// that name yet — per this package's own choice of ordering (title/
// description first, then sections, matching buildCreateBody), a new
// section always lands after every existing one.
func SetSection(body, name, content string) string {
	block := "## " + name + "\n\n" + content + "\n\n"

	for _, sp := range sectionSpans(body) {
		if sp.name != name {
			continue
		}
		return body[:sp.headingStart] + block + body[sp.end:]
	}

	trimmed := strings.TrimRight(body, "\n")
	if trimmed == "" {
		return block
	}
	return trimmed + "\n\n" + block
}
