package reviewserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Christophe1997/goalship/internal/ledger"
)

// noopBrowser is the OpenBrowser every test must inject — a test must never
// let Run spawn a real open/xdg-open/cmd process.
func noopBrowser(string) error { return nil }

// TestRun_LockAlreadyHeld_FailsFast proves the CLI-level acceptance
// criterion "second goalship review against the same run-id fails fast,
// naming the lock" at the reviewserver.Run level: AcquireReviewLock never
// blocks (it's TryLock-based), so a lock already held by someone else
// (simulated directly here, exactly as a concurrent `goalship review`
// invocation would leave it) makes Run fail immediately with no listener
// ever bound.
func TestRun_LockAlreadyHeld_FailsFast(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"

	holder, err := ledger.AcquireReviewLock(dir, runID)
	if err != nil {
		t.Fatalf("AcquireReviewLock: %v", err)
	}
	defer holder.Release()

	var out strings.Builder
	err = Run(context.Background(), Options{
		RepoRoot:    dir,
		RunID:       runID,
		Stdout:      &out,
		OpenBrowser: noopBrowser,
	})
	if err == nil {
		t.Fatal("Run: want an error while the lock is held, got nil")
	}
	var held *ledger.LockHeldError
	if !errors.As(err, &held) {
		t.Fatalf("Run error = %v (%T), want it to wrap *ledger.LockHeldError", err, err)
	}
	if held.RunID != runID {
		t.Errorf("LockHeldError.RunID = %q, want %q", held.RunID, runID)
	}
	if !strings.Contains(err.Error(), ".review.lock") {
		t.Errorf("error = %q, want it to name the lock file", err.Error())
	}
	if out.Len() != 0 {
		t.Errorf("Stdout = %q, want nothing written when the lock can't be acquired", out.String())
	}
}

// TestRun_LockReleased_LaterInvocationSucceeds proves the second CLI-level
// acceptance criterion: once a lock-holder releases — exactly what the OS
// does on an unclean kill, already proven by internal/ledger's own
// TestAcquireReviewLock_KillReleasesLock, so this test only needs to prove
// Run's own behavior once the lock is free again — a fresh Run call
// against the same run-id succeeds. The freed-lock Run's URL is captured
// via the OpenBrowser callback (same goroutine as Run's own code, and
// synchronized back to the test goroutine by closing urlSeen) rather than
// by reading Stdout concurrently from the test goroutine, which would race.
func TestRun_LockReleased_LaterInvocationSucceeds(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"

	holder, err := ledger.AcquireReviewLock(dir, runID)
	if err != nil {
		t.Fatalf("AcquireReviewLock: %v", err)
	}

	if err := Run(context.Background(), Options{
		RepoRoot:    dir,
		RunID:       runID,
		Stdout:      io.Discard,
		OpenBrowser: noopBrowser,
	}); !errors.As(err, new(*ledger.LockHeldError)) {
		t.Fatalf("Run while lock held: error = %v, want *ledger.LockHeldError", err)
	}

	if err := holder.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	var gotURL string
	urlSeen := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			RepoRoot: dir,
			RunID:    runID,
			Stdout:   io.Discard,
			OpenBrowser: func(u string) error {
				gotURL = u
				close(urlSeen)
				return nil
			},
		})
	}()

	select {
	case <-urlSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to acquire the freed lock and bind")
	}
	if !strings.Contains(gotURL, "127.0.0.1:") {
		t.Errorf("url = %q, want a 127.0.0.1 address", gotURL)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after lock release: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to shut down cleanly")
	}
}

// TestRun_PrintsURLBeforeOpenAndSurvivesOpenFailure covers three criteria
// in one race-free test: the tokened URL is on Stdout before OpenBrowser
// is invoked, a failing OpenBrowser doesn't fail Run or stop the server,
// and the server is genuinely reachable on the announced ephemeral port
// (a real end-to-end round trip). Stdout is read inside the OpenBrowser
// callback itself — the same goroutine, sequentially after the Fprintf
// that wrote it — never from the test goroutine concurrently.
func TestRun_PrintsURLBeforeOpenAndSurvivesOpenFailure(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"

	var stdoutAtOpenTime string
	var gotURL string
	openCalled := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var out strings.Builder
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			RepoRoot: dir,
			RunID:    runID,
			Stdout:   &out,
			OpenBrowser: func(u string) error {
				stdoutAtOpenTime = out.String()
				gotURL = u
				close(openCalled)
				return errors.New("simulated headless failure: no DISPLAY")
			},
		})
	}()

	select {
	case <-openCalled:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for OpenBrowser to be called")
	}

	if stdoutAtOpenTime == "" {
		t.Error("Stdout was empty at the time OpenBrowser was called; want the tokened URL already printed")
	}
	if !strings.Contains(stdoutAtOpenTime, gotURL) {
		t.Errorf("Stdout at open time = %q, want it to contain the opened URL %q", stdoutAtOpenTime, gotURL)
	}

	// A failing OpenBrowser must not stop the server: prove it's still
	// reachable with a real HTTP round trip against the announced URL.
	resp, err := http.Get(gotURL)
	if err != nil {
		t.Fatalf("GET %s: %v", gotURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("GET %s: status = %d, want 200", gotURL, resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to shut down after ctx cancellation")
	}
}

// TestRun_BindsLoopbackOnly proves R20: the listener address is always
// 127.0.0.1, an OS-assigned ephemeral port — never 0.0.0.0 or another
// routable interface. The URL is captured via the OpenBrowser callback
// (see TestRun_PrintsURLBeforeOpenAndSurvivesOpenFailure's doc comment for
// why) rather than by polling Stdout from the test goroutine.
func TestRun_BindsLoopbackOnly(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"

	ctx, cancel := context.WithCancel(context.Background())
	var gotURL string
	urlSeen := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			RepoRoot: dir,
			RunID:    runID,
			Stdout:   io.Discard,
			OpenBrowser: func(u string) error {
				gotURL = u
				close(urlSeen)
				return nil
			},
		})
	}()
	t.Cleanup(func() {
		cancel()
		<-done
	})

	select {
	case <-urlSeen:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to announce its URL")
	}
	if !strings.Contains(gotURL, "127.0.0.1:") {
		t.Errorf("url = %q, want the announced address to be on 127.0.0.1", gotURL)
	}
	if strings.Contains(gotURL, "0.0.0.0") {
		t.Errorf("url = %q, must never bind 0.0.0.0", gotURL)
	}
}
