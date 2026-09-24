package loop

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/Christophe1997/goalship/internal/gitops"
)

// fakeGHSweep mirrors internal/gitops/sweepbranches_test.go's helper of the
// same name: a different package's test file, so it can't import that
// unexported test helper directly. Answers `gh pr list ...` with
// openPRsJSON and `gh pr view <ref> ...` by looking ref up in prStates.
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

func TestSweepBranchesCmd_DeleteFlagDefaultsFalse_ReportOnly(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "eligible")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/eligible\npr: PR1")

	fakeGHSweep(t, "[]", map[string]string{"PR1": "MERGED"})

	out := execCmd(t, NewSweepBranchesCmd(), []string{repoRoot, "gh"})

	var got []sweepCandidateJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got) != 1 || got[0].Outcome != gitops.SweepOutcomeWouldDelete {
		t.Errorf("got = %+v, want exactly one would-delete candidate", got)
	}
}

func TestSweepBranchesCmd_JSONShape_SnakeCaseKeys(t *testing.T) {
	repoRoot := newLoopTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "eligible")
	tkStart(t, repoRoot, ticketID)
	tkAddNote(t, repoRoot, ticketID, "branch: feat/eligible\npr: PR1")

	fakeGHSweep(t, "[]", map[string]string{"PR1": "MERGED"})

	out := execCmd(t, NewSweepBranchesCmd(), []string{repoRoot, "gh"})

	for _, key := range []string{`"ticket_id"`, `"branch"`, `"pr_ref"`, `"outcome"`, `"error"`} {
		if !strings.Contains(out, key) {
			t.Errorf("output %q missing expected key %q", out, key)
		}
	}
}

func TestSweepBranchesCmd_ReportOnly_NamesOnlyTheEligibleBranch(t *testing.T) {
	repoRoot := newLoopTestRepo(t)

	eligible := tkCreate(t, repoRoot, "eligible")
	tkStart(t, repoRoot, eligible)
	tkAddNote(t, repoRoot, eligible, "branch: feat/eligible\npr: PR-ELIGIBLE")

	ineligible := tkCreate(t, repoRoot, "still open PR")
	tkStart(t, repoRoot, ineligible)
	tkAddNote(t, repoRoot, ineligible, "branch: feat/ineligible\npr: PR-INELIGIBLE")

	fakeGHSweep(t, "[]", map[string]string{"PR-ELIGIBLE": "MERGED", "PR-INELIGIBLE": "OPEN"})

	out := execCmd(t, NewSweepBranchesCmd(), []string{repoRoot, "gh"})

	var got []sweepCandidateJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal %q: %v", out, err)
	}
	if len(got) != 1 || got[0].TicketID != eligible || got[0].Branch != "feat/eligible" {
		t.Errorf("got = %+v, want exactly one candidate naming ticket %q / branch feat/eligible", got, eligible)
	}
}
