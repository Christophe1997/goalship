package tk

import (
	"bytes"
	"testing"
)

func TestNewBlockedCmd_NoDepsNotBlocked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	if _, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2}); err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	cmd := NewBlockedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want empty", out.String())
	}
}

func TestNewBlockedCmd_OpenDepIsBlockedWithFormattedBlockerList(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	depID, err := runCreate(dir, createOptions{title: "Dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	id, err := runCreate(dir, createOptions{title: "Main", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	setDeps(t, dir, id, depID)

	cmd := NewBlockedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := id + " [P2][open] - Main <- [" + depID + "]\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

func TestNewBlockedCmd_ClosedDepNotBlocked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	depID, err := runCreate(dir, createOptions{title: "Dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, depID, "closed"); err != nil {
		t.Fatal(err)
	}
	id, err := runCreate(dir, createOptions{title: "Main", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	setDeps(t, dir, id, depID)

	cmd := NewBlockedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("output = %q, want empty (dep is closed)", out.String())
	}
}

// TestNewBlockedCmd_DanglingDepCountsAsBlocker documents the asymmetry
// with `ready`'s exclusion: cmd_blocked's awk only ever compares
// statuses[dep] against "closed" (never checks the dep id actually
// exists), so a dangling dep makes the ticket blocked and appears
// verbatim — with empty status/title — in its blocker list. Unlike
// `ready`, this is not a silent exclusion.
func TestNewBlockedCmd_DanglingDepCountsAsBlocker(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "Main", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	setDeps(t, dir, id, "no-such-ticket-id")

	cmd := NewBlockedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := id + " [P2][open] - Main <- [no-such-ticket-id]\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

// TestNewBlockedCmd_NonBracketDepsStillAppearsBlocked covers the other
// half of goa-ioe4's acceptance criteria: a deps value that isn't
// "[a, b]" bracket syntax no longer hard-errors the whole ticket out of
// existence — it's parsed leniently (matching bash tk's awk reader, which
// never required brackets in the first place) and the ticket still shows
// up as blocked, with a warning on stderr.
func TestNewBlockedCmd_NonBracketDepsStillAppearsBlocked(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	depID, err := runCreate(dir, createOptions{title: "Dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	writeRawTicket(t, dir, "goa-mal2.md",
		"---\nid: goa-mal2\nstatus: open\ndeps: "+depID+"\nlinks: []\ncreated: 2026-01-01T00:00:00Z\ntype: task\npriority: 2\n---\n# Non-bracket deps\n")

	cmd := NewBlockedCmd()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := "goa-mal2 [P2][open] - Non-bracket deps <- [" + depID + "]\n"
	if !bytes.Contains(out.Bytes(), []byte(want)) {
		t.Errorf("output missing blocked ticket with non-bracket deps, got:\n%s", out.String())
	}
	if !bytes.Contains(errOut.Bytes(), []byte("goa-mal2.md")) {
		t.Errorf("stderr should warn naming the file with the non-bracket deps value, got:\n%s", errOut.String())
	}
}

func TestNewBlockedCmd_OnlyListsUnresolvedBlockers(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	closedDep, err := runCreate(dir, createOptions{title: "Closed dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runStatus(dir, closedDep, "closed"); err != nil {
		t.Fatal(err)
	}
	openDep, err := runCreate(dir, createOptions{title: "Open dep", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	id, err := runCreate(dir, createOptions{title: "Main", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	setDeps(t, dir, id, closedDep, openDep)

	cmd := NewBlockedCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := id + " [P2][open] - Main <- [" + openDep + "]\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q (only the still-open dep listed)", out.String(), want)
	}
}
