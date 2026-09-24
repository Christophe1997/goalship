package gitops

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// fakeGHSweep installs a fake `gh` that answers `gh pr list ...` with
// openPRsJSON and `gh pr view <ref> ...` by looking ref up in prStates,
// echoing its value — an unrecognized ref exits 1, simulating a failed
// lookup. Mirrors reconcile_test.go's fakeGH, adapted for SweepBranches'
// two distinct host-tool calls (list the open-PR graph, then view each
// candidate's own PR) within a single test.
func fakeGHSweep(t *testing.T, openPRsJSON string, prStates map[string]string) {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, "case \"$2\" in\n  list) echo '%s' ;;\n  view)\n    case \"$3\" in\n", openPRsJSON)
	for ref, state := range prStates {
		fmt.Fprintf(&b, "      %s) echo %s ;;\n", ref, state)
	}
	b.WriteString("      *) exit 1 ;;\n    esac\n    ;;\nesac\n")
	withFakeHostTool(t, "gh", b.String())
}

// pushBranchFixture creates branch off main, commits a file, and pushes it
// to origin — the shape a real goalship-managed, since-merged branch left
// behind on origin.
func pushBranchFixture(t *testing.T, repoRoot, branch string) {
	t.Helper()
	createBranch(t, repoRoot, branch, "main")
	writeFile(t, filepath.Join(repoRoot, "f.txt"), "x\n")
	commitAll(t, repoRoot, "feat: "+branch)
	if err := PushBranch(repoRoot, branch); err != nil {
		t.Fatalf("PushBranch(%s): %v", branch, err)
	}
}

func TestSweepBranches_ReportOnly_MergedNotInOpenGraph_WouldDelete(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "shipped")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/sweep-me\npr: PR1")
	tkClose(t, repoRoot, ticketID)
	pushBranchFixture(t, repoRoot, "feat/sweep-me")
	fakeGHSweep(t, "[]", map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	want := []SweepCandidate{{TicketID: ticketID, Branch: "feat/sweep-me", PRRef: "PR1", Outcome: "would-delete"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}

	// Report-only: no delete call was ever made, so origin still has it.
	out := runOK(t, repoRoot, "git", "ls-remote", "origin", "feat/sweep-me")
	if strings.TrimSpace(out) == "" {
		t.Errorf("origin no longer has feat/sweep-me after a report-only run")
	}
}

func TestSweepBranches_MergedButReusedAsOpenPRHead_NotEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "reused branch")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/reused\npr: PR1")
	tkClose(t, repoRoot, ticketID)

	openPRsJSON := `[{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"main","headRefName":"feat/reused"}]`
	fakeGHSweep(t, openPRsJSON, map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want none: branch is a currently open PR's head", got)
	}
}

func TestSweepBranches_MergedButIsOpenPRBase_NotEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "base in use")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/base-in-use\npr: PR1")
	tkClose(t, repoRoot, ticketID)

	openPRsJSON := `[{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"feat/base-in-use","headRefName":"feat/child"}]`
	fakeGHSweep(t, openPRsJSON, map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want none: branch is a currently open PR's base", got)
	}
}

func TestSweepBranches_BranchNoteWithoutPRNote_NotEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "no pr note")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/no-pr")
	tkClose(t, repoRoot, ticketID)

	fakeGHSweep(t, "[]", nil)

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want none: no pr note recorded", got)
	}
}

func TestSweepBranches_PRStateOpen_NotEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "still open")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/still-open\npr: PR1")

	fakeGHSweep(t, "[]", map[string]string{"PR1": "OPEN"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want none: PR state is open, not merged", got)
	}
}

func TestSweepBranches_PRStateClosed_NotEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "closed without merge")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/closed-unmerged\npr: PR1")
	tkClose(t, repoRoot, ticketID)

	fakeGHSweep(t, "[]", map[string]string{"PR1": "CLOSED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want none: PR closed without merging", got)
	}
}

