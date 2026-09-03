---
id: goa-krro
status: closed
deps: [goa-g7ei]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 2
assignee: Christophe1997
external-ref: U3
---
# goalship tk: CRUD, status, and notes commands

Thin Cobra RunE commands over U2's storage layer for create, start, close, reopen, status, show, edit, add-note, ls/list, ready, blocked, closed — read bash tk's own implementations directly (/opt/homebrew/Cellar/ticket/0.3.2/bin/tk, bash source) for exact behavior. edit shells out to $EDITOR on the raw file (matches tk's cmd_edit) — kept separate from the future review page's structured editing. add-note appends '## Notes' + '**<ISO-timestamp>**\n\n<text>' matching the shape /Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts/reconciliation.py's _NOTES_HEADING_RE/_NOTE_MARKER_RE/_KV_LINE_RE parse today — verify against those regexes directly, not a reimplementation. ready/blocked/closed filter the same way tk's awk does, including silently excluding a ticket with a dangling deps reference (do not fix this — out of scope for this ticket).

## Acceptance Criteria

- `create` writes a new-shape ID with every required frontmatter field populated.
- `add-note`'s appended block parses successfully against reconciliation.py's actual _NOTES_HEADING_RE/_NOTE_MARKER_RE/_KV_LINE_RE regexes.
- `close`/`reopen`/`status` transitions match bash tk's state model exactly.
- `ready` on a ticket with a dangling deps reference silently excludes it, matching bash tk's existing behavior.
- Running the same operation sequence against bash tk and goalship tk on a scratch .tickets/ directory produces identical resulting files.


## Notes

**2026-09-03T07:50:13Z**

branch: feature/goalship-tk-crud-status-and-notes-commands
base: feature/ticket-storage-layer-frontmatter-parse-write-resolve-id-scheme
claim_sha: e4db0e56a1bb25f1ed3b598403fa2c424d2db898

**2026-09-03T08:12:18Z**

branch: feature/goalship-tk-crud-status-and-notes-commands
pr: https://github.com/Christophe1997/goalship/pull/5
sha: 1d515ae2d0b3ead952657952441c35d42d773ecf

**2026-09-03T08:12:36Z**

Discovered: goa-ioe4 — internal/ticket.Parse's hard-error-on-malformed-frontmatter behavior makes tk ls/ready/blocked/closed silently drop unparseable tickets instead of degrading gracefully like bash tk does; risky specifically for ready since silent-omission and genuinely-blocked look identical to a caller.
