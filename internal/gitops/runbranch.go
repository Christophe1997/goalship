package gitops

// RunBranch returns the first of ticketIDs whose recorded branch: note
// exists, or "" if none is claimed yet (this ticket is the run's first) or
// none of them has a branch note yet. Mirrors reconciliation.py's
// find_run_branch — the mechanism a commit-mode run's tickets 2..N use to
// discover ticket 1's shared branch.
func RunBranch(repoRoot string, ticketIDs []string) (string, error) {
	for _, id := range ticketIDs {
		fields, err := noteFieldsForTicket(repoRoot, id)
		if err != nil {
			return "", err
		}
		if branch := fields["branch"]; branch != "" {
			return branch, nil
		}
	}
	return "", nil
}
