package gitops

import "testing"

func TestResolveBranchBase(t *testing.T) {
	cases := []struct {
		name  string
		trunk string
		deps  []DependencyPR
		want  string
	}{
		{"no dependencies uses trunk", "main", nil, "main"},
		{
			"single open dependency uses its branch", "main",
			[]DependencyPR{{TicketID: "T-1", Branch: "feat/dep-a", State: "open"}},
			"feat/dep-a",
		},
		{
			"merged dependency uses trunk not stale branch", "main",
			[]DependencyPR{{TicketID: "T-1", Branch: "feat/dep-a", State: "merged"}},
			"main",
		},
		{
			"closed unmerged dependency uses trunk", "main",
			[]DependencyPR{{TicketID: "T-1", Branch: "feat/dep-a", State: "closed"}},
			"main",
		},
		{
			"fan-in two simultaneously open dependencies uses trunk", "main",
			[]DependencyPR{
				{TicketID: "T-1", Branch: "feat/dep-a", State: "open"},
				{TicketID: "T-2", Branch: "feat/dep-b", State: "open"},
			},
			"main",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolveBranchBase(c.trunk, c.deps); got != c.want {
				t.Errorf("ResolveBranchBase(%q, %v) = %q, want %q", c.trunk, c.deps, got, c.want)
			}
		})
	}
}

func fakePRState(state string) PRStateFunc {
	return func(repoRoot, hostTool, prRef string) (string, bool) {
		return state, true
	}
}

func fakePRStateFails() PRStateFunc {
	return func(repoRoot, hostTool, prRef string) (string, bool) {
		return "", false
	}
}

// failIfCalledPRState fails the test outright if invoked — used where no
// dependency should ever reach a host-tool lookup.
func failIfCalledPRState(t *testing.T) PRStateFunc {
	return func(repoRoot, hostTool, prRef string) (string, bool) {
		t.Helper()
		t.Fatalf("prState unexpectedly called for hostTool=%q prRef=%q", hostTool, prRef)
		return "", false
	}
}

func TestResolveBase_NoDependencies_ResolvesToTrunk(t *testing.T) {
	repoRoot := newTestRepo(t)
	ticketID := tkCreate(t, repoRoot, "Standalone ticket")

	base, err := resolveBase(repoRoot, ticketID, "main", "", failIfCalledPRState(t))
	if err != nil {
		t.Fatalf("resolveBase: %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want %q", base, "main")
	}
}

func TestResolveBase_SingleOpenDependency_ResolvesToItsBranch(t *testing.T) {
	repoRoot := newTestRepo(t)
	depID := tkCreate(t, repoRoot, "Dependency")
	tkAddNote(t, repoRoot, depID, "branch: feat/dep-branch\npr: https://example.com/pr/1")

	ticketID := tkCreate(t, repoRoot, "Dependent")
	tkDep(t, repoRoot, ticketID, depID)

	base, err := resolveBase(repoRoot, ticketID, "main", "gh", fakePRState("open"))
	if err != nil {
		t.Fatalf("resolveBase: %v", err)
	}
	if base != "feat/dep-branch" {
		t.Errorf("base = %q, want %q", base, "feat/dep-branch")
	}
}

func TestResolveBase_MergedDependency_ResolvesToTrunk(t *testing.T) {
	repoRoot := newTestRepo(t)
	depID := tkCreate(t, repoRoot, "Dependency")
	tkAddNote(t, repoRoot, depID, "branch: feat/dep-branch\npr: https://example.com/pr/1")

	ticketID := tkCreate(t, repoRoot, "Dependent")
	tkDep(t, repoRoot, ticketID, depID)

	base, err := resolveBase(repoRoot, ticketID, "main", "gh", fakePRState("merged"))
	if err != nil {
		t.Fatalf("resolveBase: %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want %q", base, "main")
	}
}

// TestResolveBase_DependencyNeverClaimedByThisTool_ResolvesToTrunk covers a
// predecessor with no recorded branch note (closed by hand, or predating
// this loop) — it can't be looked up, so it's treated as resolved, same as
// a merged one.
func TestResolveBase_DependencyNeverClaimedByThisTool_ResolvesToTrunk(t *testing.T) {
	repoRoot := newTestRepo(t)
	depID := tkCreate(t, repoRoot, "Manually closed dependency")
	tkClose(t, repoRoot, depID)

	ticketID := tkCreate(t, repoRoot, "Dependent")
	tkDep(t, repoRoot, ticketID, depID)

	base, err := resolveBase(repoRoot, ticketID, "main", "gh", failIfCalledPRState(t))
	if err != nil {
		t.Fatalf("resolveBase: %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want %q", base, "main")
	}
}

// TestResolveBase_DependencyPRLookupFailure_ReturnsError covers a lookup
// failure (expired credential, host outage) — distinct from a legitimately
// closed PR: folding it into "closed" would silently rebase the dependent
// ticket onto trunk instead of its still-open predecessor's branch during a
// transient outage.
func TestResolveBase_DependencyPRLookupFailure_ReturnsError(t *testing.T) {
	repoRoot := newTestRepo(t)
	depID := tkCreate(t, repoRoot, "Dependency with an unresolvable PR")
	tkAddNote(t, repoRoot, depID, "branch: feat/dep-branch\npr: https://example.com/pr/1")

	ticketID := tkCreate(t, repoRoot, "Dependent")
	tkDep(t, repoRoot, ticketID, depID)

	_, err := resolveBase(repoRoot, ticketID, "main", "gh", fakePRStateFails())
	if err == nil {
		t.Fatal("resolveBase: expected error on PR-state lookup failure, got nil")
	}
}

// TestResolveBase_DependencyWithNoPRNote_TreatedAsClosed covers a
// dependency that has a branch note but never got a pr: field — no PR was
// ever recorded, so it's legitimately resolved, same as a merged/closed one.
func TestResolveBase_DependencyWithNoPRNote_TreatedAsClosed(t *testing.T) {
	repoRoot := newTestRepo(t)
	depID := tkCreate(t, repoRoot, "Dependency claimed but never shipped")
	tkAddNote(t, repoRoot, depID, "branch: feat/dep-branch")

	ticketID := tkCreate(t, repoRoot, "Dependent")
	tkDep(t, repoRoot, ticketID, depID)

	base, err := resolveBase(repoRoot, ticketID, "main", "gh", failIfCalledPRState(t))
	if err != nil {
		t.Fatalf("resolveBase: %v", err)
	}
	if base != "main" {
		t.Errorf("base = %q, want %q", base, "main")
	}
}
