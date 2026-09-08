package ticket

import (
	"regexp"
	"strings"
	"testing"
)

func TestTicket_AddNote_AppendsNotesHeadingWhenMissing(t *testing.T) {
	tk := &Ticket{Body: "\n"}
	tk.AddNote("hello world")

	if !regexp.MustCompile(`\n## Notes\n\n\*\*[^*]+\*\*\n\nhello world\n$`).MatchString(tk.Body) {
		t.Fatalf("unexpected body: %q", tk.Body)
	}
}

func TestTicket_AddNote_SecondNoteDoesNotDuplicateHeading(t *testing.T) {
	tk := &Ticket{}
	tk.AddNote("first")
	tk.AddNote("second")

	if n := strings.Count(tk.Body, "## Notes"); n != 1 {
		t.Fatalf("## Notes heading count = %d, want 1; body:\n%s", n, tk.Body)
	}
	if !strings.Contains(tk.Body, "first") || !strings.Contains(tk.Body, "second") {
		t.Fatalf("both notes not present:\n%s", tk.Body)
	}
}

func TestTicket_AddNote_MultilineKeyValueTextRoundTrips(t *testing.T) {
	tk := &Ticket{}
	tk.AddNote("branch: feat/x\npr: https://example.com/pull/1\nsha: deadbeef")

	if !strings.Contains(tk.Body, "branch: feat/x\npr: https://example.com/pull/1\nsha: deadbeef\n") {
		t.Errorf("note text not found verbatim in body:\n%s", tk.Body)
	}
}
