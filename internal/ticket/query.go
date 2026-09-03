// query.go wraps github.com/itchyny/gojq for the two commands whose bash tk
// counterparts shell out to real jq: cmd_query (tk query) and
// cmd_migrate_beads (tk migrate-beads). Both read raw frontmatter with a
// dedicated scanner rather than Parse/Load: cmd_query's JSON needs every
// frontmatter line in file order (including keys this package has no
// struct field for), which Parse's fieldOrder doesn't expose outside the
// package, and — like cmd_dep_tree/cmd_dep_cycle's own awk passes — it
// tolerates a ticket missing a core field, which Parse deliberately does
// not.
package ticket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/itchyny/gojq"
)

// AwkField is one frontmatter line as bash tk's cmd_query awk pass sees
// it: Value is a string for a scalar line, []any for an "[a, b]"-shaped
// one. Every scalar — including priority — stays a JSON string, never a
// number: the awk pass quotes unconditionally, so `tk query .` reports
// priority as "2", not 2, and this package matches it.
type AwkField struct {
	Key   string
	Value any
}

// AwkTicket is one .tickets/*.md file's data as bash tk's various awk
// passes (cmd_query, cmd_dep_tree, cmd_dep_cycle) derive it directly from
// the raw file. ID/Status/Title/Deps serve dep tree/cycle; Fields (every
// frontmatter line, core and extra alike, in file order) serves query.
type AwkTicket struct {
	ID     string
	Status string
	Title  string
	Deps   []string
	Fields []AwkField
}

// ScanAwkTicket parses data the way bash tk's awk passes do: every line
// that is exactly "---" toggles an in-frontmatter flag (not just the
// first two, unlike Parse's fixed two-delimiter rule — a stray "---"
// later in the body, e.g. a markdown rule, would re-open "frontmatter" in
// the real bash tool too), any in-frontmatter line starting with a letter
// is a field (key = text before the first ": ", value = the rest,
// trimmed), and the title is the first "# "-prefixed line seen while not
// in frontmatter.
func ScanAwkTicket(data []byte) AwkTicket {
	var t AwkTicket
	inFront := false
	titleSet := false

	for _, line := range strings.Split(string(data), "\n") {
		if line == "---" {
			inFront = !inFront
			continue
		}
		if inFront {
			if line == "" {
				continue
			}
			c := line[0]
			if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
				continue
			}
			key, val := splitAwkField(line)
			switch key {
			case "id":
				t.ID = val
			case "status":
				t.Status = val
			case "deps":
				t.Deps = splitAwkArray(val)
			}
			t.Fields = append(t.Fields, AwkField{Key: key, Value: awkFieldValue(val)})
			continue
		}
		if !titleSet && strings.HasPrefix(line, "# ") {
			t.Title = strings.TrimPrefix(line, "# ")
			titleSet = true
		}
	}
	return t
}

func splitAwkField(line string) (key, val string) {
	if idx := strings.Index(line, ": "); idx >= 0 {
		return line[:idx], strings.TrimSpace(line[idx+2:])
	}
	return line, ""
}

func awkFieldValue(val string) any {
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		items := splitAwkArray(val)
		arr := make([]any, len(items))
		for i, it := range items {
			arr[i] = it
		}
		return arr
	}
	return val
}

// splitAwkArray decodes a "[a, b]"/"[]" value the way both cmd_query's
// (strip brackets, split on ", *") and cmd_dep_tree/cmd_dep_cycle's
// (strip "[] " entirely, split on ",") awk implementations do — the two
// approaches only disagree on inputs no real tk ID produces (spaces
// inside an element), so one shared implementation covers both callers.
func splitAwkArray(val string) []string {
	inner := strings.TrimSuffix(strings.TrimPrefix(val, "["), "]")
	inner = strings.TrimSpace(inner)
	if inner == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(inner, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// LoadAwkTickets reads every "*.md" file directly under ticketsDir and
// scans each with ScanAwkTicket, in the sorted-by-filename order
// os.ReadDir guarantees — matching the shell glob "$TICKETS_DIR"/*.md
// bash tk's awk passes iterate (bash glob order is lexicographic).
func LoadAwkTickets(ticketsDir string) ([]AwkTicket, error) {
	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		return nil, fmt.Errorf("ticket: load awk tickets: %w", err)
	}

	var tickets []AwkTicket
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ticketsDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("ticket: load awk tickets: %w", err)
		}
		tickets = append(tickets, ScanAwkTicket(data))
	}
	return tickets, nil
}

