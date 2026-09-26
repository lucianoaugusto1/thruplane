# Playground provider credentials specification

**Status:** Approved
**Date:** September 26, 2026

## Problem

The developer playground can exercise only aliases whose credentials already
exist in gateway YAML. A prospective user cannot validate a provider key and
physical model from the UI before editing deployment configuration. That adds
friction to provider onboarding and makes the playground less useful as a
commercial product surface.

## Goals

- Let a user construct a provider credential setup in the playground.
- Verify that setup with a small real request before enabling it for chat.
- Support every initial provider adapter through one normalized UI and API.
- Keep the feature isolated from the configured production routing path.
- Prevent persistence, credential disclosure, open-proxy use, and unbounded
  metrics cardinality.

## Requirements

### PCR-01: Explicit and safe enablement

WHEN `server.playground.credential_testing.enabled` is false or omitted THEN
the credential-testing API SHALL return `404` and its UI SHALL remain hidden.
WHEN enabled THEN the playground itself SHALL also be enabled and
`server.api_key` SHALL be non-empty.

### PCR-02: Provider coverage

WHEN constructing credentials THEN the UI and endpoint SHALL accept `openai`,
`anthropic`, `gemini`, `vertex`, `bedrock`, `azure-openai`, `ollama`,
`openai-compatible`, `nexoroute-inference`, and `xai`. The form SHALL expose
only the fields relevant to the selected adapter.

### PCR-03: Ephemeral secret boundary

WHEN credentials are entered or sent THEN they SHALL remain in page memory and
the active request only. They SHALL NOT be written to browser storage, cookies,
server configuration, logs, response bodies, diagnostics, metrics, or copied
commands.

### PCR-04: Authenticated endpoint

WHEN the browser calls the credential-testing API THEN the existing gateway
bearer key SHALL be required using the same constant-time comparison and error
contract as `/v1` routes.

### PCR-05: Upstream destination control

WHEN no base URL is supplied THEN the adapter's built-in official default
SHALL be used. WHEN a base URL is supplied THEN its normalized value SHALL
exactly match `allowed_base_urls`; otherwise the request SHALL fail before any
network call. URLs with credentials, query strings, or fragments SHALL be
rejected.

### PCR-06: Request-scoped execution

WHEN a credential request is accepted THEN NexoRoute SHALL construct a
request-scoped provider client and one-target gateway without mutating shared
providers, aliases, limiters, breakers, or routes. It SHALL use no automatic
retry or fallback so a test cannot silently create duplicate billable calls.

### PCR-07: Connection verification

WHEN **Test connection** is selected THEN the browser SHALL send a small,
buffered completion to the selected physical model. Success SHALL mark the
credential setup ready for the current form state; editing any provider field
SHALL invalidate that state. The UI SHALL warn that testing can incur provider
charges.

### PCR-08: Chat and multimodal parity

WHEN a tested credential setup is active THEN text, function tools, images,
PDFs, audio, buffered responses, SSE streaming, and cancellation SHALL use the
same normalized request and response behavior as configured gateway aliases.

### PCR-09: Secret-safe observability

WHEN credential testing is used THEN access logs SHALL contain only the fixed
route, method, status, request ID, and duration. HTTP metrics SHALL use a fixed
route label. Dynamic provider/model route-selection metrics SHALL not be
recorded for this endpoint.

### PCR-10: Safe portability

WHEN copying a request in credential mode THEN the generated command SHALL use
environment-variable placeholders for every secret and SHALL never contain
the values currently present in the form.

### PCR-11: Validation and errors

WHEN input is malformed, too large, unsupported, unauthenticated, unverified,
or points outside the allowlist THEN the endpoint SHALL return an OpenAI-shaped
error with a stable code and without echoing secrets.

### PCR-12: Verification

WHEN implementation completes THEN configuration, authentication, destination
allowlist, upstream credential/model forwarding, secret-redaction, buffered,
SSE, cancellation, UI contract, JavaScript syntax, browser smoke, tests, race,
vet, build, and diff checks SHALL pass without live provider credentials.

## Out of scope

- Persisting provider credentials or implementing a credential vault.
- Virtual keys, users, teams, budgets, quotas, and RBAC.
- Automatic OAuth, workload identity, AWS credential chains, or token refresh.
- Proving a credential without making an upstream model request.
- Using this endpoint as a production hot path or general-purpose HTTP proxy.

## Success criteria

- A fresh installation exposes no credential endpoint by default.
- An operator can opt in with an inbound key and destination allowlist.
- A user can test and chat with an OpenAI-compatible synthetic upstream.
- The same UI represents all initial provider-specific credential fields.
- No test, log, metric, inspector, or copied command contains a submitted key.

## Traceability

| Requirement | Task |
| --- | --- |
| PCR-01, PCR-04, PCR-05 | T2, T4 |
| PCR-02, PCR-03, PCR-06, PCR-11 | T3, T4 |
| PCR-07, PCR-08, PCR-10 | T5, T6 |
| PCR-09 | T4 |
| PCR-12 | T2-T7 |
