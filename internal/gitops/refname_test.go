package gitops

import (
	"errors"
	"testing"
)

// TestFlagLikeRefs_RejectedBeforeReachingGit: an agent-supplied ref that
// starts with "-" would be parsed by git as an option (`rev-parse --all`
// prints every SHA, `push -u origin --force` force-pushes the current
// branch), so every such call must fail before git runs — an *ExitError
// would mean git already saw it.
func TestFlagLikeRefs_RejectedBeforeReachingGit(t *testing.T) {
	cases := []struct {
		name string
		call func(repoRoot string) error
	}{
		{"HeadSHA --all", func(r string) error { _, err := HeadSHA(r, "--all"); return err }},
		{"PushBranch --force", func(r string) error { return PushBranch(r, "--force") }},
		{"CheckoutBranch --detach", func(r string) error { return CheckoutBranch(r, "--detach") }},
		{"CreateBranch flag-like base", func(r string) error { return CreateBranch(r, "feat/x", "-f") }},
		{"Reset flag-like base", func(r string) error { return Reset(r, "-f") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repoRoot := newTestRepo(t)
			err := c.call(repoRoot)
			if err == nil {
				t.Fatal("expected an error for a flag-like ref, got nil")
			}
			var exitErr *ExitError
			if errors.As(err, &exitErr) {
				t.Fatalf("error came from git (%v); the ref must be rejected before git runs", err)
			}
		})
	}
}
