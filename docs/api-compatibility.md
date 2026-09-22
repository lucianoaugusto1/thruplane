# Chat Completions compatibility contract

**Contract revision:** 2026-09-22. This describes the current implementation,
not a claim of live conformance for every provider/model combination.

NexoRoute exposes `POST /v1/chat/completions`. A model alias selects an
upstream target; the gateway replaces `model` with that target's identifier.
There are two request contracts:

| Adapter family | Request behavior | Successful response behavior |
| --- | --- | --- |
| OpenAI-compatible (`openai`, `azure-openai`, `ollama`, `openai-compatible`, `nexoroute-inference`, `xai`/`grok`) | Preserves extra JSON fields; translates the model and, for Ollama, the token-limit name | Relays the upstream body and stream |
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

This is a request-field contract, not yet the complete versioned response
compatibility and live provider conformance matrix planned in the
[delivery plan](next-steps.md#1-define-and-enforce-api-compatibility).
