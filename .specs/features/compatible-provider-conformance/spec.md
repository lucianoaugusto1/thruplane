# OpenAI-compatible provider conformance specification

**Status:** Complete
**Date:** September 25, 2026

## Problem

NexoRoute has direct deterministic fixtures for native adapters, but the
OpenAI-compatible family is covered only by focused integration tests. Beta
claims need one fixture-driven contract that proves every compatible provider
variant uses the correct endpoint and authentication while preserving request
fields and upstream responses.

## Requirements

### CPC-01: Provider variants

WHEN the compatible conformance suite runs THEN it SHALL cover `openai`,
`azure-openai`, `ollama`, `openai-compatible`, `nexoroute-inference`, and
`xai`; the existing configuration suite SHALL continue to verify that `grok`
normalizes to `xai`.

### CPC-02: Transparent requests

WHEN a compatible request contains text, media, tools, structured output, or
unknown fields THEN the adapter SHALL preserve them while replacing the public
model alias. Ollama SHALL additionally translate `max_completion_tokens` to
`max_tokens`.

### CPC-03: Transparent responses

WHEN an upstream returns a buffered response, SSE stream, or non-success body
THEN the adapter SHALL preserve its status, relevant headers, and body without
normalization.

### CPC-04: Safety

WHEN the request isn't a JSON object or its context is canceled THEN the
provider client SHALL fail without treating the compatible protocol as a
native translation path.

### CPC-05: Evidence

WHEN the feature is complete THEN deterministic tests, the response matrix,
and the project build gate SHALL pass without credentials or public network
access.

## Out of scope

- Claiming that every provider accepts every forwarded OpenAI field.
- Live provider calls or credential provisioning.
- Normalizing compatible response bodies.
- Adding new public API operations.

## Traceability

| Requirement | Evidence | Status |
| --- | --- | --- |
| CPC-01 | Provider fixture variants and configuration alias test | Verified |
| CPC-02 | Buffered opaque-request fixture | Verified |
| CPC-03 | Buffered, SSE, and upstream-error fixtures | Verified |
| CPC-04 | Invalid-object fixture and cancellation integration test | Verified |
| CPC-05 | Validation report and project gate | Verified |
