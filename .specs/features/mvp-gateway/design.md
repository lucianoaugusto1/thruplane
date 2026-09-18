# MVP gateway design

**Spec:** `.specs/features/mvp-gateway/spec.md`
**Status:** Approved for implementation

## Architecture overview

The public server performs authentication, request ID assignment, and request
logging before dispatching to the gateway handlers. The chat handler reads a
bounded request body, validates the stable subset it needs, rewrites the model
for each configured target, and calls a reusable upstream client. Responses are
copied directly to the caller; SSE writes flush after each upstream read.

The gateway does not deserialize completion responses. This avoids coupling it
to provider-specific extensions and preserves current and future fields.

## Research findings applied

- Go's standard `net/http` router supports method-aware routes and is enough for
  the MVP.
- A streaming route must not use `http.Client.Timeout` or `http.TimeoutHandler`
  because both can terminate long-running streams.
- The shared transport uses `ResponseHeaderTimeout`; request cancellation comes
  from `r.Context()`.
- YAML decoding uses `KnownFields(true)` and accepts exactly one document.
- Ollama's OpenAI compatibility is partial. Its adapter translates
  `max_completion_tokens` to `max_tokens`, while the public gateway rejects
  `n` values other than `1` instead of silently changing semantics.
- SSE content is relayed without parsing or manufacturing a terminal event.

Primary references:

- <https://pkg.go.dev/net/http>
- <https://pkg.go.dev/net/http/httputil#ReverseProxy>
- <https://pkg.go.dev/go.yaml.in/yaml/v3>
- <https://developers.openai.com/api/docs/guides/streaming-responses>
- <https://developers.openai.com/api/docs/guides/error-codes>
- <https://docs.ollama.com/api/openai-compatibility>

## Code reuse analysis

This is a greenfield project, so there is no existing application code to
reuse. The implementation reuses Go standard-library HTTP primitives,
`httptest`, `slog`, and context cancellation rather than adding a framework.

## Components

### Configuration

- **Purpose:** Decode, default, normalize, and validate deployment settings.
- **Location:** `internal/config/config.go`
- **Interfaces:**
  - `Load(path string) (Config, error)`
  - `Validate() error`
- **Dependencies:** `go.yaml.in/yaml/v3`, `net/url`, `time`
- **Reuses:** Standard environment expansion through `os.ExpandEnv`.

### Provider client

- **Purpose:** Build credentialed upstream requests and own the reusable HTTP
  transport.
- **Location:** `internal/provider/client.go`
- **Interfaces:**
  - `NewClients(config.Config) (map[string]*Client, error)`
  - `Do(ctx context.Context, body []byte) (*http.Response, error)`
- **Dependencies:** Configuration and `net/http`.

### Gateway

- **Purpose:** Validate the chat request, resolve aliases, execute retries and
  fallbacks, transform provider-specific fields, and relay responses.
- **Location:** `internal/gateway/gateway.go`
- **Interfaces:**
  - `ChatCompletions(http.ResponseWriter, *http.Request)`
  - `ListModels(http.ResponseWriter, *http.Request)`
  - `GetModel(http.ResponseWriter, *http.Request)`
- **Dependencies:** Configuration and provider clients.

### Public HTTP server

- **Purpose:** Wire routes and apply request ID, authentication, recovery, and
  structured logging middleware.
- **Location:** `internal/httpapi/server.go`
- **Interfaces:**
  - `New(config.Config, *gateway.Gateway, *slog.Logger) http.Handler`
- **Dependencies:** Gateway and `log/slog`.

### Command

- **Purpose:** Load configuration, initialize dependencies, and manage graceful
  HTTP shutdown.
- **Location:** `cmd/gateway/main.go`
- **Dependencies:** All internal packages.

## Data models

### Configuration

```go
type Config struct {
    Server    ServerConfig
    Providers map[string]ProviderConfig
    Models    map[string]ModelConfig
    Routing   RoutingConfig
}

type ModelConfig struct {
    Targets []TargetConfig
}

type TargetConfig struct {
    Provider string
    Model    string
}
```

### Inspected chat fields

The gateway decodes the request into `map[string]json.RawMessage` and only
interprets `model`, `stream`, `n`, `max_tokens`, and `max_completion_tokens`.
It preserves every other field.

## Error handling strategy

| Error scenario | Handling | Client impact |
| --- | --- | --- |
| Invalid JSON or model | OpenAI-shaped local error | 400 or 404 |
| Invalid inbound key | OpenAI-shaped auth error | 401 |
| Non-retryable upstream status | Relay status and body | Provider detail preserved |
| Retryable upstream status | Retry, then ordered fallback | Transparent when recovery succeeds |
| All transports fail | Safe local gateway error | 502 with request ID |
| Failure after SSE starts | Close stream without `[DONE]` | Client detects truncation |

## Header policy

- Always return the gateway's `x-request-id`.
- Send provider credentials only to the selected provider.
- Relay `Content-Type`, `Cache-Control`, `Retry-After`, and rate-limit headers.
- Expose an upstream request ID as `x-upstream-request-id` when available.
- Do not forward hop-by-hop headers.

## Technical decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| HTTP framework | Standard library | Method routing and middleware needs are small |
| Provider abstraction | OpenAI-compatible HTTP client | Avoids provider SDK coupling |
| Response handling | Opaque streaming copy | Preserves extensions and bounds memory |
| Request handling | Bounded buffer and minimal JSON rewrite | Enables retries while retaining unknown fields |
| Request IDs | `crypto/rand` hex | No extra dependency and compatible with Go 1.26 |
| Configuration | Strict YAML plus environment expansion | Familiar operations model with early failures |

## Requirement mapping

| Requirement | Component |
| --- | --- |
| GW-01, GW-02, GW-03, GW-08 | Gateway and provider client |
| GW-04 | Configuration |
| GW-05, GW-06 | Public HTTP server |
| GW-07 | Gateway model handlers |
