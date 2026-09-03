package reviewserver

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Christophe1997/goalship/internal/ledger"
)

func TestHandleStatus_ReflectsCurrentRunState(t *testing.T) {
	repoRoot := t.TempDir()
	mustSaveRunState(t, repoRoot, &ledger.RunState{
		RunID: "run-a", ReviewState: ledger.ReviewStatePending, ReviewUpdatedAt: "2026-01-01T00:00:00Z",
	})

	state := &apiState{
		repoRoot: repoRoot, runID: "run-a", ticketsDir: t.TempDir(), cancel: func() {},
		broadcaster: newReviewUpdateBroadcaster(), done: make(chan struct{}),
	}
	h := newReviewHandler(testToken, state)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newAPIRequest(t, "GET", "/api/status", testToken, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	got := decodeJSON[statusJSON](t, rec.Body)
	if got.ReviewState != ledger.ReviewStatePending {
		t.Errorf("review_state = %q, want %q", got.ReviewState, ledger.ReviewStatePending)
	}
	if got.ReviewUpdatedAt != "2026-01-01T00:00:00Z" {
		t.Errorf("review_updated_at = %q, want %q", got.ReviewUpdatedAt, "2026-01-01T00:00:00Z")
	}
}

// TestHandleStatusAndEvents_MissingOrWrongToken_Refused proves goa-7cxd's
// required point-4(e) coverage: GET /api/events and GET /api/status go
// through the exact same newReviewHandler composition as every other
// route, so a wrong/missing token must be rejected identically.
func TestHandleStatusAndEvents_MissingOrWrongToken_Refused(t *testing.T) {
	state := &apiState{
		repoRoot: t.TempDir(), runID: "run-a", ticketsDir: t.TempDir(), cancel: func() {},
		broadcaster: newReviewUpdateBroadcaster(), done: make(chan struct{}),
	}
	h := newReviewHandler(testToken, state)

	for _, path := range []string{"/api/status", "/api/events"} {
		t.Run(path+"/missing token", func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			req.Host = "127.0.0.1"
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
		t.Run(path+"/wrong token", func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, newAPIRequest(t, "GET", path, "wrong-token", nil))
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", rec.Code)
			}
		})
	}
}

// runForSSETest starts a real Run in the background (real listener, real
// fsnotify watcher — DisableWatch is never set here) and returns the
// tokened base URL plus a done channel for Run's return value. Every test
// in this file needs the exact same setup: a real HTTP round trip against
// an ephemeral loopback port is the only way to prove an SSE connection is
// genuinely open, not just requested.
//
// runForSSETest returns cancel alongside done because Run's shutdown here
// can be triggered two different ways depending on the test: internally
// (POST /api/approve calling apiState.cancel, an unrelated CancelFunc bound
// inside Run) or externally via the returned cancel. Callers that don't
// trigger approve must call cancel and drain done themselves so a leaked
// watcher goroutine from one test can't bleed into the next.
//
// When seedReviewUpdatedAt is non-empty, a ledger is saved with that value
// *before* Run starts, so a subsequent reject/withdraw/approve exercises
// atomicfile.Write's rename-over-an-existing-destination path — the one
// production always takes (Run only ever starts against a run whose ledger
// already exists) — rather than only the first-ever-write path a fresh
// TempDir would otherwise produce.
func runForSSETest(t *testing.T, seedReviewUpdatedAt string) (baseURL string, cancel context.CancelFunc, done chan error) {
	t.Helper()
	repoRoot := t.TempDir()
	ticketsDir := filepath.Join(repoRoot, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatalf("mkdir ticketsDir: %v", err)
	}
	t.Setenv("TICKETS_DIR", "")

	if seedReviewUpdatedAt != "" {
		mustSaveRunState(t, repoRoot, &ledger.RunState{
			RunID: "run-a", ReviewState: ledger.ReviewStatePending, ReviewUpdatedAt: seedReviewUpdatedAt,
		})
	}

	var ctx context.Context
	ctx, cancel = context.WithCancel(context.Background())

	var gotURL string
	urlSeen := make(chan struct{})
	done = make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			RepoRoot: repoRoot,
			RunID:    "run-a",
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
		t.Fatal("timed out waiting for Run to announce its URL")
	}
	return gotURL, cancel, done
}

func sseURL(baseURL, apiPath string) string {
	return strings.Replace(baseURL, "/?token=", apiPath+"?token=", 1)
}

