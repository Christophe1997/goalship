package gitops

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestExitError_Error_IncludesArgvExitCodeAndStderr(t *testing.T) {
	err := &ExitError{
		Argv:     []string{"git", "rev-parse", "no-such-ref"},
		ExitCode: 128,
		Stderr:   "fatal: ambiguous argument 'no-such-ref'\n",
	}
	msg := err.Error()
	for _, want := range []string{"git rev-parse no-such-ref", "128", "ambiguous argument"} {
		if !strings.Contains(msg, want) {
			t.Errorf("Error() = %q, missing %q", msg, want)
		}
	}
}

func TestExitError_Error_TimedOutOmitsExitCodeAndStderr(t *testing.T) {
	err := &ExitError{Argv: []string{"gh", "pr", "view", "1"}, TimedOut: true}
	msg := err.Error()
	if !strings.Contains(msg, "gh pr view 1") || !strings.Contains(msg, "timed out") {
		t.Errorf("Error() = %q, want it to mention the argv and a timeout", msg)
	}
}

func TestRunContext_NonzeroExit_ReturnsExitError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	_, err := runContext(ctx, t.TempDir(), "sh", "-c", "exit 2")
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if exitErr.ExitCode != 2 || exitErr.TimedOut {
		t.Errorf("ExitError = %+v, want ExitCode 2 and TimedOut false", exitErr)
	}
}

// TestRunContext_DeadlineExceeded_KillsProcessAndReportsTimeoutDistinctly
// proves the process is actually killed (the test completes near the
// deadline, not after the sleep's full duration) and that the resulting
// *ExitError is distinguishable from a plain non-zero exit.
func TestRunContext_DeadlineExceeded_KillsProcessAndReportsTimeoutDistinctly(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runContext(ctx, t.TempDir(), "sleep", "5")
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("runContext took %v, want it killed near the 20ms deadline", elapsed)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want *ExitError", err)
	}
	if !exitErr.TimedOut {
		t.Errorf("TimedOut = false, want true")
	}
}

// orphaningSleep is a hung host tool whose sleep is a forked child rather
// than the killed process itself, so the child outlives the watchdog's
// kill and keeps the stdout/stderr pipes open (issue #29).
var orphaningSleep = []string{"sh", "-c", "sleep 5 & wait"}

func TestRunContext_DeadlineExceeded_ReturnsDespiteOrphanHoldingPipes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runContext(ctx, t.TempDir(), orphaningSleep...)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("runContext took %v, want it to return near the 20ms deadline even with an orphaned child", elapsed)
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || !exitErr.TimedOut {
		t.Errorf("err = %v, want *ExitError with TimedOut true", err)
	}
}

func TestRunUnchecked_DeadlineExceeded_ReturnsDespiteOrphanHoldingPipes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, exitCode := runUnchecked(ctx, t.TempDir(), orphaningSleep...)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("runUnchecked took %v, want it to return near the 20ms deadline even with an orphaned child", elapsed)
	}
	if exitCode == 0 {
		t.Errorf("exitCode = 0, want nonzero for a killed process")
	}
}
