# Community beta design

**Spec:** `.specs/features/community-beta/spec.md`  
**Status:** Approved by the request to continue through a functional beta

## Architecture overview

The beta work keeps the request data plane unchanged. A metrics collector sits
inside the existing HTTP layer, where it can measure end-to-end handler time
and read the existing safe route headers. The endpoint derives readiness
gauges from the gateway's aggregate snapshot at scrape time. The CLI reuses the
existing strict configuration loader.

## Code reuse analysis

| Existing component | Location | Reuse |
| --- | --- | --- |
| Strict configuration loader | `internal/config/load.go` | Power `-check-config` without a second parser. |
| Route response headers | `internal/gateway/gateway.go` | Record final provider, model, attempts, and fallbacks without inspecting bodies. |
| Readiness snapshot | `internal/gateway/circuit_breaker.go` | Export aggregate target states at scrape time. |
| HTTP response recorder | `internal/httpapi/server.go` | Record status and keep streaming flush support through `Unwrap`. |
| Existing Go gates | `.specs/codebase/TESTING.md` | Define the beta verification commands. |

## Components

### Metrics configuration

- **Purpose:** Enable the fixed `/metrics` endpoint explicitly.
- **Location:** `internal/config/` and `config.example.yaml`.
- **Interface:** `server.metrics.enabled` defaults to `false`.
- **Dependencies:** Existing strict YAML parsing and defaulting.

### Prometheus collector

- **Purpose:** Collect bounded, process-local counters, gauges, and
  histograms, then encode Prometheus text without a new dependency.
- **Location:** `internal/telemetry/metrics.go`.
- **Interfaces:** `BeginHTTPRequest`, `ObserveHTTPRequest`, and
  `ServeHTTP`.
- **Dependencies:** Standard library and a readiness callback.
- **Reuses:** Existing safe route headers and readiness snapshot.

### HTTP instrumentation

- **Purpose:** Register `/metrics` only when enabled and observe requests after
  authentication/recovery without reading request or response bodies.
- **Location:** `internal/httpapi/server.go`.
- **Dependencies:** Collector and existing response recorder.
- **Reuses:** Existing route metadata headers.

### CLI preflight and build identity

- **Purpose:** Validate configuration without listening and report build
  identity without reading configuration.
- **Location:** `cmd/nexoroute/main.go`.
- **Dependencies:** Existing loader; build variables supplied through
  `-ldflags` when packaging.

### CI and operator runbook

- **Purpose:** Turn the beta contract into repeatable automation and human
  procedures.
- **Location:** `.github/workflows/ci.yml`, `docs/beta.md`, and related index
  pages.

## Metrics model

All labels are bounded by route templates, HTTP status codes, and configured
provider/model targets. Metrics never use raw request paths, request IDs,
client identities, prompts, response text, file bytes, credentials, or
authorization headers.

| Metric | Type | Labels |
| --- | --- | --- |
| `nexoroute_http_requests_total` | Counter | method, route, status |
| `nexoroute_http_request_duration_seconds` | Histogram | method, route |
| `nexoroute_http_requests_in_flight` | Gauge | none |
| `nexoroute_route_selections_total` | Counter | provider, model |
| `nexoroute_request_attempts` | Histogram | provider, model |
| `nexoroute_request_fallbacks` | Histogram | provider, model |
| `nexoroute_targets` | Gauge | state |
| `nexoroute_build_info` | Gauge | version, revision, build_date |

The attempt and fallback histograms describe the complete request and use the
final selected route as labels. They do not claim per-attempt attribution.

## Error handling strategy

| Scenario | Handling | User impact |
| --- | --- | --- |
| Metrics disabled | Do not register or wrap the handler. | `/metrics` returns `404`. |
| Metrics scrape | Snapshot under a mutex, then encode outside request data. | Concurrent updates stay race-free. |
| Invalid config preflight | Return the existing loader error. | Nonzero exit before a listener opens. |
| Missing build metadata | Report stable `dev`, `unknown`, and `unknown` values. | Local builds remain identifiable as development builds. |

## Technical decisions

| Decision | Choice | Rationale |
| --- | --- | --- |
| Metrics library | Standard-library collector | Preserve the one-dependency runtime and cover the small fixed metric set. |
| Metrics authentication | Public when enabled | Match health/readiness probes; deployments must restrict the listener or scrape path at the network layer. |
| Default metrics state | Disabled | Avoid exposing deployment metadata without operator intent. |
| Label cardinality | Fixed routes plus configured targets | Prevent raw paths and user data from creating unbounded series. |
| Beta provider claim | Deterministic contract, live status explicit | Credentials and provider availability are external to the build. |
