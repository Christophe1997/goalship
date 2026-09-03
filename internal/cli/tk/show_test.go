package tk

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func runShowForTest(t *testing.T, dir, id string) string {
	t.Helper()
	var out bytes.Buffer
	if err := runShow(dir, id, &out); err != nil {
		t.Fatalf("runShow: %v", err)
	}
	return out.String()
}

func TestRunShow_NoRelationshipsMatchesRawFileExactly(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "Solo", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatal(err)
	}

	got := runShowForTest(t, dir, id)
	if got != string(raw) {
		t.Errorf("show output != raw file.\ngot:  %q\nwant: %q", got, raw)
	}
}

func TestRunShow_ParentLineAnnotatedWithTitleWhenKnown(t *testing.T) {
	dir := t.TempDir()
	parentID, err := runCreate(dir, createOptions{title: "The Parent"})
	if err != nil {
		t.Fatal(err)
	}
	childID, err := runCreate(dir, createOptions{title: "The Child", parent: parentID})
	if err != nil {
		t.Fatal(err)
	}

	got := runShowForTest(t, dir, childID)
	want := "parent: " + parentID + "  # The Parent\n"
	if !bytes.Contains([]byte(got), []byte(want)) {
		t.Errorf("show output missing annotated parent line %q, got:\n%s", want, got)
	}
}

func TestRunShow_DanglingParentLineUnchanged(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	setParentDirect(t, dir, id, "no-such-parent")

	got := runShowForTest(t, dir, id)
	if !bytes.Contains([]byte(got), []byte("parent: no-such-parent\n")) {
		t.Errorf("show output should leave a dangling parent line unannotated, got:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte("no-such-parent  #")) {
		t.Errorf("dangling parent line should not be annotated, got:\n%s", got)
	}
}

func TestRunShow_BlockersSectionListsUnresolvedDepsInOrder(t *testing.T) {
	dir := t.TempDir()
	closedDep, err := runCreate(dir, createOptions{title: "Closed dep"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, closedDep, "closed"); err != nil {
		t.Fatal(err)
	}
	openDep, err := runCreate(dir, createOptions{title: "Open dep"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := runCreate(dir, createOptions{title: "Main"})
	if err != nil {
		t.Fatal(err)
	}
	setDeps(t, dir, id, closedDep, openDep)

	got := runShowForTest(t, dir, id)
	if !bytes.Contains([]byte(got), []byte("\n## Blockers\n\n- "+openDep+" [open] Open dep\n")) {
		t.Errorf("show output missing Blockers section, got:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte(closedDep+" [closed]")) {
		t.Errorf("closed dep should not appear as a blocker, got:\n%s", got)
	}
}

func TestRunShow_BlockingSectionListsOpenDependents(t *testing.T) {
	dir := t.TempDir()
	target, err := runCreate(dir, createOptions{title: "Target"})
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := runCreate(dir, createOptions{title: "Dependent"})
	if err != nil {
		t.Fatal(err)
	}
	setDeps(t, dir, dependent, target)

	got := runShowForTest(t, dir, target)
	if !bytes.Contains([]byte(got), []byte("\n## Blocking\n\n- "+dependent+" [open] Dependent\n")) {
		t.Errorf("show output missing Blocking section, got:\n%s", got)
	}
}

func TestRunShow_BlockingExcludesClosedDependents(t *testing.T) {
	dir := t.TempDir()
	target, err := runCreate(dir, createOptions{title: "Target"})
	if err != nil {
		t.Fatal(err)
	}
	dependent, err := runCreate(dir, createOptions{title: "Dependent"})
	if err != nil {
		t.Fatal(err)
	}
	setDeps(t, dir, dependent, target)
	if _, err := runStatus(dir, dependent, "closed"); err != nil {
		t.Fatal(err)
	}

	got := runShowForTest(t, dir, target)
	if bytes.Contains([]byte(got), []byte("## Blocking")) {
		t.Errorf("closed dependent should not produce a Blocking section, got:\n%s", got)
	}
}

func TestRunShow_ChildrenSectionListsTicketsWithMatchingParent(t *testing.T) {
	dir := t.TempDir()
	parentID, err := runCreate(dir, createOptions{title: "Parent"})
	if err != nil {
		t.Fatal(err)
	}
	childID, err := runCreate(dir, createOptions{title: "Child", parent: parentID})
	if err != nil {
		t.Fatal(err)
	}

	got := runShowForTest(t, dir, parentID)
	if !bytes.Contains([]byte(got), []byte("\n## Children\n\n- "+childID+" [open] Child\n")) {
		t.Errorf("show output missing Children section, got:\n%s", got)
	}
}

func TestRunShow_LinkedSectionListsLinksRegardlessOfStatus(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "Main"})
	if err != nil {
		t.Fatal(err)
	}
	other, err := runCreate(dir, createOptions{title: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, other, "closed"); err != nil {
		t.Fatal(err)
	}
	setLinks(t, dir, id, other)

	got := runShowForTest(t, dir, id)
	if !bytes.Contains([]byte(got), []byte("\n## Linked\n\n- "+other+" [closed] Other\n")) {
		t.Errorf("show output missing Linked section, got:\n%s", got)
	}
}

func TestRunShow_ResolvesPartialID(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T"})
	if err != nil {
		t.Fatal(err)
	}
	got := runShowForTest(t, dir, id[len(id)-4:])
	if !bytes.Contains([]byte(got), []byte("id: "+id+"\n")) {
		t.Errorf("show output for partial id doesn't contain full id line, got:\n%s", got)
	}
}

func TestRunShow_UnknownIDErrors(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	if err := runShow(dir, "no-such-ticket", &out); err == nil {
		t.Fatal("runShow: want error for unknown id, got nil")
	}
}

func setParentDirect(t *testing.T, dir, id, parent string) {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	tk := loadForTest(t, path)
	tk.Extra = append(tk.Extra, ticketFieldForTest("parent", parent))
	if err := tk.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func setLinks(t *testing.T, dir, id string, links ...string) {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	tk := loadForTest(t, path)
	tk.Links = links
	if err := tk.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
}
