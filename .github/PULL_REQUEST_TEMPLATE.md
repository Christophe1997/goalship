## Summary

<!-- What does this change do, and why? -->

## Related

<!-- Issue number, ticket ID (e.g. goa-xxxx), or plan/unit reference, if any -->

## Verification

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `gofmt -l .` (no output)
- [ ] `go test ./...` (add `-race` for anything touching concurrency, locking, or subprocess I/O)

<!-- For a behavior change ported from bash tk or loop_runner.py: note any verified parity (or documented divergence) against the original. -->

## Notes for reviewers

<!-- Anything a reviewer should know that isn't obvious from the diff: design tradeoffs, follow-ups filed, known limitations. -->
