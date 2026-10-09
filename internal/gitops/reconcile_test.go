package gitops

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/ticket"
)

// fakeGH installs a fake `gh` on PATH (via withFakeHostTool) that answers
// `gh auth status` with authExit and `gh pr view <ref> ...` by looking ref
// up in prStates, echoing its value — an unrecognized ref exits 1,
// simulating a failed lookup. Reconcile's own auth check and its per-ticket
// PR-state checks need to diverge within a single test (e.g. auth succeeds
// but one ticket's own PR lookup fails), so a single blanket exit code
// isn't enough.
func fakeGH(t *testing.T, authExit int, prStates map[string]string) {
	t.Helper()
	script := fmt.Sprintf("case \"$1\" in\n  auth) exit %d ;;\n  pr)\n%s    ;;\nesac\n", authExit, prStateCaseBlock(prStates))
	withFakeHostTool(t, "gh", script)
}

// pathWithoutHostTools returns a PATH value with git's own directory but no
// gh/glab anywhere on it — used to prove reconcile's needs_host_lookup guard
// actually skips host-tool detection, and to simulate "neither tool is
// installed" for auth_failure.
func pathWithoutHostTools(t *testing.T) string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath git: %v", err)
	}
	return strings.Join([]string{filepath.Dir(gitPath), "/bin"}, string(os.PathListSeparator))
}

func ticketStatus(t *testing.T, repoRoot, ticketID string) string {
	t.Helper()
	matches, err := queryTickets(repoRoot, fmt.Sprintf(`select(.id=="%s")`, ticketID))
	if err != nil {
		t.Fatalf("queryTickets: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("queryTickets(%s) = %d matches, want 1", ticketID, len(matches))
	}
	status, _ := matches[0]["status"].(string)
	return status
}

func requireNote(t *testing.T, repoRoot, ticketID, want string) {
	t.Helper()
	notes, err := readNotes(repoRoot, ticketID)
	if err != nil {
		t.Fatalf("readNotes: %v", err)
	}
	if !containsString(notes, want) {
		t.Errorf("notes = %q, want one equal to %q", notes, want)
	}
}

func TestReconcile_ClosedMerged(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "shipped")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	fakeGH(t, 0, map[string]string{"PR1": "MERGED"})

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if report.AuthFailure != "" {
		t.Fatalf("AuthFailure = %q, want none", report.AuthFailure)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeClosedMerged, Detail: "PR1"}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}
	if got := ticketStatus(t, repoRoot, ticketID); got != "closed" {
		t.Errorf("ticket status = %q, want closed", got)
	}
	requireNote(t, repoRoot, ticketID, "Reconciliation: PR PR1 merged externally; closing.")
}

func TestReconcile_FailedClosedUnmerged(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "pr closed")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	fakeGH(t, 0, map[string]string{"PR1": "CLOSED"})

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeFailedClosedUnmerged, Detail: "PR1"}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}
	if got := ticketStatus(t, repoRoot, ticketID); got != "open" {
		t.Errorf("ticket status = %q, want open (reopened)", got)
	}
	requireNote(t, repoRoot, ticketID, "Reconciliation: PR PR1 closed without merging; left open.")
}

func TestReconcile_NoRecoverableState_SkipsHostLookup(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "no state at all")
	tkStart(t, repoRoot, ticketID)

	t.Setenv("PATH", pathWithoutHostTools(t))

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if report.AuthFailure != "" {
		t.Fatalf("AuthFailure = %q, want none — no ticket carries pr/branch, so host-tool detection must never run", report.AuthFailure)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeNoRecoverableState}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}
}

func TestReconcile_RetryPRCreation(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "claimed, no pr yet")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x")
	fakeGH(t, 0, nil)

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeRetryPRCreation, Detail: "feat/x"}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}
}

// TestReconcile_BranchOnlyTicket_SkipsHostLookup: a ticket claimed but never
// shipped has no PR to look up, so a missing or unauthenticated gh/glab must
// not abort the whole run before its retry_pr_creation is reported.
func TestReconcile_BranchOnlyTicket_SkipsHostLookup(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "claimed, no pr yet")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x")

	t.Setenv("PATH", pathWithoutHostTools(t))

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if report.AuthFailure != "" {
		t.Fatalf("AuthFailure = %q, want none — a branch-only ticket needs no host lookup", report.AuthFailure)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeRetryPRCreation, Detail: "feat/x"}
	if len(report.Actions) != 1 || report.Actions[0] != want {
		t.Errorf("Actions = %+v, want exactly [%+v]", report.Actions, want)
	}
}

