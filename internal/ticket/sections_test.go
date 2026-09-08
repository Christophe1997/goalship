package ticket

import (
	"strings"
	"testing"
)

// exampleSectionsBody mirrors buildCreateBody's own layout (create.go),
// with an extra "## Notes" section after "## Acceptance Criteria" — the
// edited section in most tests below sits in the middle, not last, so a
// test can prove bytes *after* it survive untouched too, not just before.
const exampleSectionsBody = "# Sample Title\n\n" +
	"Sample description.\n\n" +
	"## Design\n\nSome design notes.\n\n" +
	"## Acceptance Criteria\n\n- criterion one\n- criterion two\n\n" +
	"## Notes\n\nSome notes.\n\n"

func TestParseSections_TitleDescriptionAndSections(t *testing.T) {
	title, description, sections, order := ParseSections(exampleSectionsBody)

	if title != "Sample Title" {
		t.Errorf("title = %q, want %q", title, "Sample Title")
	}
	if description != "Sample description." {
		t.Errorf("description = %q, want %q", description, "Sample description.")
	}

	wantOrder := []string{"Design", "Acceptance Criteria", "Notes"}
	if len(order) != len(wantOrder) {
		t.Fatalf("order = %v, want %v", order, wantOrder)
	}
	for i, name := range wantOrder {
		if order[i] != name {
			t.Errorf("order[%d] = %q, want %q", i, order[i], name)
		}
	}

	wantSections := map[string]string{
		"Design":              "Some design notes.",
		"Acceptance Criteria": "- criterion one\n- criterion two",
		"Notes":               "Some notes.",
	}
	for name, want := range wantSections {
		if got := sections[name]; got != want {
			t.Errorf("sections[%q] = %q, want %q", name, got, want)
		}
	}
}

func TestParseSections_NoTitle_DescriptionStartsAtByteZero(t *testing.T) {
	body := "Just prose, no heading.\n\n## Notes\n\nSome notes.\n\n"
	title, description, sections, order := ParseSections(body)

	if title != "" {
		t.Errorf("title = %q, want empty", title)
	}
	if description != "Just prose, no heading." {
		t.Errorf("description = %q, want %q", description, "Just prose, no heading.")
	}
	if len(order) != 1 || order[0] != "Notes" {
		t.Errorf("order = %v, want [Notes]", order)
	}
	if sections["Notes"] != "Some notes." {
		t.Errorf("sections[Notes] = %q, want %q", sections["Notes"], "Some notes.")
	}
}

// TestSetTitle_OnlyTitleLineChanges proves the splice touches nothing but
// the title line: every byte from the description onward, including every
// "## " section, is untouched.
func TestSetTitle_OnlyTitleLineChanges(t *testing.T) {
	got := SetTitle(exampleSectionsBody, "New Title")

	wantSuffix := exampleSectionsBody[len("# Sample Title\n"):]
	gotSuffix := got[len("# New Title\n"):]
	if gotSuffix != wantSuffix {
		t.Errorf("bytes after the title line changed:\ngot:  %q\nwant: %q", gotSuffix, wantSuffix)
	}

	title, _, _, _ := ParseSections(got)
	if title != "New Title" {
		t.Errorf("title = %q, want %q", title, "New Title")
	}
}

// TestSetSection_EditsOnlyThatSection proves editing "Acceptance Criteria"
// (positioned between "Design" and "Notes" in the fixture) leaves the
// title/description block AND the trailing "## Notes" section — bytes both
// before and after the edited range — byte-identical.
func TestSetSection_EditsOnlyThatSection(t *testing.T) {
	got, err := SetSection(exampleSectionsBody, "Acceptance Criteria", "- new criterion")
	if err != nil {
		t.Fatalf("SetSection: %v", err)
	}

	acStart := indexOf(t, exampleSectionsBody, "## Acceptance Criteria")
	notesStart := indexOf(t, exampleSectionsBody, "## Notes")

	wantPrefix := exampleSectionsBody[:acStart]
	if got[:len(wantPrefix)] != wantPrefix {
		t.Errorf("bytes before the edited section changed:\ngot:  %q\nwant: %q", got[:len(wantPrefix)], wantPrefix)
	}

	wantSuffix := exampleSectionsBody[notesStart:]
	gotSuffix := got[len(got)-len(wantSuffix):]
	if gotSuffix != wantSuffix {
		t.Errorf("bytes after the edited section (## Notes onward) changed:\ngot:  %q\nwant: %q", gotSuffix, wantSuffix)
	}

	title, description, sections, _ := ParseSections(got)
	if title != "Sample Title" || description != "Sample description." {
		t.Errorf("title/description changed: title=%q description=%q", title, description)
	}
	if sections["Acceptance Criteria"] != "- new criterion" {
		t.Errorf("sections[Acceptance Criteria] = %q, want %q", sections["Acceptance Criteria"], "- new criterion")
	}
}

