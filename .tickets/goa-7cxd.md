---
id: goa-7cxd
status: open
deps: [goa-et9p, goa-4ufc]
links: []
created: 2026-09-03T06:41:47Z
type: feature
priority: 2
assignee: Christophe1997
external-ref: U8C
---
# Live refresh (fsnotify+SSE+poll) and embedded front-end assets

internal/reviewserver/watch.go + assets/index.html, app.js, app.css: fsnotify (Go 1.23+) watches the ledger file's parent directory (not the file itself — avoids the inode-invalidation problem a rename-based atomic write causes for a file-level watch), filtering to this run's ledger path, starting when the review server opens and running for the whole session (not lazily on first reject, so a regeneration landing before the first rejection is still picked up). On a review_updated_at change, GET /api/events (Server-Sent Events, stdlib net/http, token via query param since native EventSource can't set custom headers) pushes a notification keyed on review_updated_at (not raw ticket data); the page re-fetches GET /api/tickets on receipt. Because fsnotify events can be coalesced or dropped (and a network-mounted .goalship/ may not support notifications at all), the page also polls review_updated_at on a short fixed interval as a fallback — SSE is a latency optimization, not the sole detection mechanism. The front-end (vanilla JS/CSS, no framework, no build step) tracks per-ticket-form dirty state client-side: a live-refresh event never overwrites an open unsaved edit, it queues the update instead; a PATCH arriving just as the graph flips read-only fails with the operator's typed content still visible on the form. The read-only view shown during a pending rejection displays an explicit banner stating regeneration is pending, updating the moment a refresh lands. Ticket content rendered by the page — including any markdown preview — passes through an allowlisted sanitizer (no raw HTML) consistent with U8A's CSP.

## Acceptance Criteria

- Reject with notes; a separate process writes a regenerated graph to the ledger; the still-running review server pushes an SSE refresh the open page picks up without the page reloading or goalship review restarting (AE1).
- With fsnotify events artificially suppressed, the page still picks up a review_updated_at change within one polling interval (fallback path proven independently of SSE).
- A ticket whose title or description contains an HTML/script payload renders as inert text (or sanitized markdown) in the page, never as executable script.
- An open, unsaved ticket-edit form is never silently overwritten by an incoming live-refresh event — the update queues instead.
- The read-only banner shown during a pending rejection updates the moment a regeneration lands, rather than the page silently re-enabling with no explanation.

