package gitops

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestHeadSHA_DefaultsToHEAD(t *testing.T) {
	repoRoot := newTestRepo(t)
	want := trimmed(runOK(t, repoRoot, "git", "rev-parse", "HEAD"))

	got, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if got != want {
		t.Errorf("HeadSHA = %q, want %q", got, want)
	}
}

func TestHeadSHA_ResolvesArbitraryRefWithoutCheckingItOut(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/other-branch", "origin/main")
	writeFile(t, filepath.Join(repoRoot, "file.txt"), "x\n")
	branchSHA := commitAll(t, repoRoot, "feat: add file")
	runOK(t, repoRoot, "git", "checkout", "main")

	got, err := HeadSHA(repoRoot, "feat/other-branch")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if got != branchSHA {
		t.Errorf("HeadSHA(feat/other-branch) = %q, want %q", got, branchSHA)
	}

	head, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	if head == branchSHA {
		t.Errorf("HeadSHA(HEAD) unexpectedly equals the other branch's tip")
	}
}

func TestCommitLanded_FalseImmediatelyAfterClaimWithNoNewCommit(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/x", "origin/main")
	claimSHA, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	landed, err := CommitLanded(repoRoot, "feat/x", claimSHA)
	if err != nil {
		t.Fatalf("CommitLanded: %v", err)
	}
	if landed {
		t.Errorf("CommitLanded = true, want false")
	}
}

func TestCommitLanded_TrueOnceACommitLandsAfterClaim(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/x", "origin/main")
	claimSHA, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	writeFile(t, filepath.Join(repoRoot, "file.txt"), "x\n")
	commitAll(t, repoRoot, "feat: add file")

	landed, err := CommitLanded(repoRoot, "feat/x", claimSHA)
	if err != nil {
		t.Fatalf("CommitLanded: %v", err)
	}
	if !landed {
		t.Errorf("CommitLanded = false, want true")
	}
}

// TestCommitLanded_NotFooledByCommitsThatPredateThisTicketsOwnClaim is this
// ticket's acceptance criterion made concrete: a shared branch already
// carries a predecessor ticket's landed commit before this ticket's own
// claim_sha is captured. A branch-wide "any commits past base" check would
// report true immediately; commit-landed must not be fooled the same way.
func TestCommitLanded_NotFooledByCommitsThatPredateThisTicketsOwnClaim(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/shared", "origin/main")
	writeFile(t, filepath.Join(repoRoot, "predecessor.txt"), "done\n")
	commitAll(t, repoRoot, "feat: predecessor's work")

	claimSHA, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}

	landed, err := CommitLanded(repoRoot, "feat/shared", claimSHA)
	if err != nil {
		t.Fatalf("CommitLanded: %v", err)
	}
	if landed {
		t.Errorf("CommitLanded = true immediately after claim (fooled by predecessor's commit), want false")
	}

	writeFile(t, filepath.Join(repoRoot, "own-work.txt"), "this ticket's work\n")
	commitAll(t, repoRoot, "feat: this ticket's own work")

	landed, err = CommitLanded(repoRoot, "feat/shared", claimSHA)
	if err != nil {
		t.Fatalf("CommitLanded: %v", err)
	}
	if !landed {
		t.Errorf("CommitLanded = false after this ticket's own commit, want true")
	}
}

// TestCommitLanded_NonexistentBranch_ReturnsExitErrorWithArgvExitCodeAndStderr
// is this ticket's third acceptance criterion: a failing git command must
// surface *ExitError with argv, exit code, and stderr accessible to the
// caller.
func TestCommitLanded_NonexistentBranch_ReturnsExitErrorWithArgvExitCodeAndStderr(t *testing.T) {
	repoRoot := newTestRepo(t)

	_, err := CommitLanded(repoRoot, "no-such-branch", "deadbeef")
	if err == nil {
		t.Fatal("CommitLanded: expected error for a nonexistent branch, got nil")
	}
	var exitErr *ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("CommitLanded: error is not *ExitError: %v (%T)", err, err)
	}
	if exitErr.Argv[0] != "git" {
		t.Errorf("Argv[0] = %q, want %q", exitErr.Argv[0], "git")
	}
	if exitErr.ExitCode == 0 {
		t.Errorf("ExitCode = 0, want nonzero")
	}
	if exitErr.Stderr == "" {
		t.Errorf("Stderr is empty, want git's error message")
	}
}
