# Target circuit breaker and readiness validation

**Status:** Passed
**Date:** September 25, 2026

## Coverage

| Requirement | Evidence | Result |
| --- | --- | --- |
| CB-01 | Default, explicit policy, unknown field, and invalid-value tests | Passed |
| CB-02 | Shared-alias routing test and unique readiness counts | Passed |
| CB-03 | Retryable status, `429`, cancellation, and permit outcome tests | Passed |
| CB-04 | Threshold, open skip, ordered fallback, and no-network checks | Passed |
| CB-05 | Single concurrent probe, success, failure, cancellation, and stale-result tests | Passed |
| CB-06 | OpenAI-shaped `503 circuit_open` and `Retry-After` integration test | Passed |
| CB-07 | Public ready/not-ready endpoint and unchanged liveness test | Passed |
| CB-08 | Full test, race, vet, build, and diff gates | Passed |

## Gates

```text
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./cmd/nexoroute
git diff --check
```

All gates passed without live credentials or public network calls. Tests use
local `httptest` upstreams and an injected clock for deterministic open and
half-open transitions.

## Deliberate boundary

Circuit state is process-local and passive. NexoRoute learns health from real
requests and doesn't run background probes. `429` and rate-limit cooldown don't
affect readiness because restarting a gateway doesn't restore provider quota.
Streaming responses count as healthy when upstream headers succeed; detecting
a later stream-body failure remains an observability follow-up.
