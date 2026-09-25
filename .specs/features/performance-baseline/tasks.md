# Performance baseline tasks

**Design:** `design.md`
**Status:** Complete

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5 -> T6 -> T7 -> T8 -> T9
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

### T6: Add versioned repeated-run artifacts — Complete

**What:** Capture at least three raw runs with schema, revision, environment,
time, and compatibility metadata; read and validate saved artifacts.
**Where:** `bench/performance/artifact.go`, `artifact_test.go`.
**Depends on:** T5.
**Requirements:** PERF-11, PERF-12, PERF-15.
**Tests:** Unit. **Gate:** Package test.

### T7: Add the configurable regression comparator — Complete

**What:** Aggregate scenario medians, apply noise-resistant directional
thresholds, explain every failure, and support JSON threshold overrides.
**Where:** `bench/performance/compare.go`, `compare_test.go`,
`bench/performance/thresholds.json`, and failure-count coverage in
`harness_test.go`.
**Depends on:** T6.
**Requirements:** PERF-13, PERF-15.
**Tests:** Unit. **Gate:** Package test.

### T8: Expose CLI modes and add the CI gate — Complete

**What:** Add repeated runs, artifact output, the `compare` subcommand, and a
pull-request workflow that benchmarks base and candidate on one runner.
**Where:** `bench/performance/main.go`, `main_test.go`,
`.github/workflows/performance.yml`.
**Depends on:** T7.
**Requirements:** PERF-11 through PERF-15.
**Tests:** CLI integration and workflow review. **Gate:** Full.

### T9: Document and validate the regression workflow — Complete

**What:** Publish local and CI usage, limitations, acceptance evidence, and
project state.
**Where:** `docs/performance.md`, validation, testing strategy, roadmap, and
project state.
**Depends on:** T8.
**Requirements:** PERF-11 through PERF-15.
**Tests:** Documentation review and complete project gate. **Gate:** Build.

## Granularity check

| Task | Component | Status |
| --- | --- | --- |
| T1 | Result calculation | Granular |
| T2 | Scenario definitions | Granular |
| T3 | Load engine | Granular |
| T4 | User-facing runners | Cohesive |
| T5 | Operator documentation | Cohesive |
| T6 | Artifact model | Granular |
| T7 | Regression decision | Granular |
| T8 | CLI and CI integration | Cohesive |
| T9 | Operator documentation | Cohesive |

## Dependency cross-check

| Task | Declared dependency | Diagram dependency | Status |
| --- | --- | --- | --- |
| T1 | None | None | Match |
| T2 | T1 | T1 | Match |
| T3 | T2 | T2 | Match |
| T4 | T3 | T3 | Match |
| T5 | T4 | T4 | Match |
| T6 | T5 | T5 | Match |
| T7 | T6 | T6 | Match |
| T8 | T7 | T7 | Match |
| T9 | T8 | T8 | Match |

## Test co-location check

| Task | Layer | Required | Planned | Status |
| --- | --- | --- | --- | --- |
| T1 | Metrics utility | Unit | Unit | Match |
| T2 | Scenario fixtures | Unit | Unit | Match |
| T3 | Gateway and HTTP | End-to-end | End-to-end | Match |
| T4 | CLI and benchmark | Build and integration | Both | Match |
| T5 | Documentation | Build | Complete gate | Match |
| T6 | Artifact model | Unit | Unit | Match |
| T7 | Comparison policy | Unit | Unit | Match |
| T8 | CLI and workflow | Integration | Full | Match |
| T9 | Documentation | Build | Complete gate | Match |
