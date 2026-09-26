# Community beta validation

**Date:** September 26, 2026  
**Spec:** `.specs/features/community-beta/spec.md`  
**Candidate:** `6368837` plus the documentation qualification commit

## Task completion

| Task | Status | Evidence |
| --- | --- | --- |
| T1 | Done | Beta scope, design, and traceable tasks committed. |
| T2 | Done | Metrics configuration tests pass. |
| T3 | Done | Collector unit and concurrency tests pass. |
| T4 | Done | HTTP endpoint, privacy, readiness, and route tests pass. |
| T5 | Done | CLI tests, real preflight, and metadata build pass. |
| T6 | Done | Read-only, SHA-pinned CI workflow added. |
| T7 | Done | Runbook published and all qualification gates pass. |

## User story validation

### Validate a deployment before start

| Criterion | Result |
| --- | --- |
| Valid configuration exits zero and confirms the path. | Pass |
| Invalid configuration exits nonzero without exposing the API key. | Pass |
| Version mode reports version, revision, and date without config. | Pass |

### Observe gateway health and traffic

| Criterion | Result |
| --- | --- |
| Disabled metrics return `404` without metrics middleware. | Pass |
| Enabled metrics expose the specified bounded metric families. | Pass |
| Prompts, keys, headers, and raw unknown paths do not appear. | Pass |
| Concurrent updates pass the race detector. | Pass |

### Reproduce the beta gate

| Criterion | Result |
| --- | --- |
| CI checks formatting, race tests, vet, and build. | Pass |
| The runbook covers start, smoke, diagnosis, and rollback. | Pass |
| Deterministic, race, build, and performance gates pass. | Pass |

## Edge cases

- [x] A scrape before chat traffic emits metadata and zero-safe gauges.
- [x] Unknown paths use the fixed `unmatched` label.
- [x] Quotes, backslashes, and newlines are escaped in labels.
- [x] Streaming remains incremental through nested response recorders.
- [x] In-flight accounting remains active until a handler returns.

## Tests

| Gate | Result |
| --- | --- |
| `gofmt -l ./cmd ./internal ./bench ./tests` | Pass; no output. |
| `go test ./...` | Pass; no failures. |
| `go test -race ./...` | Pass; no races or failures. |
| `go vet ./...` | Pass. |
| `go build ./cmd/nexoroute` | Pass. |
| One-run benchmark matrix | Pass for all gateway and direct scenarios. |
| Metadata binary and example-config preflight | Pass. |

The repository contained 171 top-level `Test` functions before this feature
and contains 180 after it, a net increase of nine. No deterministic test was
deleted or skipped. The opt-in live-provider suite remains outside the default
gate because it requires scoped credentials and a spending cap.

## Code quality

| Principle | Result |
| --- | --- |
| Minimum implementation | Pass; the collector covers a fixed metric set. |
| Surgical changes | Pass; runtime changes are limited to config, HTTP, CLI, and telemetry. |
| Dependency discipline | Pass; no external Go dependency was added. |
| Existing patterns | Pass; strict config and `httptest` patterns are reused. |
| Streaming and cancellation | Pass; the full race and performance suites cover both. |

Interactive UAT was not required because this slice is backend and operator
infrastructure. The existing playground was not visually changed.

## Requirement traceability

| Requirement | Result |
| --- | --- |
| BETA-01 | Verified |
| BETA-02 | Verified |
| BETA-03 | Verified |
| BETA-04 | Verified |
| BETA-05 | Verified |
| BETA-06 | Verified |
| BETA-07 | Verified |

## Summary

**Overall:** Ready for Community beta testing.

The remaining high-value external gate is recorded live conformance for chosen
provider/model/region pairs. It does not invalidate the deterministic beta
contract, but it must stay visible in capability claims and production rollout
decisions.