// TestSSEOpenDuringApprove_ShutdownDoesNotHang is goa-7cxd's most important
// required test: it proves the real bug class described in the ticket. An
// SSE handler that blocks forever on <-ch is still "in flight" when
// srv.Shutdown starts, so Shutdown would block for the full 5s
// shutdownTimeout and return a deadline-exceeded error that Run wraps and
// returns — breaking POST /api/approve's (and SIGINT's) previously-clean
// shutdown the moment one browser tab has the review page open. This test
// opens a real SSE connection, actively drains it (proving it's genuinely
// open, not just requested), then triggers approve and asserts Run returns
// nil well within 1s — not anywhere near the 5s timeout.
func TestSSEOpenDuringApprove_ShutdownDoesNotHang(t *testing.T) {
	baseURL, cancel, done := runForSSETest(t, "")
	defer cancel() // no-op once approve has already triggered Run's internal shutdown

	resp, err := http.Get(sseURL(baseURL, "/api/events"))
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/events status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}
	// Actively read the body in the background: this is what makes the SSE
	// connection genuinely open rather than merely requested.
	go io.Copy(io.Discard, resp.Body)

	aResp, err := http.Post(sseURL(baseURL, "/api/approve"), "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/approve: %v", err)
	}
	defer aResp.Body.Close()
	if aResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(aResp.Body)
		t.Fatalf("POST /api/approve status = %d, body: %s", aResp.StatusCode, body)
	}

	start := time.Now()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after approve with an open SSE connection: %v", err)
		}
		if elapsed := time.Since(start); elapsed > time.Second {
			t.Errorf("Run took %v to return after approve; want well under the 5s shutdown timeout (the SSE handler is blocking Shutdown)", elapsed)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("Run did not return within 4s of POST /api/approve — the SSE handler is blocking Shutdown")
	}
}

// TestSSE_PushesReviewUpdatedAt_OnReject proves AC1's Go-testable half
// end-to-end: a real running review server, a real fsnotify watch, and a
// real open SSE connection. POST /api/reject performs the exact write path
// a separate regenerating process would also use (RunState.Save's atomic
// rename), and the already-open SSE connection must receive a push carrying
// the new review_updated_at — without any reload of the connection itself.
func TestSSE_PushesReviewUpdatedAt_OnReject(t *testing.T) {
	baseURL, cancel, done := runForSSETest(t, "2026-01-01T00:00:00Z")

	resp, err := http.Get(sseURL(baseURL, "/api/events"))
	if err != nil {
		t.Fatalf("GET /api/events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /api/events status = %d, want 200", resp.StatusCode)
	}

	lines := make(chan string, 8)
	go func() {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, "data: ") {
				select {
				case lines <- strings.TrimPrefix(line, "data: "):
				default:
				}
			}
		}
	}()

	// handleEvents subscribes before it writes/flushes its 200 response, so
	// by the time the GET above returned, this connection is already
	// registered — no sleep needed before triggering the change.
	rResp, err := http.Post(sseURL(baseURL, "/api/reject"), "application/json", strings.NewReader(`{"notes":"needs rework"}`))
	if err != nil {
		t.Fatalf("POST /api/reject: %v", err)
	}
	defer rResp.Body.Close()
	if rResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(rResp.Body)
		t.Fatalf("POST /api/reject status = %d, body: %s", rResp.StatusCode, body)
	}
	decision := decodeJSON[reviewDecisionJSON](t, rResp.Body)

	select {
	case data := <-lines:
		if data != decision.ReviewUpdatedAt {
			t.Errorf("SSE pushed review_updated_at = %q, want %q", data, decision.ReviewUpdatedAt)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for an SSE push after POST /api/reject")
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to shut down")
	}
}

// TestRun_DisableWatch_StatusStillReflectsChangesAfterReject proves the
// data path the client-side polling fallback depends on works correctly
// even with fsnotify entirely disabled: GET /api/status always reads the
// ledger fresh (ledger.LoadRunState), so it reflects a reject's effect
// regardless of whether any filesystem watch fired. This is the
// SSE-independent half of the required "polling fallback picks up a change
// within one interval" acceptance criterion — the interval/timing itself is
// a browser-side behavior this suite can't observe (see final report).
func TestRun_DisableWatch_StatusStillReflectsChangesAfterReject(t *testing.T) {
	repoRoot := t.TempDir()
	ticketsDir := filepath.Join(repoRoot, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatalf("mkdir ticketsDir: %v", err)
	}
	t.Setenv("TICKETS_DIR", "")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var gotURL string
	urlSeen := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Run(ctx, Options{
			RepoRoot:     repoRoot,
			RunID:        "run-a",
			Stdout:       io.Discard,
			DisableWatch: true,
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

	before, err := http.Get(sseURL(gotURL, "/api/status"))
	if err != nil {
		t.Fatalf("GET /api/status: %v", err)
	}
	beforeStatus := decodeJSON[statusJSON](t, before.Body)
	before.Body.Close()

	rResp, err := http.Post(sseURL(gotURL, "/api/reject"), "application/json", strings.NewReader(`{"notes":"needs rework"}`))
	if err != nil {
		t.Fatalf("POST /api/reject: %v", err)
	}
	rResp.Body.Close()

	after, err := http.Get(sseURL(gotURL, "/api/status"))
	if err != nil {
		t.Fatalf("GET /api/status: %v", err)
	}
	afterStatus := decodeJSON[statusJSON](t, after.Body)
	after.Body.Close()

	if afterStatus.ReviewUpdatedAt == beforeStatus.ReviewUpdatedAt {
		t.Errorf("review_updated_at did not change after reject: before=%q after=%q", beforeStatus.ReviewUpdatedAt, afterStatus.ReviewUpdatedAt)
	}
	if afterStatus.ReviewState != ledger.ReviewStateRejected {
		t.Errorf("review_state = %q, want %q", afterStatus.ReviewState, ledger.ReviewStateRejected)
	}

	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to shut down")
	}
}
