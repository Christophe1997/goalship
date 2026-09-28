package gitops

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// fakeGHListFails as a fakeGHSweepLists response makes that `gh pr list`
// call exit non-zero.
const fakeGHListFails = "list-fails"

// fakeGHSweepLists installs a fake `gh` whose nth `pr list` call answers
// lists[n-1] (the last entry repeats for every later call) and whose
// `pr view <ref>` looks ref up in prStates, echoing its value — an
// unrecognized ref exits 1, simulating a failed lookup. Every invocation
// appends "<subcommand> <arg>" to the returned callLog file. Mirrors
// reconcile_test.go's fakeGH, adapted for SweepBranches' distinct host-tool
// calls (list the open-PR graph, then view each candidate's own PR).
func fakeGHSweepLists(t *testing.T, lists []string, prStates map[string]string) (callLog string) {
	t.Helper()
	dir := t.TempDir()
	callLog = filepath.Join(dir, "calls")
	counter := filepath.Join(dir, "list-count")
	var listCases strings.Builder
	for i, list := range lists {
		pattern := strconv.Itoa(i + 1)
		if i == len(lists)-1 {
			pattern = "*"
		}
		response := fmt.Sprintf("echo '%s'", list)
		if list == fakeGHListFails {
			response = "exit 1"
		}
		fmt.Fprintf(&listCases, "      %s) %s ;;\n", pattern, response)
	}
	script := fmt.Sprintf(`echo "$2 $3" >> %[1]s
case "$2" in
  list)
    n=$(( $(cat %[2]s 2>/dev/null || echo 0) + 1 ))
    echo $n > %[2]s
    case $n in
%[3]s    esac
    ;;
  view)
%[4]s    ;;
esac
`, callLog, counter, listCases.String(), prStateCaseBlock(prStates))
	withFakeHostTool(t, "gh", script)
	return callLog
}

// fakeGHSweep is fakeGHSweepLists with one open-PR listing for every call.
func fakeGHSweep(t *testing.T, openPRsJSON string, prStates map[string]string) {
	t.Helper()
	fakeGHSweepLists(t, []string{openPRsJSON}, prStates)
}

// pushBranchFixture creates branch off main, commits a file, and pushes it
// to origin — the shape a real goalship-managed, since-merged branch left
// behind on origin. Stages only its own file: a blanket `git add -A` would
// commit the fixture's .tickets/ onto the branch, and the next fixture's
// checkout back to main would then delete those tickets from the worktree.
func pushBranchFixture(t *testing.T, repoRoot, branch string) {
	t.Helper()
	createBranch(t, repoRoot, branch, "main")
	writeFile(t, filepath.Join(repoRoot, "f.txt"), "x\n")
	runOK(t, repoRoot, "git", "add", "f.txt")
	runOK(t, repoRoot, "git", "commit", "-q", "-m", "feat: "+branch)
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
	want := []SweepCandidate{{TicketID: ticketID, Branch: "feat/sweep-me", PRRef: "PR1", Outcome: SweepOutcomeWouldDelete}}
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
	pushBranchFixture(t, repoRoot, "feat/reused")

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
	pushBranchFixture(t, repoRoot, "feat/base-in-use")

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

// A fork PR's base is a real branch in this repo: deleting it would close
// the fork PR, so it must stay protected even though the fork PR's head is
// not a branch here.
func TestSweepBranches_MergedButIsForkPRBase_NotEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "base of a fork pr")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/fork-target\npr: PR1")
	tkClose(t, repoRoot, ticketID)
	pushBranchFixture(t, repoRoot, "feat/fork-target")

	openPRsJSON := `[{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"feat/fork-target","headRefName":"contrib","isCrossRepository":true}]`
	fakeGHSweep(t, openPRsJSON, map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want none: branch is a fork PR's base", got)
	}
}

