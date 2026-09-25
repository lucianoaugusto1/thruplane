# Performance baseline specification

**Status:** Verified
**Date:** September 25, 2026

## Problem

NexoRoute has protocol and failure tests, but it doesn't have a reproducible
end-to-end performance baseline. A microbenchmark for one parser can't show
gateway-added latency, streaming first-token time, throughput, connection
reuse, or behavior under failure and backpressure.

## Goals

- Measure gateway-added p50, p95, and p99 latency against a direct mock
  upstream baseline.
- Measure time to first SSE event and total requests per second.
- Expose allocation, memory, CPU profile, and heap profile evidence.
- Measure client-to-gateway and gateway-to-upstream connection reuse.
- Cover text, tools, images, PDFs, audio, retry, fallback, `429`, slow clients,
  and cancellation.
- Keep the default benchmark deterministic, local, and free of provider cost.

## Requirements

### PERF-01: Latency percentiles

WHEN a load scenario completes THEN the harness SHALL report p50, p95, p99,
maximum, and mean latency for direct and gateway paths, plus their percentile
difference.

### PERF-02: Streaming first token

WHEN an SSE scenario runs THEN the harness SHALL measure the interval from
request start until the first response body byte for direct and gateway paths.

### PERF-03: Throughput

WHEN a load phase completes THEN the harness SHALL report completed requests,
wall-clock duration, requests per second, and unexpected failures.

### PERF-04: Runtime cost

WHEN the Go benchmark suite runs with `-benchmem` THEN it SHALL report
nanoseconds, bytes, and allocations per operation. The load runner SHALL report
memory deltas and support optional CPU and heap profiles.

### PERF-05: Connection reuse

WHEN requests use HTTP keep-alive THEN the harness SHALL report connection
reuse for the load client and infer gateway-to-upstream reuse from request and
accepted-connection counts.

### PERF-06: Modality coverage

WHEN the modality suite runs THEN text, function tools, an inline image, an
inline PDF, and inline audio SHALL pass through compatible or native adapters.
Media payloads SHALL contain at least 48 KiB before base64 encoding.

### PERF-07: Failure and backpressure coverage

WHEN the failure suite runs THEN retry, fallback, permanent `429`, slow SSE
reader, and request cancellation scenarios SHALL produce their defined
outcomes without leaked or retried canceled work.

### PERF-08: Reproducibility

WHEN results are emitted THEN they SHALL include the scenario, request count,
concurrency, payload size, Go version, operating system, architecture, and
logical CPU count. JSON output SHALL be available for later comparison.

### PERF-09: Safety

WHEN the default suite runs THEN it SHALL bind only local `httptest` servers,
use synthetic payloads, and require no credentials or public network calls.

### PERF-10: Verification

WHEN implementation completes THEN unit, integration, race, vet, build, load
smoke, and benchmark smoke gates SHALL pass without skipped tests.

## Out of scope

- Live provider latency or throughput claims.
- A universal latency, RPS, CPU, or memory threshold.
- Distributed load generation across hosts.
- Long-running soak, chaos, or capacity tests in the default test gate.
- Persisting benchmark history in a hosted service.

## Success criteria

- One command runs the deterministic load matrix.
- One command emits machine-readable JSON.
- One command reports `ns/op`, `B/op`, and `allocs/op`.
- Optional commands generate CPU and heap profiles.
- The complete project gate passes.

## Traceability

| Requirement | Task | Status |
| --- | --- | --- |
| PERF-01, PERF-03 | T1, T3 | Verified |
| PERF-02, PERF-05 | T3 | Verified |
| PERF-04 | T3, T4 | Verified |
| PERF-06, PERF-07 | T2, T3 | Verified |
| PERF-08, PERF-09 | T2, T4 | Verified |
| PERF-10 | T1-T5 | Verified |
