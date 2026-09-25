# Developer playground tasks

**Design:** `design.md`
**Status:** Complete

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5
```

## Tasks

### T1: Add configuration and embedded asset handler — Complete

**What:** Add opt-in configuration, exact static routes, browser security
headers, and tests for disabled, enabled, redirect, asset, and auth behavior.
**Where:** `internal/config`, `internal/httpapi/playground.go`, server wiring,
handler tests, and `config.example.yaml`.
**Depends on:** None.
**Requirements:** PLG-01, PLG-02, PLG-10, PLG-12.
**Tests:** Unit and HTTP integration. **Gate:** Full.

### T2: Expose safe route diagnostics — Complete

**What:** Track final provider/model, upstream attempts, and fallbacks in the
gateway execution result and expose them as response headers.
**Where:** `internal/gateway/execution.go`, `gateway.go`, and gateway tests.
**Depends on:** T1.
**Requirements:** PLG-08, PLG-12.
**Tests:** Integration. **Gate:** Full.

### T3: Implement the dependency-free browser client — Complete

**What:** Add model discovery, request building, SSE parsing, cancellation,
tools, media encoding, `curl` export, and route/usage measurement.
**Where:** `internal/httpapi/playground/app.js` and asset contract tests.
**Depends on:** T2.
**Requirements:** PLG-03 through PLG-10, PLG-12.
**Tests:** Static contract, JavaScript syntax, and browser integration.
**Gate:** Full.

### T4: Build and visually verify the interface — Complete

**What:** Add semantic HTML and responsive CSS, then inspect desktop and narrow
layouts with the embedded server.
**Where:** `internal/httpapi/playground/index.html`, `styles.css`, and asset
contract tests.
**Depends on:** T3.
**Requirements:** PLG-10 through PLG-12.
**Tests:** Static contract and browser smoke. **Gate:** Full.

**Evidence:** The embedded build completed a streamed request through a local
OpenAI-compatible upstream at 1440×900. The Route Inspector reported provider,
model, status, attempts, fallbacks, TTFT, total latency, and usage. A 390×844
check confirmed stacked panels with no horizontal overflow. Browser QA also
caught and verified the fix for an invisible file input overlapping Send.

### T5: Document and validate — Complete

**What:** Add enablement, privacy, usage, and limitation guidance; update
project state and record validation evidence.
**Where:** README, `docs/playground.md`, feature validation, roadmap, and state.
**Depends on:** T4.
**Requirements:** PLG-01 through PLG-12.
**Tests:** Documentation review and complete project gate. **Gate:** Build.

## Dependency cross-check

| Task | Declared dependency | Diagram dependency | Status |
| --- | --- | --- | --- |
| T1 | None | None | Match |
| T2 | T1 | T1 | Match |
| T3 | T2 | T2 | Match |
| T4 | T3 | T3 | Match |
| T5 | T4 | T4 | Match |

## Test co-location check

| Task | Layer | Required | Planned | Status |
| --- | --- | --- | --- | --- |
| T1 | Configuration and HTTP | Unit and integration | Both | Match |
| T2 | Gateway transport | Integration | Integration | Match |
| T3 | Browser client | Static and browser | Both | Match |
| T4 | Static interface | Static and browser | Both | Match |
| T5 | Documentation | Build | Complete gate | Match |
