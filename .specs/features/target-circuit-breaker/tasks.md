# Target circuit breaker and readiness tasks

**Design:** `design.md`
**Status:** In progress

## Execution plan

```text
T1 -> T2 -> T3 -> T4
```

## Tasks

### T1: Add circuit-breaker configuration

**What:** Add strict global routing policy fields, defaults, validation, and
example configuration.
**Where:** `internal/config`, `config.example.yaml`.
**Depends on:** None.
**Reuses:** Existing duration defaults and validation patterns.
**Requirements:** CB-01, CB-08.
**Tools:** Local filesystem and Go test runner; TLC skill.
**Tests:** Unit. **Gate:** Quick.

### T2: Implement the breaker state machine

**What:** Add a concurrency-safe breaker with closed, open, and half-open
behavior, one-shot permits, generations, snapshots, and deterministic tests.
**Where:** `internal/circuitbreaker`.
**Depends on:** T1.
**Reuses:** `ratelimit` package structure and clock injection pattern.
**Requirements:** CB-02 through CB-05, CB-08.
**Tools:** Local filesystem and Go test runner; TLC skill.
**Tests:** Unit. **Gate:** Quick.

### T3: Integrate routing and exhaustion behavior

**What:** Share breakers by target identity, classify results, skip open
targets, preserve fallback, and return `503 circuit_open` when exhausted.
**Where:** `internal/gateway`.
**Depends on:** T2.
**Reuses:** Target keys, response classification, and error transport.
**Requirements:** CB-02 through CB-06, CB-08.
**Tools:** Local filesystem and Go test runner; TLC skill.
**Tests:** Integration with `httptest`. **Gate:** Full.

### T4: Add readiness and operator guidance

**What:** Add public `/readyz`, non-sensitive target counts, documentation,
project state, and final validation evidence.
**Where:** `internal/httpapi`, README, `docs`, and `.specs`.
**Depends on:** T3.
**Reuses:** `/healthz`, middleware, and documentation patterns.
**Requirements:** CB-07, CB-08.
**Tools:** Local filesystem and Go test runner; TLC and docs-writer skills.
**Tests:** End-to-end with `httptest`. **Gate:** Build.

## Dependency cross-check

| Task | Declared dependency | Diagram dependency | Status |
| --- | --- | --- | --- |
| T1 | None | None | Match |
| T2 | T1 | T1 | Match |
| T3 | T2 | T2 | Match |
| T4 | T3 | T3 | Match |

## Test co-location check

| Task | Code layer | Matrix requires | Task says | Status |
| --- | --- | --- | --- | --- |
| T1 | Configuration | Unit | Unit | Match |
| T2 | Internal state machine | Unit | Unit | Match |
| T3 | Routing and transport | Integration | Integration | Match |
| T4 | Public HTTP endpoint | End-to-end | End-to-end | Match |
