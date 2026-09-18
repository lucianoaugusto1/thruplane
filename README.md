# GoLLM Gateway

GoLLM Gateway is a small OpenAI-compatible LLM gateway written in Go. It gives
applications one endpoint for hosted and local models, then handles model
aliases, provider credentials, retries, fallbacks, and SSE streaming centrally.

The current release is a focused MVP. It supports OpenAI and Ollama through
their OpenAI-compatible chat APIs.

## Features

- `POST /v1/chat/completions`, including incremental SSE streaming
- `GET /v1/models` and `GET /v1/models/{model}`
- YAML model aliases with ordered provider targets
- Bounded retries and fallback for transient upstream failures
- OpenAI-shaped local errors and opaque upstream responses
- Optional bearer authentication for all `/v1/*` endpoints
- Request IDs, JSON access logs, health checks, and graceful shutdown
- Strict startup validation and environment-variable expansion
- One runtime dependency and one external Go module

## Requirements

- Go 1.26 or newer
- An OpenAI API key, a running Ollama server, or both
- Optional: Docker for container builds

## Run locally

1. Copy the example configuration.

   ```sh
   cp config.example.yaml config.yaml
   ```

2. Set the credentials you plan to use.

   ```sh
   export GATEWAY_API_KEY="change-me"
   export OPENAI_API_KEY="your-openai-key"
   ```

   Leave `GATEWAY_API_KEY` empty only for trusted local development. The Ollama
   provider does not require `OPENAI_API_KEY`.

3. Optional: pull the example local model.

   ```sh
   ollama pull llama3.2
   ```

4. Start the gateway.

   ```sh
   go run ./cmd/gateway -config config.yaml
   ```

5. Check its health.

   ```sh
   curl http://localhost:8080/healthz
   ```

## Send requests

List the public model aliases:

```sh
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer ${GATEWAY_API_KEY}"
```

Send a buffered chat completion to Ollama through the `local` alias:

```sh
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ${GATEWAY_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local",
    "messages": [{"role": "user", "content": "Why is the sky blue?"}]
  }'
```

Stream a completion:

```sh
curl --no-buffer http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ${GATEWAY_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local",
    "messages": [{"role": "user", "content": "Write one short sentence."}],
    "stream": true
  }'
```

For OpenAI-compatible SDKs, set the base URL to
`http://localhost:8080/v1` and use the gateway key as the client API key.

## Configure routing

Each public model name has an ordered list of targets:

```yaml
models:
  fast:
    targets:
      - provider: openai
        model: gpt-4o-mini
      - provider: ollama
        model: llama3.2:latest
```

The gateway retries the current target before moving to the next target. It
retries HTTP `408`, `429`, `500`, `502`, `503`, and `504`, plus transport
errors. It returns non-transient `4xx` responses immediately because another
provider cannot fix an invalid request or credential.

The gateway replaces only the upstream `model` field and preserves JSON fields
it does not interpret. For Ollama, it also translates
`max_completion_tokens` to `max_tokens`.

### Configuration reference

| Field | Meaning | Default |
| --- | --- | --- |
| `server.address` | HTTP listen address | `:8080` |
| `server.api_key` | Optional inbound bearer token | Empty |
| `server.max_body_bytes` | Maximum chat request body | `1048576` |
| `server.read_header_timeout` | Client header timeout | `5s` |
| `server.shutdown_timeout` | Graceful shutdown limit | `10s` |
| `providers.*.type` | `openai` or `ollama` | `openai` |
| `providers.*.base_url` | Provider root URL without `/v1` | Required |
| `providers.*.api_key` | Provider bearer token | Empty |
| `models.*.targets` | Ordered provider and upstream model pairs | Required |
| `routing.retries` | Extra attempts per target | `1` |
| `routing.response_header_timeout` | Upstream header timeout | `30s` |

The loader expands `${VARIABLE}` placeholders before validation and rejects
unknown YAML fields or multiple YAML documents.

## Run with Docker

Build the image:

```sh
docker build -t gollm-gateway .
```

Run it with your configuration mounted read-only:

```sh
docker run --rm -p 8080:8080 \
  -e GATEWAY_API_KEY \
  -e OPENAI_API_KEY \
  -v "$PWD/config.yaml:/etc/gollm/config.yaml:ro" \
  gollm-gateway
```

When Ollama runs on the Docker host, replace its URL in `config.yaml` with a
host address reachable from the container. Docker Desktop commonly provides
`http://host.docker.internal:11434`.

## Develop and verify

Run the complete project gate:

```sh
go test ./...
go vet ./...
go build ./cmd/gateway
```

Run the race detector before merging concurrency-related changes:

```sh
go test -race ./...
```

## MVP limitations

- The gateway implements Chat Completions, not the Responses, embeddings,
  image, audio, or batch APIs.
- `n` must be omitted or set to `1` so providers do not silently diverge.
- Tool use, vision, and structured output remain provider- and model-specific.
- Ollama's OpenAI compatibility is partial and can vary by version and model.
- The gateway has no cost tracking, persistent usage history, tenant policies,
  rate limits, dynamic reload, Prometheus metrics, or distributed tracing yet.
- If an upstream stream fails after headers are sent, the gateway closes the
  stream without inventing a `[DONE]` event.

See the implementation plan and requirement traceability under `.specs/`.
