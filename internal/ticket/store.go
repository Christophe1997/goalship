// Package ticket is the storage layer for .tickets/*.md files: frontmatter
// parsing/writing byte-compatible with bash tk 0.3.2 (see store.go), plus
// ID resolution and generation (see id.go).
package ticket

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Christophe1997/goalship/internal/atomicfile"
)

// coreFields are the frontmatter keys with dedicated Ticket struct fields —
// the ones tk's cmd_create always writes. Everything else (tk's own
// optional fields like assignee/external-ref/parent/tags, or anything this
// package has never seen) round-trips through Extra (R8).
var coreFields = map[string]bool{
	"id": true, "status": true, "deps": true, "links": true,
	"created": true, "type": true, "priority": true,
}

// coreOrder is the field order tk's cmd_create writes; Bytes falls back to
// it for a Ticket that was never parsed from a file (fieldOrder empty).
var coreOrder = []string{"id", "status", "deps", "links", "created", "type", "priority"}

// Field is a frontmatter key this package doesn't give a dedicated struct
// field to. Value is everything after the key's ':' exactly as it appeared
// in the source file — including any leading space — so re-emitting
// Key + ":" + Value reproduces the original line byte-for-byte (R8).
type Field struct {
	Key   string
	Value string
}

// Ticket is one .tickets/*.md file: frontmatter plus opaque body.
type Ticket struct {
	ID       string
	Status   string
	Deps     []string
	Links    []string
	Created  string
	Type     string
	Priority int

	// Extra holds every frontmatter key besides the core seven above, in
	// their original order, values unparsed. tk's own optional fields
	// (assignee, external-ref, parent, tags) live here rather than as
	// named struct fields, so a genuinely unrecognized key needs no
	// different handling than one tk documents (R8).
	Extra []Field

	// Body is everything after the closing "---" line, byte-for-byte,
	// including the leading newline and the "## Notes" section: this
	// package treats it as opaque (a later ticket parses Notes semantics).
	Body string

	// fieldOrder is the exact frontmatter key order as parsed from disk,
	// used to reconstruct that same order on write. A zero-value Ticket
	// (never parsed) falls back to coreOrder + Extra's order.
	fieldOrder []string
}

// frontmatterDelim matches a line that is exactly "---", tk's frontmatter
// fence. A third or later occurrence (e.g. a markdown horizontal rule in
// the body) is not special: only the first two delimit the frontmatter.
var frontmatterDelim = regexp.MustCompile(`(?m)^---$`)

// Parse decodes a .tickets/*.md file's raw bytes into a Ticket. It is a
// targeted line-based parser, not a general YAML library: tk's frontmatter
// is unquoted "key: value" pairs with a fixed "[a, b]" array syntax, and a
// generic YAML decoder would reorder keys or normalize quoting/spacing —
// breaking the byte-identical round-trip R8 and the acceptance criteria
// require.
func Parse(data []byte) (*Ticket, error) {
	s := string(data)
	locs := frontmatterDelim.FindAllStringIndex(s, -1)
	if len(locs) < 2 || locs[0][0] != 0 {
		return nil, fmt.Errorf("ticket: no frontmatter delimiters found")
	}

	fmStart := locs[0][1]
	if fmStart < len(s) && s[fmStart] == '\n' {
		fmStart++
	}
	fmEnd := locs[1][0]

	bodyStart := locs[1][1]
	if bodyStart < len(s) && s[bodyStart] == '\n' {
		bodyStart++
	}
	body := ""
	if bodyStart <= len(s) {
		body = s[bodyStart:]
	}

	t := &Ticket{Body: body}
	seen := make(map[string]bool)

	for _, line := range strings.Split(s[fmStart:fmEnd], "\n") {
		if line == "" {
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx < 0 {
			return nil, fmt.Errorf("ticket: malformed frontmatter line %q", line)
		}
		key := line[:idx]
		rawValue := line[idx+1:]
		if seen[key] {
			return nil, fmt.Errorf("ticket: duplicate frontmatter key %q", key)
		}
		seen[key] = true
		t.fieldOrder = append(t.fieldOrder, key)

		if !coreFields[key] {
			t.Extra = append(t.Extra, Field{Key: key, Value: rawValue})
			continue
		}

		value := strings.TrimLeft(rawValue, " ")
		var err error
		switch key {
		case "id":
			t.ID = value
		case "status":
			t.Status = value
		case "created":
			t.Created = value
		case "type":
			t.Type = value
		case "deps":
			t.Deps, err = parseArray(value)
		case "links":
			t.Links, err = parseArray(value)
		case "priority":
			t.Priority, err = strconv.Atoi(value)
		}
		if err != nil {
			return nil, fmt.Errorf("ticket: field %q: %w", key, err)
		}
	}

	var missing []string
	for key := range coreFields {
		if !seen[key] {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return nil, fmt.Errorf("ticket: missing required frontmatter field(s): %s", strings.Join(missing, ", "))
	}

	return t, nil
}

// Load reads and parses a ticket file from disk.
func Load(path string) (*Ticket, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("ticket: load %s: %w", path, err)
	}
	t, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("ticket: load %s: %w", path, err)
	}
	return t, nil
}

