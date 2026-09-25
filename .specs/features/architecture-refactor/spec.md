# Architecture refactor specification

**Status:** Verified
**Date:** September 25, 2026

## Problem

The gateway has clear package boundaries, but several files now combine
unrelated responsibilities. The chat handler owns request validation, route
planning, upstream execution, retries, fallback, and response relay. The native
provider contract and configuration package have similar concentration.

## Goals

- Separate chat request planning from upstream execution and HTTP rendering.
- Keep only the runtime settings that the gateway needs.
- Separate native request, tool, response, and wire types by responsibility.
- Separate configuration types, loading, defaults, and validation.
- Preserve every existing public and operator-visible behavior.

## Requirements

### AR-01: Behavior preservation

WHEN the refactored gateway receives an existing valid or invalid request THEN
it SHALL return the same status, headers, body, retry, fallback, streaming, and
rate-limit behavior as before the refactor.

### AR-02: Gateway responsibilities

WHEN chat completions are handled THEN request planning, upstream execution,
and HTTP transport SHALL live in focused components with explicit results.

### AR-03: Runtime configuration

WHEN a gateway is constructed THEN it SHALL retain only routing, model,
catalog-policy, body-limit, and provider-type settings. It SHALL not retain
provider credentials or unrelated server settings.

### AR-04: Provider contract organization

WHEN native adapters use the OpenAI-shaped contract THEN wire types, request
validation, tool validation, and response normalization SHALL live in separate
files within the provider package.

### AR-05: Configuration organization

WHEN configuration is loaded THEN schema types, loading, defaults,
normalization, and validation SHALL remain independently discoverable without
changing the YAML contract.

### AR-06: Verification

WHEN the refactor is complete THEN all existing tests, race checks, vet checks,
and the production build SHALL pass without skipped tests or new dependencies.

## Out of scope

- New routing behavior, providers, endpoints, or configuration fields.
- New public interfaces or package-level abstractions for single-use code.
- Changes to retry, fallback, rate-limit, or capability semantics.
- Performance claims or benchmark changes.

## Success criteria

- `ChatCompletions` only coordinates input, planning, execution, and rendering.
- The gateway does not retain provider API keys or cloud credentials.
- Native contract responsibilities are split into focused files.
- Configuration responsibilities are split into focused files.
- The complete project gate passes.

## Traceability

| Requirement | Task | Status |
| --- | --- | --- |
| AR-01 | T1-T4 | Verified |
| AR-02 | T2 | Verified |
| AR-03 | T1 | Verified |
| AR-04 | T3 | Verified |
| AR-05 | T4 | Verified |
| AR-06 | T1-T4 | Verified |
