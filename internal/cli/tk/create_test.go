package tk

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func TestRunCreate_PopulatesAllRequiredFrontmatterFields(t *testing.T) {
	dir := t.TempDir()

	id, err := runCreate(dir, createOptions{title: "My Ticket", ticketType: "bug", priority: 1})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	idRe := regexp.MustCompile(`^[A-Za-z0-9]+-\d{8}-\d{4}-[a-z0-9]{4}$`)
	if !idRe.MatchString(id) {
		t.Errorf("id = %q, want new-shape ID", id)
	}

	tk, err := ticket.Load(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tk.ID != id {
		t.Errorf("ID = %q, want %q", tk.ID, id)
	}
	if tk.Status != "open" {
		t.Errorf("Status = %q, want open", tk.Status)
	}
	if tk.Deps != nil {
		t.Errorf("Deps = %v, want nil/empty", tk.Deps)
	}
	if tk.Links != nil {
		t.Errorf("Links = %v, want nil/empty", tk.Links)
	}
	if tk.Created == "" {
		t.Error("Created is empty, want an ISO timestamp")
	}
	if tk.Type != "bug" {
		t.Errorf("Type = %q, want bug", tk.Type)
	}
	if tk.Priority != 1 {
		t.Errorf("Priority = %d, want 1", tk.Priority)
	}
	if got := firstTitle(tk.Body); got != "My Ticket" {
		t.Errorf("title = %q, want %q", got, "My Ticket")
	}
}

func TestRunCreate_DefaultsTitleTypeAndPriority(t *testing.T) {
	dir := t.TempDir()

	id, err := runCreate(dir, createOptions{})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	tk, err := ticket.Load(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := firstTitle(tk.Body); got != "Untitled" {
		t.Errorf("title = %q, want Untitled", got)
	}
}

func TestRunCreate_ExtraFieldsAndBodyLayout(t *testing.T) {
	dir := t.TempDir()

	id, err := runCreate(dir, createOptions{
		title:       "T",
		description: "the desc",
		design:      "the design",
		acceptance:  "the acceptance",
		ticketType:  "task",
		priority:    2,
		assignee:    "alice",
		externalRef: "gh-123",
		tags:        "ui,backend",
	})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	want := "---\n" +
		"id: " + id + "\n" +
		"status: open\n" +
		"deps: []\n" +
		"links: []\n" +
		"created: "
	got, err := os.ReadFile(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.HasPrefix(got, []byte(want)) {
		t.Fatalf("frontmatter prefix mismatch:\ngot:  %q\nwant prefix: %q", got, want)
	}

	wantSuffix := "type: task\n" +
		"priority: 2\n" +
		"assignee: alice\n" +
		"external-ref: gh-123\n" +
		"tags: [ui, backend]\n" +
		"---\n" +
		"# T\n\n" +
		"the desc\n\n" +
		"## Design\n\n" +
		"the design\n\n" +
		"## Acceptance Criteria\n\n" +
		"the acceptance\n\n"
	if !bytes.HasSuffix(got, []byte(wantSuffix)) {
		t.Fatalf("body/tail mismatch:\ngot:  %q\nwant suffix: %q", got, wantSuffix)
	}
}

func TestRunCreate_ParentResolvesToFullIDViaSubstring(t *testing.T) {
	dir := t.TempDir()
	parentID, err := runCreate(dir, createOptions{title: "Parent"})
	if err != nil {
		t.Fatalf("runCreate parent: %v", err)
	}
	partial := parentID[len(parentID)-4:]

	childID, err := runCreate(dir, createOptions{title: "Child", parent: partial})
	if err != nil {
		t.Fatalf("runCreate child: %v", err)
	}

	tk, err := ticket.Load(filepath.Join(dir, childID+".md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := extraValue(tk, "parent"); got != parentID {
		t.Errorf("parent = %q, want %q", got, parentID)
	}
}

func TestRunCreate_InvalidParentErrorsAndCreatesNoFile(t *testing.T) {
	dir := t.TempDir()

	_, err := runCreate(dir, createOptions{title: "Child", parent: "does-not-exist"})
	if err == nil {
		t.Fatal("runCreate: want error for unresolvable parent, got nil")
	}

	entries, rerr := os.ReadDir(dir)
	if rerr != nil {
		t.Fatalf("ReadDir: %v", rerr)
	}
	if len(entries) != 0 {
		t.Errorf("ReadDir = %v, want no files created on parent-resolution failure", entries)
	}
}

func TestRunCreate_CreatesTicketsDirIfMissing(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, ".tickets")

	if _, err := runCreate(dir, createOptions{title: "T"}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("tickets dir not created: %v", err)
	}
}

func TestResolveAssignee(t *testing.T) {
	orig := gitUserName
	defer func() { gitUserName = orig }()
	gitUserName = func() string { return "from-git-config" }

	if got := resolveAssignee(false, "flag-value"); got != "from-git-config" {
		t.Errorf("resolveAssignee(unchanged) = %q, want git config value", got)
	}
	if got := resolveAssignee(true, "flag-value"); got != "flag-value" {
		t.Errorf("resolveAssignee(changed) = %q, want flag value", got)
	}
}

func TestNewCreateCmd_PrintsIDToStdout(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	orig := gitUserName
	defer func() { gitUserName = orig }()
	gitUserName = func() string { return "" }

	cmd := NewCreateCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"My Title", "-d", "desc"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	id := bytesTrimNewline(out.Bytes())
	if _, err := os.Stat(filepath.Join(dir, id+".md")); err != nil {
		t.Fatalf("expected ticket file for printed id %q: %v", id, err)
	}
}

func bytesTrimNewline(b []byte) string {
	s := string(b)
	for len(s) > 0 && (s[len(s)-1] == '\n' || s[len(s)-1] == '\r') {
		s = s[:len(s)-1]
	}
	return s
}
