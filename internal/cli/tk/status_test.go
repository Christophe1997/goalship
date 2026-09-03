package tk

import (
	"path/filepath"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

func TestRunStatus_ValidTransitions(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	for _, status := range []string{"in_progress", "closed", "open"} {
		gotID, err := runStatus(dir, id, status)
		if err != nil {
			t.Fatalf("runStatus(%q): %v", status, err)
		}
		if gotID != id {
			t.Errorf("runStatus resolved id = %q, want %q", gotID, id)
		}
		tk, err := ticket.Load(filepath.Join(dir, id+".md"))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if tk.Status != status {
			t.Errorf("Status = %q, want %q", tk.Status, status)
		}
	}
}

func TestRunStatus_InvalidStatusErrors(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	if _, err := runStatus(dir, id, "bogus"); err == nil {
		t.Fatal("runStatus: want error for invalid status, got nil")
	}

	tk, err := ticket.Load(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tk.Status != "open" {
		t.Errorf("Status = %q, want unchanged open after rejected transition", tk.Status)
	}
}

func TestRunStatus_ResolvesPartialID(t *testing.T) {
	dir := t.TempDir()
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}
	partial := id[len(id)-4:]

	gotID, err := runStatus(dir, partial, "closed")
	if err != nil {
		t.Fatalf("runStatus: %v", err)
	}
	if gotID != id {
		t.Errorf("resolved id = %q, want %q", gotID, id)
	}
}

func TestRunStatus_UnknownIDErrors(t *testing.T) {
	dir := t.TempDir()
	if _, err := runStatus(dir, "no-such-ticket", "closed"); err == nil {
		t.Fatal("runStatus: want error for unknown id, got nil")
	}
}

func TestStartCloseReopenCmds_DriveStatus(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	startCmd := NewStartCmd()
	startCmd.SetArgs([]string{id})
	if err := startCmd.Execute(); err != nil {
		t.Fatalf("start Execute: %v", err)
	}
	assertStatus(t, dir, id, "in_progress")

	closeCmd := NewCloseCmd()
	closeCmd.SetArgs([]string{id})
	if err := closeCmd.Execute(); err != nil {
		t.Fatalf("close Execute: %v", err)
	}
	assertStatus(t, dir, id, "closed")

	reopenCmd := NewReopenCmd()
	reopenCmd.SetArgs([]string{id})
	if err := reopenCmd.Execute(); err != nil {
		t.Fatalf("reopen Execute: %v", err)
	}
	assertStatus(t, dir, id, "open")
}

func assertStatus(t *testing.T, dir, id, want string) {
	t.Helper()
	tk, err := ticket.Load(filepath.Join(dir, id+".md"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if tk.Status != want {
		t.Errorf("Status = %q, want %q", tk.Status, want)
	}
}
