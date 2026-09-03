package tk

import (
	"bytes"
	"testing"
)

func TestNewLsCmd_ListsAllByDefaultInFilenameOrder(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	idA, _ := runCreate(dir, createOptions{title: "Alpha", ticketType: "task", priority: 2})
	idB, _ := runCreate(dir, createOptions{title: "Beta", ticketType: "task", priority: 2})

	cmd := NewLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	got := out.String()
	if !bytes.Contains([]byte(got), []byte(idA+" [open] - Alpha")) {
		t.Errorf("output missing ticket A line, got:\n%s", got)
	}
	if !bytes.Contains([]byte(got), []byte(idB+" [open] - Beta")) {
		t.Errorf("output missing ticket B line, got:\n%s", got)
	}
}

func TestNewLsCmd_DepsSuffixFormat(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	depID, _ := runCreate(dir, createOptions{title: "Dep", ticketType: "task", priority: 2})
	id, err := runCreate(dir, createOptions{title: "Main", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	if _, err := runStatus(dir, id, "open"); err != nil {
		t.Fatal(err)
	}
	addDep(t, dir, id, depID)

	cmd := NewLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	want := id + " [open] - Main <- [" + depID + "]\n"
	if !bytes.Contains(out.Bytes(), []byte(want)) {
		t.Errorf("output = %q, want a line containing %q", out.String(), want)
	}
}

func TestNewLsCmd_StatusFilter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	openID, _ := runCreate(dir, createOptions{title: "Open one", ticketType: "task", priority: 2})
	closedID, _ := runCreate(dir, createOptions{title: "Closed one", ticketType: "task", priority: 2})
	if _, err := runStatus(dir, closedID, "closed"); err != nil {
		t.Fatal(err)
	}

	cmd := NewLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--status=closed"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if bytes.Contains(out.Bytes(), []byte(openID)) {
		t.Errorf("output contains open ticket %q despite --status=closed:\n%s", openID, out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte(closedID)) {
		t.Errorf("output missing closed ticket %q:\n%s", closedID, out.String())
	}
}

func TestNewLsCmd_TagFilterUsesCapitalTFlag(t *testing.T) {
	// bash tk's cmd_ls has no type filter at all; -T/--tag is a tag
	// filter (confirmed by reading cmd_ls's arg parsing directly).
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	uiID, err := runCreate(dir, createOptions{title: "UI ticket", ticketType: "task", priority: 2, tags: "ui,frontend"})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	otherID, err := runCreate(dir, createOptions{title: "Other ticket", ticketType: "task", priority: 2, tags: "backend"})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	cmd := NewLsCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"-T", "ui"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte(uiID)) {
		t.Errorf("output missing tagged ticket %q:\n%s", uiID, out.String())
	}
	if bytes.Contains(out.Bytes(), []byte(otherID)) {
		t.Errorf("output contains untagged ticket %q:\n%s", otherID, out.String())
	}
}

func TestNewLsCmd_AssigneeFilter(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	aliceID, err := runCreate(dir, createOptions{title: "A", ticketType: "task", priority: 2, assignee: "alice"})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	bobID, err := runCreate(dir, createOptions{title: "B", ticketType: "task", priority: 2, assignee: "bob"})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	cmd := NewLsCmd()
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

func TestNewLsCmd_AliasList(t *testing.T) {
	cmd := NewLsCmd()
	found := false
	for _, a := range cmd.Aliases {
		if a == "list" {
			found = true
		}
	}
	if !found {
		t.Errorf("Aliases = %v, want to include %q", cmd.Aliases, "list")
	}
}

func addDep(t *testing.T, dir, id, depID string) {
	t.Helper()
	setDeps(t, dir, id, depID)
}
