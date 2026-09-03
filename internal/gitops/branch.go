package gitops

import "errors"

// LocalBranchExists reports whether branch already exists locally — mirrors
// branching.py's local_branch_exists. A non-zero exit from `git rev-parse`
// means "doesn't exist", not a real error (same treatment PRState gives a
// failed host-tool lookup): a missing branch is the expected, common case
// callers probe for, not a failure worth an *ExitError.
func LocalBranchExists(repoRoot, branch string) (bool, error) {
	_, err := run(repoRoot, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+branch)
	if err != nil {
		var exitErr *ExitError
		if errors.As(err, &exitErr) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// CreateBranch creates branch off baseRef and checks it out — mirrors
// branching.py's create_branch. Called at claim time, before implementation
// starts, so a crash mid-implementation never leaves work sitting on trunk.
func CreateBranch(repoRoot, branch, baseRef string) error {
	_, err := run(repoRoot, "git", "checkout", "-b", branch, baseRef)
	return err
}

// CheckoutBranch switches the working tree to branch, which must already
// exist locally.
func CheckoutBranch(repoRoot, branch string) error {
	_, err := run(repoRoot, "git", "checkout", branch)
	return err
}
