# Developer playground design

**Specification:** `spec.md`
**Status:** Approved

## Architecture

```text
browser /playground/
    |
    +-- GET /v1/models and /v1/models/{alias}
    |
    +-- POST /v1/chat/completions
             |
             +-- existing gateway planning, limits, retry, and fallback
```

The playground has no privileged internal API. Static assets are embedded in
the Go binary and served only when explicitly enabled. All model and chat calls
go through existing authentication and public handlers.

## Components

### Configuration

`config.PlaygroundConfig` is nested under `server.playground`. The zero value
keeps the feature disabled, so existing deployments do not expose a new route.

### Static handler

`internal/httpapi/playground.go` embeds and serves an allowlist of HTML,
JavaScript, and CSS files. It sets security headers before responding and does
not provide directory listings or arbitrary filesystem access.

### Route diagnostics

The gateway execution result records the final configured provider, provider
model, number of upstream calls, and number of skipped targets. Successful or
relayed upstream responses expose these as `X-Thruplane-*` headers. The values
contain route metadata only, never credentials or prompt content.

### Browser client

The dependency-free client keeps conversation state, attachments, and the
gateway API key in JavaScript memory. It builds public Chat Completions JSON,
parses buffered or SSE responses, and reads response headers for Route
Inspector. It never uses cookies, `localStorage`, or third-party resources.

### Interface

The responsive layout uses an operations-console visual language: a narrow
configuration rail, a central conversation workspace, and a route-inspection
rail. On small screens the rails become sequential panels. Semantic HTML,
visible labels, live regions, and focus styles support keyboard use.

## Security decisions

| Concern | Decision |
| --- | --- |
| Default exposure | Disabled unless YAML explicitly enables it. |
| Configured provider credentials | Remain server-side in provider clients. |
| Gateway key | Password input held only in page memory. |
| External content | No CDN, analytics, fonts, scripts, or images. |
| Browser policy | Restrictive CSP and anti-framing headers. |
| Uploaded media | Data URLs in the current in-memory conversation only. |
| Export | Uses an environment-variable placeholder, never the entered key. |

## Limits

The playground is a developer tool, not an administrative control plane. Its
timings are browser-observed and its usage values depend on the provider
response. Route headers describe the executed path but are not durable audit
records.