// A fork PR's head lives in the contributor's repo, so a same-named local
// ticket branch is unrelated to it.
func TestSweepBranches_MergedBranchNamedLikeForkPRHead_StillEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "name clash with fork head")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/clash\npr: PR1")
	tkClose(t, repoRoot, ticketID)
	pushBranchFixture(t, repoRoot, "feat/clash")

	openPRsJSON := `[{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"main","headRefName":"feat/clash","isCrossRepository":true}]`
	fakeGHSweep(t, openPRsJSON, map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	want := []SweepCandidate{{TicketID: ticketID, Branch: "feat/clash", PRRef: "PR1", Outcome: SweepOutcomeWouldDelete}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

func TestSweepBranches_ForkPRFromItsMainIntoMain_DoesNotHardFail(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "shipped")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/sweep-me\npr: PR1")
	tkClose(t, repoRoot, ticketID)
	pushBranchFixture(t, repoRoot, "feat/sweep-me")

	openPRsJSON := `[{"number":2,"url":"https://github.com/o/r/pull/2","baseRefName":"main","headRefName":"main","isCrossRepository":true}]`
	fakeGHSweep(t, openPRsJSON, map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 1 || got[0].Branch != "feat/sweep-me" {
		t.Errorf("got = %+v, want feat/sweep-me still evaluated", got)
	}
}

func TestSweepBranches_BranchNoteWithoutPRNote_NotEligible(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "no pr note")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/no-pr")
	tkClose(t, repoRoot, ticketID)
	pushBranchFixture(t, repoRoot, "feat/no-pr")

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
	pushBranchFixture(t, repoRoot, "feat/still-open")

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
	pushBranchFixture(t, repoRoot, "feat/closed-unmerged")

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
	pushBranchFixture(t, repoRoot, "feat/lookup-fails")

	okTicket := tkCreate(t, repoRoot, "lookup ok")
	tkStart(t, repoRoot, okTicket)
	tkAddNote(t, repoRoot, okTicket, "branch: feat/lookup-ok\npr: PROK")
	tkClose(t, repoRoot, okTicket)
	pushBranchFixture(t, repoRoot, "feat/lookup-ok")

	fakeGHSweep(t, "[]", map[string]string{"PROK": "MERGED"}) // PRFAIL unrecognized -> lookup failure

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	want := []SweepCandidate{{TicketID: okTicket, Branch: "feat/lookup-ok", PRRef: "PROK", Outcome: SweepOutcomeWouldDelete}}
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
	want := []SweepCandidate{{TicketID: ticketID, Branch: "feat/execute-me", PRRef: "PR1", Outcome: SweepOutcomeDeleted}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got = %+v, want %+v", got, want)
	}

	out := runOK(t, repoRoot, "git", "ls-remote", "origin", "feat/execute-me")
	if strings.TrimSpace(out) != "" {
		t.Errorf("origin still has feat/execute-me after execute-mode delete: %q", out)
	}
}

func originDir(repoRoot string) string {
	return filepath.Join(filepath.Dir(repoRoot), "origin.git")
}

// installOriginHook writes an executable hook script into repoRoot's bare
// origin. Install it after any fixture pushes it would otherwise reject.
func installOriginHook(t *testing.T, repoRoot, hook, body string) {
	t.Helper()
	path := filepath.Join(originDir(repoRoot), "hooks", hook)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatalf("write origin %s hook: %v", hook, err)
	}
}

// originHasBranch reads repoRoot's bare origin directly, so it stays usable
// after a test has repointed or broken the work repo's origin remote.
func originHasBranch(t *testing.T, repoRoot, branch string) bool {
	t.Helper()
	return strings.TrimSpace(runOK(t, originDir(repoRoot), "git", "for-each-ref", "refs/heads/"+branch)) != ""
}

// TestSweepBranches_Execute_OneDeleteFailsOtherStillDeletes proves a
// per-candidate delete failure doesn't stop the batch: both branches exist
// on origin, but an update hook there rejects changes to one of them, so
// exactly that delete fails while the other still succeeds.
func TestSweepBranches_Execute_OneDeleteFailsOtherStillDeletes(t *testing.T) {
	repoRoot := newTestRepo(t)
	pushBranchFixture(t, repoRoot, "feat/pushed")
	pushBranchFixture(t, repoRoot, "feat/protected")
	installOriginHook(t, repoRoot, "update", `[ "$1" = refs/heads/feat/protected ] && exit 1
exit 0`)

	pushedTicket := tkCreate(t, repoRoot, "pushed branch")
	tkStart(t, repoRoot, pushedTicket)
	tkAddNote(t, repoRoot, pushedTicket, "branch: feat/pushed\npr: PR-PUSHED")
	tkClose(t, repoRoot, pushedTicket)

	protectedTicket := tkCreate(t, repoRoot, "protected branch")
	tkStart(t, repoRoot, protectedTicket)
	tkAddNote(t, repoRoot, protectedTicket, "branch: feat/protected\npr: PR-PROTECTED")
	tkClose(t, repoRoot, protectedTicket)

	fakeGHSweep(t, "[]", map[string]string{"PR-PUSHED": "MERGED", "PR-PROTECTED": "MERGED"})

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
	if pushed.Outcome != SweepOutcomeDeleted || pushed.Error != "" {
		t.Errorf("pushed candidate = %+v, want Outcome deleted with no error", pushed)
	}
	protected := byTicket[protectedTicket]
	if protected.Outcome != SweepOutcomeFailed || protected.Error == "" {
		t.Errorf("protected candidate = %+v, want Outcome failed with a non-empty error", protected)
	}
	if originHasBranch(t, repoRoot, "feat/pushed") {
		t.Errorf("origin still has feat/pushed after its successful delete")
	}
	if !originHasBranch(t, repoRoot, "feat/protected") {
		t.Errorf("origin lost feat/protected despite its rejected delete")
	}
}

