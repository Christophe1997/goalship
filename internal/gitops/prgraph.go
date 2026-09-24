package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// OpenPR is one open PR/MR as reported by ListOpenPRs.
type OpenPR struct {
	Number int
	URL    string
	Branch string
	Base   string
}

// ghOpenPRsLimit bounds a single `gh pr list` call. A result count equal to
// the limit is indistinguishable from "there were more" — ListOpenPRs
// treats that as unsafe truncation rather than a complete result.
const ghOpenPRsLimit = 1000

// glabOpenPRsPageSize is the `--per-page` ListOpenPRs requests from glab; a
// page returning fewer than this ends pagination.
const glabOpenPRsPageSize = 100

// glabOpenPRsMaxPages caps the pagination loop so a host that never returns
// a short page (buggy or adversarial) fails loudly instead of looping
// forever.
const glabOpenPRsMaxPages = 500

// ListOpenPRs lists every open PR/MR for repoRoot via hostTool ("gh" or
// "glab"), hard-failing rather than silently returning a partial result if
// the listing can't be proven complete (gh: hit --limit; glab: pagination
// loop exceeded its safety cap).
func ListOpenPRs(repoRoot, hostTool string) ([]OpenPR, error) {
	switch hostTool {
	case "gh":
		return listOpenPRsGH(repoRoot)
	case "glab":
		return listOpenPRsGlab(repoRoot)
	default:
		return nil, fmt.Errorf("gitops: unsupported host tool %q", hostTool)
	}
}

func listOpenPRsGH(repoRoot string) ([]OpenPR, error) {
	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()
	out, err := runContext(ctx, repoRoot, "gh", "pr", "list",
		"--state", "open", "--json", "number,url,baseRefName,headRefName",
		"--limit", strconv.Itoa(ghOpenPRsLimit))
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Number      int    `json:"number"`
		URL         string `json:"url"`
		BaseRefName string `json:"baseRefName"`
		HeadRefName string `json:"headRefName"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		return nil, fmt.Errorf("gitops: parsing gh pr list output: %w", err)
	}
	if len(raw) == ghOpenPRsLimit {
		return nil, fmt.Errorf("gitops: gh pr list returned %d results (== --limit %d): treating as unsafe truncation", len(raw), ghOpenPRsLimit)
	}

	prs := make([]OpenPR, len(raw))
	for i, r := range raw {
		prs[i] = OpenPR{Number: r.Number, URL: r.URL, Branch: r.HeadRefName, Base: r.BaseRefName}
	}
	return prs, nil
}

func listOpenPRsGlab(repoRoot string) ([]OpenPR, error) {
	var prs []OpenPR
	for page := 1; page <= glabOpenPRsMaxPages; page++ {
		ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
		out, err := runContext(ctx, repoRoot, "glab", "mr", "list",
			"-F", "json", "--per-page", strconv.Itoa(glabOpenPRsPageSize), "--page", strconv.Itoa(page))
		cancel()
		if err != nil {
			return nil, err
		}

		var raw []struct {
			IID          int    `json:"iid"`
			WebURL       string `json:"web_url"`
			SourceBranch string `json:"source_branch"`
			TargetBranch string `json:"target_branch"`
		}
		if err := json.Unmarshal([]byte(out), &raw); err != nil {
			return nil, fmt.Errorf("gitops: parsing glab mr list output: %w", err)
		}
		for _, r := range raw {
			prs = append(prs, OpenPR{Number: r.IID, URL: r.WebURL, Branch: r.SourceBranch, Base: r.TargetBranch})
		}
		if len(raw) < glabOpenPRsPageSize {
			return prs, nil
		}
	}
	return nil, fmt.Errorf("gitops: glab mr list pagination exceeded %d pages without a short page: aborting", glabOpenPRsMaxPages)
}

// ValidateOpenPRGraph checks prs for the malformed-stack conditions that
// would make an ordering decision over them unsafe: a self-referential PR,
// an empty base/branch, a cycle, or a fan-in ambiguity (two open PRs
// sharing a head branch that some other PR's base also targets — which of
// them it actually stacks on is then undecidable).
func ValidateOpenPRGraph(prs []OpenPR) error {
	branchToPRs := map[string][]int{}
	for i, pr := range prs {
		if pr.Branch == "" {
			return fmt.Errorf("gitops: PR #%d has an empty head branch", pr.Number)
		}
		if pr.Base == "" {
			return fmt.Errorf("gitops: PR #%d has an empty base branch", pr.Number)
		}
		if pr.Base == pr.Branch {
			return fmt.Errorf("gitops: PR #%d is self-referential: base and branch are both %q", pr.Number, pr.Branch)
		}
		branchToPRs[pr.Branch] = append(branchToPRs[pr.Branch], i)
	}

	baseUsed := map[string]bool{}
	for _, pr := range prs {
		baseUsed[pr.Base] = true
	}
	// Iterate prs (not the map) so a repeated run reports the same PR on a
	// tie — map iteration order is randomized in Go.
	for _, pr := range prs {
		if idxs := branchToPRs[pr.Branch]; len(idxs) >= 2 && baseUsed[pr.Branch] {
			return fmt.Errorf("gitops: fan-in ambiguity: %d open PRs share head branch %q, and another PR's base references it", len(idxs), pr.Branch)
		}
	}

	// branchOwner only ever holds branches with a single owning PR: any
	// branch shared by 2+ PRs and also used as a base was already rejected
	// above, so every base an edge can legitimately point at resolves to
	// exactly one node here.
	branchOwner := map[string]int{}
	for branch, idxs := range branchToPRs {
		if len(idxs) == 1 {
			branchOwner[branch] = idxs[0]
		}
	}

	return detectCycle(prs, branchOwner)
}

// detectCycle runs Kahn's algorithm over the "X depends on Y" edges (X.Base
// == Y.Branch): repeatedly strip nodes with no remaining outgoing edge to a
// node that's still present. Anything left when no more can be stripped is
// part of a cycle.
func detectCycle(prs []OpenPR, branchOwner map[string]int) error {
	n := len(prs)
	outDegree := make([]int, n)
	dependents := make([][]int, n) // dependents[j] = nodes i with i -> j

	for i, pr := range prs {
		if j, ok := branchOwner[pr.Base]; ok {
			outDegree[i]++
			dependents[j] = append(dependents[j], i)
		}
	}

	queue := make([]int, 0, n)
	for i, deg := range outDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}
	removed := make([]bool, n)
	count := 0
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		removed[node] = true
		count++
		for _, dependent := range dependents[node] {
			outDegree[dependent]--
			if outDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if count == n {
		return nil
	}
	var stuck []string
	for i, pr := range prs {
		if !removed[i] {
			stuck = append(stuck, fmt.Sprintf("#%d (%s -> %s)", pr.Number, pr.Branch, pr.Base))
		}
	}
	return fmt.Errorf("gitops: cycle detected in open PR graph among: %s", strings.Join(stuck, ", "))
}
