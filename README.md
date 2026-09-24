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
- Route one public model alias to ordered direct-provider targets.
- Preserve unknown JSON fields and opaque responses on OpenAI-compatible
  routes; reject untranslatable fields on native routes.
- Relay SSE data incrementally without a global stream timeout.
- Run a small, stateless binary with one external Go dependency.
- Inspect, self-host, modify, and redistribute the Community source.

## Current capabilities

- `POST /v1/chat/completions`, including SSE streaming
- `GET /v1/models` and `GET /v1/models/{model}`
- Direct adapters for OpenAI, Anthropic, Gemini, Vertex AI, Amazon Bedrock,
  Azure OpenAI, Ollama, xAI, custom OpenAI-compatible APIs, and NexoRoute
  Inference
- Rate-aware retries, per-target admission control, and ordered fallback
- Strict YAML configuration with environment expansion
- Sourced model catalog with capability-aware routing and model metadata
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
The [delivery plan](docs/next-steps.md) explains what remains to build and
the validation gates before public and paid releases.

## Requirements

- Go 1.26 or newer
- Credentials for at least one configured provider, or a local Ollama server
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
retries temporary HTTP `408`, `429`, `500`, `502`, `503`, and `504` responses,
plus transport errors. Retries honor `Retry-After`; otherwise they use bounded
exponential backoff with jitter. Quota, billing, and spend-limit `429` errors
skip same-target retries but can still use an independent fallback. Other
`4xx` responses return immediately because another provider cannot fix an
invalid request or credential.

Each physical provider/model target can also enforce an in-process request
rate, burst, concurrency ceiling, and queue timeout:

```yaml
models:
  fast:
    targets:
      - provider: openai
        model: gpt-4o-mini
        rate_limit:
          requests_per_minute: 500
          burst: 10
          max_concurrency: 32
          queue_timeout: 250ms

routing:
  retries: 1
  retry:
    base_delay: 200ms
    max_delay: 5s
    budget: 15s
```

Aliases that name the same configured provider and upstream model share one
limiter. Zero request rate or concurrency means that dimension is unlimited;
zero queue timeout fails over immediately. Set limits from the actual quota for
the provider account, region, and model. Exact local token-per-minute
accounting is not implemented because token reservation differs by provider;
NexoRoute does observe supported upstream remaining/reset headers. See the
[rate-limit operations guide](docs/rate-limits.md).

OpenAI-compatible adapters replace the upstream `model` field and preserve
other JSON fields they do not interpret. For Ollama, the gateway also
translates `max_completion_tokens` to `max_tokens`. Native adapters translate
only their documented subset and return `400` for untranslatable fields,
including when a model is not cataloged. See the
[Chat Completions compatibility contract](docs/api-compatibility.md).

See the [provider adapter guide](docs/providers.md) for direct endpoints,
credentials, native translation behavior, and current feature limits.
See [provider validation](docs/provider-validation.md) for deterministic
fixture coverage and the opt-in live smoke-test procedure.

For cataloged targets, NexoRoute checks the request against both model support
and adapter support before sending it upstream. Native adapters translate
supported image and PDF inputs; Gemini and Vertex also translate inline audio.
For example, an audio request skips an Anthropic target and can use a Gemini
fallback. HTTPS image URLs are portable to Anthropic but not Gemini, Vertex,
or Bedrock through the current adapters.
When no target qualifies, the gateway returns `unsupported_capability`.

Unknown model IDs pass through by default so private deployments keep working.
Set `catalog.unknown_models: reject` to require a catalog match. Use
`catalog_model` for an Azure deployment name or Bedrock inference-profile ID;
`catalog_provider` optionally selects a different catalog namespace. See the
[model catalog](docs/model-catalog.md) for data semantics and limitations.

### Configuration reference

| Field | Meaning | Default |
| --- | --- | --- |
| `server.address` | HTTP listen address | `:8080` |
| `server.api_key` | Optional inbound bearer token | Empty |
| `server.max_body_bytes` | Maximum chat request body | `1048576` |
| `server.read_header_timeout` | Client header timeout | `5s` |
| `server.shutdown_timeout` | Graceful shutdown limit | `10s` |
| `catalog.unknown_models` | `allow` passthrough or `reject` unknown target IDs | `allow` |
| `providers.*.type` | Provider protocol name | `openai` |
| `providers.*.base_url` | Provider root URL without an API path | Provider default or required for custom endpoints |
| `providers.*.api_key` | Bearer, Azure, Anthropic, or Gemini key | Empty |
| `providers.*.api_version` | Anthropic header or optional Azure query version | Provider default |
| `providers.*.access_token` | Vertex OAuth access token | Empty |
| `providers.*.project` | Google Cloud project for Vertex | Empty |
| `providers.*.location` | Google Cloud region for Vertex | Empty |
| `providers.*.region` | AWS region for Bedrock | Empty |
| `providers.*.access_key_id` | AWS access key for Bedrock | Empty |
| `providers.*.secret_access_key` | AWS secret key for Bedrock | Empty |
| `providers.*.session_token` | Optional AWS temporary-session token | Empty |
| `models.*.targets` | Ordered provider and model pairs | Required |
| `models.*.targets[].catalog_model` | Catalog ID for a deployment ID | Upstream model ID |
| `models.*.targets[].catalog_provider` | Override catalog provider namespace | Provider type |
| `models.*.targets[].rate_limit.requests_per_minute` | Local request token-bucket rate; `0` disables | `0` |
| `models.*.targets[].rate_limit.burst` | Immediate request burst when RPM is enabled | `1` with RPM, otherwise `0` |
| `models.*.targets[].rate_limit.max_concurrency` | Maximum in-flight responses/streams; `0` disables | `0` |
| `models.*.targets[].rate_limit.queue_timeout` | Maximum admission wait; `0` fails over immediately | `0s` |
| `routing.retries` | Extra attempts per target | `1` |
| `routing.response_header_timeout` | Upstream header timeout | `30s` |
| `routing.retry.base_delay` | Initial fallback delay without `Retry-After` | `200ms` |
| `routing.retry.max_delay` | Maximum fallback backoff delay | `5s` |
| `routing.retry.budget` | Total retry time per target, including attempts | `15s` |

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
- Compatible adapters pass through tool use, vision, and structured-output
  fields. Native adapters translate user-message image and PDF input, plus WAV
  and MP3 audio input on Gemini and Vertex, and client-executed function tools.
  They do not normalize image or audio output, video, provider file IDs, or
  multimodal tool results.
- Catalog entries describe provider model features; `effective_capabilities`
  in `GET /v1/models/{model}` shows the subset this gateway can use today.
- Unknown models in `allow` mode bypass capability filtering. Use `reject`
  for a closed, capability-checked deployment.
- Bedrock supports buffered Converse responses; Bedrock streaming remains
  deferred until the binary AWS event-stream decoder is available.
- Ollama's OpenAI compatibility can vary by version and model.
- Local limits are process-local. Community does not yet include persistent
  usage history, dynamic reload, Prometheus metrics, distributed tracing,
  tenant-level policies, or distributed limits across replicas.
- If an upstream stream fails after headers are sent, NexoRoute closes the
  stream without inventing a `[DONE]` event.

## License and brand

NexoRoute Community is licensed under [Apache License 2.0](LICENSE). The
license covers the source code in this repository. Planned commercial modules
may use separate terms.

NexoRoute is a working brand pending formal trademark, domain, and registry
clearance. See the [brand guide](docs/brand.md) for current naming rules.
