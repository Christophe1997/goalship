// state.go is a Go port of run_state.py: the durable per-run JSON ledger,
// its fixed v1 caps, and the mutators execution-loop.md's cycle drives
// (claim/ship/fail/terminal). Preflight/dirty-tree logic (preflight.py) is
// a separate concern, ported instead in internal/cli/loop where its own
// git/gh subprocess calls live.
package ledger

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Christophe1997/goalship/internal/atomicfile"
)

// Fixed v1 caps mirroring run_state.py's SHIP_CAP/FAILURE_CAP — not
// user-configurable, reset per invocation (never persisted).
const (
	ShipCap    = 10
	FailureCap = 3
)

// Terminal states execution-loop.md's cycle can end on — mirrors
// run_state.py's TERMINAL_* constants.
const (
	TerminalExhausted  = "exhausted"
	TerminalDeadlocked = "deadlocked"
	TerminalCapped     = "capped"
	TerminalUserStop   = "user_stop"
	TerminalAborted    = "aborted"
)

var terminalStates = map[string]bool{
	TerminalExhausted:  true,
	TerminalDeadlocked: true,
	TerminalCapped:     true,
	TerminalUserStop:   true,
	TerminalAborted:    true,
}

// execution-loop.md's two shipping modes — mirrors run_state.py's
// TICKET_MODES.
const (
	TicketModeBranch = "branch"
	TicketModeCommit = "commit"
)

var ticketModes = map[string]bool{TicketModeBranch: true, TicketModeCommit: true}

// ReviewStatePending is what an absent review_state key loads back as
// (R7): a ledger written before review-state existed, or one this
// ticket's own CLI surface never touched (goa-rqrf's claim-gate flags
// land in a later ticket), always reads as not-yet-reviewed rather than
// "approved" or empty.
const ReviewStatePending = "pending"

// ReviewStateApproved is the terminal review_state `goalship review` refuses
// to reopen (R9): once a run's ticket graph is approved, there's nothing
// left to review.
const ReviewStateApproved = "approved"

// ReviewStateRejected marks a run's ticket graph as sent back for
// regeneration (R11): PATCH /api/tickets/:id refuses edits while a run sits
// in this state, until POST /api/withdraw or a fresh review re-opens it.
const ReviewStateRejected = "rejected"

// LedgerDirName exposes ledgerDirName (defined in exclude.go) to callers
// outside this package — e.g. internal/cli/loop's dirty-tree check, which
// must exclude this same directory.
const LedgerDirName = ledgerDirName

// ValidTicketMode reports whether mode is one of execution-loop.md's two
// shipping modes. Checked by the `loop ledger` CLI command before
// assigning --ticket-mode — mirrored here rather than in
// run_state.py-equivalent setters because Python's own validation lives
// in loop_runner.py's cmd_ledger too, not run_state.py.
func ValidTicketMode(mode string) bool { return ticketModes[mode] }

// ValidTerminalState reports whether reason is one of the four
// ticket-graph outcomes or the preflight/auth/dirty-tree abort — the same
// set MarkTerminal itself validates against.
func ValidTerminalState(reason string) bool { return terminalStates[reason] }

// ExtraField is one JSON key this package's RunState doesn't know about,
// preserved verbatim in original order (R8) — mirrors internal/ticket's
// Field for frontmatter's own unrecognized-key handling.
type ExtraField struct {
	Key   string
	Value json.RawMessage
}

// RunState is one run's durable ledger — a Go port of run_state.py's
// RunState dataclass. RunID through TrunkBranch are Python's own eight
// fields, in its exact key order (see Bytes); ReviewState onward are new
// to the Go port (R7) with no Python schema to stay compatible with.
//
// TicketMode, TerminalState, and TrunkBranch are *string (not string) to
// preserve Python's Optional[str] = None: a loaded ledger's null must
// round-trip as null, not as an indistinguishable "".
type RunState struct {
	RunID               string
	ShippedCount        int
	ConsecutiveFailures int
	ClaimedTicketIDs    []string
	Goal                string
	TicketMode          *string
	TerminalState       *string
	TrunkBranch         *string

	ReviewState       string
	ReviewNotes       string
	ReviewUpdatedAt   string
	ApprovedTicketIDs []string

	// Extra holds every JSON key neither Python's schema nor the R7
	// additions above declare, in original order, values unparsed — so a
	// hand-edited or forward-compatible ledger round-trips unrecognized
	// data unchanged through a load/save cycle (R8).
	Extra []ExtraField
}

