# Project state

**Updated:** September 17, 2026

## Decisions

- The MVP is stateless and uses one YAML file as its source of truth.
- The public API is an OpenAI-compatible subset centered on chat completions.
- Request bodies are minimally inspected so unknown fields pass through.
- Provider support begins with OpenAI-compatible HTTP APIs, specifically
  OpenAI and Ollama.
- Standard Go tests and `httptest` provide unit and end-to-end coverage.

## Blockers

- None.

## Deferred ideas

- Cost tracking, budgets, rate limiting, and persistent usage records
- Responses API and embeddings
- Hot reload and distributed configuration
- Full observability stack integration

## Preferences

- Keep dependencies minimal and code idiomatic.
