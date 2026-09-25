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

## Commands

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/nexoroute
go test -run '^$' -bench . -benchtime=1x -benchmem ./bench/performance
go run ./bench/performance -scenario all -requests 20 -concurrency 4 -warmup 4
```

The command and benchmark smoke tests completed without unexpected request
failures. CPU and heap profile files were generated as gzip-compressed Go
profile artifacts during validation. This machine's Go distribution didn't
include the optional `go tool pprof` viewer, so profile visualization wasn't
part of this validation.

## Claim boundary

This validation proves the harness and local gateway paths work. It doesn't
establish a release threshold or make a production performance claim. A
published baseline still requires repeated runs on named hardware with a
pinned Go version and stored raw output.
