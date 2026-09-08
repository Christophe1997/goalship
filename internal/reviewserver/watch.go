// watch.go pushes live-refresh notifications to GET /api/events subscribers
// when a regeneration lands: an fsnotify watch on the ledger's parent
// directory (never the file itself — RunState.Save's atomic
// write-then-rename would invalidate a file-level watch's inode) detects a
// review_updated_at change and fans it out via reviewUpdateBroadcaster. The
// client-side poll (assets/app.js) is the fallback for when this watch
// can't run at all (unavailable inotify/kqueue, a network-mounted
// .goalship/) or drops/coalesces an event — this package never assumes the
// watch is reliable enough to be the only detection path.
package reviewserver

import (
	"fmt"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"

	"github.com/Christophe1997/goalship/internal/ledger"
)

// newFsWatcher constructs the fsnotify watcher watchLedger uses. It's a
// package-level var, not a hard call to fsnotify.NewWatcher, solely so a
// test can force watcher construction to fail deterministically: there is
// no portable way to make a real fsnotify.NewWatcher fail in a CI sandbox,
// but Run must still be proven to degrade to a non-fatal Stderr diagnostic
// (rather than aborting review) when the platform can't watch at all.
var newFsWatcher = fsnotify.NewWatcher

// reviewUpdateBroadcaster fans out review_updated_at changes to any number
// of concurrent GET /api/events subscribers (multiple browser tabs, or a
// tab left open across reconnects). One topic only — this run's
// review_updated_at — so a generic pub-sub abstraction would be
// over-engineering.
type reviewUpdateBroadcaster struct {
	mu   sync.Mutex
	subs map[chan string]struct{}
}

func newReviewUpdateBroadcaster() *reviewUpdateBroadcaster {
	return &reviewUpdateBroadcaster{subs: make(map[chan string]struct{})}
}

// subscribe registers a new subscriber and returns its channel plus an
// unsubscribe func the caller must invoke exactly once (typically deferred)
// so a closed/abandoned SSE connection can't leak a map entry.
func (b *reviewUpdateBroadcaster) subscribe() (ch chan string, unsubscribe func()) {
	// Buffered by 1: a subscriber that's mid-write when the next change
	// lands doesn't block the broadcaster; broadcast drops instead (see
	// below) rather than growing unboundedly.
	ch = make(chan string, 1)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

// broadcast delivers reviewUpdatedAt to every current subscriber,
// non-blockingly: a subscriber whose buffer is already full gets the next
// notification dropped, not queued, since each handler's job on wake is to
// re-fetch current state (GET /api/tickets, GET /api/status) rather than
// replay a history of values.
func (b *reviewUpdateBroadcaster) broadcast(reviewUpdatedAt string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- reviewUpdatedAt:
		default:
		}
	}
}

// watchLedger watches repoRoot/runID's ledger parent directory and
// broadcasts to b whenever review_updated_at changes, until done fires. It
// returns a non-nil error only when the watch itself couldn't be
// established (fsnotify unavailable, or the directory can't be watched —
// e.g. a network-mounted .goalship/); the caller (Run) must treat that as
// non-fatal and fall back entirely to the client-side poll.
func watchLedger(done <-chan struct{}, repoRoot, runID string, b *reviewUpdateBroadcaster) error {
	ledgerPath := ledger.ResolveLedgerPath(repoRoot, runID)
	dir := filepath.Dir(ledgerPath)

	watcher, err := newFsWatcher()
	if err != nil {
		return fmt.Errorf("create fsnotify watcher: %w", err)
	}

	// Baseline must be captured before watcher.Add starts delivering events
	// for this directory: reading it after Add would let a Save landing in
	// between be folded into the "baseline" instead of producing a
	// detectable change, silently swallowing that notification.
	lastSeen := ""
	if st, err := ledger.LoadRunState(repoRoot, runID); err == nil {
		lastSeen = st.ReviewUpdatedAt
	}

	if err := watcher.Add(dir); err != nil {
		watcher.Close()
		return fmt.Errorf("watch %s: %w", dir, err)
	}

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-done:
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				// Never filter on event.Op: RunState.Save renames a temp
				// file into place, which fsnotify reports as a Create (or
				// platform-specific Rename) for the destination path, not a
				// Write. Filtering on Name alone also excludes the
				// runID.review.lock file's flock activity in the same
				// directory, since its path never matches ledgerPath.
				if event.Name != ledgerPath {
					continue
				}
				st, err := ledger.LoadRunState(repoRoot, runID)
				if err != nil {
					continue
				}
				if st.ReviewUpdatedAt != lastSeen {
					lastSeen = st.ReviewUpdatedAt
					b.broadcast(lastSeen)
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// Internal fsnotify errors (e.g. a kernel watch-limit hit)
				// aren't actionable per-event, and the client-side poll
				// already covers a watch that degrades mid-session.
			}
		}
	}()

	return nil
}
