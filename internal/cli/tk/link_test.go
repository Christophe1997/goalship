package tk

import (
	"path/filepath"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func TestLink_SymmetricBetweenTwo(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	writeTestTicket(t, dir, "goa-b", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewLinkCmd(), []string{"goa-a", "goa-b"})
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	// count is the total number of new link entries written across all
	// files (bash: `((count += result))` sums each file's own added
	// count) — a pairwise link touches two files, each gaining one new
	// entry, so count is 2, not 1.
	if want := "Added 2 link(s) between 2 tickets\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}

	a, _ := ticket.Load(filepath.Join(dir, "goa-a.md"))
	b, _ := ticket.Load(filepath.Join(dir, "goa-b.md"))
	if len(a.Links) != 1 || a.Links[0] != "goa-b" {
		t.Errorf("a.Links = %v, want [goa-b]", a.Links)
	}
	if len(b.Links) != 1 || b.Links[0] != "goa-a" {
		t.Errorf("b.Links = %v, want [goa-a]", b.Links)
	}
}

func TestLink_ThreeWay(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	writeTestTicket(t, dir, "goa-b", nil, nil)
	writeTestTicket(t, dir, "goa-c", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewLinkCmd(), []string{"goa-a", "goa-b", "goa-c"})
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if want := "Added 6 link(s) between 3 tickets\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}

	for _, id := range []string{"goa-a", "goa-b", "goa-c"} {
		tk, err := ticket.Load(filepath.Join(dir, id+".md"))
		if err != nil {
			t.Fatal(err)
		}
		if len(tk.Links) != 2 {
			t.Errorf("%s.Links = %v, want 2 entries", id, tk.Links)
		}
	}
}

func TestLink_AllAlreadyExist(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, []string{"goa-b"})
	writeTestTicket(t, dir, "goa-b", nil, []string{"goa-a"})
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewLinkCmd(), []string{"goa-a", "goa-b"})
	if err != nil {
		t.Fatalf("link: %v", err)
	}
	if want := "All links already exist\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}
}

func TestLink_NonexistentIDErrors(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	_, err := execCmd(t, NewLinkCmd(), []string{"goa-a", "goa-zzzz"})
	if err == nil || err.Error() != "Error: ticket 'goa-zzzz' not found" {
		t.Errorf("err = %v, want ticket-not-found", err)
	}
}

func TestUnlink_RemovesBothSides(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, []string{"goa-b"})
	writeTestTicket(t, dir, "goa-b", nil, []string{"goa-a"})
	t.Setenv("TICKETS_DIR", dir)

	out, err := execCmd(t, NewUnlinkCmd(), []string{"goa-a", "goa-b"})
	if err != nil {
		t.Fatalf("unlink: %v", err)
	}
	if want := "Removed link: goa-a <-> goa-b\n"; out != want {
		t.Errorf("output = %q, want %q", out, want)
	}

	a, _ := ticket.Load(filepath.Join(dir, "goa-a.md"))
	b, _ := ticket.Load(filepath.Join(dir, "goa-b.md"))
	if len(a.Links) != 0 {
		t.Errorf("a.Links = %v, want empty", a.Links)
	}
	if len(b.Links) != 0 {
		t.Errorf("b.Links = %v, want empty", b.Links)
	}
}

func TestUnlink_NotFound(t *testing.T) {
	dir := t.TempDir()
	writeTestTicket(t, dir, "goa-a", nil, nil)
	writeTestTicket(t, dir, "goa-b", nil, nil)
	t.Setenv("TICKETS_DIR", dir)

	_, err := execCmd(t, NewUnlinkCmd(), []string{"goa-a", "goa-b"})
	if err == nil || err.Error() != "Link not found" {
		t.Errorf("err = %v, want \"Link not found\"", err)
	}
}
