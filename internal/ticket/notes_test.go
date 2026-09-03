package ticket

import (
	"regexp"
	"strings"
	"testing"
)

func TestAppendNote_InsertsHeadingWhenAbsent(t *testing.T) {
	got := AppendNote("", "hello world")
	if !regexp.MustCompile(`^\n## Notes\n\n\*\*[^*]+\*\*\n\nhello world\n$`).MatchString(got) {
		t.Fatalf("unexpected note block, got: %q", got)
	}
}

func TestAppendNote_SecondCallDoesNotDuplicateHeading(t *testing.T) {
	body := AppendNote("", "first")
	body = AppendNote(body, "second")

	if n := strings.Count(body, "## Notes"); n != 1 {
		t.Fatalf("## Notes heading count = %d, want 1; content:\n%s", n, body)
	}
	if !strings.Contains(body, "first") || !strings.Contains(body, "second") {
		t.Fatalf("both notes not present:\n%s", body)
	}
}

func TestAppendNote_PreservesExistingBodyBeforeHeading(t *testing.T) {
	got := AppendNote("# Ticket\n\nSome description.\n", "a note")
	if !strings.HasPrefix(got, "# Ticket\n\nSome description.\n\n## Notes\n") {
		t.Fatalf("existing body not preserved before the heading, got:\n%s", got)
	}
}
