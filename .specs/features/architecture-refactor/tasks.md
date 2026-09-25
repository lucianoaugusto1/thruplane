# Architecture refactor tasks

**Design:** `design.md`
**Status:** In progress

## Execution plan

```text
T1 -> T2 -> T3 -> T4
```

## Tasks

### T1: Add focused gateway runtime settings

**What:** Replace retained `config.Config` with an internal settings snapshot.
**Where:** `internal/gateway/settings.go`, gateway callers.
**Depends on:** None.
**Requirements:** AR-01, AR-03, AR-06.
**Tests:** Existing gateway integration tests. **Gate:** Full.

### T2: Separate chat planning, execution, and HTTP transport

**What:** Extract the chat planner, upstream executor, and transport helpers.
**Where:** `internal/gateway`.
**Depends on:** T1.
**Requirements:** AR-01, AR-02, AR-06.
**Tests:** Existing gateway and HTTP API integration tests. **Gate:** Full.

### T3: Split the native provider contract by responsibility

**What:** Separate native types, requests, tools, and response normalization.
**Where:** `internal/provider/native_*.go`.
**Depends on:** T2.
**Requirements:** AR-01, AR-04, AR-06.
**Tests:** Existing provider unit and conformance tests. **Gate:** Quick.

### T4: Split configuration by responsibility and validate the project

**What:** Separate schema types, loading, defaults, and validation, then record
the final verification result.
**Where:** `internal/config`, feature validation, and project state.
**Depends on:** T3.
**Requirements:** AR-01, AR-05, AR-06.
**Tests:** Existing configuration tests and complete project gate. **Gate:** Build.

## Task granularity check

| Task | Component | Status |
| --- | --- | --- |
| T1 | Gateway runtime settings | Granular |
| T2 | Chat completion orchestration | Cohesive |
| T3 | Native provider contract | Cohesive |
| T4 | Configuration organization | Cohesive |

## Dependency cross-check

| Task | Declared dependency | Diagram dependency | Status |
| --- | --- | --- | --- |
| T1 | None | None | Match |
| T2 | T1 | T1 | Match |
| T3 | T2 | T2 | Match |
| T4 | T3 | T3 | Match |

## Test co-location check

| Task | Layer | Required | Planned | Status |
| --- | --- | --- | --- | --- |
| T1 | Routing | Integration | Integration | Match |
| T2 | Routing and HTTP | Integration and end-to-end | Both | Match |
| T3 | Provider transport | Integration | Integration | Match |
| T4 | Configuration | Unit | Unit plus build gate | Match |
