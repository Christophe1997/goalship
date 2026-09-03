// Package ledger holds the per-run review lock. It depends on none of the
// ledger's own data types (added in a later ticket), so the lock can be
// acquired before any ledger state exists.
package ledger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/gofrs/flock"
)

// ReviewLock is an OS-native advisory lock held for the lifetime of the
// process that acquired it, regardless of whether the caller keeps the
// returned handle reachable: os.File finalizes (and closes) an unreachable
// file descriptor, which would silently drop the flock out from under a
// live process, so the package roots every acquired lock itself (see live)
// until Release. The OS additionally releases it if the process exits or is
// killed without calling Release.
type ReviewLock struct {
	path string
	fl   *flock.Flock
}

var (
	liveMu sync.Mutex
	live   = map[string]*ReviewLock{}
)

// LockHeldError reports that a run's review lock is already held by another
// process. Callers can match it with errors.As to distinguish "someone else
// is running this review" from other acquisition failures (e.g. a
// permissions error creating .goalship/).
type LockHeldError struct {
	RunID string
	Path  string
}

func (e *LockHeldError) Error() string {
	return fmt.Sprintf("ledger: review lock for run %q already held at %s", e.RunID, e.Path)
}

// AcquireReviewLock takes the per-run review lock at
// <repoRoot>/.goalship/<runID>.review.lock. It returns a *LockHeldError
// immediately if another process already holds it — it never blocks, since
// a caller with no lock has no way to know how long to wait, and this
// package's whole reason for existing is to fail fast instead.
func AcquireReviewLock(repoRoot, runID string) (*ReviewLock, error) {
	dir := filepath.Join(repoRoot, ".goalship")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("ledger: create %s: %w", dir, err)
	}

	path := filepath.Join(dir, runID+".review.lock")
	fl := flock.New(path)
	ok, err := fl.TryLock()
	if err != nil {
		return nil, fmt.Errorf("ledger: acquire review lock %s: %w", path, err)
	}
	if !ok {
		return nil, &LockHeldError{RunID: runID, Path: path}
	}

	lock := &ReviewLock{path: path, fl: fl}
	liveMu.Lock()
	live[path] = lock
	liveMu.Unlock()
	return lock, nil
}

// Release releases the lock. Safe to call once; the OS also releases it
// automatically if the process exits or is killed without calling this.
func (l *ReviewLock) Release() error {
	liveMu.Lock()
	delete(live, l.path)
	liveMu.Unlock()

	if err := l.fl.Unlock(); err != nil {
		return fmt.Errorf("ledger: release review lock %s: %w", l.path, err)
	}
	return nil
}

// Path returns the lock file's path, for logging.
func (l *ReviewLock) Path() string {
	return l.path
}
