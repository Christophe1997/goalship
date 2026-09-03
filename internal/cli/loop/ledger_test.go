package loop

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

type ledgerJSON struct {
	RunID               string   `json:"run_id"`
	ShippedCount        int      `json:"shipped_count"`
	ConsecutiveFailures int      `json:"consecutive_failures"`
	ClaimedTicketIDs    []string `json:"claimed_ticket_ids"`
	Goal                string   `json:"goal"`
	TicketMode          *string  `json:"ticket_mode"`
	TerminalState       *string  `json:"terminal_state"`
	TrunkBranch         *string  `json:"trunk_branch"`
	ReviewState         string   `json:"review_state"`
	CapsExceeded        *string  `json:"caps_exceeded"`
}

func decodeLedgerJSON(t *testing.T, out string) ledgerJSON {
	t.Helper()
	var got ledgerJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	return got
}

func TestLedgerCmd_FreshRun_DefaultsAndExcludeFile(t *testing.T) {
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewLedgerCmd(), []string{repoRoot})
	got := decodeLedgerJSON(t, out)

	if got.RunID == "" {
		t.Error("run_id is empty, want a generated id")
	}
	if got.ShippedCount != 0 || got.ConsecutiveFailures != 0 {
		t.Errorf("fresh run not zeroed: %+v", got)
	}
	if got.ReviewState != "pending" {
		t.Errorf("review_state = %q, want \"pending\"", got.ReviewState)
	}
	if got.CapsExceeded != nil {
		t.Errorf("caps_exceeded = %v, want null", got.CapsExceeded)
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".git", "info", "exclude")); err != nil {
		t.Errorf(".git/info/exclude not created: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(repoRoot, ".git", "info", "exclude"))
	if err != nil {
		t.Fatalf("read exclude file: %v", err)
	}
	// `git init` seeds this file with its own template comments, so assert
	// containment rather than exact content (EnsureExcluded's own
	// idempotency/exact-content behavior is covered directly by
	// internal/ledger's TestEnsureExcluded_* suite).
	if !bytes.Contains(data, []byte("/.goalship/\n")) {
		t.Errorf("exclude file content = %q, want it to contain %q", data, "/.goalship/\n")
	}

	if _, err := os.Stat(filepath.Join(repoRoot, ".goalship", got.RunID+".json")); err != nil {
		t.Errorf("ledger file not written: %v", err)
	}
}

func TestLedgerCmd_RunIDPersistsAcrossInvocations(t *testing.T) {
	repoRoot := newLoopTestRepo(t)

	first := decodeLedgerJSON(t, execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "fixed-run", "--goal", "ship things"}))
	if first.RunID != "fixed-run" || first.Goal != "ship things" {
		t.Fatalf("first call = %+v", first)
	}

	second := decodeLedgerJSON(t, execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "fixed-run", "--claim", "t-1"}))
	if second.Goal != "ship things" {
		t.Errorf("goal not preserved across invocations: %+v", second)
	}
	if len(second.ClaimedTicketIDs) != 1 || second.ClaimedTicketIDs[0] != "t-1" {
		t.Errorf("claimed_ticket_ids = %v, want [t-1]", second.ClaimedTicketIDs)
	}
}