func TestReconcile_RetargetBaseMerged(t *testing.T) {
	repoRoot := newTestRepo(t)
	baseTicket := tkCreate(t, repoRoot, "base ticket")
	tkAddNote(t, repoRoot, baseTicket, "branch: feat/base\npr: PRBASE")
	tkClose(t, repoRoot, baseTicket) // merged bases are normally already closed

	ticketID := tkCreate(t, repoRoot, "stacked ticket")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/stacked\npr: PR1\nbase: feat/base")

	fakeGH(t, 0, map[string]string{"PR1": "OPEN", "PRBASE": "MERGED"})

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeRetargetBaseMerged, Detail: "feat/base", PRRef: "PR1"}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}
	if got := ticketStatus(t, repoRoot, ticketID); got != "in_progress" {
		t.Errorf("ticket status = %q, want unchanged in_progress — retargeting doesn't close this ticket's own lifecycle", got)
	}
}

func TestReconcile_BlockedStaleBase(t *testing.T) {
	repoRoot := newTestRepo(t)
	baseTicket := tkCreate(t, repoRoot, "base ticket")
	tkAddNote(t, repoRoot, baseTicket, "branch: feat/base\npr: PRBASE")
	tkClose(t, repoRoot, baseTicket)

	ticketID := tkCreate(t, repoRoot, "stacked ticket")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/stacked\npr: PR1\nbase: feat/base")

	fakeGH(t, 0, map[string]string{"PR1": "OPEN", "PRBASE": "CLOSED"})

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeBlockedStaleBase, Detail: "feat/base"}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}

	requireNote(t, repoRoot, ticketID, "Reconciliation: base feat/base closed without merging; blocked.")
	if got := ticketStatus(t, repoRoot, ticketID); got != "in_progress" {
		t.Errorf("ticket status = %q, want unchanged in_progress — blocked tickets stay put, excluded from tk ready by their own unresolved base", got)
	}
}

func TestReconcile_ClosedShipNoteOrphaned(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "crashed after ship note")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/y\npr: PR1\nsha: deadbeef")
	fakeGH(t, 0, map[string]string{"PR1": "OPEN"})

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomeClosedShipNoteOrphaned, Detail: "feat/y", PRRef: "PR1"}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}
	if got := ticketStatus(t, repoRoot, ticketID); got != "closed" {
		t.Errorf("ticket status = %q, want closed", got)
	}
}

func TestReconcile_PRStateUnresolved(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "lookup fails")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	fakeGH(t, 0, nil) // auth succeeds, but PR1 is unrecognized -> lookup fails

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	want := ReconciliationAction{TicketID: ticketID, Outcome: OutcomePRStateUnresolved, Detail: "PR1"}
	if report.Actions[0] != want {
		t.Errorf("action = %+v, want %+v", report.Actions[0], want)
	}
}

// TestReconcile_HealthyOpenPR_NoAction proves a genuinely healthy ticket —
// PR open, no base, no ship-note sha — produces no action at all: Actions
// can be shorter than the in-progress set, "one per ticket touched" rather
// than "one per ticket examined".
func TestReconcile_HealthyOpenPR_NoAction(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "healthy")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	fakeGH(t, 0, map[string]string{"PR1": "OPEN"})

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 0 {
		t.Fatalf("Actions = %v, want none", report.Actions)
	}
}

// TestReconcile_StackedBaseMerged_WinsOverShipNoteOrphan pins the
// precedence between the two "open PR" sub-checks: a ticket with both a
// merged base AND a ship-note sha reports retarget_base_merged, not
// closed_ship_note_orphaned — reconcile only falls through to the sha check
// when the stacked-base check found nothing stale.
func TestReconcile_StackedBaseMerged_WinsOverShipNoteOrphan(t *testing.T) {
	repoRoot := newTestRepo(t)
	baseTicket := tkCreate(t, repoRoot, "base ticket")
	tkAddNote(t, repoRoot, baseTicket, "branch: feat/base\npr: PRBASE")
	tkClose(t, repoRoot, baseTicket)

	ticketID := tkCreate(t, repoRoot, "stacked and ship-noted")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/stacked\npr: PR1\nbase: feat/base\nsha: deadbeef")

	fakeGH(t, 0, map[string]string{"PR1": "OPEN", "PRBASE": "MERGED"})

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if len(report.Actions) != 1 {
		t.Fatalf("Actions = %v, want exactly 1", report.Actions)
	}
	if got := report.Actions[0].Outcome; got != OutcomeRetargetBaseMerged {
		t.Errorf("outcome = %q, want %q (stacked-base check wins over the ship-note-orphan fallback)", got, OutcomeRetargetBaseMerged)
	}
}

