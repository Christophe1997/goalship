// api.go wires goa-4ufc's ticket-graph and review-decision routes onto the
// review server: GET/PATCH /api/tickets(/:id), and POST /api/reject,
// /api/withdraw, /api/approve. Ticket content returned here is treated as
// data — no server-side HTML generation from ticket text — the browser
// side (U8C) sanitizes and renders it.
package reviewserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Christophe1997/goalship/internal/ledger"
	"github.com/Christophe1997/goalship/internal/ticket"
)

// acceptanceCriteriaSection is the "## " heading name PATCH's
// acceptance_criteria field maps to — matches buildCreateBody's own
// section name (internal/cli/tk/create.go).
const acceptanceCriteriaSection = "Acceptance Criteria"

// apiState is the shared, per-invocation data every API route handler
// closes over. Run resolves ticketsDir exactly once when it builds the
// handler; no handler may re-resolve it from os.Getwd() — a test that
// accidentally did would PATCH this actual repository's live .tickets/*.md
// files instead of a fixture directory.
type apiState struct {
	repoRoot   string
	runID      string
	ticketsDir string

	// cancel triggers Run's shutdown (POST /api/approve only). Must be
	// non-nil in any apiState a handler can actually be invoked through.
	cancel context.CancelFunc

	// broadcaster fans out review_updated_at changes to GET /api/events
	// subscribers (watch.go). Must be non-nil in any apiState a handler can
	// actually be invoked through — handleEvents subscribes to it directly.
	broadcaster *reviewUpdateBroadcaster
	// done mirrors Run's runCtx.Done(): closed when shutdown begins, so
	// handleEvents' write loop can return promptly instead of blocking
	// srv.Shutdown for the full shutdownTimeout with a live SSE connection
	// still open.
	done <-chan struct{}
}

// registerAPIRoutes wires goa-4ufc's and goa-7cxd's routes onto mux.
// Host/token enforcement happens outside this mux entirely
// (newReviewHandler's composition) — nothing here re-checks either.
func registerAPIRoutes(mux *http.ServeMux, state *apiState) {
	mux.HandleFunc("GET /api/tickets", state.handleListTickets)
	mux.HandleFunc("PATCH /api/tickets/{id}", state.handlePatchTicket)
	mux.HandleFunc("POST /api/reject", state.handleReject)
	mux.HandleFunc("POST /api/withdraw", state.handleWithdraw)
	mux.HandleFunc("POST /api/approve", state.handleApprove)
	mux.HandleFunc("GET /api/status", state.handleStatus)
	mux.HandleFunc("GET /api/events", state.handleEvents)
}

// newReviewHandler builds the full request chain for one review-server
// invocation: the same securityHeaders(tokenCheck(hostCheck(...)))
// composition security.go's newHandler uses (that file is intentionally
// left untouched — hostCheck/tokenCheck are method-based, not
// path-based, so reusing them here needs no change there), extended with
// goa-4ufc's API routes alongside routes.go's existing index/asset ones.
func newReviewHandler(token string, state *apiState) http.Handler {
	mux := http.NewServeMux()
	registerRoutes(mux, token)
	registerAPIRoutes(mux, state)

	var h http.Handler = mux
	h = hostCheck(h)
	h = tokenCheck(token, h)
	h = securityHeaders(h)
	return h
}

// ticketJSON is one ticket's GET /api/tickets / PATCH /api/tickets/{id}
// wire representation.
type ticketJSON struct {
	ID                 string   `json:"id"`
	Status             string   `json:"status"`
	Title              string   `json:"title"`
	Description        string   `json:"description"`
	AcceptanceCriteria string   `json:"acceptance_criteria"`
	Priority           int      `json:"priority"`
	Deps               []string `json:"deps"`
}

func ticketJSONFrom(t *ticket.Ticket) ticketJSON {
	title, description, sections, _ := ticket.ParseSections(t.Body)
	return ticketJSON{
		ID:                 t.ID,
		Status:             t.Status,
		Title:              title,
		Description:        description,
		AcceptanceCriteria: sections[acceptanceCriteriaSection],
		Priority:           t.Priority,
		Deps:               orEmpty(t.Deps),
	}
}