func (t AwkTicket) fieldsMap() map[string]any {
	m := make(map[string]any, len(t.Fields))
	for _, f := range t.Fields {
		m[f.Key] = f.Value
	}
	return m
}

func (t AwkTicket) fieldOrder() []string {
	order := make([]string, len(t.Fields))
	for i, f := range t.Fields {
		order[i] = f.Key
	}
	return order
}

// Query runs filter — a jq expression, "." when empty — against every
// ticket in ticketsDir, mirroring bash tk's cmd_query: `jq -c
// "select($filter)"` fed one JSON object per ticket (not a JSON array —
// each object is an independent top-level jq input, so a filter runs
// once per ticket and each match becomes its own returned line).
//
// gojq's own encoder always sorts object keys (its map[string]any
// representation has no order — see the library's own README on why it
// doesn't support -S: "sorts by default"), which would scramble a
// ticket's field order on every filter, including the identity filter
// bash tk's callers use to read a ticket back unchanged. To avoid that,
// Query keeps each ticket's original frontmatter field order alongside
// the map[string]any handed to gojq, and encodes results with its own
// order-preserving encoder — keys present in that order print first, any
// the filter introduced (an object-constructing query outside the three
// shapes this project's own loop sends) print alphabetically after.
func Query(ticketsDir, filter string) ([][]byte, error) {
	if filter == "" {
		filter = "."
	}
	q, err := gojq.Parse(filter)
	if err != nil {
		return nil, fmt.Errorf("ticket: query: parse filter %q: %w", filter, err)
	}

	tickets, err := LoadAwkTickets(ticketsDir)
	if err != nil {
		return nil, err
	}

	var lines [][]byte
	for _, t := range tickets {
		if len(t.Fields) == 0 {
			continue
		}
		order := t.fieldOrder()
		iter := q.Run(t.fieldsMap())
		for {
			v, ok := iter.Next()
			if !ok {
				break
			}
			if e, isErr := v.(error); isErr {
				return nil, fmt.Errorf("ticket: query: %w", e)
			}
			b, err := encodeOrdered(v, order)
			if err != nil {
				return nil, fmt.Errorf("ticket: query: %w", err)
			}
			lines = append(lines, b)
		}
	}
	return lines, nil
}

// encodeOrdered JSON-encodes v compactly (matching jq -c). For an
// object, keys in order print first (skipping any order names v doesn't
// have — a filter can drop fields), followed by any of v's keys order
// doesn't name, alphabetically. order is nil below the top level: tk's
// ticket data never nests an object inside an array or another object,
// so only the top-level result needs ordering.
func encodeOrdered(v any, order []string) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeOrdered(&buf, v, order); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeOrdered(buf *bytes.Buffer, v any, order []string) error {
	switch val := v.(type) {
	case map[string]any:
		buf.WriteByte('{')
		written := make(map[string]bool, len(val))
		first := true
		emit := func(k string) error {
			if !first {
				buf.WriteByte(',')
			}
			first = false
			kb, err := json.Marshal(k)
			if err != nil {
				return err
			}
			buf.Write(kb)
			buf.WriteByte(':')
			written[k] = true
			return writeOrdered(buf, val[k], nil)
		}
		for _, k := range order {
			if _, ok := val[k]; ok {
				if err := emit(k); err != nil {
					return err
				}
			}
		}
		rest := make([]string, 0, len(val)-len(written))
		for k := range val {
			if !written[k] {
				rest = append(rest, k)
			}
		}
		sort.Strings(rest)
		for _, k := range rest {
			if err := emit(k); err != nil {
				return err
			}
		}
		buf.WriteByte('}')
		return nil
	case []any:
		buf.WriteByte('[')
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			if err := writeOrdered(buf, item, nil); err != nil {
				return err
			}
		}
		buf.WriteByte(']')
		return nil
	case string:
		b, err := json.Marshal(val)
		if err != nil {
			return err
		}
		buf.Write(b)
		return nil
	case nil:
		buf.WriteString("null")
		return nil
	case bool:
		if val {
			buf.WriteString("true")
		} else {
			buf.WriteString("false")
		}
		return nil
	default:
		// Numbers and anything else gojq's own value model produces:
		// tk's ticket data is only ever strings/arrays/objects of those,
		// so this path is a defensive fallback, not a hot path.
		b, err := gojq.Marshal(val)
		if err != nil {
			return fmt.Errorf("encode %T: %w", v, err)
		}
		buf.Write(b)
		return nil
	}
}

