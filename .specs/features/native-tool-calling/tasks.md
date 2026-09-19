# Native tool calling tasks

**Design:** `design.md`
**Status:** Done

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5
```

### T1: Add the shared native tool contract ✅

**Where:** `internal/provider/native.go`, native provider tests
**Depends on:** None
**Requirements:** TC-01, TC-02, TC-03, TC-04, TC-06
**Tests:** Unit
**Gate:** Quick

### T2: Implement Anthropic tool translation ✅

**Where:** `internal/provider/anthropic.go`, native provider tests
**Depends on:** T1
**Requirements:** TC-01 through TC-06
**Tests:** Integration with local HTTP/SSE fixtures
**Gate:** Full

### T3: Implement Gemini and Vertex tool translation ✅

**Where:** `internal/provider/google.go`, native provider tests
**Depends on:** T1
**Requirements:** TC-01 through TC-06
**Tests:** Integration with local HTTP/SSE fixtures
**Gate:** Full

### T4: Implement Bedrock Converse tool translation ✅

**Where:** `internal/provider/bedrock.go`, Bedrock provider tests
**Depends on:** T1
**Requirements:** TC-01 through TC-04, TC-06
**Tests:** Integration with local HTTP fixtures
**Gate:** Full

### T5: Document and validate native tool portability ✅

**Where:** `docs/providers.md`, project state, feature validation
**Depends on:** T2, T3, T4
**Requirements:** TC-01 through TC-07
**Tests:** Full race, vet, and build gates
**Gate:** Build

## Pre-execution checks

| Task | Scope | Dependency matches plan | Tests colocated | Status |
| --- | --- | --- | --- | --- |
| T1 | Shared model | Yes | Yes | Done |
| T2 | One adapter | Yes | Yes | Done |
| T3 | One shared protocol family | Yes | Yes | Done |
| T4 | One adapter | Yes | Yes | Done |
| T5 | Documentation and validation | Yes | Yes | Done |
