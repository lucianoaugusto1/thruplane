# Model capability catalog design

**Spec**: `.specs/features/model-capability-catalog/spec.md`
**Status**: Approved by direct implementation request

## Architecture

The `internal/catalog` package embeds provider YAML files and builds an immutable
lookup keyed by provider type and model ID. A request inspector derives hard
requirements from the OpenAI Chat Completions payload. The gateway intersects
those requirements with both the cataloged model capabilities and the adapter's
implemented protocol capabilities before selecting a target.

Unknown models retain passthrough behavior by default. Operators can set
`catalog.unknown_models: reject` for a closed catalog. A target can set
`catalog_model` when its routing identifier is a deployment name or inference
profile rather than a public model ID.

## Components

### Catalog loader

- **Location**: `internal/catalog/`
- **Purpose**: Parse strict embedded YAML, validate facts, and resolve models.
- **Dependencies**: Existing YAML library and `embed` from the standard library.

### Request inspector

- **Location**: `internal/catalog/request.go`
- **Purpose**: Detect modality, tools, and streaming requirements without
  changing the request body.

### Gateway integration

- **Location**: `internal/gateway/`
- **Purpose**: Filter incompatible targets and expose effective metadata.
- **Reuses**: Existing ordered fallback loop and Models API.

## Data model

Each catalog file records provider metadata, model identity and lifecycle,
input/output modalities, Chat Completions features, context and output limits,
USD prices with explicit units, cache prices, qualitative latency, optional
measured TTFT or output throughput, retrieval date, and official sources.

Missing numeric performance fields are meaningful: the provider didn't publish
a comparable value. Runtime measurements must include methodology, region,
hardware where relevant, and observation time before they can populate those
fields.

## Error handling

Malformed embedded data fails catalog initialization. Unknown models either pass
through or fail according to policy. Known but incompatible targets are skipped;
if no target remains, the gateway returns HTTP 400 with
`unsupported_capability`.

