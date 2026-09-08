package ledger

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func strPtr(s string) *string { return &s }

// pythonFixture is a ledger JSON blob shaped exactly like run_state.py's
// dataclasses.asdict + json.dumps(indent=2): 2-space indent, the fixed
// 8-key order, no R7 keys.
const pythonFixture = `{
  "run_id": "835a170dff29",
  "shipped_count": 1,
  "consecutive_failures": 0,
  "claimed_ticket_ids": [
    "goa-5zwn",
    "goa-9nhc"
  ],
  "goal": "Implement the goalship CLI plan",
  "ticket_mode": "branch",
  "terminal_state": null,
  "trunk_branch": "main"
}`

func TestParseRunState_DecodesPythonFixture(t *testing.T) {
	s, err := ParseRunState([]byte(pythonFixture))
	if err != nil {
		t.Fatalf("ParseRunState: %v", err)
	}
	if s.RunID != "835a170dff29" {
		t.Errorf("RunID = %q, want %q", s.RunID, "835a170dff29")
	}
	if s.ShippedCount != 1 {
		t.Errorf("ShippedCount = %d, want 1", s.ShippedCount)
	}
	if s.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", s.ConsecutiveFailures)
	}
	if got, want := s.ClaimedTicketIDs, []string{"goa-5zwn", "goa-9nhc"}; !equalStrings(got, want) {
		t.Errorf("ClaimedTicketIDs = %v, want %v", got, want)
	}
	if s.Goal != "Implement the goalship CLI plan" {
		t.Errorf("Goal = %q", s.Goal)
	}
	if s.TicketMode == nil || *s.TicketMode != "branch" {
		t.Errorf("TicketMode = %v, want \"branch\"", s.TicketMode)
	}
	if s.TerminalState != nil {
		t.Errorf("TerminalState = %v, want nil", s.TerminalState)
	}
	if s.TrunkBranch == nil || *s.TrunkBranch != "main" {
		t.Errorf("TrunkBranch = %v, want \"main\"", s.TrunkBranch)
	}
	if s.ReviewState != ReviewStatePending {
		t.Errorf("ReviewState = %q, want %q (absent key must default to pending)", s.ReviewState, ReviewStatePending)
	}
	if s.ReviewNotes != "" || s.ReviewUpdatedAt != "" {
		t.Errorf("ReviewNotes/ReviewUpdatedAt = %q/%q, want both empty", s.ReviewNotes, s.ReviewUpdatedAt)
	}
	if len(s.ApprovedTicketIDs) != 0 {
		t.Errorf("ApprovedTicketIDs = %v, want empty", s.ApprovedTicketIDs)
	}
	if len(s.Extra) != 0 {
		t.Errorf("Extra = %v, want empty for a pure Python fixture", s.Extra)
	}
}

// TestBytes_KnownFieldsMatchPythonFixtureThenR7Extension verifies the
// achievable form of "byte-identical round-trip": the Go port always
// writes its 4 R7 fields (ReviewState defaults to "pending", never
// absent), so a loaded-then-saved Python ledger cannot stay byte-for-byte
// identical to the original 8-key file. What must hold instead: the saved
// bytes start with exactly the original 8 fields' bytes (order and
// indentation preserved), and the only addition is the 4 R7 keys before
// the closing brace.
func TestBytes_KnownFieldsMatchPythonFixtureThenR7Extension(t *testing.T) {
	s, err := ParseRunState([]byte(pythonFixture))
	if err != nil {
		t.Fatalf("ParseRunState: %v", err)
	}
	out, err := s.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	prefix := strings.TrimSuffix(pythonFixture, "\n}")
	if !strings.HasPrefix(string(out), prefix) {
		t.Fatalf("Bytes() = %s\n\nwant prefix:\n%s", out, prefix)
	}
	suffix := strings.TrimPrefix(string(out), prefix)
	wantSuffix := `,
  "review_state": "pending",
  "review_notes": "",
  "review_updated_at": "",
  "approved_ticket_ids": []
}`
	if suffix != wantSuffix {
		t.Errorf("R7 suffix = %s, want %s", suffix, wantSuffix)
	}
}

func TestBytes_MatchesLiveGoalshipFixtures(t *testing.T) {
	// Read-only fixtures written by the still-running Python loop_runner.py
	// in this session's own working tree — must decode cleanly and, after
	// re-encoding, keep exactly the original 8-field prefix (see the test
	// above for why full byte-identity isn't the achievable bar).
	repoRoot := findRepoRoot(t)
	for _, name := range []string{"4ba7e9564aed.json", "835a170dff29.json"} {
		path := filepath.Join(repoRoot, ".goalship", name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("fixture %s not present: %v", path, err)
		}
		s, err := ParseRunState(data)
		if err != nil {
			t.Fatalf("ParseRunState(%s): %v", name, err)
		}
		out, err := s.Bytes()
		if err != nil {
			t.Fatalf("Bytes(%s): %v", name, err)
		}
		prefix := strings.TrimSuffix(string(data), "\n}")
		if !strings.HasPrefix(string(out), prefix) {
			t.Errorf("%s: Bytes() = %s\n\nwant prefix:\n%s", name, out, prefix)
		}
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (no go.mod found)")
		}
		dir = parent
	}
}

