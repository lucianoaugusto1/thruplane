# Testing strategy

## Test coverage matrix

| Code layer | Required test type | Parallel-safe |
| --- | --- | --- |
| Configuration parsing and validation | Unit | Yes |
| Routing and upstream transport | Integration with `httptest` | Yes |
| Public HTTP endpoints and middleware | End-to-end with `httptest` | Yes |
| Local performance harness | End-to-end with `httptest` | No |
| CLI bootstrap | Build | Yes |
| Static packaging and examples | Build or manual parse check | Yes |

## Gate check commands

| Gate | Command |
| --- | --- |
| Quick | `go test ./internal/...` |
| Full | `go test ./...` |
| Build | `go test ./... && go vet ./... && go build ./cmd/nexoroute` |
| Performance smoke | `go test -run '^$' -bench . -benchtime=1x -benchmem ./bench/performance` |
| Performance regression | `go run ./bench/performance compare -baseline baseline.json -candidate candidate.json` |

## Conventions

- Write tests before implementation for executable behavior.
- Use local `httptest.Server` instances instead of live provider calls.
- Assert status, headers, request transformation, and response bodies.
- Do not use skipped tests in the MVP.
- Run tests with `-race` as an additional final check when the environment
  allows it.
- Keep performance tests local and deterministic by default. Record live
  provider performance separately with approved credentials and budgets.
- Compare direct and gateway paths on the same Go version and hardware class.
- Use at least three runs for a comparison and five runs in the default CI
  gate. Never compare different load shapes or runner identifiers.