// knownRunStateFields is the fixed key order Bytes writes: run_state.py's
// own 8-field dataclasses.asdict order first (required for byte-identical
// round-trips against ledgers the Python tool wrote), then this Go port's
// R7 fields — Python has no schema opinion on where those go.
type knownRunStateFields struct {
	RunID               string   `json:"run_id"`
	ShippedCount        int      `json:"shipped_count"`
	ConsecutiveFailures int      `json:"consecutive_failures"`
	ClaimedTicketIDs    []string `json:"claimed_ticket_ids"`
	Goal                string   `json:"goal"`
	TicketMode          *string  `json:"ticket_mode"`
	TerminalState       *string  `json:"terminal_state"`
	TrunkBranch         *string  `json:"trunk_branch"`

	ReviewState       string   `json:"review_state"`
	ReviewNotes       string   `json:"review_notes"`
	ReviewUpdatedAt   string   `json:"review_updated_at"`
	ApprovedTicketIDs []string `json:"approved_ticket_ids"`
}

var knownRunStateKeys = map[string]bool{
	"run_id": true, "shipped_count": true, "consecutive_failures": true,
	"claimed_ticket_ids": true, "goal": true, "ticket_mode": true,
	"terminal_state": true, "trunk_branch": true,
	"review_state": true, "review_notes": true, "review_updated_at": true,
	"approved_ticket_ids": true,
}

// nonNilStrings substitutes a non-nil empty slice for nil: encoding/json
// marshals a nil []string as `null`, but both Python's `[]` default and
// this ticket's round-trip fixtures expect `[]`.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

// encodeIndented renders v as indented JSON without HTML-escaping —
// encoding/json's default escapes '<', '>', '&', which Python's json.dumps
// never does, so leaving it enabled would corrupt round-trips of arbitrary
// prose (e.g. a goal string) containing those characters.
func encodeIndented(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n")), nil
}

// appendJSONFields splices extra "key": value entries after the last
// field of a JSON object rendered by encodeIndented, preserving its
// existing 2-space nesting. Used both to re-append Extra fields peeled off
// during Parse (R8) and, by the `loop ledger` CLI command, to add a
// caps_exceeded annotation without perturbing the persisted schema order.
func appendJSONFields(objJSON []byte, fields []ExtraField) ([]byte, error) {
	if len(fields) == 0 {
		return objJSON, nil
	}
	body := bytes.TrimSuffix(objJSON, []byte("\n}"))
	if len(body) == len(objJSON) {
		return nil, fmt.Errorf("ledger: expected an indented JSON object, got %q", objJSON)
	}

	var b bytes.Buffer
	b.Write(body)
	for _, f := range fields {
		var compact bytes.Buffer
		if err := json.Compact(&compact, f.Value); err != nil {
			return nil, fmt.Errorf("ledger: field %q: %w", f.Key, err)
		}
		var indented bytes.Buffer
		if err := json.Indent(&indented, compact.Bytes(), "  ", "  "); err != nil {
			return nil, fmt.Errorf("ledger: field %q: %w", f.Key, err)
		}
		fmt.Fprintf(&b, ",\n  %q: ", f.Key)
		b.Write(indented.Bytes())
	}
	b.WriteString("\n}")
	return b.Bytes(), nil
}

// Bytes renders the ledger to its on-disk JSON form: the fixed key order
// above, then any Extra fields appended verbatim (R8). Mirrors
// internal/ticket's Ticket.Bytes, and save_run_state's
// json.dumps(state.to_dict(), indent=2).
func (s *RunState) Bytes() ([]byte, error) {
	known := knownRunStateFields{
		RunID:               s.RunID,
		ShippedCount:        s.ShippedCount,
		ConsecutiveFailures: s.ConsecutiveFailures,
		ClaimedTicketIDs:    nonNilStrings(s.ClaimedTicketIDs),
		Goal:                s.Goal,
		TicketMode:          s.TicketMode,
		TerminalState:       s.TerminalState,
		TrunkBranch:         s.TrunkBranch,
		ReviewState:         s.ReviewState,
		ReviewNotes:         s.ReviewNotes,
		ReviewUpdatedAt:     s.ReviewUpdatedAt,
		ApprovedTicketIDs:   nonNilStrings(s.ApprovedTicketIDs),
	}
	knownJSON, err := encodeIndented(known)
	if err != nil {
		return nil, fmt.Errorf("ledger: marshal: %w", err)
	}
	return appendJSONFields(knownJSON, s.Extra)
}

