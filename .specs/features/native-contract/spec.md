# Native Chat Completions contract specification

## Problem

Native adapters decode only selected fields. Other fields can be silently
discarded, especially when `catalog.unknown_models: allow` bypasses catalog
filtering. This violates the gateway's promise to preserve request semantics.

## Goals

- [x] Reject untranslated native request fields before upstream I/O.
- [x] Reject unsupported nested message, tool, and content-part fields.
- [x] Keep OpenAI-compatible adapters' passthrough behavior unchanged.
- [x] Document the precise native subset and intentional no-op defaults.

## Out of scope

- Implementing structured output, media output, prompt caching, or new tools.
- Changing the provider catalog or live provider availability.
- Adding cross-target fallback for a request rejected by a native adapter.

## Acceptance criteria

1. WHEN a native request contains an unknown top-level or message/tool field,
   THEN it SHALL return `400 unsupported_field` before contacting a provider.
2. WHEN a native content part has untranslatable nested fields, THEN it SHALL
   return `400 unsupported_content` before contacting a provider.
3. WHEN an assistant tool-call type is not `function`, THEN it SHALL be
   rejected rather than translated as a function.
4. WHEN the request uses known equivalent defaults (`n: 1`, text-only
   modalities or response format), THEN it SHALL remain accepted.
5. WHEN an unknown model is allowed, THEN the same native validation SHALL
   still run. Compatible passthrough SHALL retain extra JSON fields.
6. WHEN a native PDF file part includes `filename`, THEN it SHALL be rejected:
   the adapters cannot preserve its semantics consistently, and Bedrock uses a
   neutral document name for safety.

## Traceability

| ID | Requirement | Status |
| --- | --- | --- |
| NC-01 | Strict root, message, and tool validation | Done |
| NC-02 | Strict user and text content-part validation | Done |
| NC-03 | Unknown-model and passthrough regression coverage | Done |
| NC-04 | Published field contract and migration note | Done |
