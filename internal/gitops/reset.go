package gitops

import "github.com/Christophe1997/goalship/internal/ticket"

// Reset returns the working tree to a clean checkout of baseRef: check out
// baseRef, hard-reset to it, then clean untracked files except the tickets
// dir (".tickets", or a TICKETS_DIR override resolving inside repoRoot) —
// that dir is never git-ignored by this tool (that's the target repo's own
// call), so a plain `git clean -fd` would wipe out the entire ticket store
// on the first gate failure. It never deletes baseRef, or any other
// branch. The checkout is load-bearing, not redundant with the reset that
// follows: it moves HEAD off the abandoned ticket branch, which a bare
// `git reset --hard baseRef` would leave checked out on. Mirrors
// branching.py's reset_to_clean_base (abort cleanup on a gate failure or
// interruption).
func Reset(repoRoot, baseRef string) error {
	if err := CheckoutBranch(repoRoot, baseRef); err != nil {
		return err
	}
	if _, err := run(repoRoot, "git", "reset", "--hard", "HEAD"); err != nil {
		return err
	}
	cleanArgs := []string{"git", "clean", "-fd"}
	if relTicketsDir, ok := ticket.RelativeTicketsDir(repoRoot); ok {
		cleanArgs = append(cleanArgs, "-e", relTicketsDir)
	}
	if _, err := run(repoRoot, cleanArgs...); err != nil {
		return err
	}
	return nil
}