func TestRoundTrip_FreshStateFidelity(t *testing.T) {
	s := &RunState{
		RunID:               "abc123def456",
		ShippedCount:        3,
		ConsecutiveFailures: 1,
		ClaimedTicketIDs:    []string{"t-1", "t-2"},
		Goal:                "ship things",
		TicketMode:          strPtr(TicketModeBranch),
		TerminalState:       nil,
		TrunkBranch:         strPtr("main"),
		ReviewState:         "approved",
		ReviewNotes:         "looks good",
		ReviewUpdatedAt:     "2026-09-03T00:00:00Z",
		ApprovedTicketIDs:   []string{"t-1"},
	}

	out, err := s.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	got, err := ParseRunState(out)
	if err != nil {
		t.Fatalf("ParseRunState: %v", err)
	}

	got2, err := got.Bytes()
	if err != nil {
		t.Fatalf("Bytes (second pass): %v", err)
	}
	if !bytes.Equal(out, got2) {
		t.Errorf("round trip not stable:\nfirst:  %s\nsecond: %s", out, got2)
	}
}

func TestRoundTrip_PreservesHTMLSensitiveCharactersUnescaped(t *testing.T) {
	s := &RunState{RunID: "r1", Goal: "fix <thing> & other's \"stuff\"", ReviewState: ReviewStatePending}
	out, err := s.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}
	// encoding/json's default HTML-escapes '<', '>', '&' into < etc.;
	// Python's json.dumps never does. Assert the literal characters
	// survived, which is only possible if escaping was disabled.
	if !bytes.Contains(out, []byte(`<thing>`)) || !bytes.Contains(out, []byte(` & `)) {
		t.Errorf("Bytes escaped HTML-sensitive characters (Python's json.dumps never does): %s", out)
	}
	got, err := ParseRunState(out)
	if err != nil {
		t.Fatalf("ParseRunState: %v", err)
	}
	if got.Goal != s.Goal {
		t.Errorf("Goal round-trip = %q, want %q", got.Goal, s.Goal)
	}
}

func TestRoundTrip_UnrecognizedKeyPreserved(t *testing.T) {
	fixture := `{
  "run_id": "abc123",
  "shipped_count": 0,
  "consecutive_failures": 0,
  "claimed_ticket_ids": [],
  "goal": "",
  "ticket_mode": null,
  "terminal_state": null,
  "trunk_branch": null,
  "future_field": {"nested": [1, 2, 3], "flag": true}
}`
	s, err := ParseRunState([]byte(fixture))
	if err != nil {
		t.Fatalf("ParseRunState: %v", err)
	}
	if len(s.Extra) != 1 || s.Extra[0].Key != "future_field" {
		t.Fatalf("Extra = %+v, want one field named future_field", s.Extra)
	}

	out, err := s.Bytes()
	if err != nil {
		t.Fatalf("Bytes: %v", err)
	}

	var roundTripped map[string]json.RawMessage
	if err := json.Unmarshal(out, &roundTripped); err != nil {
		t.Fatalf("Unmarshal saved bytes: %v", err)
	}
	raw, ok := roundTripped["future_field"]
	if !ok {
		t.Fatal("future_field missing after round trip")
	}
	var got, want map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("Unmarshal future_field: %v", err)
	}
	if err := json.Unmarshal([]byte(`{"nested": [1, 2, 3], "flag": true}`), &want); err != nil {
		t.Fatalf("Unmarshal want: %v", err)
	}
	gotJSON, _ := json.Marshal(got)
	wantJSON, _ := json.Marshal(want)
	if string(gotJSON) != string(wantJSON) {
		t.Errorf("future_field round-tripped as %s, want %s", gotJSON, wantJSON)
	}

	// Round-trip again to ensure stability (load what we just saved).
	s2, err := ParseRunState(out)
	if err != nil {
		t.Fatalf("ParseRunState (second pass): %v", err)
	}
	if len(s2.Extra) != 1 || s2.Extra[0].Key != "future_field" {
		t.Fatalf("Extra after second parse = %+v", s2.Extra)
	}
}

