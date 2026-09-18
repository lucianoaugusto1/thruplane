# Testing strategy

## Test coverage matrix

| Code layer | Required test type | Parallel-safe |
| --- | --- | --- |
| Configuration parsing and validation | Unit | Yes |
| Routing and upstream transport | Integration with `httptest` | Yes |
| Public HTTP endpoints and middleware | End-to-end with `httptest` | Yes |
| CLI bootstrap | Build | Yes |
| Static packaging and examples | Build or manual parse check | Yes |

## Gate check commands

| Gate | Command |
| --- | --- |
| Quick | `go test ./internal/...` |
| Full | `go test ./...` |
| Build | `go test ./... && go vet ./... && go build ./cmd/nexoroute` |

## Conventions

- Write tests before implementation for executable behavior.
- Use local `httptest.Server` instances instead of live provider calls.
- Assert status, headers, request transformation, and response bodies.
- Do not use skipped tests in the MVP.
- Run tests with `-race` as an additional final check when the environment
  allows it.
