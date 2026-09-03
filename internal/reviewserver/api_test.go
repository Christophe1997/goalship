package reviewserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
)

// fixtureBody mirrors buildCreateBody's layout (internal/cli/tk/create.go)
// with an extra "## Notes" section after Acceptance Criteria, so a PATCH
// to acceptance_criteria alone has bytes on both sides of the edit to
// prove untouched.
func fixtureBody(title, description, acceptance string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", title)
	if description != "" {
		fmt.Fprintf(&b, "%s\n\n", description)
	}
	if acceptance != "" {
		fmt.Fprintf(&b, "## Acceptance Criteria\n\n%s\n\n", acceptance)
	}
	b.WriteString("## Notes\n\nDo not touch.\n\n")
	return b.String()
}

func mustSaveTicket(t *testing.T, dir string, tk *ticket.Ticket) {
	t.Helper()
	if err := tk.Save(filepath.Join(dir, tk.ID+".md")); err != nil {
		t.Fatalf("save fixture ticket %s: %v", tk.ID, err)
	}
}

func mustSaveRunState(t *testing.T, repoRoot string, s *ledger.RunState) {
	t.Helper()
	if err := s.Save(repoRoot); err != nil {
		t.Fatalf("save fixture run state: %v", err)
	}
}

// newAPIRequest builds a request that passes tokenCheck and (for a
// mutating method) hostCheck, with an optional JSON body.
func newAPIRequest(t *testing.T, method, path, token string, body any) *http.Request {
	t.Helper()
	var r io.Reader
	if body != nil {
		r = jsonReader(t, body)
	}
	req := httptest.NewRequest(method, path+"?token="+token, r)
	req.Host = "127.0.0.1"
	return req
}

func jsonReader(t *testing.T, v any) io.Reader {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return bytes.NewReader(b)
}

func decodeJSON[T any](t *testing.T, r io.Reader) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(r).Decode(&v); err != nil {
		t.Fatalf("decode JSON response: %v", err)
	}
	return v
}

