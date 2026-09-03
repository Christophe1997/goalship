package loop

import (
	"encoding/json"
	"testing"
)

func TestResumeCandidatesCmd_NoLedgerDir_PrintsEmptyArray(t *testing.T) {
	repoRoot := newLoopTestRepo(t)

	out := execCmd(t, NewResumeCandidatesCmd(), []string{repoRoot})

	var got []resumeCandidate
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if len(got) != 0 {
		t.Errorf("resume candidates = %v, want empty", got)
	}
}

func TestResumeCandidatesCmd_ExcludesTerminalRuns(t *testing.T) {
	repoRoot := newLoopTestRepo(t)

	execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "run-active", "--goal", "still going", "--claim", "t-1"})
	execCmd(t, NewLedgerCmd(), []string{repoRoot, "--run-id", "run-done", "--goal", "finished", "--terminal", "exhausted"})

	out := execCmd(t, NewResumeCandidatesCmd(), []string{repoRoot})

	var got []resumeCandidate
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("unmarshal output %q: %v", out, err)
	}
	if len(got) != 1 {
		t.Fatalf("resume candidates = %+v, want exactly one (run-active)", got)
	}
	c := got[0]
	if c.RunID != "run-active" {
		t.Errorf("run_id = %q, want %q", c.RunID, "run-active")
	}
	if c.Goal != "still going" {
		t.Errorf("goal = %q, want %q", c.Goal, "still going")
	}
	if len(c.ClaimedTicketIDs) != 1 || c.ClaimedTicketIDs[0] != "t-1" {
		t.Errorf("claimed_ticket_ids = %v, want [t-1]", c.ClaimedTicketIDs)
	}
}
