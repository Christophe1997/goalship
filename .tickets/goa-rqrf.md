---
id: goa-rqrf
status: closed
deps: [goa-9nhc, goa-g7ei]
links: []
created: 2026-09-03T06:41:46Z
type: feature
priority: 2
assignee: Christophe1997
external-ref: U5C
---
# Claim structural approval gate + ledger flag-surface assertion (R16/R12)

internal/cli/loop/claim.go: 'claim' loads the run's ledger (U5A) and refuses to claim any ticket unless review_state == "approved" AND the claimed ticket ID is present in approved_ticket_ids — both checks live in claim itself, not left to a caller's discretion (R16, mirrors how this codebase's other safety invariants — merge/force-push/branch-delete prevention — are enforced structurally and asserted directly against source, not left to agent-followed prose). On success, claim creates the branch off base_ref (or checks it out if it already exists — commit mode's second-and-later tickets) and records the claim note (branch:, plus base: only when base_ref differs from trunk) via U2's ticket-notes primitive, the Go equivalent of loop_runner.py's record_claim_note -> tk_add_note. Read loop_runner.py's cmd_claim directly (/Users/christophe/.claude/plugins/cache/agent-extensions/goalship/0.4.0/scripts/loop_runner.py) for exact argv/JSON shape: claim <repo_root> <ticket_id> <branch_name> <base_ref> <trunk_branch>. Separately, write a test asserting directly against U5A's ledger.go CLI command's flag definitions that no flag sets review_state or approved_ticket_ids (R12) — the same source-assertion discipline as claim's own test, applied to the one command capable of bypassing the gate if it ever grew such a flag.

## Acceptance Criteria

- claim succeeds only when review_state == "approved" AND the ticket ID is in approved_ticket_ids; refuses for pending, rejected, an absent review_state field, and an approved run whose approved_ticket_ids doesn't contain the claimed ticket — enumerate all five cases in the test suite.
- `goalship loop ledger`'s flag set has no flag capable of writing review_state or approved_ticket_ids, asserted directly against ledger.go's flag definitions (not by exercising every flag combination).
- A successful claim's branch creation and claim-note recording happen as one atomic sequence with no window where the branch exists but the note doesn't (or vice versa).


## Notes

**2026-09-03T13:26:32Z**

branch: feature/claim-structural-approval-gate-ledger-flag-surface-assertion
base: feature/ledger-core-state-codec-r7-r8-plus-preflight-dirty-resume-candidates
claim_sha: 2009b78a9628b303e19a31eec82a444920734aed

**2026-09-03T13:44:35Z**

branch: feature/claim-structural-approval-gate-ledger-flag-surface-assertion
pr: https://github.com/Christophe1997/goalship/pull/16
sha: ea3aac98e4bd983d100753c954c39b6ca4beea35