// TestSweepBranches_PRStateLookupFails_SkipsThatCandidateOnly proves a
// failed PRState lookup (ok == false) on one candidate is not a hard
// failure for the whole run: it's simply not eligible, and every other
// candidate is still evaluated and returned.
func TestSweepBranches_PRStateLookupFails_SkipsThatCandidateOnly(t *testing.T) {
	repoRoot := newTestRepo(t)
	failTicket := tkCreate(t, repoRoot, "lookup fails")
	tkStart(t, repoRoot, failTicket)
	tkAddNote(t, repoRoot, failTicket, "branch: feat/lookup-fails\npr: PRFAIL")
	tkClose(t, repoRoot, failTicket)

	okTicket := tkCreate(t, repoRoot, "lookup ok")
	tkStart(t, repoRoot, okTicket)
	tkAddNote(t, repoRoot, okTicket, "branch: feat/lookup-ok\npr: PROK")
	tkClose(t, repoRoot, okTicket)

	fakeGHSweep(t, "[]", map[string]string{"PROK": "MERGED"}) // PRFAIL unrecognized -> lookup failure

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	want := []SweepCandidate{{TicketID: okTicket, Branch: "feat/lookup-ok", PRRef: "PROK", Outcome: "would-delete"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

// TestSweepBranches_MalformedOpenPRGraph_HardFailsBeforeEvaluatingCandidates
// proves the open-PR graph must validate before any ticket is evaluated: a
// self-referential PR (a ValidateOpenPRGraph error) must hard-fail the
// whole call even though a real ticket's own PR would otherwise resolve
// eligible. The fake gh answers PR1 with MERGED specifically so a bug that
// evaluates candidates before validating the graph would make this ticket
// eligible instead of erroring.
func TestSweepBranches_MalformedOpenPRGraph_HardFailsBeforeEvaluatingCandidates(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "would be eligible if reached")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/trap\npr: PR1")
	tkClose(t, repoRoot, ticketID)

	openPRsJSON := `[{"number":9,"url":"https://github.com/o/r/pull/9","baseRefName":"feat/self","headRefName":"feat/self"}]`
	fakeGHSweep(t, openPRsJSON, map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err == nil {
		t.Fatal("SweepBranches: want an error for a malformed open PR graph, got nil")
	}
	if !strings.Contains(err.Error(), "self-referential") {
		t.Errorf("err = %v, want an error naming the self-referential PR", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil on a hard graph-validation failure", got)
	}
}

func TestSweepBranches_Execute_DeletesEligibleBranchFromOrigin(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "execute me")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/execute-me\npr: PR1")
	tkClose(t, repoRoot, ticketID)
	pushBranchFixture(t, repoRoot, "feat/execute-me")
	fakeGHSweep(t, "[]", map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", true)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	want := []SweepCandidate{{TicketID: ticketID, Branch: "feat/execute-me", PRRef: "PR1", Outcome: "deleted"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}

	out := runOK(t, repoRoot, "git", "ls-remote", "origin", "feat/execute-me")
	if strings.TrimSpace(out) != "" {
		t.Errorf("origin still has feat/execute-me after execute-mode delete: %q", out)
	}
}

// TestSweepBranches_Execute_OneDeleteFailsOtherStillDeletes proves a
// per-candidate delete failure doesn't stop the batch: one branch was
// really pushed to origin (its delete succeeds); the other only ever
// exists in ticket-note data, so `git push origin --delete` on it fails
// realistically, without any fake git.
func TestSweepBranches_Execute_OneDeleteFailsOtherStillDeletes(t *testing.T) {
	repoRoot := newTestRepo(t)

	pushedTicket := tkCreate(t, repoRoot, "pushed branch")
	tkStart(t, repoRoot, pushedTicket)
	tkAddNote(t, repoRoot, pushedTicket, "branch: feat/pushed\npr: PR-PUSHED")
	tkClose(t, repoRoot, pushedTicket)
	pushBranchFixture(t, repoRoot, "feat/pushed")

	missingTicket := tkCreate(t, repoRoot, "never pushed")
	tkStart(t, repoRoot, missingTicket)
	tkAddNote(t, repoRoot, missingTicket, "branch: feat/never-pushed\npr: PR-MISSING")
	tkClose(t, repoRoot, missingTicket)

	fakeGHSweep(t, "[]", map[string]string{"PR-PUSHED": "MERGED", "PR-MISSING": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", true)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got = %+v, want exactly 2 candidates", got)
	}
	byTicket := make(map[string]SweepCandidate, len(got))
	for _, c := range got {
		byTicket[c.TicketID] = c
	}

	pushed := byTicket[pushedTicket]
	if pushed.Outcome != "deleted" || pushed.Error != "" {
		t.Errorf("pushed candidate = %+v, want Outcome deleted with no error", pushed)
	}
	missing := byTicket[missingTicket]
	if missing.Outcome != "failed" || missing.Error == "" {
		t.Errorf("missing candidate = %+v, want Outcome failed with a non-empty error", missing)
	}
}
