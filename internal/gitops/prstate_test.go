package gitops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// prStateCaseBlock renders the `case "$3" in <ref>) echo <state> ;; ... *)
// exit 1 ;; esac` shell body a fake `gh` binary uses to answer a `pr view
// <ref>` call with a ref-specific state, shared by every fake-gh builder
// that needs to simulate `PRState` lookups (e.g. reconcile_test.go's
// fakeGH, sweepbranches_test.go's fakeGHSweep) alongside their own,
// differing outer dispatch.
func prStateCaseBlock(prStates map[string]string) string {
	var b strings.Builder
	b.WriteString("case \"$3\" in\n")
	for ref, state := range prStates {
		fmt.Fprintf(&b, "  %s) echo %s ;;\n", ref, state)
	}
	b.WriteString("  *) exit 1 ;;\nesac\n")
	return b.String()
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
