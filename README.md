# goalship

A single Go CLI that replaces bash `tk` ([wedow/ticket](https://github.com/wedow/ticket))
and goalship's Python `loop_runner.py` as the mechanics behind the goal-to-PR
execution loop: ticket tracking (`goalship tk`), git/gh loop mechanics
(`goalship loop`), and a browser-based review checkpoint (`goalship review`)
where the decomposed ticket graph is edited and approved — or rejected with
notes for revision — before any ticket starts executing.

`goalship review`'s checkpoint is the one exception to "no persistent
process": it runs a loopback-only HTTP server for the lifetime of a single
invocation (token-gated, live-refreshed over SSE), then exits once you
approve, reject, or withdraw. Outside of that, `goalship` and the
orchestrating agent coordinate entirely by reading and atomically writing
the run's ledger under `.goalship/`.

## Status

Implemented. Every `tk` command, every `loop_runner.py` subcommand, and the
`review`/`review-status` checkpoint have shipped (see the closed tickets
under [`.tickets/`](.tickets/)). The full product and technical design that
drove the build lives in
[`docs/plans/2026-08-27-2010-feat-goalship-cli-plan.md`](docs/plans/2026-08-27-2010-feat-goalship-cli-plan.md).

## Prerequisites

- `git`, on your `PATH`.
- `gh` or `glab`, on your `PATH` and authenticated, for the loop's PR
  mechanics.

## Install

Once published, build/install from source with Go 1.23+:

```sh
go install github.com/Christophe1997/goalship/cmd/goalship@latest
```

## Scope

`goalship` is a faithful 1:1 port of bash `tk`'s command surface plus
`loop_runner.py`'s existing CLI surface, with one addition: the `review`
checkpoint. It is not an occasion to add new ticket-tool or loop capability
beyond that.

- **In scope:** every `tk` command, every `loop_runner.py` subcommand, and
  the `review`/`review-status` checkpoint.
- **Out of scope:** an MCP server or any other persistent listening process;
  migrating existing ticket IDs to the new ID shape; deleting or renaming
  tickets through the wizard; run-scoping the ticket store itself.

## License

MIT — see [LICENSE](LICENSE).
