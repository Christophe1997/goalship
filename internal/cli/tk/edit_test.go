package tk

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestNewEditCmd_NonInteractivePrintsPathWithoutLaunching(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	launched := false
	origRunEditor := runEditor
	runEditor = func(editor, path string) error { launched = true; return nil }
	defer func() { runEditor = origRunEditor }()

	origTTY := stdinAndStdoutAreTTY
	stdinAndStdoutAreTTY = func() bool { return false }
	defer func() { stdinAndStdoutAreTTY = origTTY }()

	cmd := NewEditCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if launched {
		t.Error("runEditor was called in non-interactive mode, want no launch")
	}
	want := "Edit ticket file: " + filepath.Join(dir, id+".md") + "\n"
	if out.String() != want {
		t.Errorf("stdout = %q, want %q", out.String(), want)
	}
}

func TestNewEditCmd_InteractiveLaunchesEditorOnResolvedPath(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	t.Setenv("EDITOR", "my-editor")
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	var gotEditor, gotPath string
	origRunEditor := runEditor
	runEditor = func(editor, path string) error {
		gotEditor, gotPath = editor, path
		return nil
	}
	defer func() { runEditor = origRunEditor }()

	origTTY := stdinAndStdoutAreTTY
	stdinAndStdoutAreTTY = func() bool { return true }
	defer func() { stdinAndStdoutAreTTY = origTTY }()

	cmd := NewEditCmd()
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if gotEditor != "my-editor" {
		t.Errorf("editor = %q, want %q (from $EDITOR)", gotEditor, "my-editor")
	}
	if want := filepath.Join(dir, id+".md"); gotPath != want {
		t.Errorf("path = %q, want %q", gotPath, want)
	}
}

func TestNewEditCmd_DefaultsToViWhenEditorUnset(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)
	t.Setenv("EDITOR", "")
	id, err := runCreate(dir, createOptions{title: "T", ticketType: "task", priority: 2})
	if err != nil {
		t.Fatalf("runCreate: %v", err)
	}

	var gotEditor string
	origRunEditor := runEditor
	runEditor = func(editor, path string) error { gotEditor = editor; return nil }
	defer func() { runEditor = origRunEditor }()

	origTTY := stdinAndStdoutAreTTY
	stdinAndStdoutAreTTY = func() bool { return true }
	defer func() { stdinAndStdoutAreTTY = origTTY }()

	cmd := NewEditCmd()
	cmd.SetArgs([]string{id})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotEditor != "vi" {
		t.Errorf("editor = %q, want vi default", gotEditor)
	}
}

func TestNewEditCmd_UnknownIDErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TICKETS_DIR", dir)

	cmd := NewEditCmd()
	cmd.SetArgs([]string{"no-such-ticket"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("Execute: want error for unknown id, got nil")
	}
}
