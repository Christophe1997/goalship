package tk

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewClosedCmd_ListsClosedAndDone(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	closedID, err := runCreate(dir, createOptions{title: "Closed", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, closedID, "closed"); err != nil {
		t.Fatal(err)
	}
	doneID, err := runCreate(dir, createOptions{title: "Done", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	setStatusDirect(t, dir, doneID, "done")
	openID, err := runCreate(dir, createOptions{title: "Open", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}

	cmd := NewClosedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	got := out.String()
	if !bytes.Contains([]byte(got), []byte(closedID)) {
		t.Errorf("output missing closed ticket:\n%s", got)
	}
	if !bytes.Contains([]byte(got), []byte(doneID)) {
		t.Errorf("output missing done-status ticket:\n%s", got)
	}
	if bytes.Contains([]byte(got), []byte(openID)) {
		t.Errorf("output contains open ticket:\n%s", got)
	}
}

func TestNewClosedCmd_MostRecentlyModifiedFirst(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	oldID, err := runCreate(dir, createOptions{title: "Old", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, oldID, "closed"); err != nil {
		t.Fatal(err)
	}
	touchOlder(t, dir, oldID, 2*time.Hour)

	newID, err := runCreate(dir, createOptions{title: "New", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, newID, "closed"); err != nil {
		t.Fatal(err)
	}

	cmd := NewClosedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	newIdx := bytes.Index(out.Bytes(), []byte(newID))
	oldIdx := bytes.Index(out.Bytes(), []byte(oldID))
	if newIdx < 0 || oldIdx < 0 || newIdx > oldIdx {
		t.Errorf("want most-recently-modified (new) first, got:\n%s", out.String())
	}
}

func TestNewClosedCmd_LimitCapsOutput(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	for i := 0; i < 5; i++ {
		id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runStatus(dir, id, "closed"); err != nil {
			t.Fatal(err)
		}
	}

	cmd := NewClosedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--limit=2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	lines := bytes.Count(out.Bytes(), []byte("\n"))
	if lines != 2 {
		t.Errorf("line count = %d, want 2 (--limit=2)", lines)
	}
}

func TestNewClosedCmd_DefaultLimitTwenty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	for i := 0; i < 25; i++ {
		id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runStatus(dir, id, "closed"); err != nil {
			t.Fatal(err)
		}
	}

	cmd := NewClosedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	lines := bytes.Count(out.Bytes(), []byte("\n"))
	if lines != 20 {
		t.Errorf("line count = %d, want default limit 20", lines)
	}
}

func TestNewClosedCmd_MissingLinksClosedTicketStillAppears(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	writeRawTicket(t, dir, "goa-mal4.md",
		"---\nid: goa-mal4\nstatus: closed\ndeps: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 2\n---\n# Closed despite missing links\n")

	cmd := NewClosedCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("goa-mal4")) {
		t.Errorf("output missing closed ticket with missing links field, got:\n%s", out.String())
	}
	if !bytes.Contains(errOut.Bytes(), []byte("goa-mal4.md")) {
		t.Errorf("stderr should warn naming the file, got:\n%s", errOut.String())
	}
}

func TestNewClosedCmd_AssigneeAndTagFilters(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	aliceID, err := runCreate(dir, createOptions{title: "A", ticketType: "task", priority: 2, assignee: "alice", tags: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, aliceID, "closed"); err != nil {
		t.Fatal(err)
	}
	bobID, err := runCreate(dir, createOptions{title: "B", ticketType: "task", priority: 2, assignee: "bob", tags: "y"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, bobID, "closed"); err != nil {
		t.Fatal(err)
	}

	cmd := NewClosedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-a", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(aliceID)) || bytes.Contains(out.Bytes(), []byte(bobID)) {
		t.Errorf("assignee filter wrong, got:\n%s", out.String())
	}
}

func setStatusDirect(t *testing.T, dir, id, status string) {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	tk := loadForTest(t, path)
	tk.Status = status
	if err := tk.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
}

func touchOlder(t *testing.T, dir, id string, age time.Duration) {
	t.Helper()
	path := filepath.Join(dir, id+".md")
	older := time.Now().Add(-age)
	if err := os.Chtimes(path, older, older); err != nil {
		t.Fatalf("Chtimes: %v", err)
	}
}
