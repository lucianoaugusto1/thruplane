# Performance testing

NexoRoute includes a local performance and load harness in
`bench/performance`. It runs the real HTTP API, gateway routing, provider
client, adapter, retry, fallback, and streaming paths against synthetic local
upstreams.

The default harness doesn't use provider credentials or make public network
calls. It measures loopback performance on the current machine. Don't treat a
single run as a universal latency or capacity claim.

## Measurements

The load runner reports:

- Direct and gateway latency mean, p50, p95, p99, and maximum.
- Gateway-added percentile deltas against the direct mock upstream.
- Direct and gateway time to first response byte for SSE scenarios.
- Requests per second and request outcomes.
- Total allocated bytes, allocation count, heap state, and GC cycles.
- Client-to-gateway and gateway-to-upstream connection reuse.
- Upstream calls, error responses, and observed cancellations.
- Go version, operating system, architecture, logical CPUs, payload size,
  request count, concurrency, and warmup count.

Go benchmarks add `ns/op`, `B/op`, `allocs/op`, transferred bytes per second,
average upstream calls per operation, and average TTFT for streaming cases.

## Scenarios

| Scenario | Path | Purpose |
| --- | --- | --- |
| `text` | OpenAI-compatible | Small buffered passthrough request. |
| `tools` | Anthropic native | Function definition translation and normalized response. |
| `image` | Anthropic native | Inline PNG payload with 48 KiB of source bytes. |
| `pdf` | Anthropic native | Inline PDF payload with 48 KiB of source bytes. |
| `audio` | Gemini native | Inline WAV payload with 48 KiB of source bytes. |
| `sse` | Anthropic native | Stream transformation and first-token latency. |
| `retry` | OpenAI-compatible | One `503`, backoff, and same-target success. |
| `fallback` | OpenAI-compatible | Primary `503` and secondary success. |
| `rate-limit-429` | OpenAI-compatible | Permanent quota `429` and fallback without same-target retry. |
| `slow-client` | Anthropic native | SSE response consumed with deliberate read delays. |
| `cancellation` | OpenAI-compatible | Client deadline before upstream completion and no retry. |

Image, PDF, and audio payloads are generated in memory. The repository doesn't
store synthetic media fixtures.

## Run the load harness

List the scenarios:

```sh
go run ./bench/performance -list
```

Run a quick text baseline:

```sh
go run ./bench/performance \
  -scenario text \
  -requests 1000 \
  -concurrency 16 \
  -warmup 100
```

Run the complete matrix with the default local profile:

```sh
go run ./bench/performance
```

Emit JSON for storage or comparison:

```sh
go run ./bench/performance \
  -scenario all \
  -requests 1000 \
  -concurrency 16 \
  -warmup 100 \
  -format json > performance.json
```

The runner uses a fixed number of requests. Requests per second equals the
measured request count divided by wall-clock time for that phase.

## Measure CPU and heap

Generate Go CPU and heap profiles while running the matrix:

```sh
go run ./bench/performance \
  -scenario all \
  -requests 5000 \
  -concurrency 32 \
  -warmup 200 \
  -cpuprofile /tmp/nexoroute-cpu.pprof \
  -memprofile /tmp/nexoroute-heap.pprof
```

Inspect the profiles:

```sh
go tool pprof -top /tmp/nexoroute-cpu.pprof
go tool pprof -top /tmp/nexoroute-heap.pprof
```

Use a long enough run for a meaningful CPU sample. Very short smoke runs can
produce a valid profile with too few samples for analysis.

## Measure allocations

Run all direct and gateway benchmarks:

```sh
go test \
  -run '^$' \
  -bench . \
  -benchmem \
  -benchtime 3s \
  -count 5 \
  ./bench/performance
```

Run only the text paths:

```sh
go test \
  -run '^$' \
  -bench '^Benchmark(Direct|Gateway)/text$' \
  -benchmem \
  ./bench/performance
```

The benchmark measures the end-to-end local client and server path. Compare
`BenchmarkGateway` with `BenchmarkDirect`; don't interpret the gateway number
as gateway-only allocation.

## Interpret results

The added latency fields subtract each direct percentile from the matching
gateway percentile. Independent load phases and scheduler noise can produce a
negative delta or percentile deltas that aren't monotonic. Increase the
request count and repeat the run before treating a small difference as real.

Connection reuse on the client side comes from `httptrace`. Upstream reuse is
inferred from measured upstream requests minus newly accepted connections.
Warmup occurs before measurement, so a healthy keep-alive path can report 100%
reuse during the measured phase.

Memory deltas include the load client, gateway server, provider client, and
local mock because they run in one process. Use `-benchmem` for stable
per-operation comparisons and heap profiles to locate retained objects.

The retry, fallback, and `429` scenarios intentionally make more than one
upstream call. The `upstream-calls/op` benchmark metric makes that cost
visible. Cancellation is an expected failure outcome, not an unexpected load
error.

## Build a regression baseline

Use the following process before setting a threshold:

1. Pin the Go version and hardware class.
2. Keep the host idle and use the same power mode.
3. Fix request count, concurrency, warmup, and `GOMAXPROCS`.
4. Run at least five benchmark counts.
5. Store JSON and benchmark output with the commit SHA and environment data.
6. Set thresholds from observed variance, not from a marketing target.

Local mocks isolate gateway overhead. Run separate opt-in provider tests when
you need real network, model, account, or regional performance evidence.
