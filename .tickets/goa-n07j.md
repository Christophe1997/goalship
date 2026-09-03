---
id: goa-n07j
status: closed
deps: [goa-g7ei]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 2
assignee: Christophe1997
external-ref: U4
---
# goalship tk: dependency graph and jq-backed query

dep, undep, dep tree, dep cycle, link, unlink mutate the deps/links arrays with no referential-integrity check (matches bash tk's own looseness). dep tree/dep cycle walk the in-memory graph, no external dependency. query and migrate-beads route through internal/ticket/query.go, wrapping github.com/itchyny/gojq (pin latest stable tag) — confirmed directly against the installed tk binary (/opt/homebrew/Cellar/ticket/0.3.2/bin/tk): cmd_query passes a caller-supplied filter into jq -c "select($filter)" with no restriction; cmd_migrate_beads runs a genuinely non-trivial jq program (def functions, map, string interpolation, the // alternative operator). Read tk's cmd_query/cmd_migrate_beads directly (bash source) for the exact jq program shapes to reproduce.

## Acceptance Criteria

- `query` against the three filter shapes goalship's loop sends today (`.`, `select(.status=="in_progress")`, `select(.id=="X")`) returns identical JSON to bash tk on the same fixture.
- `migrate-beads` against a sample .beads/issues.jsonl fixture matches bash tk's output; any divergence found during implementation is flagged in the ticket's own notes rather than silently accepted.
- `dep`/`undep` on a nonexistent dependency ID matches bash tk's existing error behavior.


## Notes

**2026-09-03T08:30:02Z**

branch: feature/goalship-tk-dependency-graph-and-jq-backed-query
base: feature/ticket-storage-layer-frontmatter-parse-write-resolve-id-scheme
claim_sha: e4db0e56a1bb25f1ed3b598403fa2c424d2db898

**2026-09-03T09:09:35Z**

branch: feature/goalship-tk-dependency-graph-and-jq-backed-query
pr: https://github.com/Christophe1997/goalship/pull/7
sha: 44d7fc137465e8af860fe4e88c7958c8d840c0d3
