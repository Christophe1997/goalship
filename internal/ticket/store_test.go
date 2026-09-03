package ticket

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// realFixtures round-trip real tk-authored files copied verbatim from this
// repo's own .tickets/ (created by the actual bash tk 0.3.2 binary).
var realFixtures = []string{
	"testdata/goa-fxh3.md",
	"testdata/goa-2hib.md",
}

func TestParseBytes_RoundTripsRealFixturesByteIdentical(t *testing.T) {
	for _, path := range realFixtures {
		t.Run(path, func(t *testing.T) {
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}

			tk, err := Parse(want)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}

			got := tk.Bytes()
			if !bytes.Equal(got, want) {
				t.Fatalf("round-trip not byte-identical\n--- want ---\n%s\n--- got ---\n%s", want, got)
			}
		})
	}
}

func TestLoadSave_RoundTripsRealFixtureByteIdentical(t *testing.T) {
	src := "testdata/goa-2hib.md"
	want, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	tk, err := Load(src)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	dst := filepath.Join(t.TempDir(), "goa-2hib.md")
	if err := tk.Save(dst); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read saved file: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Load->Save round-trip not byte-identical\n--- want ---\n%s\n--- got ---\n%s", want, got)
	}
}

func TestParse_PopulatesCoreFields(t *testing.T) {
	data, err := os.ReadFile("testdata/goa-2hib.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	tk, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if tk.ID != "goa-2hib" {
		t.Errorf("ID = %q, want goa-2hib", tk.ID)
	}
	if tk.Status != "open" {
		t.Errorf("Status = %q, want open", tk.Status)
	}
	wantDeps := []string{"goa-g7ei", "goa-5zwn"}
	if !slicesEqual(tk.Deps, wantDeps) {
		t.Errorf("Deps = %v, want %v", tk.Deps, wantDeps)
	}
	if len(tk.Links) != 0 {
		t.Errorf("Links = %v, want empty", tk.Links)
	}
	if tk.Created != "2026-09-03T06:41:47Z" {
		t.Errorf("Created = %q, want 2026-09-03T06:41:47Z", tk.Created)
	}
	if tk.Type != "feature" {
		t.Errorf("Type = %q, want feature", tk.Type)
	}
	if tk.Priority != 2 {
		t.Errorf("Priority = %d, want 2", tk.Priority)
	}
	if !strings.HasPrefix(tk.Body, "# Reconcile: stacked-base PR retargeting\n") {
		t.Errorf("Body does not start with expected title, got: %q", tk.Body[:min(60, len(tk.Body))])
	}
}

func TestParse_OptionalFieldsLandInExtraPreservingOrder(t *testing.T) {
	data, err := os.ReadFile("testdata/goa-2hib.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tk, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(tk.Extra) != 2 {
		t.Fatalf("Extra = %+v, want 2 entries (assignee, external-ref)", tk.Extra)
	}
	if tk.Extra[0].Key != "assignee" || tk.Extra[0].Value != " Christophe1997" {
		t.Errorf("Extra[0] = %+v, want {assignee, \" Christophe1997\"}", tk.Extra[0])
	}
	if tk.Extra[1].Key != "external-ref" || tk.Extra[1].Value != " U6C" {
		t.Errorf("Extra[1] = %+v, want {external-ref, \" U6C\"}", tk.Extra[1])
	}
}

func TestParse_UnrecognizedKeyRoundTripsUnchanged(t *testing.T) {
	data, err := os.ReadFile("testdata/unrecognized-field.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	tk, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	found := false
	for _, f := range tk.Extra {
		if f.Key == "sprint" {
			found = true
			if f.Value != " 42" {
				t.Errorf("sprint value = %q, want \" 42\"", f.Value)
			}
		}
	}
	if !found {
		t.Fatalf("Extra = %+v, want a %q entry", tk.Extra, "sprint")
	}

	if got := tk.Bytes(); !bytes.Equal(got, data) {
		t.Fatalf("round-trip with unrecognized key not byte-identical\n--- want ---\n%s\n--- got ---\n%s", data, got)
	}
}

func TestParse_MissingFrontmatterDelimitersErrors(t *testing.T) {
	_, err := Parse([]byte("# just a markdown file\n\nno frontmatter here\n"))
	if err == nil {
		t.Fatal("Parse: want error for missing frontmatter, got nil")
	}
}

func TestParse_MissingRequiredFieldErrors(t *testing.T) {
	_, err := Parse([]byte("---\nid: goa-abcd\nstatus: open\n---\nbody\n"))
	if err == nil {
		t.Fatal("Parse: want error for missing required fields, got nil")
	}
}

func TestParse_DuplicateKeyErrors(t *testing.T) {
	data := []byte("---\nid: goa-abcd\nid: goa-abcd\nstatus: open\ndeps: []\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 1\n---\nbody\n")
	_, err := Parse(data)
	if err == nil {
		t.Fatal("Parse: want error for duplicate key, got nil")
	}
}

func TestParse_ArrayParsingToleratesNonCanonicalSpacing(t *testing.T) {
	tests := []struct {
		name string
		deps string
		want []string
	}{
		{"no space after comma", "[a,b]", []string{"a", "b"}},
		{"extra spaces", "[ a , b ]", []string{"a", "b"}},
		{"canonical", "[a, b]", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := []byte("---\nid: goa-abcd\nstatus: open\ndeps: " + tt.deps + "\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 1\n---\nbody\n")
			tk, err := Parse(data)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			if !slicesEqual(tk.Deps, tt.want) {
				t.Errorf("Deps = %v, want %v", tk.Deps, tt.want)
			}
		})
	}
}