// shippedTicket records a merged-PR ticket for branch: the fixture every
// sweep test below needs once the branch itself is also on origin.
func shippedTicket(t *testing.T, repoRoot, branch, prRef string) string {
	t.Helper()
	id := tkCreate(t, repoRoot, "ships "+branch)
	tkStart(t, repoRoot, id)
	tkAddNote(t, repoRoot, id, "branch: "+branch+"\npr: "+prRef)
	tkClose(t, repoRoot, id)
	return id
}

// A PR opened onto an already-eligible branch after the initial open-PR
// snapshot is invisible to it; the delete would silently close that PR, so
// each delete is preceded by a fresh listing.
func TestSweepBranches_Execute_RechecksOpenPRsBeforeEachDelete(t *testing.T) {
	tests := []struct {
		name        string
		relist      string
		wantOutcome string
		wantErr     string
	}{
		{"new PR based on the branch", `[{"number":7,"url":"u","baseRefName":"feat/target","headRefName":"feat/child"}]`, SweepOutcomeFailed, "#7"},
		{"new PR headed by the branch", `[{"number":7,"url":"u","baseRefName":"main","headRefName":"feat/target"}]`, SweepOutcomeFailed, "#7"},
		{"new fork PR based on the branch", `[{"number":7,"url":"u","baseRefName":"feat/target","headRefName":"contrib","isCrossRepository":true}]`, SweepOutcomeFailed, "#7"},
		{"new fork PR whose head is only named like the branch", `[{"number":7,"url":"u","baseRefName":"main","headRefName":"feat/target","isCrossRepository":true}]`, SweepOutcomeDeleted, ""},
		{"re-listing fails", fakeGHListFails, SweepOutcomeFailed, "re-listing open PRs"},
		{"fresh graph is malformed", `[{"number":9,"url":"u","baseRefName":"feat/self","headRefName":"feat/self"}]`, SweepOutcomeFailed, "self-referential"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repoRoot := newTestRepo(t)
			pushBranchFixture(t, repoRoot, "feat/target")
			ticketID := shippedTicket(t, repoRoot, "feat/target", "PR1")
			fakeGHSweepLists(t, []string{"[]", tt.relist}, map[string]string{"PR1": "MERGED"})

			got, err := SweepBranches(repoRoot, "gh", true)
			if err != nil {
				t.Fatalf("SweepBranches: %v", err)
			}
			if len(got) != 1 || got[0].TicketID != ticketID || got[0].Outcome != tt.wantOutcome {
				t.Fatalf("got = %+v, want one %s candidate for ticket %s", got, tt.wantOutcome, ticketID)
			}
			if tt.wantErr == "" && got[0].Error != "" {
				t.Errorf("Error = %q, want none", got[0].Error)
			}
			if !strings.Contains(got[0].Error, tt.wantErr) {
				t.Errorf("Error = %q, want it to contain %q", got[0].Error, tt.wantErr)
			}
			if wantOnOrigin := tt.wantOutcome != SweepOutcomeDeleted; originHasBranch(t, repoRoot, "feat/target") != wantOnOrigin {
				t.Errorf("origin has feat/target = %v, want %v", !wantOnOrigin, wantOnOrigin)
			}
		})
	}
}

