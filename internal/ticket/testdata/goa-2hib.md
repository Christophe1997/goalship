---
id: goa-2hib
status: open
deps: [goa-g7ei, goa-5zwn]
links: []
created: 2026-09-03T06:41:47Z
type: feature
priority: 2
assignee: Christophe1997
external-ref: U6C
---
# Reconcile: stacked-base PR retargeting

internal/gitops/reconcile.go + internal/cli/loop/reconcile.go: ports reconciliation.py's reconcile() (/Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts/reconciliation.py, read directly) — for each in_progress ticket, resolve its run branch and open PR, retarget the PR's base when the stack's actual base has moved, and report a ReconciliationAction per ticket touched: {ticket_id, outcome, detail, pr_ref}, where outcome is one of closed_merged, closed_ship_note_orphaned, failed_closed_unmerged, no_recoverable_state, retry_pr_creation, retarget_base_merged, blocked_stale_base, pr_state_unresolved (execution-loop.md's reconcile table documents what each outcome means and what the CLI caller does with it — match reconciliation.py's exact conditions for emitting each one).

## Acceptance Criteria

- reconcile retargets a stacked ticket's PR base when its parent branch has landed (merged), and reports one ReconciliationAction per ticket touched, matching loop_runner.py reconcile's JSON shape on an equivalent fixture.
- Each of the eight documented outcome values is reproduced under the same fixture condition reconciliation.py produces it for (closed_merged, closed_ship_note_orphaned, failed_closed_unmerged, no_recoverable_state, retry_pr_creation, retarget_base_merged, blocked_stale_base, pr_state_unresolved).
- auth_failure surfaces non-null when the same credential fails repeatedly, matching loop_runner.py's behavior.

