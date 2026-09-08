package gitops

import "github.com/Christophe1997/goalship/internal/ledger"

// CommitAll stages everything except this tool's own state dirs and
// commits, returning the new head SHA. .goalship/ is excluded by ensuring
// it's git-ignored (ledger.EnsureExcluded) rather than an explicit `git
// add` negative pathspec — see EnsureExcluded's own comment for why.
// .tickets/ is never made git-ignored (that's the target repo's own call),
// so it still needs the explicit negative pathspec below: a ticket's PR
// must carry only its own implementation diff, never this loop's or tk's
// own bookkeeping churn. Mirrors branching.py's commit_all.
func CommitAll(repoRoot, message string) (string, error) {
	if err := ledger.EnsureExcluded(repoRoot); err != nil {
		return "", err
	}
	if _, err := run(repoRoot, "git", "add", "-A", "--", ".", ":!"+ticketsDirName); err != nil {
		return "", err
	}
	if _, err := run(repoRoot, "git", "commit", "-m", message); err != nil {
		return "", err
	}
	return HeadSHA(repoRoot, "")
}

// PushBranch pushes branchName to origin and sets it to track. Never
// force. Mirrors branching.py's push_branch.
func PushBranch(repoRoot, branchName string) error {
	_, err := run(repoRoot, "git", "push", "-u", "origin", branchName)
	return err
}
