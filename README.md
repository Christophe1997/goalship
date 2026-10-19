# goalship

A single CLI for turning a goal into shipped pull requests: track the work as a
graph of tickets, review and approve that graph in your browser, then drive an
execution loop that implements each ready ticket on its own branch and opens a
PR for it.

It is the mechanics layer for the goalship agent loop, and every piece also
works on its own:

- **`goalship tk`** — a git-friendly ticket tracker. Tickets are markdown files
  with YAML frontmatter in `.tickets/`, linked by dependencies.
- **`goalship review`** — a browser checkpoint where a run's ticket graph is
  edited and approved, or rejected with notes, before any ticket executes.
- **`goalship loop`** — the git and `gh`/`glab` steps of the execution loop:
  run ledger, claiming tickets, branching, committing, pushing, opening and
  retargeting PRs, and cleaning up merged branches.

## Prerequisites

- `git`, on your `PATH`.
- `gh` or `glab`, on your `PATH` and authenticated, for the PR commands.

`jq` is not required: `goalship tk query` has a jq engine built in.

## Install

Download a prebuilt binary (linux/darwin, amd64/arm64) from the
[Releases page](https://github.com/Christophe1997/goalship/releases), or
install from source with Go 1.24+:

```sh
go install github.com/Christophe1997/goalship/cmd/goalship@latest
```

## Usage

### Track tickets

```sh
goalship tk create "Add login endpoint" -t feature     # prints the new ticket ID
goalship tk create "Write login tests"
goalship tk dep <tests-id> <endpoint-id>               # tests depend on endpoint

goalship tk ready      # open tickets whose dependencies are all closed
goalship tk blocked    # open tickets still waiting on a dependency

goalship tk start <id>
goalship tk add-note <id> "schema drafted"
goalship tk close <id>

goalship tk query '.status == "open"'                  # tickets as JSON, filtered with jq
```

Other commands include `show`, `ls`, `edit`, `link`/`unlink`, `undep`,
`reopen`, `closed`, and `migrate-beads` (import from `.beads/issues.jsonl`).
Ticket IDs can be abbreviated to any unambiguous part.

Tickets live in the nearest `.tickets/` directory above the current directory.
Set `TICKETS_DIR` to use a different one.

### Review a run's ticket graph

```sh
goalship review <run-id>          # opens the checkpoint in your browser
goalship review-status <run-id>   # prints pending, rejected, or approved
```

The review page shows the run's ticket graph and lets you edit tickets, then
approve, reject with notes, or withdraw. It updates live as tickets change on
disk. The server listens on loopback only, requires a per-invocation token,
and exits as soon as you decide. `goalship loop claim` only accepts tickets
from a run you approved, and only those that were in the graph you approved.

### Drive the execution loop

`goalship loop` is meant to be called step by step by an orchestrating agent.
Each run keeps its state in a JSON ledger at `.goalship/<run-id>.json`, which
is added to `.git/info/exclude` so it never shows up in your history.

| Step | Commands |
| --- | --- |
| Start or resume a run | `preflight`, `ledger`, `resume-candidates` |
| Pick up a ticket | `claim`, `branch-name`, `resolve-base`, `run-branch` |
| Ship it | `commit`, `push`, `find-pr`, `create-pr`, `ship` |
| Recover | `dirty`, `reset`, `reconcile`, `commit-landed`, `head-sha` |
| Manage stacked PRs | `merge-order`, `retarget-pr`, `sweep-branches` |

Ticket files are never included in feature-branch commits and are never
cleared by `reset`.

Run `goalship <command> --help` for every command's arguments and flags.

## License

MIT — see [LICENSE](LICENSE).
