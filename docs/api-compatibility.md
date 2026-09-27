# Chat Completions compatibility contract

**Contract revision:** 2026-09-25. This describes the current implementation,
not a claim of live conformance for every provider/model combination.

Thruplane exposes `POST /v1/chat/completions`. A model alias selects an
upstream target; the gateway replaces `model` with that target's identifier.
There are two request contracts:

| Adapter family | Request behavior | Successful response behavior |
| --- | --- | --- |
| OpenAI-compatible (`openai`, `azure-openai`, `ollama`, `openai-compatible`, `thruplane-inference`, `xai`/`grok`) | Preserves extra JSON fields; translates the model and, for Ollama, the token-limit name | Relays the upstream body and stream |
| Native (`anthropic`, `gemini`, `vertex`, `bedrock`) | Translates only the fields below; rejects unsupported fields before upstream I/O | Normalizes buffered text, function calls, finish reason, and basic usage; Anthropic, Gemini, and Vertex also normalize streamed text and function calls |

The compatible contract passes a field through; it does not guarantee that
the selected provider or model accepts that field. Bedrock native streaming
is not yet implemented. Native generated image/audio output and provider-
hosted tools are not normalized.

## Native request fields

| Location | Accepted | Limits |
| --- | --- | --- |
| Request | `model`, `messages`, `max_completion_tokens` **or** `max_tokens`, `temperature`, `top_p`, `stop`, `stream`, `tools`, `tool_choice`, `parallel_tool_calls` | Token limit must be positive; `stop` is a string or array of strings. Bedrock rejects `stream: true`. |
| Equivalent defaults | `n: 1`, `modalities: ["text"]`, `response_format: {"type":"text"}` | These do not change native provider payloads. Other values are rejected. |
| Message | `role`, `content`, `tool_calls`, `tool_call_id` | `tool_calls` belong to assistant messages; `tool_call_id` belongs to tool results. System, developer, assistant, and tool content is text-only. |
| Tool | `type: "function"` and `function.name`, `description`, `parameters`, `strict: false` | JSON Schema `parameters` remains an opaque object. `strict: true` and provider-hosted tools are not portable. |
| Tool choice | `auto`, `none`, `required`, or a named function | `parallel_tool_calls: false` is portable only to Anthropic when tools are present. |
| User content part | `text`/`input_text` with `text`; `image_url` with `url`; `file` with inline PDF `file_data`; `input_audio` with base64 `data` and `wav`/`mp3` `format` | Provider/model capabilities still apply. See [the media matrix](providers.md#native-media-input). |

Unrecognized root, message, and tool fields return `400 unsupported_field`.
Examples include `seed`, `frequency_penalty`, `stream_options`, `service_tier`,
`metadata`, `store`, and structured `response_format`. Unrecognized content-
part properties return `400 unsupported_content`; this includes
`image_url.detail`, `file.filename`, and cache hints. Known unsupported
options may have more specific codes, such as `unsupported_tool_option`,
`unsupported_n`, or `unsupported_streaming`. Known cataloged targets can be
filtered earlier and return `unsupported_capability` when none qualify.

Unknown models allowed by `catalog.unknown_models: allow` **do not bypass**
native field validation. A native contract error stops the request; it does
not trigger fallback to a later compatible target.

## Migration note

Native adapters previously accepted `file.filename` but discarded it. They
now reject it. Remove `filename` when sending inline PDFs to a native target.
If the filename or another provider-specific option is required, select an
OpenAI-compatible target whose upstream API supports it. Do not rely on
silently ignored fields as a compatibility mechanism.

## Response compatibility

### OpenAI-compatible adapters

OpenAI-compatible adapters don't decode successful or error responses. They
relay the upstream status, headers, and body. This includes buffered JSON, SSE
event names and data, provider-specific finish reasons, cache token details,
usage extensions, tool calls, and future fields.

Passthrough preserves evidence; it doesn't normalize semantics. A client must
understand any extension returned by the selected provider. Thruplane doesn't
add a missing `[DONE]` marker or reshape a provider-specific SSE event.

Deterministic fixtures verify the following response contract for `openai`,
`azure-openai`, `ollama`, `openai-compatible`, `thruplane-inference`, and
`xai`:

| Response shape | Status | Headers | Body |
| --- | --- | --- | --- |
| Buffered success | Preserved | Preserved | Byte-for-byte passthrough |
| SSE success | Preserved | Preserved | Byte-for-byte passthrough |
| Upstream error | Preserved | Preserved | Byte-for-byte passthrough |

Configuration normalizes `grok` to `xai` before it creates the adapter, so it
uses the same conformance contract.

### Native adapters

Native successful responses use this normalized subset:

| Field | Buffered | Anthropic, Gemini, and Vertex SSE | Notes |
| --- | --- | --- | --- |
| `id` | Yes | Yes | Uses the upstream ID when available; otherwise uses a gateway fallback. |
| `object` | `chat.completion` | `chat.completion.chunk` | Generated by Thruplane. |
| `created` | Yes | Yes | Gateway Unix time, not upstream creation time. |
| `model` | Yes | Yes | Uses the returned model when the native response provides one. |
| `choices[0].message.role` | `assistant` | Initial role delta | Exactly one choice is produced. |
| Text content | String | Incremental deltas | Multiple native text blocks are concatenated when buffered. |
| Function calls | `tool_calls` | Incremental `tool_calls` | Arguments remain JSON strings. |
| Finish reason | Normalized | Final chunk | See the mapping below. |
| Basic usage | Final response | Final chunk when available | Includes prompt, completion, and total tokens. |
| Cache details | Not normalized | Not normalized | Native cache-specific token fields aren't exposed yet. |

Bedrock currently supports only the buffered column. It rejects streaming
before network I/O.

Native finish reasons map as follows:

| Provider reason | OpenAI-compatible reason |
| --- | --- |
| Anthropic or Bedrock `max_tokens`; Google `MAX_TOKENS` | `length` |
| Anthropic or Bedrock `tool_use`; any Google function call | `tool_calls` |
| Bedrock `content_filtered` or `guardrail_intervened` | `content_filter` |
| Google reasons other than `STOP`, empty, or `MAX_TOKENS` | `content_filter` |
| Anthropic default; Bedrock default; Google `STOP` or empty | `stop` |

Successful native normalization sets `Content-Type` to the public response
format and removes `Content-Encoding`. Non-success native responses bypass
normalization and retain the upstream error status, headers, and body so the
gateway retry and fallback policy can classify them.

Native cache usage, provider-specific safety details, log probabilities,
generated image or audio output, citations, and other response extensions are
not normalized. Treat them as unsupported instead of assuming that missing
fields mean zero usage or no provider-side event.

The deterministic matrix proves protocol transformation against recorded
shapes. Live provider/model conformance remains separate in the
[provider validation guide](provider-validation.md).