// Bytes renders the ticket back to .tickets/*.md format: the original key
// order when the Ticket came from Parse/Load — so an unmodified round-trip
// is byte-identical (R8) — or tk's cmd_create order for a fresh Ticket.
func (t *Ticket) Bytes() []byte {
	var b strings.Builder
	b.WriteString("---\n")

	extraByKey := make(map[string]string, len(t.Extra))
	for _, f := range t.Extra {
		extraByKey[f.Key] = f.Value
	}

	order := t.fieldOrder
	if len(order) == 0 {
		order = make([]string, 0, len(coreOrder)+len(t.Extra))
		order = append(order, coreOrder...)
		for _, f := range t.Extra {
			order = append(order, f.Key)
		}
	}

	written := make(map[string]bool, len(order))
	for _, key := range order {
		if written[key] {
			continue
		}
		switch {
		case coreFields[key]:
			t.writeCoreField(&b, key)
			written[key] = true
		default:
			if v, ok := extraByKey[key]; ok {
				fmt.Fprintf(&b, "%s:%s\n", key, v)
				written[key] = true
			}
			// A key present in fieldOrder but no longer in Extra was
			// removed by the caller since Load: drop it from the output.
		}
	}
	// Extra fields appended after Load (absent from fieldOrder) land here,
	// at the end — not tk's own update_yaml_field, which inserts a brand
	// new unrecognized field right after the opening "---"; no code path
	// in this package or the current tk-parity commands needs that.
	for _, f := range t.Extra {
		if !written[f.Key] {
			fmt.Fprintf(&b, "%s:%s\n", f.Key, f.Value)
			written[f.Key] = true
		}
	}

	b.WriteString("---\n")
	b.WriteString(t.Body)
	return []byte(b.String())
}

func (t *Ticket) writeCoreField(b *strings.Builder, key string) {
	switch key {
	case "id":
		fmt.Fprintf(b, "id: %s\n", t.ID)
	case "status":
		fmt.Fprintf(b, "status: %s\n", t.Status)
	case "deps":
		fmt.Fprintf(b, "deps: %s\n", formatArray(t.Deps))
	case "links":
		fmt.Fprintf(b, "links: %s\n", formatArray(t.Links))
	case "created":
		fmt.Fprintf(b, "created: %s\n", t.Created)
	case "type":
		fmt.Fprintf(b, "type: %s\n", t.Type)
	case "priority":
		fmt.Fprintf(b, "priority: %d\n", t.Priority)
	}
}

// Save writes the ticket to path atomically (internal/atomicfile): a
// concurrent reader never observes a partially-written file.
func (t *Ticket) Save(path string) error {
	if err := atomicfile.Write(path, t.Bytes()); err != nil {
		return fmt.Errorf("ticket: save %s: %w", path, err)
	}
	return nil
}

// parseArray decodes tk's "[a, b]"/"[]" array syntax. Splitting on "," and
// trimming each element (rather than requiring the canonical ", ") matches
// tk's own reader — awk's `gsub(/[\[\] ]/, "", deps); split(deps, arr, ",")`
// strips every space before splitting — so a hand-edited "[a,b]" or
// "[a , b]" still parses instead of silently becoming one element (R2: an
// existing .tickets/ directory keeps working unmodified). Bytes always
// re-emits the canonical "[a, b]" form.
func parseArray(raw string) ([]string, error) {
	if !strings.HasPrefix(raw, "[") || !strings.HasSuffix(raw, "]") {
		return nil, fmt.Errorf("expected array syntax (e.g. \"[a, b]\"), got %q", raw)
	}
	inner := strings.TrimSpace(raw[1 : len(raw)-1])
	if inner == "" {
		return nil, nil
	}
	parts := strings.Split(inner, ",")
	items := make([]string, len(parts))
	for i, p := range parts {
		items[i] = strings.TrimSpace(p)
	}
	return items, nil
}

func formatArray(items []string) string {
	return "[" + strings.Join(items, ", ") + "]"
}
