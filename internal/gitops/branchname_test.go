package gitops

import "testing"

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Add input validation!", "add-input-validation"},
		{"  Leading and trailing  ", "leading-and-trailing"},
		{"multiple---dashes", "multiple-dashes"},
		{"existing remote only", "existing-remote-only"},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestBranchName_SlugifiesTypeAndTitle(t *testing.T) {
	repoRoot := newTestRepo(t)
	name, err := BranchName(repoRoot, "feat", "Add input validation!")
	if err != nil {
		t.Fatalf("BranchName: %v", err)
	}
	if name != "feat/add-input-validation" {
		t.Errorf("name = %q, want %q", name, "feat/add-input-validation")
	}
}

func TestBranchName_CollisionAppliesNumericSuffix(t *testing.T) {
	repoRoot := newTestRepo(t)
	first, err := BranchName(repoRoot, "feat", "Add login form")
	if err != nil {
		t.Fatalf("BranchName: %v", err)
	}
	createBranch(t, repoRoot, first, "origin/main")

	second, err := BranchName(repoRoot, "feat", "Add login form")
	if err != nil {
		t.Fatalf("BranchName: %v", err)
	}
	if second == first {
		t.Fatalf("second == first (%q), want a numeric suffix", first)
	}
	if want := first + "-2"; second != want {
		t.Errorf("second = %q, want %q", second, want)
	}
}

// TestBranchName_CollisionCheckedAgainstRemoteRefsToo covers a branch that
// exists only on origin (left by a prior run, deleted locally) — must
// still be treated as taken.
func TestBranchName_CollisionCheckedAgainstRemoteRefsToo(t *testing.T) {
	repoRoot := newTestRepo(t)
	createBranch(t, repoRoot, "feat/existing-remote-only", "main")
	runOK(t, repoRoot, "git", "push", "-q", "-u", "origin", "feat/existing-remote-only")
	runOK(t, repoRoot, "git", "checkout", "main")
	runOK(t, repoRoot, "git", "branch", "-D", "feat/existing-remote-only")
	runOK(t, repoRoot, "git", "fetch", "-q", "origin")

	name, err := BranchName(repoRoot, "feat", "existing remote only")
	if err != nil {
		t.Fatalf("BranchName: %v", err)
	}
	if name != "feat/existing-remote-only-2" {
		t.Errorf("name = %q, want %q", name, "feat/existing-remote-only-2")
	}
}
