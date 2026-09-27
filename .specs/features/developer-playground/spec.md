# Developer playground specification

**Status:** Complete
**Date:** September 25, 2026

## Problem

Beta users need a fast way to exercise Thruplane's public contract without
building a client first. A generic chat page would help with onboarding, but
it wouldn't expose the routing, streaming, modality, and failure behavior that
differentiates the gateway.

## Goals

- Provide an optional, self-contained browser playground in the Go binary.
- Exercise only the public `/v1` API used by real clients.
- Support text, SSE, tools, images, PDFs, and audio input.
- Show request timing and safe route diagnostics without persisting content.
- Preserve the gateway's authentication and configured-credential boundary.

## Requirements

### PLG-01: Explicit enablement

WHEN `server.playground.enabled` is false or omitted THEN the server SHALL
return `404` for playground paths. WHEN enabled THEN `/playground` SHALL
redirect to `/playground/` and serve the embedded interface.

### PLG-02: Authentication boundary

WHEN the playground calls `/v1` routes THEN existing gateway authentication
SHALL apply unchanged. Credentials configured in gateway YAML SHALL never be
sent to or embedded in the browser. The later, separately opt-in transient
credential flow is specified in `../playground-provider-credentials/spec.md`.

### PLG-03: Model discovery

WHEN a user supplies the optional gateway API key THEN the playground SHALL
load configured aliases from `/v1/models` and target capabilities from
`/v1/models/{model}`.

### PLG-04: Chat controls

WHEN composing a request THEN the user SHALL be able to set the system prompt,
model, temperature, output token limit, and streaming mode.

### PLG-05: Streaming and cancellation

WHEN streaming is enabled THEN the interface SHALL render SSE text deltas as
they arrive, measure first-byte time, and allow the user to abort the request.

### PLG-06: Tools

WHEN function-tool JSON is supplied THEN the playground SHALL validate that it
is an array and include it in the public Chat Completions request. Tool calls
in buffered or streamed responses SHALL be visible without executing them.

### PLG-07: Media input

WHEN supported image, PDF, WAV, or MP3 files are attached THEN the playground
SHALL encode them into the documented OpenAI-compatible content parts and keep
the files only in browser memory.

### PLG-08: Route inspector

WHEN an upstream response is available THEN the gateway SHALL expose safe
headers for provider name, provider model, upstream attempts, and fallback
count. The playground SHALL show those values with request ID, upstream
request ID, HTTP status, TTFT, total latency, and usage when present.

### PLG-09: Request portability

WHEN a request is ready THEN the user SHALL be able to copy a `curl` command
that uses `$THRUPLANE_API_KEY` rather than the entered key.

### PLG-10: Privacy and browser security

WHEN the interface is served THEN it SHALL set restrictive content security,
framing, MIME-sniffing, and referrer headers. It SHALL not use cookies,
analytics, browser storage, third-party assets, or server-side history.

### PLG-11: Responsive accessibility

WHEN used with keyboard, desktop, or narrow-screen layouts THEN controls SHALL
remain labeled, focus-visible, readable, and operable.

### PLG-12: Verification

WHEN implementation completes THEN configuration, route, authentication,
diagnostic-header, asset, JavaScript syntax, browser smoke, test, race, vet,
and build checks SHALL pass.

## Out of scope

- Automatic execution of model-requested tools.
- Persisted history, shared workspaces, accounts, or a database.
- Side-by-side evaluations, replay, shadow traffic, and AutoRouter controls.
- Provider API keys entered through the original configured-alias flow. The
  later opt-in transient flow is a separate feature and security boundary.
- Uploading media to a separate file service.
- A hosted public playground.

## Success criteria

- One YAML switch enables the interface.
- A user can discover a model and complete a streamed text request.
- Tools and supported media produce valid public API payloads.
- Route Inspector explains the completed upstream path without secrets.
- Disabling the feature removes every playground route.

## Traceability

| Requirement | Task | Status |
| --- | --- | --- |
| PLG-01, PLG-02, PLG-10 | T1 | Verified |
| PLG-08 | T2 | Verified |
| PLG-03 through PLG-09 | T3 | Verified |
| PLG-10, PLG-11 | T4 | Verified |
| PLG-12 | T1-T5 | Verified |
