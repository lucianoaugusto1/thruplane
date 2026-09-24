# Provider adapters

NexoRoute calls provider APIs directly from the customer-owned data plane. No
aggregation gateway or NexoRoute-hosted control plane sits in the request path.

The public endpoint remains `POST /v1/chat/completions`. Each model alias maps
to the provider-specific model identifier configured in `models.*.targets`.

## Support matrix

| Type | Upstream protocol | Buffered | Streaming | Authentication |
| --- | --- | --- | --- | --- |
| `openai` | OpenAI Chat Completions | Yes | Yes | Bearer key |
| `anthropic` | Anthropic Messages | Text, image, PDF, and function tools | Yes, text/tool output | `x-api-key` |
| `gemini` | Gemini `generateContent` | Text, image, PDF, audio, and function tools | Yes, text/tool output | `x-goog-api-key` |
| `vertex` | Vertex `generateContent` | Text, image, PDF, audio, and function tools | Yes, text/tool output | OAuth bearer token |
| `bedrock` | Bedrock Converse | Text, image, PDF, and function tools | Not yet | AWS Signature Version 4 |
| `azure-openai` | Azure OpenAI v1 chat | Yes | Yes | `api-key` |
| `ollama` | Ollama OpenAI compatibility | Yes | Yes | Optional bearer key |
| `openai-compatible` | Configurable OpenAI compatibility | Yes | Yes | Optional bearer key |
| `nexoroute-inference` | NexoRoute OpenAI compatibility | Yes | Yes | Bearer key |
| `xai` | xAI Chat Completions | Yes | Yes | Bearer key |

`grok` is accepted as a configuration alias and normalizes to the `xai`
provider type. Grok is the model family; xAI is the API provider.

Compatible adapters preserve request fields they don't interpret and relay
successful response bodies without conversion. Native adapters translate
text and supported media input, client-executed function tools, tool results,
and successful provider responses. Streaming accepts the same input modalities
as buffered calls, but only text and function tool output is normalized. Native
adapters reject unsupported content instead of silently dropping it.
The [Chat Completions compatibility contract](api-compatibility.md) lists
accepted native fields, errors, and migration guidance.
The [provider validation guide](provider-validation.md) separates deterministic
fixture coverage from opt-in live evidence.

## Native media input

Native adapters accept media only in `role: user` messages on the Chat
Completions endpoint. The gateway preserves the order of text and media parts
and sends them directly in a single provider request; it does not download
remote files, upload to provider Files APIs, inspect media content, or execute
tools. The supported public shapes are:

```json
{
  "role": "user",
  "content": [
    {"type": "text", "text": "Summarize these inputs"},
    {"type": "image_url", "image_url": {
      "url": "data:image/png;base64,<base64>"
    }},
    {"type": "file", "file": {
      "file_data": "data:application/pdf;base64,<base64>"
    }},
    {"type": "input_audio", "input_audio": {
      "data": "<base64>", "format": "wav"
    }}
  ]
}
```

Replace `<base64>` with encoded bytes. The example combines all shapes to
show their syntax; only Gemini and Vertex accept the full combination.

| Input | Anthropic | Gemini/Vertex | Bedrock |
| --- | --- | --- | --- |
| `text` or `input_text` | Yes | Yes | Yes |
| `image_url` data URI | Yes | Yes | Yes |
| `image_url` HTTPS URL | Yes, passed through | No | No |
| PDF `file.file_data` data URI | Yes | Yes | Yes |
| `input_audio` base64 | No | WAV or MP3 | No |

Inline images support PNG, JPEG, GIF, and WebP. PDF is the only supported
document MIME type. Provider-specific `file_id`, non-HTTPS image URLs, other
audio formats, video, media in other roles, and multimodal tool results are
not portable. The gateway returns `400 unsupported_content` or
`400 invalid_media` for unsupported shapes or malformed base64 before
contacting the provider. For a cataloged model lacking the modality, routing
returns `400 unsupported_capability` if no suitable target remains. Individual
providers may impose smaller media or page limits than the gateway's
`server.max_body_bytes` setting, which defaults to 1 MiB.

Native adapters reject `file.filename` because they cannot preserve it
consistently; earlier versions silently discarded it. They also reject
untranslated content properties such as `image_url.detail`. Use an
OpenAI-compatible target if the upstream requires those fields.

Compatible adapters preserve the request body instead of translating it;
actual media support depends on the selected upstream model and endpoint.
The gateway does not currently normalize generated image or audio output.
For local translation overhead, run:

```sh
go test ./internal/provider -run '^$' \
  -bench BenchmarkNativeInlineImageTranslation -benchmem
```

The benchmark excludes HTTP, provider latency, and network transfer.

## Function tools

Native adapters support the OpenAI Chat Completions tool loop:

1. Send `tools` with one or more `type: function` definitions.
2. Read `choices[0].message.tool_calls` from the response.
3. Execute each function in your application.
4. Append the assistant response and one `role: tool` message per result.
5. Send the full conversation again to receive the final answer.

NexoRoute preserves function names, JSON Schema parameters, call identifiers,
JSON arguments, multiple calls, tool results, and `finish_reason: tool_calls`.
Anthropic and Google tool calls are also normalized during streaming.

The native portability contract has these deliberate limits:

- Only `type: function` tools are portable. Provider-hosted web search, code
  execution, computer use, and remote MCP tools aren't translated.
- `strict: true` is rejected until every native protocol can preserve the same
  JSON Schema guarantee.
- Anthropic honors `parallel_tool_calls: false`. Gemini, Vertex, and Bedrock
  reject that setting because they don't expose an equivalent portable control
  through these APIs. Parallel calls remain supported when the field is
  omitted or `true`.
- Legacy `functions` and `function_call` fields are rejected. Use `tools` and
  `tool_choice`.
- Tool execution stays in your application. NexoRoute translates the protocol
  but never executes a function on the customer's behalf.

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
`https://generativelanguage.googleapis.com` endpoint. Function declarations,
calls, and results use native Google content parts.

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
production. Function tools use Converse `toolConfig`, `toolUse`, and
`toolResult` blocks. Streaming returns a `400 unsupported_streaming` error
before any network request until NexoRoute includes a CRC-validated AWS
event-stream decoder.

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
