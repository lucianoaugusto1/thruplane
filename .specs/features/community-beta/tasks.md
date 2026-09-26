# Community beta tasks

**Design:** `.specs/features/community-beta/design.md`  
**Status:** In progress

## Execution plan

All tasks run sequentially because each one extends the same candidate and the
final gate validates their integration.

```text
T1 -> T2 -> T3 -> T4 -> T5 -> T6 -> T7
```

## Task breakdown

### T1: Define the beta contract

**What:** Add the beta specification, design, and verified task plan.  
**Where:** `.specs/features/community-beta/`  
**Depends on:** None  
**Requirement:** BETA-01 through BETA-07  
**Tests:** Static documentation review  
**Gate:** Build

**Done when:**

- [x] Scope and exclusions are explicit.
- [x] Every requirement maps to a task.
- [x] Granularity, dependency, and test co-location checks pass.

**Verify:** Read all three files and run the current build gate.

### T2: Add the metrics configuration contract

**What:** Add the opt-in `server.metrics.enabled` configuration and tests.  
**Where:** `internal/config/`, `config.example.yaml`  
**Depends on:** T1  
**Requirement:** BETA-03  
**Tests:** Unit  
**Gate:** Quick

**Done when:**

- [x] The default is disabled.
- [x] Strict YAML accepts only the documented metrics field.
- [x] `go test ./internal/config` passes with no test deletion.

**Verify:** `go test ./internal/config`

### T3: Implement the Prometheus collector

**What:** Add a concurrency-safe standard-library metrics collector and unit
tests.  
**Where:** `internal/telemetry/metrics.go`,
`internal/telemetry/metrics_test.go`  
**Depends on:** T2  
**Requirement:** BETA-03, BETA-04  
**Tests:** Unit  
**Gate:** Quick

**Done when:**

- [x] All metric families encode valid Prometheus text.
- [x] Labels escape quotes, backslashes, and newlines.
- [x] The collector never accepts request or response bodies.
- [x] `go test ./internal/telemetry` passes with no skipped tests.

**Verify:** `go test ./internal/telemetry`

### T4: Expose and instrument `/metrics`

**What:** Register the opt-in endpoint and instrument bounded HTTP routes and
safe route headers.  
**Where:** `internal/httpapi/server.go`, `internal/httpapi/server_test.go`  
**Depends on:** T3  
**Requirement:** BETA-03, BETA-04  
**Tests:** End-to-end with `httptest`  
**Gate:** Full

**Done when:**

- [x] Disabled metrics return `404`.
- [x] Enabled metrics include HTTP, selected route, request attempts,
  fallbacks, readiness, and build series.
- [x] Raw unknown paths and sensitive test values do not appear.
- [x] `go test ./...` passes with no skipped tests.

**Verify:** `go test ./...`

### T5: Add CLI preflight and build identity

**What:** Add `-check-config` and `-version` modes with deterministic tests.  
**Where:** `cmd/nexoroute/main.go`, `cmd/nexoroute/main_test.go`  
**Depends on:** T4  
**Requirement:** BETA-01, BETA-02  
**Tests:** Unit and build  
**Gate:** Build

**Done when:**

- [x] Preflight accepts valid config and rejects invalid config without
  starting a listener.
- [x] Version mode works without a config file.
- [x] `go test ./... && go vet ./... && go build ./cmd/nexoroute` passes.

**Verify:** Run the build gate and invoke both CLI modes.

### T6: Add the standard CI gate

**What:** Add a pinned GitHub Actions workflow for formatting, race tests,
vet, and build.  
**Where:** `.github/workflows/ci.yml`  
**Depends on:** T5  
**Requirement:** BETA-05  
**Tests:** Static packaging check  
**Gate:** Build

**Done when:**

- [ ] Pull requests and default-branch pushes run the complete gate.
- [ ] Actions are commit-pinned and permissions are read-only.
- [ ] Local build gate passes.

**Verify:** Review the workflow and run the build gate locally.

### T7: Publish and qualify the beta

**What:** Add the beta operator runbook, update project status and public docs,
then run the complete beta gate.  
**Where:** `docs/beta.md`, `README.md`, `docs/next-steps.md`,
`.specs/project/ROADMAP.md`, `.specs/project/STATE.md`, and this task file  
**Depends on:** T6  
**Requirement:** BETA-06, BETA-07  
**Tests:** Full, race, and performance smoke  
**Gate:** Build plus performance smoke

**Done when:**

- [ ] The runbook covers install, preflight, start, probes, metrics, buffered
  and streamed smoke requests, rollback, and known limitations.
- [ ] Public docs call the artifact a beta and do not imply live provider
  certification.
- [ ] Build, race, and one-run performance gates pass.
- [ ] All requirements are marked verified.

**Verify:**

```sh
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/nexoroute
go test -run '^$' -bench . -benchtime=1x -benchmem ./bench/performance
```

## Pre-approval checks

### Task granularity

| Task | Scope | Status |
| --- | --- | --- |
| T1 | One planning artifact set | Pass |
| T2 | One configuration contract | Pass |
| T3 | One metrics collector | Pass |
| T4 | One endpoint and its middleware | Pass |
| T5 | One CLI surface | Pass |
| T6 | One CI workflow | Pass |
| T7 | One release qualification | Pass |

### Dependency cross-check

| Task | Depends on | Diagram shows | Status |
| --- | --- | --- | --- |
| T1 | None | Start | Pass |
| T2 | T1 | T1 -> T2 | Pass |
| T3 | T2 | T2 -> T3 | Pass |
| T4 | T3 | T3 -> T4 | Pass |
| T5 | T4 | T4 -> T5 | Pass |
| T6 | T5 | T5 -> T6 | Pass |
| T7 | T6 | T6 -> T7 | Pass |

### Test co-location validation

| Task | Layer | Matrix requires | Task says | Status |
| --- | --- | --- | --- | --- |
| T1 | Documentation | Build/manual | Static/build | Pass |
| T2 | Configuration | Unit | Unit | Pass |
| T3 | Internal collector | Unit | Unit | Pass |
| T4 | Public HTTP | End-to-end | End-to-end | Pass |
| T5 | CLI bootstrap | Build | Unit/build | Pass |
| T6 | Packaging | Build | Static/build | Pass |
| T7 | Integrated candidate | Full/performance | Full/race/performance | Pass |