func TestReconcile_AuthFailure_NoHostToolOnPath(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "needs a host tool")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")

	t.Setenv("PATH", pathWithoutHostTools(t))

	report, err := Reconcile(repoRoot)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if report.AuthFailure != "gh/glab" {
		t.Errorf("AuthFailure = %q, want %q", report.AuthFailure, "gh/glab")
	}
	if len(report.Actions) != 0 {
		t.Errorf("Actions = %v, want none — no ticket is processed on an auth failure", report.Actions)
	}
}

// TestReconcile_AuthFailure_BadCredential_ResurfacesEveryCall proves
// auth_failure surfaces non-null every time the same broken credential is
// hit — reconcile itself holds no state to latch or suppress a repeat.
func TestReconcile_AuthFailure_BadCredential_ResurfacesEveryCall(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "needs a host tool")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	fakeGH(t, 1, nil) // `gh auth status` fails every time

	for i := 0; i < 2; i++ {
		report, err := Reconcile(repoRoot)
		if err != nil {
			t.Fatalf("Reconcile call %d: %v", i, err)
		}
		if report.AuthFailure != "gh" {
			t.Errorf("call %d: AuthFailure = %q, want %q", i, report.AuthFailure, "gh")
		}
		if len(report.Actions) != 0 {
			t.Errorf("call %d: Actions = %v, want none", i, report.Actions)
		}
	}
}

func TestReconcile_WritesLandInTicketsDirOverride(t *testing.T) {
	repoRoot := newTestRepo(t)
	override := filepath.Join(t.TempDir(), "elsewhere")
	t.Setenv(ticket.TicketsDirEnv, override)
	ticketID := tkCreate(t, repoRoot, "shipped")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	fakeGH(t, 0, map[string]string{"PR1": "MERGED"})

	if _, err := Reconcile(repoRoot); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	loaded, err := ticket.Load(filepath.Join(override, ticketID+".md"))
	if err != nil {
		t.Fatalf("Load from override: %v", err)
	}
	if loaded.Status != "closed" || !strings.Contains(loaded.Body, "merged externally; closing.") {
		t.Errorf("override ticket status %q, body %q; want closed with the reconciliation note", loaded.Status, loaded.Body)
	}
	if _, err := os.Stat(filepath.Join(repoRoot, ".tickets")); !os.IsNotExist(err) {
		t.Errorf("repoRoot/.tickets exists (err=%v); reconcile must not write outside the override", err)
	}
}

func TestReconcile_NoteCarriesUTCTimestampMarker(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "shipped")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x\npr: PR1")
	fakeGH(t, 0, map[string]string{"PR1": "MERGED"})

	if _, err := Reconcile(repoRoot); err != nil {
		t.Fatalf("Reconcile: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(ticket.ResolveTicketsDir(repoRoot), ticketID+".md"))
	if err != nil {
		t.Fatalf("read ticket: %v", err)
	}
	marker := regexp.MustCompile(`(?m)^\*\*\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z\*\*\n\nReconciliation: PR PR1 merged externally; closing\.$`)
	if !marker.Match(raw) {
		t.Errorf("ticket file lacks a timestamp-marked reconciliation note:\n%s", raw)
	}
}

func TestReconcile_MalformedTicketWriteFails_LeavesFileUnchanged(t *testing.T) {
	repoRoot := newTestRepo(t)
	writeMalformedTicket(t, repoRoot, "bad-1")
	path := filepath.Join(ticket.ResolveTicketsDir(repoRoot), "bad-1.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fakeGH(t, 0, map[string]string{"PR9": "MERGED"})

	_, err = Reconcile(repoRoot)
	if err == nil {
		t.Fatal("Reconcile: want an error when the ticket to close cannot be loaded, got nil")
	}
	if !strings.Contains(err.Error(), "bad-1") {
		t.Errorf("error = %q, want it to name the ticket", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("failed reconcile changed the ticket file:\n%s", after)
	}
}
