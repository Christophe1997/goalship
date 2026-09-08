---
id: goa-5zwn
status: closed
deps: [goa-g7ei, goa-jatp]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U6B
---
# Commit/PR mechanics: commit, push, find-pr, create-pr, retarget-pr, ship

internal/gitops/commit.go + pr.go + internal/cli/loop/commit.go, pr.go, ship.go: commit, head-sha, push, find-pr, create-pr, retarget-pr, ship subcommands, porting the corresponding parts of /Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts/loop_runner.py and /Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts/branching.py directly. Port commit_all's exact staging pathspec (git add -A -- . ":!.tickets") and git clean -fd -e .tickets, unchanged — commit must never stage .tickets/*.md changes. gh/glab round-trips use a 30-second watchdog timeout (HOST_TOOL_TIMEOUT_SECONDS = 30 in the Python source), wrapped with U6A's error type on failure or timeout. ship writes the closing note (branch, PR URL, head SHA) and closes the ticket in one step via U2's primitives, the Go equivalent of record_ship_note -> tk_close.

## Acceptance Criteria

- commit never stages .tickets/*.md changes, even with pending edits present in that directory.
- find-pr/create-pr/retarget-pr argv and JSON shape match loop_runner.py's contract against a mocked gh binary.
- A gh/glab call that hangs past 30 seconds is killed and reported as a timeout, not left hanging.
- ship writes the closing note and closes the ticket as one atomic sequence.


## Notes

**2026-09-03T10:02:40Z**

branch: feature/commit-pr-mechanics-commit-push-find-pr-create-pr-retarget-pr-ship
claim_sha: f3043f0b1cd4351f158b2a7b8f1eb9f1a8c3733f

**2026-09-03T10:21:55Z**

branch: feature/commit-pr-mechanics-commit-push-find-pr-create-pr-retarget-pr-ship
pr: https://github.com/Christophe1997/goalship/pull/9
sha: 2ec6b94e0edfc5b564f0b5a7648ef89d85576da7
