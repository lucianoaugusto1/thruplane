# MVP gateway tasks

**Design:** `.specs/features/mvp-gateway/design.md`
**Status:** In Progress

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5 -> T6 -> T7
```

The tasks are sequential because later packages import the earlier contracts.
Tests remain co-located with the component they verify.

## Task breakdown

### T1: Create strict configuration component ✅

**What:** Add the Go module plus YAML loading, defaults, environment expansion,
and validation.
**Where:** `go.mod`, `internal/config/config.go`,
`internal/config/config_test.go`
**Depends on:** None
**Reuses:** Go standard library and `go.yaml.in/yaml/v3`
**Requirement:** GW-04, GW-08
**Tools:** Shell and filesystem; no additional skill.
**Tests:** Unit
**Gate:** Quick

**Done when:**

- [x] Valid configuration loads with defaults and expanded environment values.
- [x] Unknown fields, bad URLs, missing targets, and missing providers fail.
- [x] `go test ./internal/...` passes with at least 5 tests.

**Verify:** `go test ./internal/config -v`

### T2: Create OpenAI-compatible provider client ✅

**What:** Add reusable upstream clients with credential isolation and Ollama
request transformation.
**Where:** `internal/provider/client.go`, `internal/provider/client_test.go`
**Depends on:** T1
**Reuses:** Configuration contracts and `http.Transport.Clone`.
**Requirement:** GW-01, GW-03, GW-08
**Tools:** Shell and filesystem; no additional skill.
**Tests:** Integration with `httptest`
**Gate:** Full

**Done when:**

- [x] Provider requests use the configured URL, key, and model.
- [x] Ollama translates token fields without dropping unrelated JSON fields.
- [x] Provider credentials never come from the inbound request.
- [x] `go test ./...` passes with at least 8 tests total.

**Verify:** `go test ./internal/provider -v`

### T3: Create chat routing handler ✅

**What:** Add request validation, alias resolution, retries, ordered fallback,
response relay, and SSE flush behavior.
**Where:** `internal/gateway/gateway.go`, `internal/gateway/gateway_test.go`
**Depends on:** T2
**Reuses:** Provider clients and configuration models.
**Requirement:** GW-01, GW-02, GW-03, GW-08
**Tools:** Shell and filesystem; no additional skill.
**Tests:** Integration with `httptest`
**Gate:** Full

**Done when:**

- [x] Unknown fields survive model rewriting.
- [x] Retryable failures use retries and then the next target.
- [x] Invalid requests do not contact upstreams.
- [x] SSE reaches the client before the upstream closes.
- [x] `go test ./...` passes with at least 14 tests total.

**Verify:** `go test ./internal/gateway -v`

### T4: Create model discovery handlers ✅

**What:** Add deterministic list and retrieve behavior for configured aliases.
**Where:** `internal/gateway/models.go`, `internal/gateway/models_test.go`
**Depends on:** T3
**Reuses:** Gateway configuration snapshot.
**Requirement:** GW-07
**Tools:** Shell and filesystem; no additional skill.
**Tests:** Unit
**Gate:** Quick

**Done when:**

- [x] List output is sorted and OpenAI-shaped.
- [x] Retrieve returns one model or an OpenAI-shaped 404.
- [x] `go test ./internal/...` passes with at least 17 tests total.

**Verify:** `go test ./internal/gateway -run Model -v`

### T5: Create the public HTTP server

**What:** Wire routes and add health, authentication, request ID, recovery, and
structured access logging.
**Where:** `internal/httpapi/server.go`, `internal/httpapi/server_test.go`
**Depends on:** T4
**Reuses:** Gateway handlers and `log/slog`.
**Requirement:** GW-05, GW-06
**Tools:** Shell and filesystem; no additional skill.
**Tests:** End-to-end with `httptest`
**Gate:** Full

**Done when:**

- [ ] `/healthz` is public and `/v1/*` enforces the configured bearer key.
- [ ] Every response has a request ID and every completed request is logged.
- [ ] Panics become a safe OpenAI-shaped 500 response.
- [ ] `go test ./...` passes with at least 22 tests total.

**Verify:** `go test ./internal/httpapi -v`

### T6: Create the executable

**What:** Add CLI flags, structured logging, dependency construction, HTTP
timeouts, and graceful shutdown.
**Where:** `cmd/gateway/main.go`
**Depends on:** T5
**Reuses:** All internal packages.
**Requirement:** GW-04, GW-06
**Tools:** Shell and filesystem; no additional skill.
**Tests:** Build
**Gate:** Build

**Done when:**

- [ ] `-config` selects the YAML file.
- [ ] SIGINT and SIGTERM trigger bounded graceful shutdown.
- [ ] `go test ./... && go vet ./... && go build ./cmd/gateway` passes.

**Verify:** `go build ./cmd/gateway`

### T7: Package and document the MVP

**What:** Add an example configuration, Dockerfile, ignore rules, and README
that matches the implemented behavior.
**Where:** `config.example.yaml`, `Dockerfile`, `.dockerignore`, `.gitignore`,
`README.md`
**Depends on:** T6
**Reuses:** Verified CLI and public API.
**Requirement:** GW-04
**Tools:** Shell and filesystem; `docs-writer` skill for the README.
**Tests:** Build and configuration parse check
**Gate:** Build

**Done when:**

- [ ] A user can run OpenAI and Ollama examples by following the README.
- [ ] The example configuration passes startup validation.
- [ ] All documented paths, flags, and fields match the code.
- [ ] `go test ./... && go vet ./... && go build ./cmd/gateway` passes.

**Verify:** Run the build gate and start the binary with the example config.

## Granularity check

| Task | Scope | Status |
| --- | --- | --- |
| T1 | One configuration component | ✅ Granular |
| T2 | One provider client component | ✅ Granular |
| T3 | One chat handler component | ✅ Granular |
| T4 | One model resource component | ✅ Granular |
| T5 | One public server component | ✅ Granular |
| T6 | One executable | ✅ Granular |
| T7 | One packaging and onboarding deliverable | ✅ Cohesive |

## Diagram-definition cross-check

| Task | Depends on | Diagram shows | Status |
| --- | --- | --- | --- |
| T1 | None | Start | ✅ Match |
| T2 | T1 | T1 -> T2 | ✅ Match |
| T3 | T2 | T2 -> T3 | ✅ Match |
| T4 | T3 | T3 -> T4 | ✅ Match |
| T5 | T4 | T4 -> T5 | ✅ Match |
| T6 | T5 | T5 -> T6 | ✅ Match |
| T7 | T6 | T6 -> T7 | ✅ Match |

## Test co-location validation

| Task | Layer | Matrix requires | Task says | Status |
| --- | --- | --- | --- | --- |
| T1 | Configuration | Unit | Unit | ✅ OK |
| T2 | Transport | Integration | Integration | ✅ OK |
| T3 | Routing | Integration | Integration | ✅ OK |
| T4 | Handler | Unit | Unit | ✅ OK |
| T5 | Public API | End-to-end | End-to-end | ✅ OK |
| T6 | CLI | Build | Build | ✅ OK |
| T7 | Packaging | Build/manual parse | Build/parse | ✅ OK |