func TestLedgerCmd_ClaimIsIdempotentAcrossInvocations(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--claim", "t-1"})
	got := decodeLedgerJSON(t, execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--claim", "t-1"}))
	if len(got.ClaimedTicketIDs) != 1 {
		t.Errorf("claimed_ticket_ids = %v, want exactly one entry", got.ClaimedTicketIDs)
	}
}

func TestLedgerCmd_ShipResetsConsecutiveFailures(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--fail"})
	execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--fail"})
	got := decodeLedgerJSON(t, execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--ship"}))
	if got.ShippedCount != 1 {
		t.Errorf("shipped_count = %d, want 1", got.ShippedCount)
	}
	if got.ConsecutiveFailures != 0 {
		t.Errorf("consecutive_failures = %d, want 0 after a ship", got.ConsecutiveFailures)
	}
}

func TestLedgerCmd_CapsExceededReflectsFailureCap(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--fail"})
	execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--fail"})
	got := decodeLedgerJSON(t, execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--fail"}))
	if got.CapsExceeded == nil {
		t.Fatal("caps_exceeded = nil, want a reason after 3 consecutive failures")
	}
	if *got.CapsExceeded != "consecutive-failure cap reached (3 failures in a row)" {
		t.Errorf("caps_exceeded = %q", *got.CapsExceeded)
	}
}

func TestLedgerCmd_TicketModeAndTrunkBranchOverwrite(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	got := decodeLedgerJSON(t, execCmd(t, NewLedgerCmd(), []string{
		repoRoot, "--run-id", "r1", "--ticket-mode", "commit", "--trunk-branch", "develop",
	}))
	if got.TicketMode == nil || *got.TicketMode != "commit" {
		t.Errorf("ticket_mode = %v, want \"commit\"", got.TicketMode)
	}
	if got.TrunkBranch == nil || *got.TrunkBranch != "develop" {
		t.Errorf("trunk_branch = %v, want \"develop\"", got.TrunkBranch)
	}
}

func TestLedgerCmd_InvalidTicketModeErrors(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	cmd := NewLedgerCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{repoRoot, "--ticket-mode", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for an invalid --ticket-mode value")
	}
}

func TestLedgerCmd_InvalidTerminalErrors(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	cmd := NewLedgerCmd()
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{repoRoot, "--terminal", "bogus"})
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected an error for an invalid --terminal value")
	}
}

func TestLedgerCmd_TerminalMarksRunFinished(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	got := decodeLedgerJSON(t, execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "r1", "--terminal", "user_stop"}))
	if got.TerminalState == nil || *got.TerminalState != "user_stop" {
		t.Errorf("terminal_state = %v, want \"user_stop\"", got.TerminalState)
	}
}

// TestLedgerCmd_NoFlagCanSetReviewGateFields is R12's structural assertion:
// `goalship loop ledger` is the one command capable of bypassing claim's
// R16 approval gate if it ever grew a flag that wrote review_state or
// approved_ticket_ids directly. It asserts against NewLedgerCmd()'s actual
// *pflag.FlagSet — not by invoking the command with every flag combination
// and inspecting the ledger file afterward — so it fails the moment such a
// flag is registered, before it ever runs.
func TestLedgerCmd_NoFlagCanSetReviewGateFields(t *testing.T) {
	cmd := NewLedgerCmd()
	flags := cmd.Flags()

	for _, name := range []string{
		"review-state", "review_state", "reviewstate",
		"approved-ticket-ids", "approved_ticket_ids", "approvedticketids",
		"approved-tickets", "approve",
	} {
		if flags.Lookup(name) != nil {
			t.Errorf("cmd.Flags().Lookup(%q) is registered; no flag may set review_state or approved_ticket_ids", name)
		}
	}

	// Defensive net for any other spelling: no registered flag's name may
	// even mention "review" or "approv" (approve/approved/approval).
	flags.VisitAll(func(f *pflag.Flag) {
		lower := strings.ToLower(f.Name)
		if strings.Contains(lower, "review") || strings.Contains(lower, "approv") {
			t.Errorf("flag %q looks capable of touching the review gate; ledger must never expose one", f.Name)
		}
	})

	// The exact flag set this command exposes today — a whitelist, so a
	// newly added flag of any name forces a deliberate look at this test
	// rather than silently passing the substring guard above.
	want := map[string]bool{
		"run-id": true, "claim": true, "ship": true, "fail": true,
		"goal": true, "ticket-mode": true, "trunk-branch": true, "terminal": true,
	}
	got := map[string]bool{}
	flags.VisitAll(func(f *pflag.Flag) { got[f.Name] = true })
	if len(got) != len(want) {
		t.Fatalf("flag set = %v, want exactly %v", got, want)
	}
	for name := range want {
		if !got[name] {
			t.Errorf("expected flag %q not registered", name)
		}
	}
}