// BytesWithCapsExceeded renders state the same way Bytes does, with one
// extra top-level "caps_exceeded" key appended (CapsExceeded's reason, or
// null under both caps) — mirrors loop_runner.py's cmd_ledger, which
// merges this same key onto state.to_dict() before printing. CLI-only:
// Save always calls plain Bytes, so caps_exceeded is never persisted.
func (s *RunState) BytesWithCapsExceeded() ([]byte, error) {
	stateJSON, err := s.Bytes()
	if err != nil {
		return nil, err
	}
	var reason *string
	if r := s.CapsExceeded(); r != "" {
		reason = &r
	}
	capsRaw, err := json.Marshal(reason)
	if err != nil {
		return nil, fmt.Errorf("ledger: marshal caps_exceeded: %w", err)
	}
	return appendJSONFields(stateJSON, []ExtraField{{Key: "caps_exceeded", Value: capsRaw}})
}

// decodeOrderedObject walks data's top-level JSON object, returning its
// keys in original file order (duplicates included) alongside each key's
// raw value — the ordering encoding/json's normal map/struct decoding
// discards, but Parse needs to preserve Extra fields' relative order (R8).
func decodeOrderedObject(data []byte) (order []string, values map[string]json.RawMessage, err error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, fmt.Errorf("ledger: %w", err)
	}
	if d, ok := tok.(json.Delim); !ok || d != '{' {
		return nil, nil, fmt.Errorf("ledger: expected a JSON object")
	}

	values = map[string]json.RawMessage{}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil, nil, fmt.Errorf("ledger: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return nil, nil, fmt.Errorf("ledger: expected a string object key")
		}
		var raw json.RawMessage
		if err := dec.Decode(&raw); err != nil {
			return nil, nil, fmt.Errorf("ledger: field %q: %w", key, err)
		}
		order = append(order, key)
		values[key] = raw
	}
	if _, err := dec.Token(); err != nil { // closing '}'
		return nil, nil, fmt.Errorf("ledger: %w", err)
	}
	return order, values, nil
}

