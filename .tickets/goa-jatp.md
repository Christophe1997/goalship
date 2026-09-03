---
id: goa-jatp
status: closed
deps: [goa-fxh3]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U6A
---
# Git/branch mechanics: branch-name, resolve-base, commit-landed, run-branch, reset

internal/gitops/branch.go + internal/cli/loop/branch.go: branch-name, resolve-base, commit-landed, run-branch, reset subcommands, porting /Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts/branching.py directly (read it for exact argv/JSON shape and git plumbing). No gh/glab calls in this ticket — pure git. Define a small Go error type wrapping subprocess argv + exit code + stderr for uniform git-command-failure reporting (a self-contained type, nothing to port — the plan's 'token-profile' reference repo does not exist on this machine).

## Acceptance Criteria

- branch-name/resolve-base/commit-landed/run-branch/reset argv and JSON output shape match branching.py's contract exactly on equivalent fixtures.
- commit-landed is ticket-scoped, not branch-wide: given a branch with commits from an earlier ticket plus a later ticket's own claim_sha, commit-landed for the later ticket correctly reports "no" when nothing landed since ITS OWN claim_sha, even though the branch has other commits.
- A git command that fails (non-zero exit) surfaces the wrapped error type with argv, exit code, and stderr accessible to the caller.


## Notes

**2026-09-03T07:32:05Z**

branch: feature/git-branch-mechanics-branch-name-resolve-base-commit-landed-run-branch-reset
base: chore/project-scaffolding-and-command-tree
claim_sha: 3fd3f22dc00165f8388793fb800d262edc88d302

**2026-09-03T07:49:44Z**

branch: feature/git-branch-mechanics-branch-name-resolve-base-commit-landed-run-branch-reset
pr: https://github.com/Christophe1997/goalship/pull/4
sha: 426d06196b0ba31ffe29b0fe2d3b9327213ce953