func TestSweepBranches_Execute_BlockedDeleteDoesNotStopTheBatch(t *testing.T) {
	repoRoot := newTestRepo(t)
	pushBranchFixture(t, repoRoot, "feat/blocked")
	pushBranchFixture(t, repoRoot, "feat/free")
	blocked := shippedTicket(t, repoRoot, "feat/blocked", "PR-BLOCKED")
	free := shippedTicket(t, repoRoot, "feat/free", "PR-FREE")
	const basedOnBlocked = `[{"number":7,"url":"u","baseRefName":"feat/blocked","headRefName":"feat/child"}]`
	fakeGHSweepLists(t, []string{"[]", basedOnBlocked}, map[string]string{"PR-BLOCKED": "MERGED", "PR-FREE": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", true)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	outcomes := map[string]string{}
	for _, c := range got {
		outcomes[c.TicketID] = c.Outcome
	}
	want := map[string]string{blocked: SweepOutcomeFailed, free: SweepOutcomeDeleted}
	if !reflect.DeepEqual(outcomes, want) {
		t.Errorf("outcomes = %v, want %v", outcomes, want)
	}
	if !originHasBranch(t, repoRoot, "feat/blocked") || originHasBranch(t, repoRoot, "feat/free") {
		t.Errorf("origin branches wrong: want feat/blocked kept and feat/free deleted")
	}
}

// An already-swept ticket keeps its branch and pr notes forever; without an
// existence check it would resurface on every later run and bury real
// failures. The check must also precede the host-tool lookup.
func TestSweepBranches_BranchAbsentFromOrigin_NotACandidate(t *testing.T) {
	for _, execute := range []bool{false, true} {
		t.Run(fmt.Sprintf("execute=%v", execute), func(t *testing.T) {
			repoRoot := newTestRepo(t)
			shippedTicket(t, repoRoot, "feat/already-swept", "PR1")
			callLog := fakeGHSweepLists(t, []string{"[]"}, map[string]string{"PR1": "MERGED"})

			got, err := SweepBranches(repoRoot, "gh", execute)
			if err != nil {
				t.Fatalf("SweepBranches: %v", err)
			}
			if len(got) != 0 {
				t.Errorf("got = %+v, want none: branch is not on origin", got)
			}
			calls, _ := os.ReadFile(callLog)
			if strings.Contains(string(calls), "view") {
				t.Errorf("gh calls = %q, want no `pr view` for a branch absent from origin", calls)
			}
		})
	}
}

// git ls-remote treats its pattern as a glob, so a branch note naming
// "feat/*" must not count as present just because some other branch matches.
func TestSweepBranches_BranchNoteMatchingOnlyOtherOriginBranches_NotACandidate(t *testing.T) {
	repoRoot := newTestRepo(t)
	pushBranchFixture(t, repoRoot, "feat/real")
	shippedTicket(t, repoRoot, "feat/*", "PR1")
	fakeGHSweep(t, "[]", map[string]string{"PR1": "MERGED"})

	got, err := SweepBranches(repoRoot, "gh", false)
	if err != nil {
		t.Fatalf("SweepBranches: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got = %+v, want none: no branch named exactly feat/* exists on origin", got)
	}
}

// Failing to ask origin is not evidence the branch is gone: the sweep must
// stop rather than silently drop the candidate.
func TestSweepBranches_OriginUnreachable_IsHardError(t *testing.T) {
	repoRoot := newTestRepo(t)
	pushBranchFixture(t, repoRoot, "feat/target")
	shippedTicket(t, repoRoot, "feat/target", "PR1")
	fakeGHSweep(t, "[]", map[string]string{"PR1": "MERGED"})
	runOK(t, repoRoot, "git", "remote", "set-url", "origin", filepath.Join(t.TempDir(), "gone.git"))

	got, err := SweepBranches(repoRoot, "gh", false)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want the ls-remote *ExitError", err)
	}
	if got != nil {
		t.Errorf("got = %+v, want nil on a hard error", got)
	}
}

var errInjectedNoteRead = errors.New("injected note read failure")

// failNoteReadFromCall makes the nth and every later ticket note read fail.
// It counts calls rather than matching a ticket ID because the query's
// ticket order over generated IDs is not controllable.
func failNoteReadFromCall(t *testing.T, n int) {
	t.Helper()
	orig := readTicketFile
	t.Cleanup(func() { readTicketFile = orig })
	calls := 0
	readTicketFile = func(path string) ([]byte, error) {
		calls++
		if calls >= n {
			return nil, errInjectedNoteRead
		}
		return orig(path)
	}
}

// deletedFromOrigin returns those of branches origin no longer holds.
func deletedFromOrigin(t *testing.T, repoRoot string, branches ...string) []string {
	t.Helper()
	var deleted []string
	for _, branch := range branches {
		if !originHasBranch(t, repoRoot, branch) {
			deleted = append(deleted, branch)
		}
	}
	return deleted
}

// requireAbortAfterOneDelete checks err leads with the single branch the
// sweep managed to delete before aborting.
func requireAbortAfterOneDelete(t *testing.T, err error, repoRoot string, branches ...string) {
	t.Helper()
	deleted := deletedFromOrigin(t, repoRoot, branches...)
	if len(deleted) != 1 {
		t.Fatalf("branches deleted from origin = %v, want exactly one before the abort", deleted)
	}
	if err == nil {
		t.Fatal("SweepBranches: want an error, got nil")
	}
	if want := "gitops: sweep aborted after deleting " + deleted[0] + ": "; !strings.HasPrefix(err.Error(), want) {
		t.Errorf("err = %q, want prefix %q", err, want)
	}
}

// requireAbortNamesOnlyDeleted is requireAbortAfterOneDelete plus a check
// that err still unwraps to the failed subprocess whose argv has subcommand
// at position 1.
func requireAbortNamesOnlyDeleted(t *testing.T, err error, repoRoot, subcommand string, branches ...string) {
	t.Helper()
	requireAbortAfterOneDelete(t, err, repoRoot, branches...)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || len(exitErr.Argv) < 2 || exitErr.Argv[1] != subcommand {
		t.Errorf("err = %v, want it to unwrap to a failed `%s` call", err, subcommand)
	}
}

func TestSweepBranches_Execute_NoteReadFailsAfterDelete_ErrorNamesDeletedBranches(t *testing.T) {
	repoRoot := newTestRepo(t)
	pushBranchFixture(t, repoRoot, "feat/one")
	pushBranchFixture(t, repoRoot, "feat/two")
	shippedTicket(t, repoRoot, "feat/one", "PR-ONE")
	shippedTicket(t, repoRoot, "feat/two", "PR-TWO")
	fakeGHSweep(t, "[]", map[string]string{"PR-ONE": "MERGED", "PR-TWO": "MERGED"})
	failNoteReadFromCall(t, 2)

	got, err := SweepBranches(repoRoot, "gh", true)
	if got != nil {
		t.Errorf("got = %+v, want nil on a hard error", got)
	}
	requireAbortAfterOneDelete(t, err, repoRoot, "feat/one", "feat/two")
	if !errors.Is(err, errInjectedNoteRead) {
		t.Errorf("err = %v, want it to unwrap to the injected note-read failure", err)
	}
}

func TestSweepBranches_Execute_OriginBreaksAfterDelete_ErrorNamesDeletedBranches(t *testing.T) {
	repoRoot := newTestRepo(t)
	pushBranchFixture(t, repoRoot, "feat/one")
	pushBranchFixture(t, repoRoot, "feat/two")
	shippedTicket(t, repoRoot, "feat/one", "PR-ONE")
	shippedTicket(t, repoRoot, "feat/two", "PR-TWO")
	fakeGHSweep(t, "[]", map[string]string{"PR-ONE": "MERGED", "PR-TWO": "MERGED"})
	installOriginHook(t, repoRoot, "post-receive", fmt.Sprintf("unset GIT_DIR\ngit -C %q remote set-url origin /nonexistent/origin.git", repoRoot))

	got, err := SweepBranches(repoRoot, "gh", true)
	if got != nil {
		t.Errorf("got = %+v, want nil on a hard error", got)
	}
	requireAbortNamesOnlyDeleted(t, err, repoRoot, "ls-remote", "feat/one", "feat/two")
}

func TestSweepBranches_Execute_NoteReadFailsBeforeAnyDelete_ErrorReturnedUnwrapped(t *testing.T) {
	repoRoot := newTestRepo(t)
	pushBranchFixture(t, repoRoot, "feat/one")
	shippedTicket(t, repoRoot, "feat/one", "PR-ONE")
	fakeGHSweep(t, "[]", map[string]string{"PR-ONE": "MERGED"})
	failNoteReadFromCall(t, 1)

	_, err := SweepBranches(repoRoot, "gh", true)
	if !errors.Is(err, errInjectedNoteRead) || strings.Contains(err.Error(), "sweep aborted") {
		t.Errorf("err = %v, want the bare note-read failure with nothing deleted to report", err)
	}
	if !originHasBranch(t, repoRoot, "feat/one") {
		t.Errorf("origin lost feat/one although the sweep aborted before deleting")
	}
}
