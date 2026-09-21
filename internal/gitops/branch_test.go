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

func TestIsAncestor(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/a", "main")
	runOK(t, repoRoot, "git", "commit", "-q", "--allow-empty", "-m", "a")
	runOK(t, repoRoot, "git", "checkout", "main")
	createBranch(t, repoRoot, "feat/b", "main")

	cases := []struct {
		name       string
		ancestor   string
		descendant string
		want       bool
	}{
		{"base is an ancestor of its descendant", "main", "feat/a", true},
		{"a ref is its own ancestor", "feat/a", "feat/a", true},
		{"descendant is not an ancestor of its base", "feat/a", "main", false},
		{"diverged branches", "feat/a", "feat/b", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := IsAncestor(repoRoot, c.ancestor, c.descendant)
			if err != nil {
				t.Fatalf("IsAncestor: %v", err)
			}
			if got != c.want {
				t.Errorf("IsAncestor(%q, %q) = %v, want %v", c.ancestor, c.descendant, got, c.want)
			}
		})
	}
}

// TestIsAncestor_UnknownRef_ErrorsRatherThanFalse: git signals "not an
// ancestor" with exit 1 but a bad ref with 128; conflating them would make
// a typo'd branch look like a legitimate lineage mismatch.
func TestIsAncestor_UnknownRef_ErrorsRatherThanFalse(t *testing.T) {
	repoRoot := newTestRepo(t)

	if got, err := IsAncestor(repoRoot, "no-such-ref", "main"); err == nil {
		t.Errorf("IsAncestor = %v with nil error, want an error for an unknown ref", got)
	}
}