// ParseRunState decodes a ledger JSON file. Every known field defaults the
// way run_state.py's RunState.from_dict does when its key is absent
// (ReviewState defaults to ReviewStatePending instead, R7's own addition);
// every other top-level key is preserved as Extra, in original order
// (R8). Mirrors internal/ticket's Parse.
func ParseRunState(data []byte) (*RunState, error) {
	order, values, err := decodeOrderedObject(data)
	if err != nil {
		return nil, err
	}

	s := &RunState{ReviewState: ReviewStatePending}

	rawRunID, ok := values["run_id"]
	if !ok {
		return nil, fmt.Errorf("ledger: missing required field \"run_id\"")
	}
	if err := json.Unmarshal(rawRunID, &s.RunID); err != nil {
		return nil, fmt.Errorf("ledger: field \"run_id\": %w", err)
	}

	for key, dst := range map[string]any{
		"shipped_count":        &s.ShippedCount,
		"consecutive_failures": &s.ConsecutiveFailures,
		"claimed_ticket_ids":   &s.ClaimedTicketIDs,
		"goal":                 &s.Goal,
		"ticket_mode":          &s.TicketMode,
		"terminal_state":       &s.TerminalState,
		"trunk_branch":         &s.TrunkBranch,
		"review_state":         &s.ReviewState,
		"review_notes":         &s.ReviewNotes,
		"review_updated_at":    &s.ReviewUpdatedAt,
		"approved_ticket_ids":  &s.ApprovedTicketIDs,
	} {
		raw, ok := values[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(raw, dst); err != nil {
			return nil, fmt.Errorf("ledger: field %q: %w", key, err)
		}
	}

	seenExtra := map[string]bool{}
	for _, key := range order {
		if knownRunStateKeys[key] || seenExtra[key] {
			continue
		}
		seenExtra[key] = true
		s.Extra = append(s.Extra, ExtraField{Key: key, Value: values[key]})
	}
	return s, nil
}

var unsafeRunIDChars = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// ResolveLedgerPath mirrors run_state.py's resolve_ledger_path: one file
// per run_id — never a shared file requiring read-modify-write locking to
// avoid a lost-update race between concurrent invocations — at
// <repoRoot>/.goalship/<safe-run-id>.json.
func ResolveLedgerPath(repoRoot, runID string) string {
	safe := unsafeRunIDChars.ReplaceAllString(runID, "_")
	if safe == "" {
		safe = "run"
	}
	return filepath.Join(repoRoot, ledgerDirName, safe+".json")
}

// GenerateRunID returns a fresh, opaque 12-lowercase-hex-char run
// identifier — mirrors run_state.py's generate_run_id (uuid4().hex[:12]).
// Not a real UUID, just the same 12-hex-char shape, drawn from a CSPRNG.
func GenerateRunID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("ledger: generate run id: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// LoadRunState reads runID's ledger, or a fresh zeroed state (ReviewState
// ReviewStatePending, everything else Python's own zero value) if none
// exists yet — mirrors run_state.py's load_run_state.
func LoadRunState(repoRoot, runID string) (*RunState, error) {
	path := ResolveLedgerPath(repoRoot, runID)
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &RunState{RunID: runID, ReviewState: ReviewStatePending}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: load %s: %w", path, err)
	}
	state, err := ParseRunState(data)
	if err != nil {
		return nil, fmt.Errorf("ledger: load %s: %w", path, err)
	}
	return state, nil
}

// Save persists state atomically (internal/atomicfile): a concurrent
// reader never observes a partially-written ledger. Mirrors
// run_state.py's save_run_state.
func (s *RunState) Save(repoRoot string) error {
	path := ResolveLedgerPath(repoRoot, s.RunID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("ledger: create %s: %w", filepath.Dir(path), err)
	}
	data, err := s.Bytes()
	if err != nil {
		return fmt.Errorf("ledger: save %s: %w", path, err)
	}
	if err := atomicfile.Write(path, data); err != nil {
		return fmt.Errorf("ledger: save %s: %w", path, err)
	}
	return nil
}

// RecordShip resets the consecutive-failure count on a successful ship —
// mirrors run_state.py's record_ship.
func (s *RunState) RecordShip() {
	s.ShippedCount++
	s.ConsecutiveFailures = 0
}

// RecordFailure counts a gate failure or a block toward the
// consecutive-failure cap — mirrors run_state.py's record_failure.
func (s *RunState) RecordFailure() {
	s.ConsecutiveFailures++
}

// ClaimTicket idempotently adds ticketID to the run's claimed set —
// mirrors run_state.py's claim_ticket.
func (s *RunState) ClaimTicket(ticketID string) {
	for _, id := range s.ClaimedTicketIDs {
		if id == ticketID {
			return
		}
	}
	s.ClaimedTicketIDs = append(s.ClaimedTicketIDs, ticketID)
}

// CapsExceeded reports the human-readable reason to stop, or "" when
// under both caps — mirrors run_state.py's caps_exceeded.
func (s *RunState) CapsExceeded() string {
	if s.ShippedCount >= ShipCap {
		return fmt.Sprintf("ship cap reached (%d tickets shipped this run)", ShipCap)
	}
	if s.ConsecutiveFailures >= FailureCap {
		return fmt.Sprintf("consecutive-failure cap reached (%d failures in a row)", FailureCap)
	}
	return ""
}

// MarkTerminal records why this run stopped, so FindResumableRuns can tell
// a finished run from one merely paused mid-cycle — mirrors
// run_state.py's mark_terminal.
func (s *RunState) MarkTerminal(reason string) error {
	if !terminalStates[reason] {
		return fmt.Errorf("ledger: unknown terminal reason %q", reason)
	}
	s.TerminalState = &reason
	return nil
}

// FindResumableRuns returns every run under repoRoot's ledger dir that
// hasn't reached a terminal state yet, sorted by filename — mirrors
// run_state.py's find_resumable_runs. A ledger file outside the atomic-
// write path (interrupted mid-write, hand-edited) must not take down
// every other run's resumability with it, so an unparseable file is
// skipped rather than failing the whole scan.
func FindResumableRuns(repoRoot string) ([]*RunState, error) {
	dir := filepath.Join(repoRoot, ledgerDirName)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("ledger: scan %s: %w", dir, err)
	}

	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	var out []*RunState
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		state, err := ParseRunState(data)
		if err != nil {
			continue
		}
		if state.TerminalState == nil {
			out = append(out, state)
		}
	}
	return out, nil
}
