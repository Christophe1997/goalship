// Package gitops shells out to git — and, for resolve-base's dependency
// PR-state lookups, gh/glab — to implement goalship's branch mechanics: a
// Go port of branching.py plus the note-reading half of reconciliation.py
// that resolve-base and run-branch depend on. It never uses a Go git
// library: every operation is the same subprocess call the Python
// original makes, so behavior stays identical to the tool this replaces.
package gitops

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ExitError wraps a failed subprocess invocation with the argv run, its
// exit code, and its stderr — the uniform shape every git/gh/glab call in
// this codebase surfaces a failure through.
type ExitError struct {
	Argv     []string
	ExitCode int
	Stderr   string
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("`%s` failed (exit %d): %s", strings.Join(e.Argv, " "), e.ExitCode, strings.TrimSpace(e.Stderr))
}

// run executes argv[0] with argv[1:] in dir, returning stdout. A non-zero
// exit becomes an *ExitError.
func run(dir string, argv ...string) (string, error) {
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		exitCode := -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
		return "", &ExitError{Argv: append([]string{}, argv...), ExitCode: exitCode, Stderr: stderr.String()}
	}
	return stdout.String(), nil
}

// runUnchecked is run's counterpart for host-tool PR-state lookups, where
// pr_state() treats a failed subprocess (bad credential, host outage,
// timeout) as "state unknown" rather than a hard error — so this reports a
// bare exit code instead of wrapping non-zero exits into an error.
func runUnchecked(ctx context.Context, dir string, argv ...string) (stdout string, exitCode int) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return out.String(), exitErr.ExitCode()
		}
		return out.String(), -1
	}
	return out.String(), 0
}
