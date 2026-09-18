# GoLLM Gateway

**Vision:** Build a small, production-minded LLM gateway in Go that exposes an
OpenAI-compatible API and routes requests to multiple upstream providers.
**For:** Go teams that want one stable API in front of hosted and local models.
**Solves:** Provider-specific credentials, URLs, model names, retries, and
fallback behavior otherwise leak into every application.

## Goals

- Serve OpenAI-compatible chat and model-list endpoints through one binary.
- Add a provider or model route through configuration without changing clients.
- Preserve streaming behavior and unknown request fields end to end.
- Keep the MVP stateless and verify it with deterministic Go tests.

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

**v1 includes:**

- `POST /v1/chat/completions` with non-streaming and SSE streaming responses
- `GET /v1/models` with configured model aliases
- OpenAI-compatible upstream adapters for OpenAI and Ollama
- Ordered fallbacks, bounded retries, timeouts, optional inbound API key, health
  checks, request IDs, and structured logs
- One executable, Docker packaging, example configuration, and automated tests

**Explicitly out of scope:**

- Billing, budgets, cost accounting, and persistent usage history
- Per-tenant rate limits or an administrative UI
- Native provider protocols that are not OpenAI-compatible
- Embeddings, image, audio, batch, and Responses API endpoints
- Dynamic configuration reload and distributed state

## Constraints

- Technical: Preserve JSON fields the gateway does not understand.
- Technical: Never expose upstream credentials in logs or error messages.
- Resources: Prefer the standard library and keep the dependency surface small.
- Compatibility: Focus on the practical OpenAI-compatible subset, not a claim of
  complete LiteLLM feature parity.
