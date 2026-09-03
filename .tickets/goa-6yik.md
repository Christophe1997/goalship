---
id: goa-6yik
status: closed
deps: [goa-fxh3]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 1
assignee: Christophe1997
external-ref: U5B
---
# Review lock: OS-native advisory file lock (R14/KTD4)

internal/ledger/lock.go: OS-native advisory locking (gofrs/flock, or equivalent) on a file at .goalship/<run_id>.review.lock, held for the process's lifetime and released automatically on exit — including a killed process, via the OS's own lock-release-on-close semantics. Deliberately NOT a PID-liveness check: a prior design using PID-liveness had two races (PID reuse false-positiving a stale lock as live; a non-atomic check-then-write acquire letting two invocations both observe 'no lock'). OS-native advisory locking has neither failure mode since the kernel arbitrates, not application logic. Small, self-contained package — a lock keyed by run_id string, no dependency on the ledger's own data types.

## Acceptance Criteria

- Two concurrent acquisitions of the same run's lock: the second fails fast, naming the lock, rather than blocking indefinitely or silently succeeding.
- Killing (not clean-exiting) the lock holder process releases the lock for a later acquisition.
- Two different run_ids' locks never contend with each other.


## Notes

**2026-09-03T07:02:29Z**

branch: feature/review-lock-os-native-advisory-file-lock
base: chore/project-scaffolding-and-command-tree
claim_sha: 3fd3f22dc00165f8388793fb800d262edc88d302

**2026-09-03T07:10:13Z**

branch: feature/review-lock-os-native-advisory-file-lock
pr: https://github.com/Christophe1997/goalship/pull/2
sha: 0b1ee213efa9045443fd1be4561eeca251ec9795