// MigrateBeadsProgram is bash tk's cmd_migrate_beads jq program,
// reproduced verbatim (its def functions, map/join, and the //
// alternative operator all matter for parity) so a migrated ticket's
// content matches bash tk's byte-for-byte.
const MigrateBeadsProgram = `
def by_type(t): [.dependencies[]? | select(.type == t) | .depends_on_id];
def to_array: if length == 0 then "[]" else "[" + (map("\(.)") | join(", ")) + "]" end;

(by_type("blocks") | to_array) as $deps |
(by_type("related") | to_array) as $links |
(by_type("parent-child") | first // null) as $parent |

"<<<FILE:\(.id)>>>\n" +
"---\n" +
"id: \(.id)\n" +
"status: \(.status // "open")\n" +
"deps: \($deps)\n" +
"links: \($links)\n" +
"created: \(.created_at // "")\n" +
"type: \(.issue_type // "task")\n" +
"priority: \(.priority // 2)\n" +
(if .assignee and .assignee != "" then "assignee: \(.assignee)\n" else "" end) +
(if .external_ref and .external_ref != "" then "external-ref: \(.external_ref)\n" else "" end) +
(if $parent then "parent: \($parent)\n" else "" end) +
"---\n" +
"# \(.title // "Untitled")\n\n" +
(if .description and .description != "" then "\(.description)\n\n" else "" end) +
(if .design and .design != "" then "## Design\n\n\(.design)\n\n" else "" end) +
(if .acceptance_criteria and .acceptance_criteria != "" then "## Acceptance Criteria\n\n\(.acceptance_criteria)\n\n" else "" end) +
(if .notes and .notes != "" then "## Notes\n\n\(.notes)\n\n" else "" end)
`

var migrateBeadsQuery = mustParseJQ(MigrateBeadsProgram)

func mustParseJQ(src string) *gojq.Query {
	q, err := gojq.Parse(src)
	if err != nil {
		panic("ticket: invalid migrate-beads jq program: " + err.Error())
	}
	return q
}

const (
	beadsMarkerPrefix = "<<<FILE:"
	beadsMarkerSuffix = ">>>\n"
)

// RenderBeadsIssue runs MigrateBeadsProgram against one decoded .beads/
// issues.jsonl line (e.g. from json.Unmarshal into map[string]any) and
// returns the issue's ticket ID plus the exact .tickets/<id>.md content
// bash tk's migrate-beads writes.
//
// bash's cmd_migrate_beads runs this same program through `jq -r` for
// all issues in one call and splits the combined stream on the
// "<<<FILE:...>>>" marker with awk; `jq -r` appends a newline after each
// top-level result, which awk's line-based split folds into the
// preceding ticket's content as one extra trailing blank line — bash tk
// writes that extra newline to every migrated file, including the last.
// RenderBeadsIssue reproduces it explicitly since running the program
// once per issue via gojq's Go API (rather than a shared multi-value
// jq -r stream) doesn't produce it for free.
func RenderBeadsIssue(issue map[string]any) (id, content string, err error) {
	iter := migrateBeadsQuery.Run(issue)
	v, ok := iter.Next()
	if !ok {
		return "", "", fmt.Errorf("ticket: migrate-beads: jq program produced no output")
	}
	if e, isErr := v.(error); isErr {
		return "", "", fmt.Errorf("ticket: migrate-beads: %w", e)
	}
	s, ok := v.(string)
	if !ok {
		return "", "", fmt.Errorf("ticket: migrate-beads: jq program produced %T, want string", v)
	}
	if !strings.HasPrefix(s, beadsMarkerPrefix) {
		return "", "", fmt.Errorf("ticket: migrate-beads: output missing %q marker", beadsMarkerPrefix)
	}
	rest := s[len(beadsMarkerPrefix):]
	end := strings.Index(rest, beadsMarkerSuffix)
	if end < 0 {
		return "", "", fmt.Errorf("ticket: migrate-beads: malformed marker")
	}
	return rest[:end], rest[end+len(beadsMarkerSuffix):] + "\n", nil
}
