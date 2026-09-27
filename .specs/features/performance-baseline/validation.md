# Performance baseline validation

**Status:** Passed
**Date:** September 25, 2026

## Coverage

| Requirement | Evidence | Result |
| --- | --- | --- |
| PERF-01 | Direct, gateway, and percentile-delta reports | Passed |
| PERF-02 | Anthropic SSE and slow-reader TTFT samples | Passed |
| PERF-03 | Fixed-load RPS and outcome reports | Passed |
| PERF-04 | `benchmem`, runtime memory, CPU, and heap profiles | Passed |
| PERF-05 | Client `httptrace` and upstream connection counters | Passed |
| PERF-06 | Text, tools, 48 KiB image, PDF, and audio cases | Passed |
| PERF-07 | Retry, fallback, `429`, slow client, and cancellation | Passed |
| PERF-08 | JSON output and environment metadata | Passed |
| PERF-09 | Local synthetic upstreams with no credentials | Passed |
| PERF-10 | Unit, integration, benchmark, race, vet, and build gates | Passed |
| PERF-11 | Five-run schema-versioned artifact smoke | Passed |
| PERF-12 | Environment, system, scenario, and load compatibility tests | Passed |
| PERF-13 | Median aggregation and configurable directional thresholds | Passed |
| PERF-14 | Same-runner base/candidate pull-request workflow | Passed |
| PERF-15 | Unit, CLI, workflow syntax, race, vet, and build gates | Passed |

## Commands

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/thruplane
go test -run '^$' -bench . -benchtime=1x -benchmem ./bench/performance
go run ./bench/performance -scenario all -requests 20 -concurrency 4 -warmup 4
ruby -e 'require "yaml"; YAML.load_file(".github/workflows/performance.yml")'
go run ./bench/performance -scenario text -requests 3 -concurrency 1 \
  -warmup 1 -runs 5 -artifact baseline.json \
  -revision smoke-base -environment smoke-runner
go run ./bench/performance compare -baseline baseline.json \
  -candidate baseline.json -output comparison.json
```

The command and benchmark smoke tests completed without unexpected request
failures. CPU and heap profile files were generated as gzip-compressed Go
profile artifacts during validation. This machine's Go distribution didn't
include the optional `go tool pprof` viewer, so profile visualization wasn't
part of this validation.

The repeated-run smoke produced a valid artifact and a passing self-comparison.
Unit tests also force regressions in latency, RPS, allocations, connection
reuse, and unexpected failures, and verify that the command returns a failing
status after writing its JSON report.

## Claim boundary

This validation proves the harness and local gateway paths work. It doesn't
establish a release threshold or make a production performance claim. A
published production claim still requires repeated measurements on named
hardware with a pinned Go version and stored raw output.
