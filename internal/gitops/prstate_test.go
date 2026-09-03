package gitops

import (
	"os"
	"path/filepath"
	"testing"
)

// withFakeHostTool puts a fake `gh` or `glab` script (any shell one-liner)
// first on PATH for the duration of the test, so PRState's real subprocess
// path can be exercised without hitting a live host.
func withFakeHostTool(t *testing.T, name, script string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+script+"\n"), 0o755); err != nil {
		t.Fatalf("write fake %s: %v", name, err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestPRState_GHOpen(t *testing.T) {
	withFakeHostTool(t, "gh", `echo OPEN`)
	state, ok := PRState(t.TempDir(), "gh", "42")
	if !ok || state != "open" {
		t.Errorf("PRState = (%q, %v), want (open, true)", state, ok)
	}
}

func TestPRState_GHMerged(t *testing.T) {
	withFakeHostTool(t, "gh", `echo MERGED`)
	state, ok := PRState(t.TempDir(), "gh", "42")
	if !ok || state != "merged" {
		t.Errorf("PRState = (%q, %v), want (merged, true)", state, ok)
	}
}

func TestPRState_GHNonzeroExit_ReturnsNotOK(t *testing.T) {
	withFakeHostTool(t, "gh", `exit 1`)
	_, ok := PRState(t.TempDir(), "gh", "42")
	if ok {
		t.Errorf("PRState ok = true, want false on nonzero exit")
	}
}

func TestPRState_GlabOpened(t *testing.T) {
	withFakeHostTool(t, "glab", `echo '{"state": "opened"}'`)
	state, ok := PRState(t.TempDir(), "glab", "7")
	if !ok || state != "open" {
		t.Errorf("PRState = (%q, %v), want (open, true)", state, ok)
	}
}

func TestPRState_GlabInvalidJSON_ReturnsNotOK(t *testing.T) {
	withFakeHostTool(t, "glab", `echo 'not json'`)
	_, ok := PRState(t.TempDir(), "glab", "7")
	if ok {
		t.Errorf("PRState ok = true, want false on invalid JSON")
	}
}

func TestPRState_UnsupportedHostTool_ReturnsNotOK(t *testing.T) {
	_, ok := PRState(t.TempDir(), "hub", "1")
	if ok {
		t.Errorf("PRState ok = true, want false for an unsupported host tool")
	}
}
