package ticket

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

func writeTicketFile(t *testing.T, dir, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte("placeholder"), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

func TestResolve_ExactFilenameMatch(t *testing.T) {
	dir := t.TempDir()
	writeTicketFile(t, dir, "goa-g7ei.md")
	writeTicketFile(t, dir, "goa-2hib.md")

	got, err := Resolve(dir, "goa-g7ei")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(dir, "goa-g7ei.md"); got != want {
		t.Errorf("Resolve = %q, want %q", got, want)
	}
}

func TestResolve_UnambiguousSubstringMatch(t *testing.T) {
	dir := t.TempDir()
	writeTicketFile(t, dir, "goa-g7ei.md")
	writeTicketFile(t, dir, "goa-2hib.md")

	got, err := Resolve(dir, "g7ei")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(dir, "goa-g7ei.md"); got != want {
		t.Errorf("Resolve = %q, want %q", got, want)
	}
}

func TestResolve_AmbiguousSubstringErrors(t *testing.T) {
	dir := t.TempDir()
	writeTicketFile(t, dir, "goa-abcd.md")
	writeTicketFile(t, dir, "goa-abce.md")

	_, err := Resolve(dir, "abc")
	if err == nil {
		t.Fatal("Resolve: want error for ambiguous substring, got nil")
	}
	if !errors.Is(err, ErrAmbiguous) {
		t.Errorf("Resolve: err = %v, want wrapping ErrAmbiguous", err)
	}
}

// TestResolve_TrimsWhitespace covers tk's own ticket_path() behavior
// ("Trim leading/trailing whitespace (handles Claude/agent quirks)"):
// callers of goalship tk are the same agent-driven kind.
func TestResolve_TrimsWhitespace(t *testing.T) {
	dir := t.TempDir()
	writeTicketFile(t, dir, "goa-g7ei.md")

	got, err := Resolve(dir, "  goa-g7ei  \n")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if want := filepath.Join(dir, "goa-g7ei.md"); got != want {
		t.Errorf("Resolve = %q, want %q", got, want)
	}
}

func TestResolve_MissingTicketsDirWrapsErrNotFound(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "does-not-exist")

	_, err := Resolve(missing, "goa-g7ei")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Resolve: err = %v, want wrapping ErrNotFound", err)
	}
}

func TestResolve_NoMatchErrors(t *testing.T) {
	dir := t.TempDir()
	writeTicketFile(t, dir, "goa-abcd.md")

	_, err := Resolve(dir, "zzzz")
	if err == nil {
		t.Fatal("Resolve: want error for no match, got nil")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Resolve: err = %v, want wrapping ErrNotFound", err)
	}
}

// TestResolve_OldAndNewShapeIDsCoexist covers AE3: an old-shape ID
// (<prefix>-<4char>) and a new-shape ID (<prefix>-<YYYYMMDD-HHMM>-<4char>)
// living in the same directory both resolve via substring match.
func TestResolve_OldAndNewShapeIDsCoexist(t *testing.T) {
	dir := t.TempDir()
	writeTicketFile(t, dir, "goa-ab12.md")
	writeTicketFile(t, dir, "goa-20260827-1755-05hv.md")

	oldPath, err := Resolve(dir, "ab12")
	if err != nil {
		t.Fatalf("Resolve old-shape: %v", err)
	}
	if want := filepath.Join(dir, "goa-ab12.md"); oldPath != want {
		t.Errorf("Resolve old-shape = %q, want %q", oldPath, want)
	}

	newPath, err := Resolve(dir, "05hv")
	if err != nil {
		t.Fatalf("Resolve new-shape: %v", err)
	}
	if want := filepath.Join(dir, "goa-20260827-1755-05hv.md"); newPath != want {
		t.Errorf("Resolve new-shape = %q, want %q", newPath, want)
	}
}

var idSuffixPattern = regexp.MustCompile(`^[a-z0-9]{4}$`)

func TestGenerateID_ShapeAndPrefix(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "my-repo")
	ticketsDir := filepath.Join(repoDir, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	id, err := GenerateID(ticketsDir)
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}

	re := regexp.MustCompile(`^mr-\d{8}-\d{4}-[a-z0-9]{4}$`)
	if !re.MatchString(id) {
		t.Errorf("GenerateID = %q, want to match %s", id, re.String())
	}
}

// TestGenerateID_SortsAfterExistingDateTimeIDs covers AE3/R4: a new ID
// sorts lexicographically after every existing date-time-prefixed ID.
func TestGenerateID_SortsAfterExistingDateTimeIDs(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "goalship")
	ticketsDir := filepath.Join(repoDir, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTicketFile(t, ticketsDir, "goa-20200101-0000-aaaa.md")
	writeTicketFile(t, ticketsDir, "goa-ab12.md") // old-shape, coexists

	id, err := GenerateID(ticketsDir)
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}

	if id <= "goa-20200101-0000-aaaa" {
		t.Errorf("GenerateID = %q, want to sort after goa-20200101-0000-aaaa", id)
	}
}

func TestRepoPrefix(t *testing.T) {
	tests := []struct {
		dirName string
		want    string
	}{
		{"goalship", "goa"},
		{"agent-extentions", "ae"},
		{"my-repo", "mr"},
		{"my_repo_name", "mrn"},
		{"ae", "ae"},
		{"a", "a"},
	}

	for _, tt := range tests {
		t.Run(tt.dirName, func(t *testing.T) {
			base := t.TempDir()
			repoDir := filepath.Join(base, tt.dirName)
			ticketsDir := filepath.Join(repoDir, ".tickets")
			if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
				t.Fatal(err)
			}

			got, err := repoPrefix(ticketsDir)
			if err != nil {
				t.Fatalf("repoPrefix: %v", err)
			}
			if got != tt.want {
				t.Errorf("repoPrefix(%q) = %q, want %q", tt.dirName, got, tt.want)
			}
		})
	}
}

func TestRandomSuffix_CharsetAndLength(t *testing.T) {
	for i := 0; i < 20; i++ {
		s, err := randomSuffix(4)
		if err != nil {
			t.Fatalf("randomSuffix: %v", err)
		}
		if !idSuffixPattern.MatchString(s) {
			t.Errorf("randomSuffix = %q, want to match %s", s, idSuffixPattern.String())
		}
	}
}

func TestGenerateID_RetriesOnCollision(t *testing.T) {
	base := t.TempDir()
	repoDir := filepath.Join(base, "goalship")
	ticketsDir := filepath.Join(repoDir, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatal(err)
	}

	id, err := GenerateID(ticketsDir)
	if err != nil {
		t.Fatalf("GenerateID: %v", err)
	}
	writeTicketFile(t, ticketsDir, id+".md")

	id2, err := GenerateID(ticketsDir)
	if err != nil {
		t.Fatalf("GenerateID second call: %v", err)
	}
	if id2 == id {
		t.Errorf("GenerateID returned a colliding id twice: %q", id)
	}
}
