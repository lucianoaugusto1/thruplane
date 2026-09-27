# Direct provider adapters validation

**Date:** September 19, 2026
**Result:** Passed

## Automated coverage

- Strict configuration tests cover every canonical provider type, the `grok`
  alias, defaults, and type-specific required fields.
- Local HTTP integration tests verify compatible provider paths,
  authentication, model replacement, passthrough, and transport reuse.
- Native adapter tests verify Anthropic, Gemini, Vertex AI, and Bedrock request
  translation plus normalized buffered responses.
- SSE fixtures verify Anthropic and Gemini streaming normalization.
- Bedrock tests verify payload hashing, Signature Version 4 headers, Converse
  response mapping, and pre-network streaming rejection.
- Gateway coverage verifies adapter request errors return a non-retryable,
  OpenAI-shaped `400` response.

## Gates

```text
go test -race ./...
go vet ./...
go build ./cmd/thruplane
```

All gates pass without live credentials or external provider calls.

## Deferred validation

- Live-provider smoke tests belong in opt-in CI jobs after secret management
  and provider accounts are available.
- Bedrock streaming needs binary event-stream fixtures before implementation.
- Tool and multimodal fixtures are required before native adapters claim those
  capabilities.
