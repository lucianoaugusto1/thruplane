# Target circuit breaker and readiness specification

**Status:** Approved
**Date:** September 25, 2026

## Problem

Retries and rate limits protect individual requests and provider capacity, but
the gateway still sends every new request to a repeatedly failing primary
target. Operators also have only a liveness endpoint, so orchestration cannot
distinguish a running process from one whose configured routes are temporarily
unavailable.

## Goals

- Stop calling a failing physical target after a configurable threshold.
- Share circuit state across aliases that use the same provider and model.
- Recover automatically through one concurrency-safe half-open probe.
- Expose readiness without turning provider rate limiting into pod churn.

## Requirements

### CB-01: Explicit policy

WHEN `routing.circuit_breaker.failure_threshold` is zero or omitted THEN the
circuit breaker SHALL be disabled. WHEN enabled THEN `open_duration` SHALL be
positive and configuration validation SHALL reject invalid values.

### CB-02: Shared target state

WHEN aliases reference the same configured provider and physical model THEN
they SHALL share one concurrency-safe breaker.

### CB-03: Failure classification

WHEN a non-canceled transport error or retryable `408`, `500`, `502`, `503`, or
`504` response occurs THEN the target breaker SHALL record a failure. Other
HTTP responses SHALL prove reachability and reset consecutive failures. Local
admission denial, adapter validation, and client cancellation SHALL not count.

### CB-04: Open and fallback

WHEN consecutive failures reach the threshold THEN the breaker SHALL open for
the configured duration. New requests SHALL skip that target without network
I/O and continue ordered fallback.

### CB-05: Half-open recovery

WHEN the open duration expires THEN exactly one request SHALL probe the target.
Other concurrent requests SHALL skip it. Probe success SHALL close the circuit;
probe failure SHALL reopen it for a full duration; probe cancellation SHALL
allow a later probe without recording failure.

### CB-06: Exhaustion response

WHEN every eligible target is circuit-open and no upstream call occurs THEN
the gateway SHALL return an OpenAI-shaped `503 circuit_open` response with
`Retry-After` when a positive retry time is known.

### CB-07: Readiness

WHEN `GET /readyz` is requested THEN the public endpoint SHALL return `200`
with non-sensitive target counts if at least one target can accept a request,
or `503` when every configured target is circuit-open. `/healthz` SHALL remain
an unconditional process-liveness check. Rate-limit cooldown SHALL not change
readiness.

### CB-08: Verification

WHEN implementation completes THEN configuration, breaker state machine,
fallback, shared-alias, cancellation, readiness, race, vet, and build checks
SHALL pass with local deterministic tests.

## Out of scope

- Active background health probes.
- Distributed circuit state across gateway replicas.
- Weighted or latency-based routing.
- Opening circuits for `429` responses or invalid credentials.
- Prometheus or OpenTelemetry export.

## Traceability

| Requirement | Task | Status |
| --- | --- | --- |
| CB-01 | T1 | Verified |
| CB-02 through CB-05 | T2-T3 | Implementing |
| CB-06 | T3 | Pending |
| CB-07 | T4 | Pending |
| CB-08 | T1-T4 | Pending |
