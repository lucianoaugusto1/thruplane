# Provider rate-limit validation

**Date:** September 24, 2026
**Result:** Passed

## Requirement results

| Requirement | Result | Evidence |
| --- | --- | --- |
| RL-01 | Pass | Retry delay tests cover seconds, HTTP-date, invalid hints, jitter, and cancellation. |
| RL-02 | Pass | Budget tests reject delays at or beyond the remaining deadline and verify fallback. |
| RL-03 | Pass | Provider-specific permanent quota fixtures skip same-target retry; unknown `429` remains temporary. |
| RL-04 | Pass | Token-bucket, burst, concurrency, shared-alias, queue, and validation tests pass. |
| RL-05 | Pass | OpenAI duration reset, Anthropic RFC3339 reset, monotonic cooldown, and header relay are tested. |
| RL-06 | Pass | Permit release is idempotent; streaming integration holds capacity until EOF or Close. |
| RL-07 | Pass | Local request-rate and concurrency exhaustion return OpenAI-shaped `429` responses and reason headers. |
| RL-08 | Pass | Defaults, negative values, burst rules, and conflicting shared-target policies are tested. |
| RL-09 | Pass | Full test, race, vet, build, and diff checks pass. |

## Gates

```text
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/nexoroute
git diff --check
```

All gates passed. Tests use local `httptest` servers and synthetic credentials;
no live provider request was made.

## Deliberate boundary

The implemented limiter is process-local and request-aware. Exact local token
reservation is deferred because providers and model families account for input,
output, cache, and maximum-output reservations differently. Supported provider
remaining/reset headers still extend the shared cooldown. Distributed tenant
quotas require identity and shared state and remain a paid-control-plane
direction rather than a hidden dependency in the Community data path.
