# Native tool calling specification

**Status:** Verified
**Date:** September 19, 2026

## Problem

OpenAI-compatible providers already receive tool fields unchanged, but the
Anthropic, Gemini, Vertex AI, and Bedrock adapters currently omit them. An
agent can therefore work against one provider and silently lose its tools when
routed to another.

## Scope

### TC-01: Function definitions

WHEN a request contains OpenAI `type: function` tools THEN every native
adapter SHALL translate the name, description, and JSON Schema parameters to
its provider-native schema.

### TC-02: Tool selection

WHEN a request contains `tool_choice` THEN every native adapter SHALL map
`auto`, `none`, `required`, and a named function choice when the provider
supports that mode. It SHALL reject a choice it cannot preserve.

### TC-03: Conversation history

WHEN conversation history contains assistant `tool_calls` and `role: tool`
messages THEN every native adapter SHALL translate the calls and matched
results without losing the tool-call identifier or function name.

### TC-04: Buffered responses

WHEN a native provider returns one or more tool calls THEN Thruplane SHALL
return OpenAI-compatible `message.tool_calls` with JSON-encoded `arguments`
and `finish_reason: tool_calls`.

### TC-05: Streaming responses

WHEN Anthropic, Gemini, or Vertex streams a tool call THEN Thruplane SHALL emit
OpenAI-compatible `delta.tool_calls` chunks and a final
`finish_reason: tool_calls`. Anthropic partial JSON SHALL remain incremental.

### TC-06: Explicit rejection

WHEN a native request contains a non-function tool, an unsupported content
part, or a tool option that cannot be preserved THEN the adapter SHALL return
an OpenAI-shaped `400` before network I/O.

### TC-07: Compatible fast path

WHEN the selected adapter uses an OpenAI-compatible protocol THEN tool fields
and response bodies SHALL continue through the existing opaque passthrough.

## Out of scope

- Provider-hosted tools such as web search, code execution, and remote MCP.
- Multimodal tool results.
- Legacy `functions` and `function_call` fields.
- Bedrock streaming, which remains blocked on AWS event-stream decoding.
- Executing tools inside Thruplane. Tools remain client-executed.

## Success criteria

- Initial tool request, model tool call, tool result, and final answer work
  through each native protocol.
- Multiple tool calls retain stable indices and identifiers.
- Unsupported native tool features never disappear silently.
- Deterministic local fixtures cover buffered and streaming mappings.

## Traceability

| Requirement | Implementation | Status |
| --- | --- | --- |
| TC-01 | Shared contract plus all native request codecs | Verified |
| TC-02 | Provider-specific tool-choice mapping | Verified |
| TC-03 | Call-name index and grouped tool-result turns | Verified |
| TC-04 | Shared normalized response and native codecs | Verified |
| TC-05 | Anthropic and Google SSE transformers | Verified |
| TC-06 | `RequestError` validation before transport | Verified |
| TC-07 | Existing compatible passthrough tests | Verified |
