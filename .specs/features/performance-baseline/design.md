# Performance baseline design

**Specification:** `spec.md`
**Status:** Approved extension

## Architecture

```text
load workers -> gateway httptest server -> provider client -> mock upstream
       |                  |                                      |
       +-> httptrace      +-> normal routing/retry path           +-> counters

direct workers ---------------------------------------------> mock upstream
```

The direct and gateway phases use the same local mock response at the same
configured concurrency. The difference between their percentile values is the
local gateway-added estimate. Failure scenarios compare recovery through the
gateway with the healthy direct path.

## Components

### Metrics

`bench/performance/metrics.go` calculates duration distributions, throughput,
memory deltas, and connection reuse. Percentiles use a deterministic nearest-
rank calculation over sorted samples.

### Scenario catalog

`bench/performance/scenarios.go` defines synthetic request payloads, provider
types, mock responses, streaming behavior, expected errors, and routing
topology. Media bytes are generated in memory and never stored as fixtures.

### Harness

`bench/performance/harness.go` constructs the real provider clients, gateway,
and HTTP API around local upstream servers. Fixed-count workers record latency,
TTFT, status, cancellation, memory, and connection data without collecting
prompt or response contents.

### Command

`bench/performance/main.go` supports one scenario or the full matrix, human or
JSON output, fixed request and concurrency settings, warmup, and optional CPU
and heap profile files.

### Go benchmarks

`bench/performance/benchmark_test.go` runs direct and gateway sub-benchmarks
with `ReportAllocs`. Standard Go flags provide benchmark duration, CPU profile,
memory profile, count, and comparison-friendly text output.

### Versioned artifacts

`bench/performance/artifact.go` stores multiple raw load-matrix runs in a
schema-versioned JSON document. The artifact records revision, environment,
generation time, and per-run reports; aggregation remains a comparison concern
so later algorithms can re-evaluate the original evidence.

### Regression comparator

`bench/performance/compare.go` validates that two artifacts have equivalent
environments and load shapes, then compares scenario medians. Thresholds are
loaded from JSON. Latency and TTFT use both relative and absolute guards;
throughput, allocations, connection reuse, and failures use directional
limits appropriate to each metric.

### CI gate

The pull-request workflow checks out the base revision into a temporary Git
worktree. Base and candidate artifacts are generated sequentially on the same
runner, then compared with the candidate implementation. Both measurements
and the comparison report are uploaded even when the gate fails.

## Decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Upstreams | Local `httptest` servers | Deterministic and free from provider cost. |
| Load shape | Fixed request count | Reproducible sample count and fast smoke tests. |
| Percentiles | Nearest rank | Simple, deterministic, and dependency-free. |
| Media | Synthetic 48 KiB bytes | Exercises body parsing without repository bloat. |
| CPU | Optional Go CPU profile | Portable evidence without an external profiler. |
| Allocations | Go benchmark `-benchmem` | Standard and comparison-friendly. |
| Connections | `httptrace` plus server `ConnState` | Covers both sides without production hooks. |
| Run aggregation | Median of at least three runs | Resists a single scheduling outlier without claiming formal significance. |
| Latency gate | Relative and absolute limit | Prevents tiny baselines from creating false regressions. |
| CI baseline | Pull-request base commit | Keeps code, hardware, and runner image comparisons aligned. |

## Measurement limits

Local results include loopback networking, scheduler noise, mock response
work, and the selected hardware. They don't predict public provider latency or
production capacity. Run repeated counts on pinned hardware before setting a
regression threshold.
