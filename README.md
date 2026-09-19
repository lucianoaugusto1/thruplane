# NexoRoute

**The open control plane for AI traffic.**

NexoRoute is an open-source AI gateway written in Go. It gives applications one
OpenAI-compatible endpoint for hosted and local models, then centralizes model
aliases, provider credentials, streaming, retries, and failover.

NexoRoute Community is available now under Apache License 2.0. NexoRoute Pro
and NexoRoute Enterprise are planned commercial editions for teams that need
cost controls, governance, high availability, and support.

## Why NexoRoute

- Keep application code independent from provider URLs and credentials.
- Route one public model alias to ordered OpenAI or Ollama targets.
- Preserve unknown JSON fields and opaque provider responses.
- Relay SSE data incrementally without a global stream timeout.
- Run a small, stateless binary with one external Go dependency.
- Inspect, self-host, modify, and redistribute the Community source.

## Current capabilities

- `POST /v1/chat/completions`, including SSE streaming
- `GET /v1/models` and `GET /v1/models/{model}`
- OpenAI and Ollama through OpenAI-compatible upstream APIs
- Bounded retries and ordered fallback for transient failures
- Strict YAML configuration with environment expansion
- Optional inbound bearer authentication
- Request IDs, structured JSON logs, health checks, and graceful shutdown
- Distroless, non-root container image

## Editions

| Edition | Status | Designed for |
| --- | --- | --- |
| Community | Available | Developers and teams that self-host the core gateway |
| Pro | Planned | Teams that need usage, cost, policy, and alerting workflows |
| Enterprise | Planned | Organizations that need SSO, audit, HA, and contracted support |

Core routing, protocols, provider adapters, streaming, and basic observability
remain part of Community. See [edition principles and roadmap](docs/editions.md)
for the proposed commercial boundary and the
[product strategy](docs/product-strategy.md) for the longer-term direction.

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
   export NEXOROUTE_API_KEY="change-me"
   export OPENAI_API_KEY="your-openai-key"
   ```

   Leave `NEXOROUTE_API_KEY` empty only for trusted local development. Ollama
   does not require `OPENAI_API_KEY`.

3. Optional: pull the example local model.

   ```sh
   ollama pull llama3.2
   ```

4. Start NexoRoute.

   ```sh
   go run ./cmd/nexoroute -config config.yaml
   ```

5. Check its health.

   ```sh
   curl http://localhost:8080/healthz
   ```

## Send requests

List the public model aliases:

```sh
curl http://localhost:8080/v1/models \
  -H "Authorization: Bearer ${NEXOROUTE_API_KEY}"
```

Send a buffered chat completion through the `local` alias:

```sh
curl http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ${NEXOROUTE_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local",
    "messages": [{"role": "user", "content": "Why is the sky blue?"}]
  }'
```

Stream a completion:

```sh
curl --no-buffer http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ${NEXOROUTE_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local",
    "messages": [{"role": "user", "content": "Write one short sentence."}],
    "stream": true
  }'
```

For OpenAI-compatible SDKs, use `http://localhost:8080/v1` as the base URL and
the NexoRoute API key as the client API key.

## Configure routing

Each public alias has an ordered list of upstream targets:

```yaml
models:
  fast:
    targets:
      - provider: openai
        model: gpt-4o-mini
      - provider: ollama
        model: llama3.2:latest
```

NexoRoute retries the current target before moving to the next target. It
retries HTTP `408`, `429`, `500`, `502`, `503`, and `504`, plus transport
errors. It returns other `4xx` responses immediately because another provider
cannot fix an invalid request or credential.

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
| `models.*.targets` | Ordered provider and model pairs | Required |
| `routing.retries` | Extra attempts per target | `1` |
| `routing.response_header_timeout` | Upstream header timeout | `30s` |

The loader expands `${VARIABLE}` placeholders before validation and rejects
unknown YAML fields or multiple YAML documents.

## Run with Docker

Build the image:

```sh
docker build -t nexoroute .
```

Run it with your configuration mounted read-only:

```sh
docker run --rm -p 8080:8080 \
  -e NEXOROUTE_API_KEY \
  -e OPENAI_API_KEY \
  -v "$PWD/config.yaml:/etc/nexoroute/config.yaml:ro" \
  nexoroute
```

When Ollama runs on the Docker host, replace its URL in `config.yaml` with a
host address reachable from the container. Docker Desktop commonly provides
`http://host.docker.internal:11434`.

## Develop and verify

Run the complete project gate:

```sh
go test ./...
go vet ./...
go build ./cmd/nexoroute
```

Run the race detector before merging concurrency-related changes:

```sh
go test -race ./...
```

Read [CONTRIBUTING.md](CONTRIBUTING.md) before opening a change. Report
vulnerabilities through the private process in [SECURITY.md](SECURITY.md).

## Current limitations

- NexoRoute implements Chat Completions, not Responses, embeddings, image,
  audio, or batch APIs.
- `n` must be omitted or set to `1` so providers do not silently diverge.
- Tool use, vision, and structured output remain provider- and model-specific.
- Ollama's OpenAI compatibility can vary by version and model.
- Community does not yet include persistent usage history, dynamic reload,
  Prometheus metrics, distributed tracing, or tenant-level policies.
- If an upstream stream fails after headers are sent, NexoRoute closes the
  stream without inventing a `[DONE]` event.

## License and brand

NexoRoute Community is licensed under [Apache License 2.0](LICENSE). The
license covers the source code in this repository. Planned commercial modules
may use separate terms.

NexoRoute is a working brand pending formal trademark, domain, and registry
clearance. See the [brand guide](docs/brand.md) for current naming rules.
