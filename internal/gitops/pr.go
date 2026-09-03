package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// FindOpenPRForBranch returns the URL of an already-open PR/MR for branch,
// or "" when none exists or the host lookup itself failed. Mirrors
// reconciliation.py's find_open_pr_for_branch: unlike PRState, this
// deliberately collapses "no PR" and "the query failed" into the same
// result — a caller that gets "" here and proceeds to CreatePullRequest on
// a branch that actually already has one fails loudly there instead (the
// host rejects a second open PR from the same head), which is an accepted
// one-step-later failure mode rather than something worth a three-state
// return here.
func FindOpenPRForBranch(repoRoot, hostTool, branch string) string {
	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()

	var argv []string
	var urlKey string
	switch hostTool {
	case "gh":
		argv = []string{"gh", "pr", "list", "--head", branch, "--state", "open", "--json", "url"}
		urlKey = "url"
	case "glab":
		argv = []string{"glab", "mr", "list", "--source-branch", branch, "-F", "json"}
		urlKey = "web_url"
	default:
		return ""
	}

	out, code := runUnchecked(ctx, repoRoot, argv...)
	if code != 0 {
		return ""
	}
	var prs []map[string]any
	if err := json.Unmarshal([]byte(out), &prs); err != nil || len(prs) == 0 {
		return ""
	}
	url, _ := prs[0][urlKey].(string)
	return url
}

// CreatePullRequest opens a pull/merge request for branch against base,
// returning its URL. branch must already be pushed (PushBranch) — this
// only opens the request, it never pushes. Mirrors branching.py's
// create_pull_request, timing out at hostToolTimeout like every other
// gh/glab round-trip this package makes.
func CreatePullRequest(repoRoot, hostTool, branch, base, title, body string) (string, error) {
	var argv []string
	switch hostTool {
	case "gh":
		argv = []string{"gh", "pr", "create", "--head", branch, "--base", base, "--title", title, "--body", body}
	case "glab":
		argv = []string{
			"glab", "mr", "create",
			"--source-branch", branch, "--target-branch", base,
			"--title", title, "--description", body, "--yes",
		}
	default:
		return "", fmt.Errorf("gitops: unsupported host tool %q", hostTool)
	}

	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()
	out, err := runContext(ctx, repoRoot, argv...)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			return line, nil
		}
	}
	return "", fmt.Errorf("gitops: %s pr create did not print a URL: %q", hostTool, out)
}

// RetargetPullRequest changes an already-open PR/MR's base branch — the
// retarget_base_merged outcome: a stacked ticket's dependency merged out
// from under its open PR, so the PR must repoint at trunk (or a further
// dependency) instead of the now-gone branch. Mirrors branching.py's
// retarget_pull_request.
func RetargetPullRequest(repoRoot, hostTool, prRef, newBase string) error {
	var argv []string
	switch hostTool {
	case "gh":
		argv = []string{"gh", "pr", "edit", prRef, "--base", newBase}
	case "glab":
		argv = []string{"glab", "mr", "update", prRef, "--target-branch", newBase}
	default:
		return fmt.Errorf("gitops: unsupported host tool %q", hostTool)
	}

	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()
	_, err := runContext(ctx, repoRoot, argv...)
	return err
}
