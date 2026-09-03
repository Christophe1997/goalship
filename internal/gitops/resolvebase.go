package gitops

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// hostToolTimeout mirrors branching.py's HOST_TOOL_TIMEOUT_SECONDS = 30,
// applied to every gh/glab round-trip PRState makes.
const hostToolTimeout = 30 * time.Second

// DependencyPR is a predecessor ticket's linked PR, as recorded in its own
// notes — mirrors branching.py's DependencyPR.
type DependencyPR struct {
	TicketID string
	Branch   string
	State    string // "open" | "merged" | "closed"
}

// ResolveBranchBase applies the dependency-aware branch model: trunk by
// default; a single still-open predecessor's branch when exactly one
// predecessor has an open PR; trunk on fan-in (two or more simultaneously
// open predecessors) or when no predecessor has an open PR — git supports
// only one base per branch, and trunk is the only base every predecessor's
// eventual merge converges on. Mirrors branching.py's resolve_branch_base.
func ResolveBranchBase(trunkBranch string, dependencyPRs []DependencyPR) string {
	var openBranch string
	openCount := 0
	for _, d := range dependencyPRs {
		if d.State == "open" {
			openCount++
			openBranch = d.Branch
		}
	}
	if openCount == 1 {
		return openBranch
	}
	return trunkBranch
}

// PRStateFunc reports a hosted PR/MR's state ("open", "merged", "closed")
// for prRef via hostTool ("gh" or "glab"). ok is false when the lookup
// itself failed (expired credential, host outage, unparseable response) —
// kept distinct from a legitimately non-open PR, mirroring
// reconciliation.py's pr_state returning None only on lookup failure. A
// func type (rather than a one-method interface) so callers — real or
// test fakes — are plain closures.
type PRStateFunc func(repoRoot, hostTool, prRef string) (state string, ok bool)

var prStateMapping = map[string]string{
	"OPEN": "open", "MERGED": "merged", "CLOSED": "closed",
	"opened": "open", "merged": "merged", "closed": "closed",
}

// PRState is the real, subprocess-backed PRStateFunc: `gh pr view` or
// `glab mr view`, timing out after hostToolTimeout. Mirrors
// reconciliation.py's pr_state. resolve-base is the only caller in this
// ticket's scope; find-pr/create-pr/retarget-pr/ship (the rest of
// branching.py's gh/glab surface) belong to a later ticket.
func PRState(repoRoot, hostTool, prRef string) (state string, ok bool) {
	ctx, cancel := context.WithTimeout(context.Background(), hostToolTimeout)
	defer cancel()

	var raw string
	switch hostTool {
	case "gh":
		out, code := runUnchecked(ctx, repoRoot, "gh", "pr", "view", prRef, "--json", "state", "-q", ".state")
		if code != 0 {
			return "", false
		}
		raw = strings.TrimSpace(out)
	case "glab":
		out, code := runUnchecked(ctx, repoRoot, "glab", "mr", "view", prRef, "-F", "json")
		if code != 0 {
			return "", false
		}
		var payload struct {
			State string `json:"state"`
		}
		if err := json.Unmarshal([]byte(out), &payload); err != nil {
			return "", false
		}
		raw = payload.State
	default:
		return "", false
	}

	mapped, known := prStateMapping[raw]
	return mapped, known
}

// ResolveBase looks up ticketID's tk dependencies and applies the
// dependency-aware branch model — mirrors branching.py's
// resolve_base_for_ticket. hostTool may be "" when no dependency carries a
// pr: note field (nothing to look up); a dependency that does have one
// with hostTool == "" always resolves as a lookup failure, same as
// PRState's default case for an unrecognized tool.
func ResolveBase(repoRoot, ticketID, trunkBranch, hostTool string) (string, error) {
	return resolveBase(repoRoot, ticketID, trunkBranch, hostTool, PRState)
}

func resolveBase(repoRoot, ticketID, trunkBranch, hostTool string, prState PRStateFunc) (string, error) {
	matches, err := tkQuery(repoRoot, fmt.Sprintf(`select(.id=="%s")`, ticketID))
	if err != nil {
		return "", err
	}
	var depIDs []string
	if len(matches) > 0 {
		depIDs = stringSlice(matches[0]["deps"])
	}

	var dependencyPRs []DependencyPR
	for _, depID := range depIDs {
		fields, err := noteFieldsForTicket(repoRoot, depID)
		if err != nil {
			return "", err
		}
		branch := fields["branch"]
		if branch == "" {
			// No recorded branch note — closed by hand, or predates this
			// loop; can't be looked up, so treat as resolved (same as a
			// merged predecessor).
			continue
		}
		prRef := fields["pr"]
		var state string
		if prRef == "" {
			// No PR was ever recorded for this predecessor — legitimately
			// resolved, same as a merged/closed one.
			state = "closed"
		} else {
			s, ok := prState(repoRoot, hostTool, prRef)
			if !ok {
				// Distinct from a legitimately closed PR: folding a failed
				// lookup into "closed" would silently rebase ticketID onto
				// trunk instead of depID's still-open branch.
				return "", fmt.Errorf("could not resolve base for %q: pr_state lookup failed for dependency %q (pr: %q)", ticketID, depID, prRef)
			}
			state = s
		}
		dependencyPRs = append(dependencyPRs, DependencyPR{TicketID: depID, Branch: branch, State: state})
	}

	return ResolveBranchBase(trunkBranch, dependencyPRs), nil
}
