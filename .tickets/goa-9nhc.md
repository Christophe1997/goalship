---
id: goa-9nhc
status: open
deps: [goa-fxh3, goa-g7ei]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U5A
---
# Ledger core: state codec (R7/R8) plus preflight/dirty/resume-candidates

internal/ledger/state.go ports run_state.py's RunState (/Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts/run_state.py, read directly) — fields run_id, shipped_count, consecutive_failures, claimed_ticket_ids, goal, ticket_mode, terminal_state, trunk_branch — plus new review_state/review_notes/review_updated_at/approved_ticket_ids fields (R7). Load/save is byte-compatible with the existing JSON ledger shape and preserves any JSON key the struct doesn't recognize, re-emitting it unchanged (R8) — decode into a generic map first or an 'extra fields' bag rather than a closed struct. An absent review_state reads back as 'pending', never 'approved'. Writes go through U1's atomicfile, one file per run_id under .goalship/<run_id>.json. Port dirty_paths' _IGNORED_DIRTY_DIR_NAMES unchanged (.goalship/, .tickets/ both excluded) and ensure_ledger_excluded unchanged (adds /.goalship/ to .git/info/exclude if absent). CLI commands (internal/cli/loop/): preflight (git remote + trunk-branch autodetect via origin/HEAD then local main/master then current branch, optional override arg; prints {ok, remote_url, trunk_branch, host_tool, failures}), dirty, ledger (bare status read; --goal/--ticket-mode/--trunk-branch on first call; --claim/--ship/--fail/--terminal flag surface per run_state.py's cmd_ledger — no flag here sets review_state or approved_ticket_ids, that guarantee is asserted by a separate ticket U5C, not this one), resume-candidates (scans .goalship/*.json directly, returns every ledger whose terminal_state is still null). Read preflight.py and loop_runner.py's cmd_ledger/cmd_resume_candidates directly at /Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts for exact behavior and JSON shape.

## Acceptance Criteria

- Ledger read/write round-trips byte-identically against a fixture matching run_state.py's schema.
- A ledger JSON fixture carrying an unrecognized key round-trips that key unchanged through a load/save cycle.
- resume-candidates parity: same JSON output shape as loop_runner.py resume-candidates on an equivalent fixture, filtering out any ledger with a non-null terminal_state.
- ensure_ledger_excluded adds /.goalship/ to a repo's .git/info/exclude exactly once, idempotent on repeated calls.
- preflight against a repo with a configured origin and a local main branch resolves trunk_branch correctly and reports {ok:true, remote_url, trunk_branch, host_tool}; an unresolvable override branch lands in failures rather than silently falling back to autodetection.

