# Target circuit breaker and readiness design

**Specification:** `spec.md`
**Status:** Approved

## Architecture

```text
chat plan
   |
   v
target breaker acquire -> local limiter acquire -> provider call
   | denied                                      |
   +-----------------> next target <-------------+ observe result

GET /readyz -> gateway readiness snapshot -> HTTP 200 or 503
```

Each `provider/model` identity owns one breaker, matching the existing limiter
identity. The breaker runs before admission so an open target doesn't consume
rate or concurrency capacity. It uses only in-process memory and the gateway's
injectable clock.

## Components

### Configuration

`routing.circuit_breaker` contains `failure_threshold` and `open_duration`.
Zero threshold disables the feature for backward compatibility. The duration
defaults to 30 seconds and becomes mandatory when the threshold is positive.

### Breaker state machine

`internal/circuitbreaker` owns the mutex, consecutive-failure count, open
deadline, half-open probe, and generation number. `Acquire` returns a permit or
a denial. A permit completes once as success, failure, or cancellation.

Generation numbers prevent late results from requests started before a trip
from closing or reopening a newer circuit generation.

### Gateway integration

The executor acquires a breaker permit before every upstream attempt. It
records only the retryable health failures already recognized by gateway
routing, excluding `429`. Circuit denial skips the target. If no upstream and
no stronger local rate-limit denial exists, the handler returns `503`.

### Readiness

The gateway returns a snapshot with total, available, open, and half-open
target counts. A disabled or closed breaker is available. An expired open
breaker is half-open and available for one probe. Readiness contains no URLs,
provider names, model IDs, credentials, or prompt content.

## Reuse

| Existing component | Reuse |
| --- | --- |
| `gateway.targetKey` | Physical target identity |
| Gateway injectable `now` clock | Deterministic state transitions |
| `classifyResponse` | Retryable health-response classification |
| `retryAfterSeconds` | Standards-compliant response delay |
| Existing OpenAI error writer | Exhaustion response |
| `/healthz` middleware stack | Public readiness routing and request IDs |

## Decisions

| Decision | Choice | Reason |
| --- | --- | --- |
| Default | Disabled | Preserve existing routing behavior until operators opt in. |
| `429` | Don't trip | Existing cooldown and quota logic already owns capacity failures. |
| `4xx` | Success for reachability | The provider responded; request/auth policy is a separate concern. |
| Stream health | Success at response headers | The current provider client doesn't report later body failures to routing. |
| Readiness and cooldown | Separate | A provider quota wait must not cause orchestrator restart loops. |