// orEmpty substitutes a non-nil empty slice for nil — encoding/json
// marshals a nil []string as `null`; a ticket with no deps should report
// `[]`, mirroring internal/ledger's own nonNilStrings convention.
func orEmpty(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// listTickets reads every "*.md" file directly under ticketsDir, in the
// sorted-by-filename order os.ReadDir already guarantees (mirrors
// ticket.LoadAwkTickets' directory-listing pattern), and decodes each with
// ticket.ParseTolerant rather than ticket.Load: a hand-edited ticket
// missing/malformed a core field still shows up in the review UI instead
// of vanishing, matching tk ls's own graceful-degradation behavior. The
// returned Ticket must never be Save'd (ParseTolerant's own contract) —
// listing only ever projects it into ticketJSON, never writes it back.
func listTickets(ticketsDir string) ([]ticketJSON, error) {
	entries, err := os.ReadDir(ticketsDir)
	if err != nil {
		return nil, fmt.Errorf("reviewserver: list tickets: %w", err)
	}

	tickets := make([]ticketJSON, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(ticketsDir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reviewserver: list tickets: %w", err)
		}
		t, _, err := ticket.ParseTolerant(data)
		if err != nil {
			return nil, fmt.Errorf("reviewserver: list tickets: %s: %w", e.Name(), err)
		}
		tickets = append(tickets, ticketJSONFrom(t))
	}
	return tickets, nil
}

// writeJSON encodes v as the response body. A json.Encode failure here
// would mean a bug in what this package hands it (every value above is a
// plain struct of strings/ints/slices) — there is nothing meaningful left
// to do about it once the status line is already written, so it's not
// separately handled.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// nowStamp is this ledger's review_updated_at format — mirrors
// internal/cli/tk/summary.go's isoNow.
func nowStamp() string {
	return time.Now().UTC().Format("2006-01-02T15:04:05Z")
}

// reviewDecisionJSON is reject/withdraw/approve's shared JSON confirmation
// shape.
type reviewDecisionJSON struct {
	ReviewState       string   `json:"review_state"`
	ReviewNotes       string   `json:"review_notes"`
	ReviewUpdatedAt   string   `json:"review_updated_at"`
	ApprovedTicketIDs []string `json:"approved_ticket_ids"`
}

func reviewDecisionJSONFrom(s *ledger.RunState) reviewDecisionJSON {
	return reviewDecisionJSON{
		ReviewState:       s.ReviewState,
		ReviewNotes:       s.ReviewNotes,
		ReviewUpdatedAt:   s.ReviewUpdatedAt,
		ApprovedTicketIDs: orEmpty(s.ApprovedTicketIDs),
	}
}

func (s *apiState) handleListTickets(w http.ResponseWriter, r *http.Request) {
	tickets, err := listTickets(s.ticketsDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, tickets)
}

// ticketPatch is PATCH /api/tickets/{id}'s request body: every field is a
// pointer so a present-but-explicit value is distinguishable from an
// absent one (partial update) — a nil field leaves that aspect of the
// ticket unchanged, a non-nil one (including a pointer to "" or to an
// empty slice) applies the change.
type ticketPatch struct {
	Title              *string   `json:"title"`
	Description        *string   `json:"description"`
	AcceptanceCriteria *string   `json:"acceptance_criteria"`
	Priority           *int      `json:"priority"`
	Deps               *[]string `json:"deps"`
}

// handlePatchTicket applies a structured, partial edit straight through to
// .tickets/<id>.md immediately — no in-memory draft, no separate flush
// step, so it's visible on the very next GET /api/tickets. Refused outright
// (409, no file touched) while the run's ledger is mid-rejection: R11's
// read-only mode.
func (s *apiState) handlePatchTicket(w http.ResponseWriter, r *http.Request) {
	runState, err := ledger.LoadRunState(s.repoRoot, s.runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if runState.ReviewState == ledger.ReviewStateRejected {
		http.Error(w, "ticket edits are read-only while a rejection is awaiting regeneration", http.StatusConflict)
		return
	}

	id := r.PathValue("id")
	path, err := ticket.Resolve(s.ticketsDir, id)
	if err != nil {
		if errors.Is(err, ticket.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		// ticket.ErrAmbiguous, or a resolve-dir failure: the caller's
		// request itself is the problem, not the server.
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var patch ticketPatch
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	t, err := ticket.Load(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if patch.Title != nil {
		t.Body = ticket.SetTitle(t.Body, *patch.Title)
	}
	if patch.Description != nil {
		t.Body = ticket.SetDescription(t.Body, *patch.Description)
	}
	if patch.AcceptanceCriteria != nil {
		t.Body = ticket.SetSection(t.Body, acceptanceCriteriaSection, *patch.AcceptanceCriteria)
	}
	if patch.Priority != nil {
		t.Priority = *patch.Priority
	}
	if patch.Deps != nil {
		t.Deps = *patch.Deps
	}

	if err := t.Save(path); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ticketJSONFrom(t))
}

// rejectRequest is POST /api/reject's body: {"notes": "..."} carries the
// reviewer's rationale for sending the ticket graph back for regeneration.
type rejectRequest struct {
	Notes string `json:"notes"`
}

// handleReject writes review_state: rejected plus the notes atomically
// (RunState.Save's existing atomic write, R11) — a subsequent PATCH is
// refused until withdrawn or regenerated.
func (s *apiState) handleReject(w http.ResponseWriter, r *http.Request) {
	var req rejectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid JSON body: "+err.Error(), http.StatusBadRequest)
		return
	}

	runState, err := ledger.LoadRunState(s.repoRoot, s.runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	runState.ReviewState = ledger.ReviewStateRejected
	runState.ReviewNotes = req.Notes
	runState.ReviewUpdatedAt = nowStamp()
	if err := runState.Save(s.repoRoot); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, reviewDecisionJSONFrom(runState))
}

// handleWithdraw writes review_state: pending directly (R19) — no agent
// call or process involved — moving the run back out of read-only mode.
// ReviewNotes is cleared: stale rejection notes on a "pending, no decision"
// state would misreport to review-status callers.
func (s *apiState) handleWithdraw(w http.ResponseWriter, r *http.Request) {
	runState, err := ledger.LoadRunState(s.repoRoot, s.runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	runState.ReviewState = ledger.ReviewStatePending
	runState.ReviewNotes = ""
	runState.ReviewUpdatedAt = nowStamp()
	if err := runState.Save(s.repoRoot); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, reviewDecisionJSONFrom(runState))
}

// handleApprove writes review_state: approved plus approved_ticket_ids —
// the exact ticket ID set GET /api/tickets would return at this moment
// (R12) — then releases U5B's lock by causing Run to return: Run already
// holds a `defer lock.Release()` that fires exactly once when Run itself
// returns, so this handler must never call Release directly (that would
// double-release).
func (s *apiState) handleApprove(w http.ResponseWriter, r *http.Request) {
	tickets, err := listTickets(s.ticketsDir)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ids := make([]string, len(tickets))
	for i, t := range tickets {
		ids[i] = t.ID
	}

	runState, err := ledger.LoadRunState(s.repoRoot, s.runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	runState.ReviewState = ledger.ReviewStateApproved
	runState.ApprovedTicketIDs = ids
	runState.ReviewUpdatedAt = nowStamp()
	if err := runState.Save(s.repoRoot); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, reviewDecisionJSONFrom(runState))

	// Fired only after the response above is fully written, and
	// asynchronously: Run's select is blocked on runCtx.Done(), and
	// Shutdown blocks until every in-flight handler — this one included —
	// returns. Calling cancel synchronously here would risk Shutdown
	// waiting on a handler that's waiting on Shutdown; a goroutine lets
	// this handler return immediately so Shutdown observes it as finished.
	go s.cancel()
}

// statusJSON is GET /api/status's response shape: just enough for the
// front end's polling fallback to detect a change without a second full
// ticket-list fetch — GET /api/tickets stays a bare array with no room for
// a review_updated_at field bolted on.
type statusJSON struct {
	ReviewState     string `json:"review_state"`
	ReviewUpdatedAt string `json:"review_updated_at"`
}

func (s *apiState) handleStatus(w http.ResponseWriter, r *http.Request) {
	runState, err := ledger.LoadRunState(s.repoRoot, s.runID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, statusJSON{
		ReviewState:     runState.ReviewState,
		ReviewUpdatedAt: runState.ReviewUpdatedAt,
	})
}

// handleEvents is the SSE push side of live refresh: it never serializes
// ticket data itself (the page re-fetches GET /api/tickets on receipt), and
// it must return promptly once s.done fires even mid-stream — otherwise a
// browser tab left open with its EventSource connected would make
// srv.Shutdown (server.go) block for the full shutdownTimeout on every
// POST /api/approve or SIGINT, turning today's clean shutdown into a
// several-second hang.
func (s *apiState) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Subscribing before the response headers are flushed guarantees that
	// by the time a client sees this connection as open, it is already
	// registered to receive the next broadcast — no window where an event
	// fired between "client connected" and "server subscribed" is missed.
	ch, unsubscribe := s.broadcaster.subscribe()
	defer unsubscribe()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	for {
		select {
		case reviewUpdatedAt := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", reviewUpdatedAt)
			flusher.Flush()
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		}
	}
}