func TestHandleListTickets_ReturnsAllFixtureTickets(t *testing.T) {
	dir := t.TempDir()
	mustSaveTicket(t, dir, &ticket.Ticket{
		ID: "goa-aaa1", Status: "open", Created: "2026-01-01T00:00:00Z",
		Type: "task", Priority: 1, Deps: nil,
		Body: fixtureBody("Open ticket", "An open ticket.", "- must open"),
	})
	mustSaveTicket(t, dir, &ticket.Ticket{
		ID: "goa-bbb2", Status: "in_progress", Created: "2026-01-01T00:00:00Z",
		Type: "task", Priority: 2, Deps: []string{"goa-aaa1"},
		Body: fixtureBody("In progress ticket", "Working on it.", "- must progress"),
	})
	mustSaveTicket(t, dir, &ticket.Ticket{
		ID: "goa-ccc3", Status: "closed", Created: "2026-01-01T00:00:00Z",
		Type: "task", Priority: 3, Deps: nil,
		Body: fixtureBody("Closed ticket", "Already done.", "- must close"),
	})

	state := &apiState{repoRoot: t.TempDir(), runID: "run-a", ticketsDir: dir, cancel: func() {}}
	h := newReviewHandler(testToken, state)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newAPIRequest(t, "GET", "/api/tickets", testToken, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	got := decodeJSON[[]ticketJSON](t, rec.Body)
	if len(got) != 3 {
		t.Fatalf("got %d tickets, want 3: %+v", len(got), got)
	}

	byID := map[string]ticketJSON{}
	for _, tk := range got {
		byID[tk.ID] = tk
	}
	for _, id := range []string{"goa-aaa1", "goa-bbb2", "goa-ccc3"} {
		if _, ok := byID[id]; !ok {
			t.Errorf("missing ticket %q in listing (statuses must not be filtered)", id)
		}
	}

	open := byID["goa-aaa1"]
	if open.Status != "open" || open.Title != "Open ticket" || open.Description != "An open ticket." ||
		open.AcceptanceCriteria != "- must open" || open.Priority != 1 {
		t.Errorf("goa-aaa1 = %+v, fields don't match fixture", open)
	}
	if len(open.Deps) != 0 {
		t.Errorf("goa-aaa1 deps = %v, want empty (not null)", open.Deps)
	}

	inProgress := byID["goa-bbb2"]
	if len(inProgress.Deps) != 1 || inProgress.Deps[0] != "goa-aaa1" {
		t.Errorf("goa-bbb2 deps = %v, want [goa-aaa1]", inProgress.Deps)
	}
}

// TestPatchTicket_RoundTripIdempotence_SameValues is sections.go's
// required round-trip test at the HTTP level: GET a ticket, PATCH it with
// the exact same title/description/acceptance_criteria/priority/deps
// values GET returned, then confirm the .tickets/<id>.md file on disk is
// byte-identical to before the PATCH.
func TestPatchTicket_RoundTripIdempotence_SameValues(t *testing.T) {
	dir := t.TempDir()
	tk := &ticket.Ticket{
		ID: "goa-idem1", Status: "open", Created: "2026-01-01T00:00:00Z",
		Type: "task", Priority: 2, Deps: []string{"goa-other"},
		Body: fixtureBody("Idempotent Title", "Idempotent description.", "- criterion one\n- criterion two"),
	}
	mustSaveTicket(t, dir, tk)
	path := filepath.Join(dir, tk.ID+".md")

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture before PATCH: %v", err)
	}

	state := &apiState{repoRoot: t.TempDir(), runID: "run-a", ticketsDir: dir, cancel: func() {}}
	h := newReviewHandler(testToken, state)

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, newAPIRequest(t, "GET", "/api/tickets", testToken, nil))
	all := decodeJSON[[]ticketJSON](t, getRec.Body)
	var got ticketJSON
	for _, tk := range all {
		if tk.ID == "goa-idem1" {
			got = tk
		}
	}

	patchRec := httptest.NewRecorder()
	patchReq := newAPIRequest(t, "PATCH", "/api/tickets/goa-idem1", testToken, ticketPatch{
		Title:              &got.Title,
		Description:        &got.Description,
		AcceptanceCriteria: &got.AcceptanceCriteria,
		Priority:           &got.Priority,
		Deps:               &got.Deps,
	})
	h.ServeHTTP(patchRec, patchReq)
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200; body: %s", patchRec.Code, patchRec.Body.String())
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture after PATCH: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("PATCH with unchanged values altered the file:\nbefore: %q\nafter:  %q", before, after)
	}
}

func TestPatchTicket_PartialEdit_VisibleOnNextGet(t *testing.T) {
	dir := t.TempDir()
	mustSaveTicket(t, dir, &ticket.Ticket{
		ID: "goa-edit1", Status: "open", Created: "2026-01-01T00:00:00Z",
		Type: "task", Priority: 2,
		Body: fixtureBody("Original Title", "Original description.", "- original"),
	})

	state := &apiState{repoRoot: t.TempDir(), runID: "run-a", ticketsDir: dir, cancel: func() {}}
	h := newReviewHandler(testToken, state)

	newTitle := "Edited Title"
	newPriority := 0
	patchRec := httptest.NewRecorder()
	h.ServeHTTP(patchRec, newAPIRequest(t, "PATCH", "/api/tickets/goa-edit1", testToken, ticketPatch{
		Title:    &newTitle,
		Priority: &newPriority,
	}))
	if patchRec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, want 200; body: %s", patchRec.Code, patchRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	h.ServeHTTP(getRec, newAPIRequest(t, "GET", "/api/tickets", testToken, nil))
	all := decodeJSON[[]ticketJSON](t, getRec.Body)
	var got ticketJSON
	for _, tk := range all {
		if tk.ID == "goa-edit1" {
			got = tk
		}
	}

	if got.Title != "Edited Title" {
		t.Errorf("title = %q, want %q", got.Title, "Edited Title")
	}
	if got.Priority != 0 {
		t.Errorf("priority = %d, want 0", got.Priority)
	}
	if got.Description != "Original description." {
		t.Errorf("description = %q, want unchanged %q", got.Description, "Original description.")
	}
	if got.AcceptanceCriteria != "- original" {
		t.Errorf("acceptance_criteria = %q, want unchanged %q", got.AcceptanceCriteria, "- original")
	}
}

