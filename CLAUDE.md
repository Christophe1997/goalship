# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
go build ./...                       # build everything
go build -o goalship ./cmd/goalship  # local binary (/goalship is gitignored)
go vet ./...
gofmt -l .                           # CI fails if this prints anything
go test -race ./...                  # what CI runs (ubuntu-latest)
go test ./internal/gitops -run TestCreatePullRequest_GH_ArgvAndURL -v   # single test
```

Releases are cut by GoReleaser (`.goreleaser.yaml`, output in `/dist/`); the version is stamped into `internal/cli.version` via `-ldflags -X`, so it must stay a string `var`.

## What this is

A Go CLI that ports two existing tools 1:1 and adds a review checkpoint. It is the mechanics layer that an orchestrating agent (the goalship skill, which lives in the sibling `agent-extentions` repo) calls at each step of a goal-to-PR loop:

- `goalship tk` — port of bash `tk` (wedow/ticket 0.3.2). Ticket files must stay **byte-compatible** with bash tk's frontmatter format.
- `goalship loop` — port of the Python `loop_runner.py` / `run_state.py` / `branching.py` / `preflight.py`. Many doc comments say "mirrors X.py's Y": keep behavior and JSON output shape identical to the Python original (e.g. `[]` not `null`, `null` for unset optionals, no HTML escaping — see `internal/cli/loop/jsonio.go`).
- `goalship review <run-id>` / `review-status` — the one new feature: a browser checkpoint where a run's ticket graph is edited and approved/rejected/withdrawn before `loop claim` may pick up any ticket.

Scope is deliberately narrow (see README "Scope"): don't add ticket-tool or loop capability beyond the ported surface without a plan. Design decisions and requirement IDs (R7, R16, KTD3, …) referenced throughout comments are defined in `docs/plans/`.

## Architecture

`cmd/goalship` → `internal/cli` (Cobra root, three command groups) → command packages `internal/cli/tk` and `internal/cli/loop`, which are thin wrappers over the domain packages:

- `internal/ticket` — storage layer for `.tickets/*.md`: parse/write (`store.go`), ID resolve/generate and `ResolveTicketsDir` (`id.go`; honors `TICKETS_DIR`), jq-backed `Query` via embedded gojq (lenient — one malformed ticket must not abort a listing, unlike strict `Load`), and `Update`, the single resolve→load→mutate→save primitive that status/note writers should route through rather than re-implementing. `ticket/tickettest` provides fixtures for other packages' tests.
- `internal/ledger` — the per-run JSON ledger at `.goalship/<run-id>.json` (state, caps, terminal states, review state), the OS-native advisory review lock (gofrs/flock), and adding `.goalship` to `.git/info/exclude` (never to the target repo's `.gitignore`).
- `internal/gitops` — all git and `gh`/`glab` work, done by **shelling out, never a Go git library**, so behavior matches the Python original. Every failure surfaces as `*ExitError` (argv, exit code, stderr, `TimedOut`). Host-tool calls run under a watchdog timeout with `WaitDelay` so orphaned child processes can't hang the CLI. Host tool (gh vs glab) is chosen from the `origin` remote's host. Seams like `PRStateFunc` exist for injecting PR state in tests.
- `internal/reviewserver` — loopback-only HTTP server for `goalship review`: per-invocation token + Host-header validation + CSP (`security.go`), embedded UI in `assets/` via `go:embed`, JSON API for ticket edit/approve/reject/withdraw (`api.go`), and live refresh via fsnotify → SSE with polling fallback (`watch.go`). It exits once the user decides.
- `internal/runfolder` — picks the tickets folder a run's commands use (`Resolve`): a `TICKETS_DIR` override, else the ledger's `tickets_dir`, else the whole tickets area for legacy runs (null `tickets_dir`); `tk` alone also accepts a ledger-less folder holding `run.json`. `All` returns the area plus every run folder.
- `internal/atomicfile` — temp-file + rename writes; all ledger writes go through it.

Coordination between goalship and the orchestrating agent happens only by reading and atomically writing the ledger — there is no long-running process other than a single `review` invocation.

Paths that hold ticket state (`.tickets/` or a `TICKETS_DIR` override inside the repo) must stay out of feature-branch commits and out of `git clean` during reset; `gitops` commit/reset and `loop dirty` all account for this, so any new code that stages or cleans files must too.

## Testing conventions

- Strict TDD is expected: write a failing test first.
- gitops/loop tests build real temporary git repos (`newTestRepo` in `internal/gitops/helpers_test.go`) and fake `gh`/`glab` with shell scripts placed on `PATH` that record their argv (`captureArgvHostTool`, `withFakeHostTool`). Tests must not depend on a real `gh`, `glab`, or bash `tk` being installed.
- Timing-sensitive watchdog tests warm the fake binary first (`warmFakeHostTool`) to stay non-flaky on CI Linux runners.

## Repo-specific notes

- `.tickets/` and `.goalship/` in this repo are goalship's own ticket graph and run ledgers from building itself — real data, not fixtures.
- `internal/clistub` is a leftover scaffolding helper with no remaining callers.
- Tickets area versus run folder: `ticket.ResolveTicketsDir`/`RelativeTicketsDir` mean the whole area (`.tickets/` or `TICKETS_DIR`) and drive the commit/reset/dirty exclusions, so no run's folder is ever staged or cleaned. Ticket reads and writes go to the run's own folder (`.tickets/<run-id>/`, recorded once in the ledger's `tickets_dir` by `loop ledger`) via `runfolder.Resolve`.
