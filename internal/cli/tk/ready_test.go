package tk

import (
	"bytes"
	"testing"
)

func TestNewReadyCmd_NoDepsIsReady(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "Ready one", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := id + " [P2][open] - Ready one\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

func TestNewReadyCmd_AllDepsClosedIsReady(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	depID, err := runCreate(dir, createOptions{title: "Dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if _, err := runStatus(dir, depID, "closed"); err != nil {
		t.Fatal(err)
	}
	id, err := runCreate(dir, createOptions{title: "Main", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	setDeps(t, dir, id, depID)

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(id)) {
		t.Errorf("output missing ready ticket with closed dep:\n%s", out.String())
	}
}

func TestNewReadyCmd_OpenDepIsNotReady(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	depID, err := runCreate(dir, createOptions{title: "Dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	id, err := runCreate(dir, createOptions{title: "Main", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	setDeps(t, dir, id, depID)

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte(id)) {
		t.Errorf("output contains ticket blocked by an open dep:\n%s", out.String())
	}
}

// TestNewReadyCmd_DanglingDepSilentlyExcludesTicket covers acceptance
// criterion 4: bash tk's cmd_ready compares statuses[dep] (an awk array)
// against "closed" — a dangling dep id that matches no ticket yields an
// uninitialized (empty-string) array element, which is never "closed", so
// the ticket is silently excluded from ready output (not an error, not
// listed as blocked by this command — `blocked` is a separate listing).
func TestNewReadyCmd_DanglingDepSilentlyExcludesTicket(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "Has dangling dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	setDeps(t, dir, id, "no-such-ticket-id")

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want empty (ticket with dangling dep silently excluded)", out.String())
	}
}

func TestNewReadyCmd_ClosedTicketNeverReady(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "Closed", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if _, err := runStatus(dir, id, "closed"); err != nil {
		t.Fatal(err)
	}

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want empty (closed tickets are never ready)", out.String())
	}
}

func TestNewReadyCmd_SortedByPriorityThenID(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	idHigh, err := runCreate(dir, createOptions{title: "P3", ticketType: "task", priority: 3})
	if err != nil {
		t.Fatal(err)
	}
	idLow, err := runCreate(dir, createOptions{title: "P0", ticketType: "task", priority: 0})
	if err != nil {
		t.Fatal(err)
	}

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	lowIdx := bytes.Index(out.Bytes(), []byte(idLow))
	highIdx := bytes.Index(out.Bytes(), []byte(idHigh))
	if lowIdx < 0 || highIdx < 0 || lowIdx > highIdx {
		t.Errorf("expected priority 0 ticket before priority 3 ticket, got:\n%s", out.String())
	}
}

func TestNewReadyCmd_MissingLinksTicketStillReady(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	writeRawTicket(t, dir, "goa-mal3.md",
		"---\nid: goa-mal3\nstatus: open\ndeps: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 2\n---\n# Ready despite missing links\n")

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := "goa-mal3 [P2][open] - Ready despite missing links\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// TestNewReadyCmd_MalformedClosedDepDoesNotWronglyBlockDependent is
// goa-ioe4's actual motivating danger, not just "the malformed ticket
// itself is visible": before ParseTolerant, a malformed-but-genuinely-
// closed dependency vanished from loadTicketInfos entirely, so
// statusByID[dep] read back "" — never "closed" — and a perfectly
// well-formed DEPENDENT ticket silently disappeared from `ready`,
// indistinguishable from one truly blocked by an open dependency.
func TestNewReadyCmd_MalformedClosedDepDoesNotWronglyBlockDependent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	writeRawTicket(t, dir, "goa-dep1.md",
		"---\nid: goa-dep1\nstatus: closed\ndeps: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 2\n---\n# Malformed but closed dep\n")
	id, err := runCreate(dir, createOptions{title: "Depends on malformed-but-closed", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	setDeps(t, dir, id, "goa-dep1")

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(id)) {
		t.Errorf("dependent should be ready (its only dep is closed, just missing links), got:\n%s", out.String())
	}
}

func TestNewReadyCmd_AssigneeAndTagFilters(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	aliceID, err := runCreate(dir, createOptions{title: "A", ticketType: "task", priority: 2, assignee: "alice", tags: "x"})
	if err != nil {
		t.Fatal(err)
	}
	bobID, err := runCreate(dir, createOptions{title: "B", ticketType: "task", priority: 2, assignee: "bob", tags: "y"})
	if err != nil {
		t.Fatal(err)
	}

	cmd := NewReadyCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-a", "alice"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(aliceID)) || bytes.Contains(out.Bytes(), []byte(bobID)) {
		t.Errorf("assignee filter wrong, got:\n%s", out.String())
	}

	cmd2 := NewReadyCmd()
	var out2 bytes.Buffer
	cmd2.SetOut(&out2)
	cmd2.SetArgs([]string{"-T", "y"})
	if err := cmd2.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(out2.Bytes(), []byte(bobID)) || bytes.Contains(out2.Bytes(), []byte(aliceID)) {
		t.Errorf("tag filter wrong, got:\n%s", out2.String())
	}
}