func TestSaveLoad_RoundTripsThroughDisk(t *testing.T) {
	repoRoot := t.TempDir()
	s := &RunState{
		RunID:            "diskrun01",
		Goal:             "test goal",
		TicketMode:       strPtr(TicketModeCommit),
		TrunkBranch:      strPtr("main"),
		ClaimedTicketIDs: []string{"t-9"},
		ReviewState:      ReviewStatePending,
	}
	if err := s.Save(repoRoot); err != nil {
		t.Fatalf("Save: %v", err)
	}

	path := ResolveLedgerPath(repoRoot, "diskrun01")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("ledger file not created at %s: %v", path, err)
	}

	loaded, err := LoadRunState(repoRoot, "diskrun01")
	if err != nil {
		t.Fatalf("LoadRunState: %v", err)
	}
	if loaded.Goal != "test goal" || loaded.TicketMode == nil || *loaded.TicketMode != TicketModeCommit {
		t.Errorf("loaded state = %+v", loaded)
	}
}

func TestLoadRunState_MissingFileReturnsFreshZeroedState(t *testing.T) {
	repoRoot := t.TempDir()
	s, err := LoadRunState(repoRoot, "never-existed")
	if err != nil {
		t.Fatalf("LoadRunState: %v", err)
	}
	if s.RunID != "never-existed" {
		t.Errorf("RunID = %q, want %q", s.RunID, "never-existed")
	}
	if s.ShippedCount != 0 || s.ConsecutiveFailures != 0 || s.Goal != "" {
		t.Errorf("fresh state not zeroed: %+v", s)
	}
	if s.TicketMode != nil || s.TerminalState != nil || s.TrunkBranch != nil {
		t.Errorf("fresh state's optional fields not nil: %+v", s)
	}
	if s.ReviewState != ReviewStatePending {
		t.Errorf("ReviewState = %q, want %q", s.ReviewState, ReviewStatePending)
	}
}

func TestGenerateRunID_TwelveLowercaseHexChars(t *testing.T) {
	id, err := GenerateRunID()
	if err != nil {
		t.Fatalf("GenerateRunID: %v", err)
	}
	if len(id) != 12 {
		t.Errorf("len(id) = %d, want 12 (%q)", len(id), id)
	}
	for _, r := range id {
		if !strings.ContainsRune("0123456789abcdef", r) {
			t.Errorf("id %q contains non-lowercase-hex rune %q", id, r)
		}
	}
}

func TestResolveLedgerPath_SanitizesUnsafeCharacters(t *testing.T) {
	got := ResolveLedgerPath("/repo", "weird/id:with*chars")
	want := filepath.Join("/repo", ".goalship", "weird_id_with_chars.json")
	if got != want {
		t.Errorf("ResolveLedgerPath = %q, want %q", got, want)
	}
}

func TestResolveLedgerPath_EmptyRunIDFallsBackToRun(t *testing.T) {
	// Mirrors resolve_ledger_path's `safe_run_id or "run"`: the fallback
	// triggers only when sanitizing yields an empty string, i.e. the input
	// itself was empty — every unsafe character maps to a literal "_", so
	// e.g. "///" sanitizes to "___" (non-empty, no fallback), not "run".
	got := ResolveLedgerPath("/repo", "")
	want := filepath.Join("/repo", ".goalship", "run.json")
	if got != want {
		t.Errorf("ResolveLedgerPath = %q, want %q", got, want)
	}
}

func TestResolveLedgerPath_AllUnsafeCharsSanitizeToUnderscoresNotRun(t *testing.T) {
	got := ResolveLedgerPath("/repo", "///")
	want := filepath.Join("/repo", ".goalship", "___.json")
	if got != want {
		t.Errorf("ResolveLedgerPath = %q, want %q", got, want)
	}
}

