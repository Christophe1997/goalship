package gitops

import "strings"

// HeadSHA resolves ref (default "HEAD" when ref == "") to its commit SHA —
// mirrors branching.py's head_sha.
func HeadSHA(repoRoot, ref string) (string, error) {
	if ref == "" {
		ref = "HEAD"
	}
	out, err := run(repoRoot, "git", "rev-parse", ref)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// CommitLanded reports whether branch's tip has moved since claimSHA — a
// ticket-scoped check: it compares against the exact SHA recorded at this
// ticket's own claim time (a plain string compare, not a ref resolution —
// claimSHA already is one), not "any commit past the branch's base", so a
// shared branch carrying an earlier ticket's already-landed commits can't
// be misattributed to a later ticket that hasn't landed anything of its
// own yet. Mirrors branching.py's commit_landed_since.
func CommitLanded(repoRoot, branch, claimSHA string) (bool, error) {
	sha, err := HeadSHA(repoRoot, branch)
	if err != nil {
		return false, err
	}
	return sha != claimSHA, nil
}
