package reviewserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Christophe1997/goalship/internal/ledger"
)

// mustMkdirGoalship creates dir's .goalship/ subdirectory up front, mirroring
// the real Run flow's precondition: AcquireReviewLock (called before
// watchLedger, in server.go) always MkdirAlls it first, so watchLedger
// itself never needs to create the directory it watches.
func mustMkdirGoalship(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ledger.LedgerDirName), 0o755); err != nil {
		t.Fatalf("mkdir .goalship: %v", err)
	}
}

// TestWatchLedger_FiresOnRealRunStateSave is goa-7cxd's required point-4
// test: it watches a real temp ledger directory and calls a real
// (*ledger.RunState).Save — not os.WriteFile — to prove the watcher's
// change notification fires off the actual write path RunState.Save uses
// (atomicfile.Write's temp-file-then-rename), which is exactly the pattern
// a naive event.Op&fsnotify.Write filter would silently miss (Save never
// produces a Write op for the destination path — only Create/Rename).
func TestWatchLedger_FiresOnRealRunStateSave(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"
	mustMkdirGoalship(t, dir)

	b := newReviewUpdateBroadcaster()
	ch, unsubscribe := b.subscribe()
	defer unsubscribe()

	done := make(chan struct{})
	defer close(done)

	if err := watchLedger(done, dir, runID, b); err != nil {
		t.Fatalf("watchLedger: %v", err)
	}

	state := &ledger.RunState{RunID: runID, ReviewState: ledger.ReviewStatePending, ReviewUpdatedAt: "2026-01-01T00:00:00Z"}
	if err := state.Save(dir); err != nil {
		t.Fatalf("Save: %v", err)
	}

	select {
	case got := <-ch:
		if got != "2026-01-01T00:00:00Z" {
			t.Errorf("broadcast value = %q, want %q", got, "2026-01-01T00:00:00Z")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watchLedger to fire on a real RunState.Save (first write, no prior ledger file)")
	}

	// A second Save overwrites an *existing* ledger file — the path
	// production always takes (Run only ever starts against a run whose
	// ledger already exists; internal/cli/review.go refuses otherwise).
	// atomicfile.Write's rename-over-an-existing-destination can, on some
	// platforms/backends, surface a different event shape than a
	// rename-onto-a-fresh-name (the first Save above) — asserting this
	// second broadcast too closes that gap.
	state.ReviewUpdatedAt = "2026-01-02T00:00:00Z"
	if err := state.Save(dir); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	select {
	case got := <-ch:
		if got != "2026-01-02T00:00:00Z" {
			t.Errorf("broadcast value = %q, want %q", got, "2026-01-02T00:00:00Z")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watchLedger to fire on a real RunState.Save (overwrite of an existing ledger file)")
	}
}

// TestWatchLedger_IgnoresReviewLockActivity proves the directory-level
// watch's filename filter does its job: a directory watch also observes the
// runID.review.lock file's flock activity (created alongside the ledger in
// .goalship/), which must never itself trigger a broadcast.
func TestWatchLedger_IgnoresReviewLockActivity(t *testing.T) {
	dir := t.TempDir()
	runID := "run-a"
	mustMkdirGoalship(t, dir)

	b := newReviewUpdateBroadcaster()
	ch, unsubscribe := b.subscribe()
	defer unsubscribe()

	done := make(chan struct{})
	defer close(done)

	if err := watchLedger(done, dir, runID, b); err != nil {
		t.Fatalf("watchLedger: %v", err)
	}

	lock, err := ledger.AcquireReviewLock(dir, runID)
	if err != nil {
		t.Fatalf("AcquireReviewLock: %v", err)
	}
	defer lock.Release()

	select {
	case got := <-ch:
		t.Fatalf("watchLedger broadcast %q from review-lock activity alone; want no broadcast", got)
	case <-time.After(500 * time.Millisecond):
		// Expected: no broadcast from lock-file activity.
	}
}

// TestWatchLedger_AddFailure_ReturnsNonNilError proves the watcher/Add
// failure this package must degrade from is real and surfaced as an error
// rather than a panic: repoRoot's .goalship/ parent directory never gets
// created here (only AcquireReviewLock or RunState.Save do that), so
// watcher.Add(dir) fails against a directory that genuinely doesn't exist —
// the same shape of failure a network-mounted, notification-incapable
// .goalship/ would produce.
func TestWatchLedger_AddFailure_ReturnsNonNilError(t *testing.T) {
	repoRoot := t.TempDir() // no .goalship/ subdirectory created
	done := make(chan struct{})
	defer close(done)

	err := watchLedger(done, repoRoot, "run-a", newReviewUpdateBroadcaster())
	if err == nil {
		t.Fatal("watchLedger: want a non-nil error when the ledger directory doesn't exist, got nil")
	}
}

// TestRun_WatcherConstructionFailure_NonFatal_LogsToStderr proves goa-7cxd's
// required point-4(b) test at the Run level: when the fsnotify watcher
// can't even be constructed (simulated here via newFsWatcher, the only
// portable way to force this deterministically — a real fsnotify.NewWatcher
// essentially never fails in a CI sandbox), Run must not fail or refuse to
// serve; it must log a visible diagnostic to Stderr and continue, exactly
// like a failed OpenBrowser call.
func TestRun_WatcherConstructionFailure_NonFatal_LogsToStderr(t *testing.T) {
	orig := newFsWatcher
	simulatedErr := errors.New("simulated: fsnotify unavailable on this platform")
	newFsWatcher = func() (*fsnotify.Watcher, error) {
		return nil, simulatedErr
	}
	defer func() { newFsWatcher = orig }()

	dir := t.TempDir()
	runID := "run-a"

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var stderr strings.Builder
	var gotURL string
	urlSeen := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			RepoRoot: dir,
			RunID:    runID,
			Stdout:   io.Discard,
			Stderr:   &stderr,
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
		t.Fatal("timed out waiting for Run to announce its URL")
	}

	if !strings.Contains(stderr.String(), simulatedErr.Error()) {
		t.Errorf("Stderr = %q, want it to contain the watcher-construction diagnostic %q", stderr.String(), simulatedErr.Error())
	}

	// Run must still be genuinely serving despite the watch failure — the
	// client-side polling fallback is what keeps live refresh working here,
	// not this watcher.
	resp, err := http.Get(gotURL)
	if err != nil {
		t.Fatalf("GET %s: %v", gotURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET %s: status = %d, want 200", gotURL, resp.StatusCode)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to shut down cleanly")
	}
}