func TestParse_MalformedArrayErrors(t *testing.T) {
	data := []byte("---\nid: goa-abcd\nstatus: open\ndeps: not-an-array\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 1\n---\nbody\n")
	_, err := Parse(data)
	if err == nil {
		t.Fatal("Parse: want error for malformed deps array, got nil")
	}
}

func TestParseTolerant_MissingLinksDefaultsToEmptyAndWarns(t *testing.T) {
	data := []byte("---\nid: goa-abcd\nstatus: open\ndeps: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 1\n---\n# T\n")
	tk, warnings, err := ParseTolerant(data)
	if err != nil {
		t.Fatalf("ParseTolerant: %v", err)
	}
	if tk.ID != "goa-abcd" || tk.Status != "open" {
		t.Errorf("ID/Status = %q/%q, want goa-abcd/open", tk.ID, tk.Status)
	}
	if len(tk.Links) != 0 {
		t.Errorf("Links = %v, want empty", tk.Links)
	}
	if !containsSubstring(warnings, "links") {
		t.Errorf("warnings = %v, want one mentioning links", warnings)
	}
}

func TestParseTolerant_NonBracketDepsParsedLeniently(t *testing.T) {
	data := []byte("---\nid: goa-abcd\nstatus: open\ndeps: goa-1,goa-2\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 1\n---\nbody\n")
	tk, warnings, err := ParseTolerant(data)
	if err != nil {
		t.Fatalf("ParseTolerant: %v", err)
	}
	want := []string{"goa-1", "goa-2"}
	if !slicesEqual(tk.Deps, want) {
		t.Errorf("Deps = %v, want %v", tk.Deps, want)
	}
	if !containsSubstring(warnings, "deps") {
		t.Errorf("warnings = %v, want one mentioning deps", warnings)
	}
}

func TestParseTolerant_MissingPriorityDefaultsToTwo(t *testing.T) {
	data := []byte("---\nid: goa-abcd\nstatus: open\ndeps: []\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\n---\nbody\n")
	tk, warnings, err := ParseTolerant(data)
	if err != nil {
		t.Fatalf("ParseTolerant: %v", err)
	}
	if tk.Priority != 2 {
		t.Errorf("Priority = %d, want 2 (bash tk's own missing-priority default)", tk.Priority)
	}
	if !containsSubstring(warnings, "priority") {
		t.Errorf("warnings = %v, want one mentioning priority", warnings)
	}
}

func TestParseTolerant_MissingStatusDefaultsToEmptyNotOpen(t *testing.T) {
	// bash tk's awk readers never default a missing status to "open" —
	// an unmatched /^status:/ just leaves the variable "", which is why
	// such a ticket still shows in `ls` but never in ready/blocked/closed
	// (neither check matches ""). ParseTolerant matches that, not the
	// more convenient-looking "open".
	data := []byte("---\nid: goa-abcd\ndeps: []\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 1\n---\nbody\n")
	tk, warnings, err := ParseTolerant(data)
	if err != nil {
		t.Fatalf("ParseTolerant: %v", err)
	}
	if tk.Status != "" {
		t.Errorf("Status = %q, want \"\"", tk.Status)
	}
	if !containsSubstring(warnings, "status") {
		t.Errorf("warnings = %v, want one mentioning status", warnings)
	}
}

func TestParseTolerant_MissingIDErrors(t *testing.T) {
	data := []byte("---\nstatus: open\ndeps: []\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 1\n---\nbody\n")
	_, _, err := ParseTolerant(data)
	if err == nil {
		t.Fatal("ParseTolerant: want error for missing id, got nil")
	}
}

func TestParseTolerant_NoFrontmatterDelimitersErrors(t *testing.T) {
	_, _, err := ParseTolerant([]byte("# just a markdown file\n\nno frontmatter here\n"))
	if err == nil {
		t.Fatal("ParseTolerant: want error for missing frontmatter, got nil")
	}
}

func TestParseTolerant_WellFormedTicketProducesNoWarnings(t *testing.T) {
	data, err := os.ReadFile("testdata/goa-2hib.md")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	tk, warnings, err := ParseTolerant(data)
	if err != nil {
		t.Fatalf("ParseTolerant: %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("warnings = %v, want none for a well-formed ticket", warnings)
	}
	if tk.ID != "goa-2hib" {
		t.Errorf("ID = %q, want goa-2hib", tk.ID)
	}
}

func containsSubstring(items []string, substr string) bool {
	for _, item := range items {
		if strings.Contains(item, substr) {
			return true
		}
	}
	return false
}

func TestBytes_FreshTicketUsesCanonicalFieldOrder(t *testing.T) {
	tk := &Ticket{
		ID:       "goa-fresh",
		Status:   "open",
		Deps:     nil,
		Links:    nil,
		Created:  "2026-09-03T00:00:00Z",
		Type:     "task",
		Priority: 2,
		Body:     "# Fresh ticket\n",
	}

	want := "---\n" +
		"id: goa-fresh\n" +
		"status: open\n" +
		"deps: []\n" +
		"links: []\n" +
		"created: 2026-09-03T00:00:00Z\n" +
		"type: task\n" +
		"priority: 2\n" +
		"---\n" +
		"# Fresh ticket\n"

	if got := string(tk.Bytes()); got != want {
		t.Fatalf("Bytes() =\n%q\nwant\n%q", got, want)
	}
}

func TestErrorsWrapSentinels(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "goa-aaaa.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "goa-aabb.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Resolve(dir, "aa")
	if !errors.Is(err, ErrAmbiguous) {
		t.Errorf("Resolve ambiguous: err = %v, want wrapping ErrAmbiguous", err)
	}

	_, err = Resolve(dir, "zzzz")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Resolve no match: err = %v, want wrapping ErrNotFound", err)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
