# Project state

**Updated:** September 18, 2026

## Decisions

- The MVP is stateless and uses one YAML file as its source of truth.
- The public API is an OpenAI-compatible subset centered on chat completions.
- Request bodies are minimally inspected so unknown fields pass through.
- Provider support begins with OpenAI-compatible HTTP APIs, specifically
  OpenAI and Ollama.
- Standard Go tests and `httptest` provide unit and end-to-end coverage.
- Go 1.26 is the tested baseline; the implementation uses only standard HTTP
  features available since Go 1.22.
- The container uses a multi-stage Go build and a non-root distroless runtime.

## Blockers

- None.

## Lessons learned

- Root binary ignore patterns must be anchored so they do not hide Go package
  directories with the same name.
- Streaming middleware must expose its wrapped writer through `Unwrap` so
  `http.ResponseController` can flush SSE chunks.

## Deferred ideas

- Cost tracking, budgets, rate limiting, and persistent usage records
- Responses API and embeddings
- Hot reload and distributed configuration
- Full observability stack integration

## Preferences

- Keep dependencies minimal and code idiomatic.
