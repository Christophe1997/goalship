package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	var b strings.Builder
	fmt.Fprintf(&b, "case \"$1\" in\n  auth) exit %d ;;\n  pr)\n    case \"$3\" in\n", authExit)
	for ref, state := range prStates {
		fmt.Fprintf(&b, "      %s) echo %s ;;\n", ref, state)
	}
	b.WriteString("      *) exit 1 ;;\n    esac\n    ;;\nesac\n")
	withFakeHostTool(t, "gh", b.String())
}

// pathWithoutHostTools returns a PATH value with a real `tk` (symlinked
// into an isolated directory, since tk and gh/glab share a bin directory on
// this machine) plus git's own directory, but no gh/glab anywhere on it —
// used to prove reconcile's needs_host_lookup guard actually skips host-tool
// detection, and to simulate "neither tool is installed" for auth_failure.
func pathWithoutHostTools(t *testing.T) string {
	t.Helper()
	tkPath, err := exec.LookPath("tk")
	if err != nil {
		t.Fatalf("LookPath tk: %v", err)
	}
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatalf("LookPath git: %v", err)
	}
	dir := t.TempDir()
	if err := os.Symlink(tkPath, filepath.Join(dir, "tk")); err != nil {
		t.Fatalf("symlink tk: %v", err)
	}
	return strings.Join([]string{dir, filepath.Dir(gitPath), "/bin"}, string(os.PathListSeparator))
}

func ticketStatus(t *testing.T, repoRoot, ticketID string) string {
	t.Helper()
	matches, err := tkQuery(repoRoot, fmt.Sprintf(`select(.id=="%s")`, ticketID))
	if err != nil {
		t.Fatalf("tkQuery: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("tkQuery(%s) = %d matches, want 1", ticketID, len(matches))
	}
	status, _ := matches[0]["status"].(string)
	return status
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

	notes, err := tkShowNotes(repoRoot, ticketID)
	if err != nil {
		t.Fatalf("tkShowNotes: %v", err)
	}
	found := false
	for _, n := range notes {
		if strings.Contains(n, "base feat/base closed without merging") {
			found = true
		}
	}
	if !found {
		t.Errorf("notes = %v, want one mentioning the stale base", notes)
	}
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
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x")

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
	tkAddNote(t, repoRoot, ticketID, "branch: feat/x")
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
