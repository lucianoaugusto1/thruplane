# Performance baseline tasks

**Design:** `design.md`
**Status:** Complete

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5
```

## Tasks

### T1: Implement deterministic performance metrics — Complete

**What:** Add percentile, throughput, memory, and connection result models.
**Where:** `bench/performance/metrics.go`, `metrics_test.go`.
**Depends on:** None.
**Requirements:** PERF-01, PERF-03, PERF-05, PERF-08.
**Tests:** Unit. **Gate:** Quick plus package test.

### T2: Define the synthetic scenario catalog — Complete

**What:** Add modality, streaming, retry, fallback, `429`, slow-reader, and
cancellation scenario definitions and mock responses.
**Where:** `bench/performance/scenarios.go`, `scenarios_test.go`.
**Depends on:** T1.
**Requirements:** PERF-02, PERF-06, PERF-07, PERF-09.
**Tests:** Unit. **Gate:** Package test.

### T3: Implement the local load harness — Complete

**What:** Build gateway and direct phases with workers, runtime metrics,
TTFT, cancellation, and connection tracking.
**Where:** `bench/performance/harness.go`, `harness_test.go`.
**Depends on:** T2.
**Requirements:** PERF-01 through PERF-09.
**Tests:** End-to-end with `httptest`. **Gate:** Full.

### T4: Add the command and Go benchmarks — Complete

**What:** Add CLI output, JSON mode, CPU and heap profiles, and allocation
benchmarks for every scenario.
**Where:** `bench/performance/main.go`, `main_test.go`, `benchmark_test.go`.
**Depends on:** T3.
**Requirements:** PERF-04, PERF-08, PERF-09.
**Tests:** CLI smoke and benchmark smoke. **Gate:** Full.

### T5: Publish usage and validation evidence — Complete

**What:** Document commands, interpretation, limitations, and validation;
update project testing strategy and state.
**Where:** `docs/performance.md`, `.specs/codebase/TESTING.md`, feature
validation, and project state.
**Depends on:** T4.
**Requirements:** PERF-08 through PERF-10.
**Tests:** Documentation review and complete project gate. **Gate:** Build.

## Granularity check

| Task | Component | Status |
| --- | --- | --- |
| T1 | Result calculation | Granular |
| T2 | Scenario definitions | Granular |
| T3 | Load engine | Granular |
| T4 | User-facing runners | Cohesive |
| T5 | Operator documentation | Cohesive |

## Dependency cross-check

| Task | Declared dependency | Diagram dependency | Status |
| --- | --- | --- | --- |
| T1 | None | None | Match |
| T2 | T1 | T1 | Match |
| T3 | T2 | T2 | Match |
| T4 | T3 | T3 | Match |
| T5 | T4 | T4 | Match |

## Test co-location check

| Task | Layer | Required | Planned | Status |
| --- | --- | --- | --- | --- |
| T1 | Metrics utility | Unit | Unit | Match |
| T2 | Scenario fixtures | Unit | Unit | Match |
| T3 | Gateway and HTTP | End-to-end | End-to-end | Match |
| T4 | CLI and benchmark | Build and integration | Both | Match |
| T5 | Documentation | Build | Complete gate | Match |
