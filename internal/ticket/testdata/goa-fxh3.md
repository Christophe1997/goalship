---
id: goa-fxh3
status: closed
deps: []
links: []
created: 2026-09-03T06:41:46Z
type: chore
priority: 1
assignee: Christophe1997
external-ref: U1
---
# Project scaffolding and command tree

Go module 'goalship'; Cobra root command wiring three cobra.Group groups (tk, loop, review — KTD3), one file per (sub)command with stub RunE bodies for every command named in R1 (tk surface) and R6 (loop surface) and R9/R13 (review, review-status). internal/atomicfile: write to a sibling '<path>.<random>.tmp', fsync, os.Rename over the target (POSIX atomic rename) — no external package to port from, the plan's 'token-profile' reference repo does not exist on this machine, implement directly. go.mod pins Go 1.23+ (KTD2's fsnotify floor; go1.27.1 is installed and on PATH). Layout: cmd/goalship/main.go, internal/cli/root.go, internal/cli/tk/*.go, internal/cli/loop/*.go, internal/cli/review.go, internal/atomicfile/atomicfile.go. Stub subcommands only — no product logic yet, later tickets fill RunE bodies.

## Acceptance Criteria

- `goalship --help` lists all three command groups (tk, loop, review) with every subcommand named in R1 (create, start, close, reopen, status, dep, undep, dep tree, dep cycle, link, unlink, ls/list, ready, blocked, closed, show, edit, add-note, query, migrate-beads) and R6 (preflight, reconcile, ledger, resume-candidates, dirty, branch-name, resolve-base, commit-landed, run-branch, find-pr, claim, commit, head-sha, push, create-pr, retarget-pr, ship, reset) and review/review-status.
- `atomicfile.Write` leaves either the old file or the fully-written new file visible after a simulated crash between temp-write and rename — never a partial file.
- Test expectation for anything else: none — this unit has no product logic beyond the two behaviors above.
- Verification: `go build ./...` succeeds; every `goalship <group> <cmd> --help` renders without error.


## Notes

**2026-09-03T06:42:38Z**

branch: chore/project-scaffolding-and-command-tree
claim_sha: fb1ca9d44b0d3300f3f057b9cf59f9d73c534dce

**2026-09-03T06:59:26Z**

branch: chore/project-scaffolding-and-command-tree
pr: https://github.com/Christophe1997/goalship/pull/1
sha: 3fd3f22dc00165f8388793fb800d262edc88d302
