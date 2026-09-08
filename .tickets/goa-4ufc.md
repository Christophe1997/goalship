---
id: goa-4ufc
status: closed
deps: [goa-et9p]
links: []
created: 2026-09-03T06:41:47Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U8B
---
# Review API routes: ticket edit, reject, withdraw, approve

internal/reviewserver/api.go: GET /api/tickets (lists every ticket in the graph via U2), PATCH /api/tickets/:id (structured field edits — title, description, acceptance criteria, priority, dependencies — writing straight through to .tickets/*.md immediately via U2, no in-memory draft, no delete/rename route; refused outright while a rejection is awaiting regeneration — read-only mode), POST /api/reject (writes review_state: rejected + notes atomically via U5A — R11), POST /api/withdraw (writes review_state: pending directly, independent of agent involvement — R19), POST /api/approve (writes review_state: approved plus approved_ticket_ids — the exact current ticket ID set at the moment of approval — R12; releases U5B's lock; the CLI process exits once the page confirms). Ticket content the API returns is treated as data: no server-side HTML generation from ticket text — the browser side (U8C) sanitizes and renders it. Every mutating route reuses U8A's token+Host validation before touching U2/U5A.

## Acceptance Criteria

- PATCH /api/tickets/:id writes the edit straight through to .tickets/*.md immediately, visible on the next GET /api/tickets with no separate flush step.
- POST /api/reject writes review_state: rejected and the notes atomically to the run ledger; a subsequent PATCH is refused (read-only) until withdrawn or regenerated.
- POST /api/withdraw writes review_state: pending directly, with no agent call or process involved, moving the run back out of read-only mode.
- POST /api/approve writes approved_ticket_ids matching exactly the ticket ID set GET /api/tickets returned at approval time, and releases the per-run lock.
- A PATCH request arriving with a missing/wrong token or a foreign Host header is refused with no ticket file touched.


## Notes

**2026-09-03T11:27:06Z**

branch: feature/review-api-routes-ticket-edit-reject-withdraw-approve
base: feature/review-server-core-bind-token-host-validation-csp-lock-open-browser
claim_sha: 0b36822229e0ff5fa755e63d06cec01f49f8f12e

**2026-09-03T11:55:01Z**

branch: feature/review-api-routes-ticket-edit-reject-withdraw-approve
pr: https://github.com/Christophe1997/goalship/pull/13
sha: b0789e3d14c5c1fe8fe43af53d8610d40d3976b1
