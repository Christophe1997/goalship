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
// this codebase surfaces a failure through. TimedOut is set instead of a
// meaningful ExitCode/Stderr when the call was killed for exceeding its
// context deadline (the host-tool watchdog), keeping "the process hung"
// distinct from "the process ran and exited non-zero".
type ExitError struct {
	Argv     []string
	ExitCode int
	Stderr   string
	TimedOut bool
}

func (e *ExitError) Error() string {
	if e.TimedOut {
		return fmt.Sprintf("`%s` timed out", strings.Join(e.Argv, " "))
	}
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

// runContext is run's checked, context-bound sibling: like run, a non-zero
// exit becomes an *ExitError, but a call that outlives ctx's deadline
// becomes one too (TimedOut: true) instead of blocking forever — used for
// gh/glab calls whose failure must propagate as a hard error (create-pr,
// retarget-pr), unlike runUnchecked's "collapse every failure into
// unknown" contract for PRState's best-effort lookups. ctx.Err() is
// checked directly rather than inspected off cmd.Run()'s returned error,
// so this doesn't depend on exactly how exec.CommandContext wraps a
// kill-on-cancel failure.
func runContext(ctx context.Context, dir string, argv ...string) (string, error) {
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.String(), nil
	}
	if ctx.Err() == context.DeadlineExceeded {
		return "", &ExitError{Argv: append([]string{}, argv...), TimedOut: true}
	}
	exitCode := -1
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		exitCode = exitErr.ExitCode()
	}
	return "", &ExitError{Argv: append([]string{}, argv...), ExitCode: exitCode, Stderr: stderr.String()}
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
