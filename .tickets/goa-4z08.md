---
id: goa-4z08
status: open
deps: [goa-9nhc]
links: []
created: 2026-09-03T06:41:47Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U7
---
# goalship review-status: plain CLI discovery command

internal/cli/loop/reviewstatus.go: a pure read-only command — reads the run's ledger review_state/review_notes/review_updated_at (via U5A) and prints as JSON, matching the existing JSON-output convention. No side effects, safe to poll freely. Reports review_state: approved explicitly (not only pending/rejected) so a caller never has to infer approval from the mere absence of a rejection. A nonexistent run-id errors clearly.

## Acceptance Criteria

- A pending run reports no actionable rejection.
- A rejected run reports the notes verbatim, byte-for-byte as written.
- An approved run reports review_state: approved explicitly.
- A nonexistent run-id errors clearly and consistently with how preflight reports a missing/invalid run elsewhere in this system.

