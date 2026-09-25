# Performance baseline design

**Specification:** `spec.md`
**Status:** Approved

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

## Measurement limits

Local results include loopback networking, scheduler noise, mock response
work, and the selected hardware. They don't predict public provider latency or
production capacity. Run repeated counts on pinned hardware before setting a
regression threshold.
