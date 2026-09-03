---
id: goa-ioe4
status: closed
deps: [goa-krro]
links: []
created: 2026-09-03T08:12:36Z
type: bug
priority: 2
assignee: Christophe1997
---
# internal/ticket.Parse strictness silently drops malformed tickets from tk list/ready/blocked output

internal/ticket.Parse hard-errors on any malformed frontmatter (missing required field, duplicate key, non-bracket deps/links value). goalship tk ls/ready/blocked/closed silently skip any ticket that fails to parse, rather than degrading gracefully the way bash tk's awk-based readers do (which default a missing/malformed field and keep the ticket visible). This is most dangerous for 'ready': a ticket silently omitted from ready output is visually indistinguishable from one that is genuinely blocked by open dependencies — a caller like this very execution loop (which picks tickets via 'tk ready') could read a malformed-but-actually-unblocked ticket as either 'blocked' or simply miss it as a pick candidate, with no error surfaced anywhere. Discovered while implementing goa-krro (U3, PR #5) — see internal/cli/tk's list commands (ls.go, ready.go, blocked.go, closed.go) and internal/ticket/store.go's Parse function. Fix direction (not settled, needs a decision): either (a) internal/ticket.Parse gains a tolerant mode that defaults missing/malformed fields instead of erroring, used by the list commands specifically (while Load/Save for direct single-ticket operations like show/edit/status can keep the strict behavior), or (b) the list commands surface a warning to stderr for any ticket they skip, so a silent drop is at least visible instead of invisible.

## Acceptance Criteria

- A ticket file with a missing optional-but-currently-required field (e.g. missing `links:`) still appears in `goalship tk ls`/`ready`/`blocked`/`closed` output, matching bash tk's tolerant behavior — OR, if tolerant parsing is rejected as a fix direction, each list command prints a clear warning to stderr naming which ticket file it skipped and why, so the omission is never silent.
- A test proves a malformed ticket (missing field, or non-bracket deps value) is not simply invisible: it either appears in listings with sane defaults, or its exclusion is visibly reported.
- Existing behavior for genuinely well-formed tickets (the common case, everything tk create produces) is unchanged.


## Notes

**2026-09-03T08:13:40Z**

branch: bug/internal-ticket-parse-strictness-silently-drops-malformed-tickets-from-tk-list-ready-blocked-output
base: feature/goalship-tk-crud-status-and-notes-commands
claim_sha: 1d515ae2d0b3ead952657952441c35d42d773ecf

**2026-09-03T08:29:22Z**

branch: bug/internal-ticket-parse-strictness-silently-drops-malformed-tickets-from-tk-list-ready-blocked-output
pr: https://github.com/Christophe1997/goalship/pull/6
sha: e1637bf5fd4ec9d78da1c9cd20a58fa01cad3b51