// TestSetSection_AppendsWhenAbsent proves a section with no existing "## "
// heading is appended after every existing section, with existing sections
// left byte-unchanged.
func TestSetSection_AppendsWhenAbsent(t *testing.T) {
	got, err := SetSection(exampleSectionsBody, "Blockers", "None yet.")
	if err != nil {
		t.Fatalf("SetSection: %v", err)
	}

	if got[:len(exampleSectionsBody)] != exampleSectionsBody {
		t.Errorf("existing bytes changed on append:\ngot:  %q\nwant prefix: %q", got[:len(exampleSectionsBody)], exampleSectionsBody)
	}

	_, _, sections, order := ParseSections(got)
	wantOrder := []string{"Design", "Acceptance Criteria", "Notes", "Blockers"}
	if len(order) != len(wantOrder) {
		t.Fatalf("order = %v, want %v", order, wantOrder)
	}
	for i, name := range wantOrder {
		if order[i] != name {
			t.Errorf("order[%d] = %q, want %q", i, order[i], name)
		}
	}
	if sections["Blockers"] != "None yet." {
		t.Errorf("sections[Blockers] = %q, want %q", sections["Blockers"], "None yet.")
	}
}

// TestRoundTrip_SameValues_ByteIdentical is sections.go's required test:
// parsing a body then writing back the exact same title/description/
// acceptance-criteria values must reproduce byte-identical output — the
// cheapest test that catches whitespace drift in a section (here, "##
// Notes") the caller never touched.
func TestRoundTrip_SameValues_ByteIdentical(t *testing.T) {
	title, description, sections, _ := ParseSections(exampleSectionsBody)

	got := SetTitle(exampleSectionsBody, title)
	got, err := SetDescription(got, description)
	if err != nil {
		t.Fatalf("SetDescription: %v", err)
	}
	got, err = SetSection(got, "Acceptance Criteria", sections["Acceptance Criteria"])
	if err != nil {
		t.Fatalf("SetSection: %v", err)
	}

	if got != exampleSectionsBody {
		t.Errorf("round-trip with unchanged values produced different bytes:\ngot:  %q\nwant: %q", got, exampleSectionsBody)
	}
}

func TestSetDescription_EmptyBecomesSingleBlankLine(t *testing.T) {
	body := "# Title\n\n## Notes\n\ncontent\n\n"
	got, err := SetDescription(body, "")
	if err != nil {
		t.Fatalf("SetDescription: %v", err)
	}
	if got != body {
		t.Errorf("SetDescription with the already-empty value changed bytes:\ngot:  %q\nwant: %q", got, body)
	}
}

// TestSetSection_RejectsContentContainingSectionHeadingLine proves a
// section-heading-shaped line inside content is refused up front instead of
// being silently swallowed as a bogus new section on the next read — see
// sections.go's SetSection doc comment for why an unchecked write would
// corrupt the very next ParseSections call.
func TestSetSection_RejectsContentContainingSectionHeadingLine(t *testing.T) {
	_, err := SetSection(exampleSectionsBody, "Acceptance Criteria", "- criterion one\n## Done\n- criterion two")
	if err == nil {
		t.Fatal("SetSection: want an error for content containing a \"## \" line, got nil")
	}
}

// TestSetDescription_RejectsContentContainingSectionHeadingLine mirrors
// TestSetSection_RejectsContentContainingSectionHeadingLine for
// descriptionSpan's identical "## " boundary.
func TestSetDescription_RejectsContentContainingSectionHeadingLine(t *testing.T) {
	_, err := SetDescription(exampleSectionsBody, "intro\n## Sneaky\nmore text")
	if err == nil {
		t.Fatal("SetDescription: want an error for a description containing a \"## \" line, got nil")
	}
}

// TestParseSections_DuplicateHeaders_FirstOccurrenceWins proves a
// duplicated "## " heading (only reachable via a hand-edited file —
// SetSection never produces one, see TestSetSection_EditsOnlyThatSection)
// reports the first occurrence's content, matching SetSection's own
// first-occurrence edit target: otherwise a successful SetSection edit of
// the first occurrence would stay invisible to every subsequent read.
func TestParseSections_DuplicateHeaders_FirstOccurrenceWins(t *testing.T) {
	body := "# Title\n\n## Notes\n\nfirst.\n\n## Notes\n\nsecond.\n\n"

	_, _, sections, order := ParseSections(body)
	if got := sections["Notes"]; got != "first." {
		t.Errorf("sections[Notes] = %q, want %q (first occurrence)", got, "first.")
	}
	if len(order) != 1 || order[0] != "Notes" {
		t.Errorf("order = %v, want [Notes] (once)", order)
	}

	edited, err := SetSection(body, "Notes", "EDITED.")
	if err != nil {
		t.Fatalf("SetSection: %v", err)
	}
	_, _, sections, _ = ParseSections(edited)
	if got := sections["Notes"]; got != "EDITED." {
		t.Errorf("after SetSection, sections[Notes] = %q, want %q — edit of the first occurrence must be visible", got, "EDITED.")
	}
}

func indexOf(t *testing.T, s, substr string) int {
	t.Helper()
	i := strings.Index(s, substr)
	if i < 0 {
		t.Fatalf("fixture missing expected substring %q", substr)
	}
	return i
}
