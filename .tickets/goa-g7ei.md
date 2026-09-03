---
id: goa-g7ei
status: closed
deps: [goa-fxh3]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U2
---
# Ticket storage layer: frontmatter parse/write/resolve/ID scheme

internal/ticket/store.go + id.go: parse/write .tickets/*.md frontmatter matching bash tk 0.3.2's format byte-for-byte (fields: id, status, deps, links, created, type, priority, plus optional fields, and the '## Notes' section). Preserve any unrecognized frontmatter key unchanged on a read/write round-trip (R8). ID resolution (R3): exact filename match, else single unambiguous substring match anywhere in .tickets/*.md filenames; ambiguous or no match errors. ID generation (R4): new IDs shaped <repo-prefix>-<YYYYMMDD-HHMM>-<4-char-random-suffix>; repo-prefix derivation matches bash tk's generate_id() — read /opt/homebrew/Cellar/ticket/0.3.2/bin/tk directly (bash source) for its exact byte format and generate_id()/ticket_path() logic. R5 needs no special code: old-shape IDs (<prefix>-<4-char>) are read/resolved by the same resolver, never rewritten. Depends on U1 for internal/atomicfile.

## Acceptance Criteria

- Round-trip: writing then reading a ticket produces output byte-identical to a real tk-authored fixture.
- ID resolution: exact match succeeds; unambiguous substring match succeeds; ambiguous substring errors; no match errors.
- A new ticket's ID sorts lexicographically after every existing date-time-prefixed ID; an old-shape ID (<prefix>-<4char>) and a new-shape ID coexist in the same directory and both resolve via substring match.
- A ticket fixture carrying an unrecognized frontmatter key round-trips that key unchanged through a read/write cycle.


## Notes

**2026-09-03T07:12:20Z**

branch: feature/ticket-storage-layer-frontmatter-parse-write-resolve-id-scheme
base: chore/project-scaffolding-and-command-tree
claim_sha: 3fd3f22dc00165f8388793fb800d262edc88d302

**2026-09-03T07:26:03Z**

branch: feature/ticket-storage-layer-frontmatter-parse-write-resolve-id-scheme
pr: https://github.com/Christophe1997/goalship/pull/3
sha: e4db0e56a1bb25f1ed3b598403fa2c424d2db898