func TestCapsExceeded(t *testing.T) {
	cases := []struct {
		name  string
		state RunState
		want  string
	}{
		{"under both caps", RunState{ShippedCount: 0, ConsecutiveFailures: 0}, ""},
		{"just under ship cap", RunState{ShippedCount: ShipCap - 1}, ""},
		{"ship cap reached", RunState{ShippedCount: ShipCap}, "ship cap reached (10 tickets shipped this run)"},
		{"ship cap exceeded", RunState{ShippedCount: ShipCap + 1}, "ship cap reached (10 tickets shipped this run)"},
		{"just under failure cap", RunState{ConsecutiveFailures: FailureCap - 1}, ""},
		{"failure cap reached", RunState{ConsecutiveFailures: FailureCap}, "consecutive-failure cap reached (3 failures in a row)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.state.CapsExceeded(); got != tc.want {
				t.Errorf("CapsExceeded() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRecordShip_ResetsConsecutiveFailures(t *testing.T) {
	s := &RunState{ShippedCount: 2, ConsecutiveFailures: 2}
	s.RecordShip()
	if s.ShippedCount != 3 {
		t.Errorf("ShippedCount = %d, want 3", s.ShippedCount)
	}
	if s.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", s.ConsecutiveFailures)
	}
}

func TestRecordFailure_Increments(t *testing.T) {
	s := &RunState{ConsecutiveFailures: 1}
	s.RecordFailure()
	if s.ConsecutiveFailures != 2 {
		t.Errorf("ConsecutiveFailures = %d, want 2", s.ConsecutiveFailures)
	}
}

func TestClaimTicket_Idempotent(t *testing.T) {
	s := &RunState{}
	s.ClaimTicket("t-1")
	s.ClaimTicket("t-2")
	s.ClaimTicket("t-1")
	if got, want := s.ClaimedTicketIDs, []string{"t-1", "t-2"}; !equalStrings(got, want) {
		t.Errorf("ClaimedTicketIDs = %v, want %v", got, want)
	}
}

func TestMarkTerminal_RejectsUnknownReason(t *testing.T) {
	s := &RunState{}
	if err := s.MarkTerminal("not-a-real-reason"); err == nil {
		t.Fatal("MarkTerminal: expected error for unknown reason, got nil")
	}
	if s.TerminalState != nil {
		t.Errorf("TerminalState = %v, want nil after a rejected MarkTerminal", s.TerminalState)
	}
}

func TestMarkTerminal_AcceptsEachValidReason(t *testing.T) {
	for _, reason := range []string{TerminalExhausted, TerminalDeadlocked, TerminalCapped, TerminalUserStop, TerminalAborted} {
		s := &RunState{}
		if err := s.MarkTerminal(reason); err != nil {
			t.Errorf("MarkTerminal(%q): %v", reason, err)
		}
		if s.TerminalState == nil || *s.TerminalState != reason {
			t.Errorf("TerminalState = %v, want %q", s.TerminalState, reason)
		}
	}
}

func TestFindResumableRuns_FiltersTerminalAndSkipsUnparseable(t *testing.T) {
	repoRoot := t.TempDir()

	resumable := &RunState{RunID: "run-a", Goal: "keep going"}
	if err := resumable.Save(repoRoot); err != nil {
		t.Fatalf("Save run-a: %v", err)
	}

	terminal := &RunState{RunID: "run-b", Goal: "done"}
	if err := terminal.MarkTerminal(TerminalExhausted); err != nil {
		t.Fatalf("MarkTerminal: %v", err)
	}
	if err := terminal.Save(repoRoot); err != nil {
		t.Fatalf("Save run-b: %v", err)
	}

	junkPath := filepath.Join(repoRoot, ledgerDirName, "run-c.json")
	if err := os.WriteFile(junkPath, []byte("{not valid json"), 0o644); err != nil {
		t.Fatalf("write junk file: %v", err)
	}

	got, err := FindResumableRuns(repoRoot)
	if err != nil {
		t.Fatalf("FindResumableRuns: %v", err)
	}
	if len(got) != 1 || got[0].RunID != "run-a" {
		t.Fatalf("FindResumableRuns = %+v, want only run-a", got)
	}
}

func TestFindResumableRuns_NoLedgerDirReturnsEmpty(t *testing.T) {
	repoRoot := t.TempDir()
	got, err := FindResumableRuns(repoRoot)
	if err != nil {
		t.Fatalf("FindResumableRuns: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("FindResumableRuns = %+v, want empty", got)
	}
}

func TestValidTicketModeAndTerminalState(t *testing.T) {
	if !ValidTicketMode(TicketModeBranch) || !ValidTicketMode(TicketModeCommit) {
		t.Error("expected branch and commit to be valid ticket modes")
	}
	if ValidTicketMode("bogus") {
		t.Error("expected bogus to be an invalid ticket mode")
	}
	if !ValidTerminalState(TerminalUserStop) {
		t.Error("expected user_stop to be a valid terminal state")
	}
	if ValidTerminalState("bogus") {
		t.Error("expected bogus to be an invalid terminal state")
	}
}

func TestBytesWithCapsExceeded_NullWhenUnderCaps(t *testing.T) {
	s := &RunState{RunID: "r1", ReviewState: ReviewStatePending}
	out, err := s.BytesWithCapsExceeded()
	if err != nil {
		t.Fatalf("BytesWithCapsExceeded: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v, ok := m["caps_exceeded"]; !ok || v != nil {
		t.Errorf("caps_exceeded = %v, want null", v)
	}
}

func TestBytesWithCapsExceeded_ReasonWhenShipCapReached(t *testing.T) {
	s := &RunState{RunID: "r1", ShippedCount: ShipCap, ReviewState: ReviewStatePending}
	out, err := s.BytesWithCapsExceeded()
	if err != nil {
		t.Fatalf("BytesWithCapsExceeded: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if v, _ := m["caps_exceeded"].(string); v != s.CapsExceeded() {
		t.Errorf("caps_exceeded = %v, want %q", m["caps_exceeded"], s.CapsExceeded())
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
