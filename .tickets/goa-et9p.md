---
id: goa-et9p
status: open
deps: [goa-g7ei, goa-9nhc, goa-6yik, goa-4z08]
links: []
created: 2026-09-03T06:41:47Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U8A
---
# Review server core: bind, token, Host validation, CSP, lock, open browser

internal/reviewserver/server.go + security.go + internal/cli/review.go: 'goalship review <run-id>' resolves the run's ledger (via U7's read path, not a duplicate); refuses to start if the run-id doesn't exist or review_state is already 'approved' (R9), reporting a clear error. Acquires U5B's per-run lock (R14) — a second concurrent invocation against the same run-id fails fast. Binds 127.0.0.1:0 (OS-assigned ephemeral port, loopback only, never a routable interface — R20). Mints a per-invocation token via crypto/rand, at least 32 bytes, encoded for a URL query parameter (never math/rand, never a cookie — a cookie would auto-attach to any-origin requests, reintroducing CSRF; a browser's native EventSource can't set custom headers either, which is why the token rides the query string uniformly on every route). security.go validates the token via URL query parameter on every request (including the future SSE route) and, on mutating routes, the port-stripped Host header against localhost/127.0.0.1/::1 — refusing with no partial effect (no ticket/ledger read or write) before any handler runs, on either check's failure. Every response carries a restrictive Content-Security-Policy (object-src 'none', base-uri 'none', frame-ancestors 'none') and Referrer-Policy: no-referrer. Prints the tokened URL to stdout BEFORE attempting to open it via the OS's default-browser call (open/xdg-open/start) — so a headless host or an SSH session without a forwarded port still gives the operator something to act on if the open call fails silently. index.html/app.js/app.css are embedded via //go:embed (assets/ subdirectory) — no Node/JS build step; index.html itself is templated at serve time to interpolate the token into app.js/app.css's asset URLs' query strings (KTD6) rather than served as untouched static embed content. This ticket ships the server skeleton, security checks, asset serving, and process lifecycle (clean exit without a decision releases the lock and leaves review_state unchanged) — the actual API route handlers (tickets CRUD, reject/withdraw/approve) are ticket U8B and the fsnotify/SSE live-refresh is ticket U8C; this ticket's own data routes can be stubs returning 501 until U8B lands, EXCEPT the token/Host/CSP checks themselves, which must be real and independently testable here.

## Acceptance Criteria

- `goalship review <bad-or-missing-run-id>` refuses to start and reports a clear error, with no server bound.
- `goalship review <run-id>` for a run whose review_state is already "approved" refuses to start and reports a clear error.
- Two `goalship review` processes started against the same run-id: the second fails fast, naming the lock (reuses U5B).
- A request with a missing or incorrect token is refused with no ticket/ledger read or write occurring, on every route including the initial page load.
- A mutating request with a Host header other than localhost/127.0.0.1/::1 (including one that only differs by port) is refused before any handler touches ticket/ledger state.
- Every HTTP response includes the specified CSP and Referrer-Policy headers.
- The tokened URL is printed to stdout before the browser-open call is attempted, so it's visible even if that call fails.
- Review server killed (not clean-exited) mid-review: the lock does not wedge a later invocation against the same run-id.

