# Performance baseline specification

**Status:** Implementing regression gate
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

### PERF-11: Versioned artifacts

WHEN a baseline run is requested THEN the runner SHALL execute at least three
repetitions and write a versioned JSON artifact containing the revision,
environment identifier, generation time, load shape, and every raw report.

### PERF-12: Comparable environments

WHEN two artifacts are compared THEN the comparator SHALL reject different
schema versions, environment identifiers, Go versions, operating systems,
architectures, CPU counts, scenario sets, or load shapes.

### PERF-13: Noise-resistant regression decisions

WHEN compatible artifacts are compared THEN the comparator SHALL use the
median across runs and configurable limits for latency, SSE TTFT, throughput,
allocation cost, connection reuse, and unexpected failures. Latency and TTFT
SHALL exceed both relative and absolute limits before failing.

### PERF-14: CI regression gate

WHEN a pull request runs the performance workflow THEN the workflow SHALL
measure the base commit and candidate commit on the same runner, compare them,
fail on a regression, and retain both artifacts plus the comparison report.

### PERF-15: Regression-gate verification

WHEN regression-gate implementation completes THEN artifact, compatibility,
threshold, CLI, workflow, full test, race, vet, and build checks SHALL pass.

## Out of scope

- Live provider latency or throughput claims.
- A universal latency, RPS, CPU, or memory threshold.
- Distributed load generation across hosts.
- Long-running soak, chaos, or capacity tests in the default test gate.
- Persisting benchmark history in a hosted service.
- Formal hypothesis testing or claims of statistical significance.
- Comparing artifacts produced on different runner classes.

## Success criteria

- One command runs the deterministic load matrix.
- One command emits machine-readable JSON.
- One command reports `ns/op`, `B/op`, and `allocs/op`.
- Optional commands generate CPU and heap profiles.
- The complete project gate passes.
- Repeated measurements can be saved and compared with one command each.
- Pull requests receive a deterministic pass/fail performance check.

## Traceability

| Requirement | Task | Status |
| --- | --- | --- |
| PERF-01, PERF-03 | T1, T3 | Verified |
| PERF-02, PERF-05 | T3 | Verified |
| PERF-04 | T3, T4 | Verified |
| PERF-06, PERF-07 | T2, T3 | Verified |
| PERF-08, PERF-09 | T2, T4 | Verified |
| PERF-10 | T1-T5 | Verified |
| PERF-11, PERF-12 | T6 | Implementing |
| PERF-13 | T7 | Verified |
| PERF-14 | T8 | Pending |
| PERF-15 | T6-T9 | Implementing |
