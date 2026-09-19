# Provider adapters

NexoRoute calls provider APIs directly from the customer-owned data plane. No
aggregation gateway or NexoRoute-hosted control plane sits in the request path.

The public endpoint remains `POST /v1/chat/completions`. Each model alias maps
to the provider-specific model identifier configured in `models.*.targets`.

## Support matrix

| Type | Upstream protocol | Buffered | Streaming | Authentication |
| --- | --- | --- | --- | --- |
| `openai` | OpenAI Chat Completions | Yes | Yes | Bearer key |
| `anthropic` | Anthropic Messages | Text | Text | `x-api-key` |
| `gemini` | Gemini `generateContent` | Text | Text | `x-goog-api-key` |
| `vertex` | Vertex `generateContent` | Text | Text | OAuth bearer token |
| `bedrock` | Bedrock Converse | Text | Not yet | AWS Signature Version 4 |
| `azure-openai` | Azure OpenAI v1 chat | Yes | Yes | `api-key` |
| `ollama` | Ollama OpenAI compatibility | Yes | Yes | Optional bearer key |
| `openai-compatible` | Configurable OpenAI compatibility | Yes | Yes | Optional bearer key |
| `nexoroute-inference` | NexoRoute OpenAI compatibility | Yes | Yes | Bearer key |
| `xai` | xAI Chat Completions | Yes | Yes | Bearer key |

`grok` is accepted as a configuration alias and normalizes to the `xai`
provider type. Grok is the model family; xAI is the API provider.

Compatible adapters preserve request fields they don't interpret and relay
successful response bodies without conversion. Native adapters translate
text messages and normalize successful provider responses. They reject
unsupported content instead of silently dropping it.

## OpenAI

```yaml
providers:
  openai:
    type: openai
    api_key: "${OPENAI_API_KEY}"
```

The default base URL is `https://api.openai.com`.

## Anthropic

```yaml
providers:
  anthropic:
    type: anthropic
    api_key: "${ANTHROPIC_API_KEY}"
    api_version: "2023-06-01"
```

The adapter uses `POST /v1/messages`. It maps `system` and `developer`
messages to the Anthropic system instruction. When neither token field is
present, it sends `max_tokens: 1024` because the Messages API requires an
output limit.

## Gemini

```yaml
providers:
  gemini:
    type: gemini
    api_key: "${GEMINI_API_KEY}"
```

The adapter calls `generateContent` or `streamGenerateContent` at the default
`https://generativelanguage.googleapis.com` endpoint.

## Vertex AI

```yaml
providers:
  vertex:
    type: vertex
    project: "${GOOGLE_CLOUD_PROJECT}"
    location: us-central1
    access_token: "${GOOGLE_ACCESS_TOKEN}"
```

NexoRoute builds the regional Vertex endpoint from `location`. Supply a
short-lived OAuth access token. Automatic workload-identity and metadata-server
token refresh are planned secret-provider integrations.

## Amazon Bedrock

```yaml
providers:
  bedrock:
    type: bedrock
    region: us-east-1
    access_key_id: "${AWS_ACCESS_KEY_ID}"
    secret_access_key: "${AWS_SECRET_ACCESS_KEY}"
    session_token: "${AWS_SESSION_TOKEN}"
```

The adapter calls the regional Bedrock Runtime Converse endpoint and signs each
request with AWS Signature Version 4. Use short-lived credentials in
production. Streaming returns a `400 unsupported_streaming` error before any
network request until NexoRoute includes a CRC-validated AWS event-stream
decoder.

## Azure OpenAI

```yaml
providers:
  azure:
    type: azure-openai
    base_url: https://your-resource.openai.azure.com
    api_key: "${AZURE_OPENAI_API_KEY}"
```

The adapter calls `/openai/v1/chat/completions`. Set `api_version` only when
your Azure deployment requires the legacy query parameter.

## Ollama

```yaml
providers:
  ollama:
    type: ollama
```

The default base URL is `http://localhost:11434`. The adapter maps
`max_completion_tokens` to Ollama's `max_tokens` compatibility field.

## xAI and Grok

```yaml
providers:
  xai:
    type: xai
    api_key: "${XAI_API_KEY}"
```

The default base URL is `https://api.x.ai`. You can write `type: grok` as an
alias, but `xai` is the canonical type.

## Custom OpenAI-compatible endpoints

```yaml
providers:
  private-models:
    type: openai-compatible
    base_url: https://models.example.com
    api_key: "${PRIVATE_MODELS_API_KEY}"
```

The base URL is required and NexoRoute appends `/v1/chat/completions`.

## NexoRoute Inference

```yaml
providers:
  inference:
    type: nexoroute-inference
    base_url: "${NEXOROUTE_INFERENCE_URL}"
    api_key: "${NEXOROUTE_INFERENCE_KEY}"
```

The protocol adapter is available now. The commercial hosted NexoRoute
Inference service remains planned, so configure this type only with an
endpoint you operate or have been given.

## Performance behavior

All providers share one concurrency-safe HTTP transport with HTTP/2 enabled,
connection pooling, 512 total idle connections, and 64 idle connections per
host. NexoRoute applies a response-header timeout but no global response-body
timeout, so request cancellation controls long-running streams.