func TestPatchTicket_WhileRejected_Returns409_FileUnchanged(t *testing.T) {
	dir := t.TempDir()
	mustSaveTicket(t, dir, &ticket.Ticket{
		ID: "goa-locked1", Status: "open", Created: "2026-01-01T00:00:00Z",
		Type: "task", Priority: 2,
		Body: fixtureBody("Locked Title", "Locked description.", "- locked"),
	})
	path := filepath.Join(dir, "goa-locked1.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	repoRoot := t.TempDir()
	mustSaveRunState(t, repoRoot, &ledger.RunState{RunID: "run-a", ReviewState: ledger.ReviewStateRejected, ReviewNotes: "needs rework"})

	state := &apiState{repoRoot: repoRoot, runID: "run-a", ticketsDir: dir, cancel: func() {}}
	h := newReviewHandler(testToken, state)

	newTitle := "Should not apply"
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newAPIRequest(t, "PATCH", "/api/tickets/goa-locked1", testToken, ticketPatch{Title: &newTitle}))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body: %s", rec.Code, rec.Body.String())
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture after rejected PATCH: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("PATCH while rejected touched the file:\nbefore: %q\nafter:  %q", before, after)
	}
}

func TestPostReject_SetsReviewStateAndNotes(t *testing.T) {
	repoRoot := t.TempDir()
	mustSaveRunState(t, repoRoot, &ledger.RunState{RunID: "run-a", ReviewState: ledger.ReviewStatePending})

	state := &apiState{repoRoot: repoRoot, runID: "run-a", ticketsDir: t.TempDir(), cancel: func() {}}
	h := newReviewHandler(testToken, state)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newAPIRequest(t, "POST", "/api/reject", testToken, rejectRequest{Notes: "needs rework"}))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	got, err := ledger.LoadRunState(repoRoot, "run-a")
	if err != nil {
		t.Fatalf("LoadRunState: %v", err)
	}
	if got.ReviewState != ledger.ReviewStateRejected {
		t.Errorf("review_state = %q, want %q", got.ReviewState, ledger.ReviewStateRejected)
	}
	if got.ReviewNotes != "needs rework" {
		t.Errorf("review_notes = %q, want %q", got.ReviewNotes, "needs rework")
	}
	if got.ReviewUpdatedAt == "" {
		t.Error("review_updated_at was not set")
	}
}

func TestPostWithdraw_MovesToPendingAndClearsNotes(t *testing.T) {
	repoRoot := t.TempDir()
	mustSaveRunState(t, repoRoot, &ledger.RunState{
		RunID: "run-a", ReviewState: ledger.ReviewStateRejected, ReviewNotes: "stale notes",
	})

	state := &apiState{repoRoot: repoRoot, runID: "run-a", ticketsDir: t.TempDir(), cancel: func() {}}
	h := newReviewHandler(testToken, state)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, newAPIRequest(t, "POST", "/api/withdraw", testToken, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}

	got, err := ledger.LoadRunState(repoRoot, "run-a")
	if err != nil {
		t.Fatalf("LoadRunState: %v", err)
	}
	if got.ReviewState != ledger.ReviewStatePending {
		t.Errorf("review_state = %q, want %q", got.ReviewState, ledger.ReviewStatePending)
	}
	if got.ReviewNotes != "" {
		t.Errorf("review_notes = %q, want cleared", got.ReviewNotes)
	}
}

