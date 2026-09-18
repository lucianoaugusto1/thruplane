# MVP gateway specification

## Problem statement

Applications that call multiple LLM providers must duplicate credentials,
model names, base URLs, retries, and streaming logic. The gateway provides one
OpenAI-compatible endpoint and moves those concerns into a small Go service.

## Goals

- [ ] Route configured model aliases to one or more upstream targets.
- [ ] Preserve buffered and streaming OpenAI-compatible chat behavior.
- [ ] Fail with stable, safe, OpenAI-shaped errors.
- [ ] Run as a stateless binary with a human-readable configuration file.

## Out of scope

| Feature | Reason |
| --- | --- |
| Usage database and cost accounting | Requires persistent state and pricing data |
| Tenant rate limits | Requires a policy and identity model |
| Full LiteLLM endpoint parity | Too broad for a focused vertical slice |
| Provider-native SDK adapters | OpenAI-compatible HTTP covers the initial providers |

---

## User stories

### P1: Route chat completions ⭐ MVP

**User story:** As an application developer, I want to use an OpenAI-compatible
chat endpoint so that my client does not depend on a specific provider.

**Why P1:** This is the gateway's primary vertical slice.

**Acceptance criteria:**

1. WHEN a valid request names a configured alias THEN the system SHALL replace
   only the upstream `model` value and preserve all other JSON fields.
2. WHEN the selected target succeeds THEN the system SHALL forward its status,
   content type, body, and relevant rate-limit headers.
3. WHEN `stream` is true THEN the system SHALL relay SSE data incrementally
   without buffering the entire response.
4. WHEN a model alias is unknown or the body is invalid THEN the system SHALL
   return an OpenAI-shaped 4xx error without calling an upstream.

**Independent test:** Send requests to a local fake provider and assert the
rewritten request plus buffered and streamed responses.

### P1: Fallback and resilience ⭐ MVP

**User story:** As an operator, I want ordered targets and bounded retries so
that transient provider failures do not immediately fail the client request.

**Why P1:** Centralized resilience is a core reason to deploy a gateway.

**Acceptance criteria:**

1. WHEN an upstream returns a retryable status or transport failure THEN the
   system SHALL retry it no more than the configured count.
2. WHEN retries are exhausted THEN the system SHALL try the next configured
   target in order.
3. WHEN every target fails THEN the system SHALL return a safe 502 or the last
   meaningful upstream error without exposing credentials.
4. WHEN the client cancels THEN the system SHALL stop upstream work through the
   request context.

**Independent test:** Configure two local fake providers, fail the first, and
assert that the second receives the request.

### P1: Configure and operate the gateway ⭐ MVP

**User story:** As an operator, I want declarative configuration and basic
operational endpoints so that I can deploy and inspect the service safely.

**Why P1:** The gateway cannot be operated reliably without validation and
health signals.

**Acceptance criteria:**

1. WHEN configuration contains environment placeholders THEN the system SHALL
   expand them before validation.
2. WHEN configuration is missing a referenced provider or contains invalid
   values THEN startup SHALL fail with an actionable error.
3. WHEN inbound authentication is configured THEN `/v1/*` requests SHALL require
   the matching bearer token while `/healthz` remains accessible.
4. WHEN a request completes THEN the system SHALL emit a structured log with a
   request ID, method, path, status, and duration, but no secrets or prompt body.

**Independent test:** Load temporary configurations and exercise middleware with
an in-memory HTTP server.

### P2: Discover model aliases

**User story:** As an application developer, I want to list available aliases so
that standard clients can discover gateway models.

**Why P2:** Discovery improves compatibility but is not required to forward a
known model.

**Acceptance criteria:**

1. WHEN a client calls `GET /v1/models` THEN the system SHALL return all
   configured aliases in deterministic order.
2. WHEN a client calls `GET /v1/models/{model}` for a configured alias THEN the
   system SHALL return the corresponding model object.
3. WHEN authentication is enabled THEN both endpoints SHALL enforce it.

**Independent test:** Configure aliases out of order and assert a sorted,
OpenAI-compatible list response.

## Edge cases

- WHEN a request body exceeds the configured limit THEN the system SHALL return
  HTTP 413.
- WHEN no upstream target remains THEN the system SHALL return a stable gateway
  error with a request ID.
- WHEN an upstream returns malformed or non-JSON error content THEN the system
  SHALL not synthesize provider secrets into the response.
- WHEN a streaming client disconnects THEN the upstream request SHALL be
  canceled.
- WHEN YAML contains unknown fields THEN startup SHALL reject it.

## Requirement traceability

| Requirement ID | Story | Phase | Status |
| --- | --- | --- | --- |
| GW-01 | Route chat completions | Design | In Design |
| GW-02 | Stream chat completions | Design | In Design |
| GW-03 | Fallback and resilience | Design | In Design |
| GW-04 | Configure and operate | Design | In Design |
| GW-05 | Inbound authentication | Design | In Design |
| GW-06 | Operational logging and health | Design | In Design |
| GW-07 | Discover model aliases | Design | In Design |
| GW-08 | Stable validation and error behavior | Design | In Design |

**Coverage:** 8 total, 0 mapped to tasks, 8 awaiting task mapping.

## Success criteria

- [ ] All build-gate commands pass without skipped tests.
- [ ] A standard OpenAI client can target the gateway by changing its base URL.
- [ ] A fake upstream proves that unknown JSON fields survive proxying.
- [ ] Streaming data reaches a test client before the upstream closes.
