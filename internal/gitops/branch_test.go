package gitops

import "testing"

func TestLocalBranchExists_UnknownBranch_ReturnsFalse(t *testing.T) {
	repoRoot := newTestRepo(t)
	exists, err := LocalBranchExists(repoRoot, "does-not-exist")
	if err != nil {
		t.Fatalf("LocalBranchExists: %v", err)
	}
	if exists {
		t.Error("exists = true, want false for a branch that was never created")
	}
}

func TestLocalBranchExists_ExistingBranch_ReturnsTrue(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/exists", "main")

	exists, err := LocalBranchExists(repoRoot, "feat/exists")
	if err != nil {
		t.Fatalf("LocalBranchExists: %v", err)
	}
	if !exists {
		t.Error("exists = false, want true for a branch just created")
	}
}

func TestCreateBranch_CreatesOffBaseRefAndChecksItOut(t *testing.T) {
	repoRoot := newTestRepo(t)

	if err := CreateBranch(repoRoot, "feat/new", "main"); err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}

	current := trimmed(runOK(t, repoRoot, "git", "branch", "--show-current"))
	if current != "feat/new" {
		t.Errorf("current branch = %q, want %q", current, "feat/new")
	}
	headSHA, err := HeadSHA(repoRoot, "")
	if err != nil {
		t.Fatalf("HeadSHA: %v", err)
	}
	baseSHA, err := HeadSHA(repoRoot, "main")
	if err != nil {
		t.Fatalf("HeadSHA(main): %v", err)
	}
	if headSHA != baseSHA {
		t.Errorf("new branch tip = %q, want it to match base main tip %q", headSHA, baseSHA)
	}
}

func TestCheckoutBranch_SwitchesToExistingBranch(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/other", "main")
	runOK(t, repoRoot, "git", "checkout", "main")

	if err := CheckoutBranch(repoRoot, "feat/other"); err != nil {
		t.Fatalf("CheckoutBranch: %v", err)
	}

	current := trimmed(runOK(t, repoRoot, "git", "branch", "--show-current"))
	if current != "feat/other" {
		t.Errorf("current branch = %q, want %q", current, "feat/other")
	}
}
