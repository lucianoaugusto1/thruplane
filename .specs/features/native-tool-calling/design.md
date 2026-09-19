# Native tool calling design

**Specification:** `spec.md`

## Common contract

`native.go` owns a small OpenAI tool model used only by native adapters:

- Function definition: name, description, and raw JSON Schema parameters.
- Tool call: identifier, function name, and JSON arguments string.
- Tool message: call identifier plus text or JSON result.
- Tool choice: string mode or named function object.

The common decoder validates that every tool is a function. It also builds a
call-ID-to-function-name index from assistant history because OpenAI tool-result
messages carry only `tool_call_id`, while Google requires the function name.

## Provider mappings

| OpenAI concept | Anthropic | Gemini and Vertex | Bedrock Converse |
| --- | --- | --- | --- |
| Function definition | `tools[].input_schema` | `tools[].functionDeclarations[].parameters` | `toolConfig.tools[].toolSpec.inputSchema.json` |
| Assistant call | `tool_use` block | `functionCall` part | `toolUse` content block |
| Tool result | user `tool_result` block | user `functionResponse` part | user `toolResult` content block |
| Model call response | `tool_use` | `functionCall` | `toolUse` |

## Streaming

The existing SSE pipe remains the transport. Anthropic block-start events emit
the OpenAI tool call identity and `input_json_delta` events emit argument
fragments. Google stream responses emit complete function-call parts, which
become one OpenAI tool-call delta per part. Bedrock remains buffered.

## Errors

Validation uses `RequestError`, so unsupported tool types, malformed schemas,
unmatched result IDs, and unrepresentable choices return a non-retryable `400`
without contacting the provider.
