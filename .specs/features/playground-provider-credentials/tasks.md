# Playground provider credentials tasks

**Design:** `design.md`
**Status:** In progress

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5 -> T6 -> T7
```

## Tasks

### T1: Lock the product and security contract — Complete

**What:** Record provider credentials as the selected feature, the LiteLLM-
inspired test flow, ephemeral handling, endpoint boundary, allowlist policy,
and complete implementation plan.
**Where:** This feature's `context.md`, `spec.md`, `design.md`, and `tasks.md`.
**Depends on:** None.
**Requirements:** PCR-01 through PCR-12.
**Tests:** Documentation review. **Gate:** None.

### T2: Add configuration contract — Complete

**What:** Add opt-in credential-testing configuration, normalize and validate
allowlisted URLs, enforce playground/API-key dependencies, and document the
YAML surface.
**Where:** `internal/config`, config tests, and `config.example.yaml`.
**Depends on:** T1.
**Requirements:** PCR-01, PCR-05, PCR-12.
**Tests:** Unit. **Gate:** Fast.

### T3: Implement request-scoped credential execution — Complete

**What:** Parse and validate the secret envelope, build a one-provider
configuration with no retries or breaker, and execute the existing adapters
without shared-state mutation.
**Where:** New `internal/httpapi` credential handler/service and focused tests.
**Depends on:** T2.
**Requirements:** PCR-02, PCR-03, PCR-05, PCR-06, PCR-08, PCR-11.
**Tests:** Unit and local upstream integration. **Gate:** Fast.

### T4: Wire protected HTTP and bounded telemetry

**What:** Register the route only when enabled, extend inbound authentication,
use a fixed metric route, omit dynamic route-selection metrics, and prove logs
and metrics do not reveal secrets.
**Where:** `internal/httpapi/server.go`, server tests, telemetry integration.
**Depends on:** T3.
**Requirements:** PCR-01, PCR-04, PCR-09, PCR-11.
**Tests:** HTTP integration. **Gate:** Full.

### T5: Build provider credential interface

**What:** Add configured/credential modes, provider-specific fields, model and
destination inputs, cost warning, test state, responsive styling, and semantic
labels without weakening existing accessibility.
**Where:** Playground HTML, CSS, and asset contract tests.
**Depends on:** T4.
**Requirements:** PCR-02, PCR-07, PCR-10.
**Tests:** Static UI contract. **Gate:** Fast.

### T6: Implement browser test-and-use flow

**What:** Build the ephemeral envelope, test a tiny completion, invalidate on
edits, send buffered/SSE/multimodal/tools requests, cancel safely, and export
only environment placeholders.
**Where:** Playground JavaScript and contract tests.
**Depends on:** T5.
**Requirements:** PCR-03, PCR-07, PCR-08, PCR-10, PCR-11.
**Tests:** Static contract, JavaScript syntax, and browser smoke. **Gate:** Full.

### T7: Document, visually verify, and validate

**What:** Update operator/user docs and project state, run desktop/mobile
browser checks, record validation evidence, and execute the complete quality
gate.
**Where:** README, playground docs, project records, feature validation.
**Depends on:** T6.
**Requirements:** PCR-01 through PCR-12.
**Tests:** Browser, test, race, vet, build, and diff checks. **Gate:** Full.

## Test co-location

| Task | Layer | Required | Planned |
| --- | --- | --- | --- |
| T2 | Configuration | Unit | Unit |
| T3 | Credential execution | Unit + integration | Both |
| T4 | HTTP + telemetry | Integration | Integration |
| T5 | Static interface | Contract | Contract |
| T6 | Browser client | Syntax + browser | Both |
| T7 | Product | Full validation | Full validation |
