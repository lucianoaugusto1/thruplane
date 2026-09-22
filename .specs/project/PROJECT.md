# NexoRoute

**Vision:** Make reliable AI traffic control accessible as open-source
infrastructure, with a clear path to managed operations and enterprise
governance.
**For:** Teams that want one stable API in front of hosted and local models.
**Solves:** Provider-specific credentials, URLs, model names, retries, and
fallback behavior otherwise leak into every application.

## Goals

- Serve OpenAI-compatible chat and model-list endpoints through one binary.
- Add a provider or model route through configuration without changing clients.
- Preserve streaming behavior and unknown request fields end to end.
- Keep the MVP stateless and verify it with deterministic Go tests.
- Keep the Community edition production-usable and free from protocol lock-in.

## Product model

- **Community:** Apache 2.0 self-hosted gateway with core routing, streaming,
  retries, fallbacks, authentication, health checks, and structured logs.
- **Pro (planned):** Team operations, cost controls, analytics, and a managed
  experience.
- **Enterprise (planned):** Organization-wide governance, security, scale,
  deployment assurance, and support.

## Tech stack

**Core:**

- Runtime: Go 1.26
- HTTP server and client: Go standard library
- Configuration: YAML with environment-variable expansion
- Storage: None for v1

**Key dependencies:**

- `go.yaml.in/yaml/v3` for strict YAML decoding
- Go standard library for HTTP, logging, retries, and streaming

## Scope

**Community implementation includes:**

- `POST /v1/chat/completions` with non-streaming and SSE streaming responses
- `GET /v1/models` with configured model aliases
- Direct initial adapters for OpenAI, Anthropic, Gemini, Vertex AI, Bedrock,
  Azure OpenAI, Ollama, xAI/Grok, and configurable compatible APIs
- Native function-tool translation, image and PDF input, and Gemini/Vertex
  WAV and MP3 input; capability-aware model routing
- Ordered fallbacks, bounded retries, timeouts, optional inbound API key, health
  checks, request IDs, and structured logs
- One executable, Docker packaging, example configuration, and automated tests

**Not in the current implementation:**

- Shipping or charging for Pro and Enterprise before their features exist
- Billing, budgets, cost accounting, and persistent usage history in v1
- Per-tenant rate limits or an administrative UI
- Provider-hosted tools, media output normalization, and Bedrock streaming
- Embeddings, image, audio, batch, and Responses API endpoints
- Dynamic configuration reload and distributed state

## Constraints

- Technical: Compatible adapters preserve unknown JSON fields; native
  adapters must reject fields they cannot translate without data loss.
- Technical: Never expose upstream credentials in logs or error messages.
- Resources: Prefer the standard library and keep the dependency surface small.
- Compatibility: Focus on the practical OpenAI-compatible subset, not a claim of
  complete LiteLLM feature parity.
- Commercial: Label roadmap capabilities as planned until they are implemented
  and validated.
- Brand: Treat NexoRoute as a working name until legal and registry clearance.
- Validation: Local protocol tests do not replace live, region-specific
  provider conformance or end-to-end latency measurements.