// TestPostApprove_SetsApprovedTicketIDs_AndShutsDownServer runs the real
// Run loop (not just handleApprove in isolation) to prove both AC4
// requirements together: approved_ticket_ids matches the current ticket ID
// set, and the server actually shuts down afterward rather than hanging —
// mirrors server_test.go's own OpenBrowser-callback pattern for
// race-free URL capture.
func TestPostApprove_SetsApprovedTicketIDs_AndShutsDownServer(t *testing.T) {
	repoRoot := t.TempDir()
	ticketsDir := filepath.Join(repoRoot, ".tickets")
	if err := os.MkdirAll(ticketsDir, 0o755); err != nil {
		t.Fatalf("mkdir ticketsDir: %v", err)
	}
	t.Setenv("TICKETS_DIR", "") // guard against ambient pollution; use Run's default resolution
	mustSaveTicket(t, ticketsDir, &ticket.Ticket{
		ID: "goa-app1", Status: "open", Created: "2026-01-01T00:00:00Z", Type: "task", Priority: 2,
		Body: fixtureBody("Approve me one", "", ""),
	})
	mustSaveTicket(t, ticketsDir, &ticket.Ticket{
		ID: "goa-app2", Status: "closed", Created: "2026-01-01T00:00:00Z", Type: "task", Priority: 2,
		Body: fixtureBody("Approve me two", "", ""),
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var gotURL string
	urlSeen := make(chan struct{})
	done := make(chan error, 1)
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

	resp, err := http.Post(strings.Replace(gotURL, "/?token=", "/api/approve?token=", 1), "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/approve: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("POST /api/approve status = %d, body: %s", resp.StatusCode, body)
	}
	decision := decodeJSON[reviewDecisionJSON](t, resp.Body)
	if decision.ReviewState != ledger.ReviewStateApproved {
		t.Errorf("response review_state = %q, want %q", decision.ReviewState, ledger.ReviewStateApproved)
	}

	wantIDs := map[string]bool{"goa-app1": true, "goa-app2": true}
	if len(decision.ApprovedTicketIDs) != len(wantIDs) {
		t.Fatalf("approved_ticket_ids = %v, want exactly %v", decision.ApprovedTicketIDs, wantIDs)
	}
	for _, id := range decision.ApprovedTicketIDs {
		if !wantIDs[id] {
			t.Errorf("unexpected approved ticket id %q", id)
		}
	}

	state, err := ledger.LoadRunState(repoRoot, "run-a")
	if err != nil {
		t.Fatalf("LoadRunState: %v", err)
	}
	if state.ReviewState != ledger.ReviewStateApproved {
		t.Errorf("ledger review_state = %q, want %q", state.ReviewState, ledger.ReviewStateApproved)
	}
	if len(state.ApprovedTicketIDs) != 2 {
		t.Errorf("ledger approved_ticket_ids = %v, want 2 entries", state.ApprovedTicketIDs)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run after approve: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for Run to shut down after POST /api/approve")
	}
}

func TestPatchTicket_MissingOrWrongToken_And_ForeignHost_Refused(t *testing.T) {
	dir := t.TempDir()
	mustSaveTicket(t, dir, &ticket.Ticket{
		ID: "goa-guard1", Status: "open", Created: "2026-01-01T00:00:00Z", Type: "task", Priority: 2,
		Body: fixtureBody("Guarded Title", "Guarded description.", "- guarded"),
	})
	path := filepath.Join(dir, "goa-guard1.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	state := &apiState{repoRoot: t.TempDir(), runID: "run-a", ticketsDir: dir, cancel: func() {}}
	h := newReviewHandler(testToken, state)
	newTitle := "Should never apply"

	t.Run("missing token", func(t *testing.T) {
		req := httptest.NewRequest("PATCH", "/api/tickets/goa-guard1", jsonReader(t, ticketPatch{Title: &newTitle}))
		req.Host = "127.0.0.1"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("wrong token", func(t *testing.T) {
		req := newAPIRequest(t, "PATCH", "/api/tickets/goa-guard1", "wrong-token", ticketPatch{Title: &newTitle})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("status = %d, want 401", rec.Code)
		}
	})

	t.Run("foreign host", func(t *testing.T) {
		req := newAPIRequest(t, "PATCH", "/api/tickets/goa-guard1", testToken, ticketPatch{Title: &newTitle})
		req.Host = "evil.example.com"
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", rec.Code)
		}
	})

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture after refused requests: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("a refused PATCH touched the file:\nbefore: %q\nafter:  %q", before, after)
	}
}
