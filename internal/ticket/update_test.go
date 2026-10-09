package ticket

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeUpdateFixture(t *testing.T, dir, id string) string {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	tk := &Ticket{ID: id, Status: "open", Created: "2026-09-28T00:00:00Z", Type: "task", Priority: 2, Body: "\n# " + id + "\n"}
	if err := tk.Save(path); err != nil {
		t.Fatalf("save fixture %s: %v", id, err)
	}
	return path
}

func TestUpdate_PartialID_AppliesMutationAndReturnsResolvedID(t *testing.T) {
	dir := t.TempDir()
	path := writeUpdateFixture(t, dir, "gs-20260928-0000-abcd")

	gotID, err := Update(dir, "abcd", func(tk *Ticket) { tk.Status = "closed" })
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if gotID != "gs-20260928-0000-abcd" {
		t.Errorf("resolved id = %q, want the full id", gotID)
	}

	saved, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved.Status != "closed" {
		t.Errorf("Status = %q, want closed", saved.Status)
	}
}

func TestUpdate_NoteAndStatusInOneCall_BothPersist(t *testing.T) {
	dir := t.TempDir()
	path := writeUpdateFixture(t, dir, "gs-20260928-0000-abcd")

	_, err := Update(dir, "gs-20260928-0000-abcd", func(tk *Ticket) {
		tk.AddNote("branch: feat/x")
		tk.Status = "closed"
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}

	saved, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if saved.Status != "closed" || !strings.Contains(saved.Body, "branch: feat/x") {
		t.Errorf("want status closed and the note saved together; status=%q body=%q", saved.Status, saved.Body)
	}
}

func TestUpdate_AmbiguousID_ReturnsErrAmbiguousAndLeavesFilesUnchanged(t *testing.T) {
	dir := t.TempDir()
	first := writeUpdateFixture(t, dir, "gs-20260928-0000-aaaa")
	second := writeUpdateFixture(t, dir, "gs-20260928-0000-aabb")
	firstBefore, secondBefore := readFile(t, first), readFile(t, second)

	called := false
	_, err := Update(dir, "gs-20260928-0000-aa", func(*Ticket) { called = true })
	if !errors.Is(err, ErrAmbiguous) {
		t.Fatalf("err = %v, want ErrAmbiguous", err)
	}
	if called {
		t.Error("mutate was called for an ambiguous id")
	}
	if !bytes.Equal(readFile(t, first), firstBefore) || !bytes.Equal(readFile(t, second), secondBefore) {
		t.Error("an ambiguous update changed a ticket file")
	}
}

func TestUpdate_UnknownID_ReturnsErrNotFound(t *testing.T) {
	dir := t.TempDir()
	writeUpdateFixture(t, dir, "gs-20260928-0000-abcd")

	_, err := Update(dir, "no-such-ticket", func(*Ticket) { t.Error("mutate was called for an unknown id") })
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestUpdate_MalformedFrontmatter_ReturnsErrorAndLeavesFileBytesIdentical(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "gs-20260928-0000-bad1.md")
	if err := os.WriteFile(path, []byte("# no frontmatter at all\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readFile(t, path)

	_, err := Update(dir, "gs-20260928-0000-bad1", func(*Ticket) { t.Error("mutate was called on an unparseable ticket") })
	if err == nil {
		t.Fatal("Update: want an error for malformed frontmatter, got nil")
	}
	if !bytes.Equal(readFile(t, path), before) {
		t.Error("a failed update changed the file")
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}
