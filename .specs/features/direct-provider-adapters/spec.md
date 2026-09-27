# Direct provider adapters specification

**Status:** In progress
**Date:** September 19, 2026

## Goal

Connect each customer-owned Thruplane data plane directly to model provider
APIs. The initial contract remains `POST /v1/chat/completions`; Thruplane must
not require another gateway, proxy, or aggregation service in the request path.

## Requirements

### PA-01: Supported providers

Thruplane must recognize `openai`, `anthropic`, `gemini`, `vertex`, `bedrock`,
`azure-openai`, `ollama`, `openai-compatible`, `thruplane-inference`, and
`xai`. It must accept `grok` as an alias for `xai`.

### PA-02: Direct transport

Every adapter must call its configured provider endpoint directly over HTTP.
Adapters must share a tuned, concurrency-safe HTTP transport and must honor
request cancellation without a global response-body timeout.

### PA-03: Compatible fast path

OpenAI, Azure OpenAI, Ollama, xAI, OpenAI-compatible, and Thruplane Inference
must preserve the OpenAI-shaped request and response whenever their protocol
allows it. The adapter may rewrite provider-required fields, paths, and
credentials without decoding the response body.

### PA-04: Native request translation

Anthropic, Gemini, Vertex AI, and Amazon Bedrock must translate text chat
messages, system instructions, generation limits, sampling settings, stop
sequences, and streaming flags to their native request schemas.

### PA-05: Normalized responses

Native adapters must translate successful buffered text responses to an
OpenAI-compatible chat completion. Anthropic, Gemini, and Vertex streaming
responses must be emitted as OpenAI-compatible SSE chunks. Bedrock streaming
is deferred until AWS event-stream decoding is implemented; the adapter must
reject it before making an upstream request.

### PA-06: Authentication

Credentials must come only from provider configuration. The adapters must
support bearer keys, Azure `api-key`, Gemini `x-goog-api-key`, Google OAuth
bearer tokens, and AWS Signature Version 4 credentials as applicable.

### PA-07: Configuration safety

Configuration must be strict and validate type-specific required fields,
provider URLs, Google project and location, and AWS region and credentials.
Secrets must never be copied from the inbound request or logged.

### PA-08: Error behavior

Invalid or unsupported request features discovered by an adapter must return
an OpenAI-shaped `400` response without failover. Provider HTTP failures remain
eligible for the existing retry and fallback policy.

### PA-09: Verifiability

All provider paths, headers, request mappings, response mappings, signing, and
streaming behavior must have deterministic tests that use local HTTP servers.
No test may require live provider credentials.

## Initial scope

- Text chat completions with `n` equal to one.
- Buffered responses for all providers.
- Streaming for OpenAI-compatible, Anthropic, Gemini, and Vertex protocols.
- Direct AWS Bedrock Converse calls with Signature Version 4 for buffered
  responses.

## Deferred scope

- Bedrock event-stream decoding and streaming normalization.
- Tool-call normalization for native protocols.
- Multimodal content normalization for native protocols.
- Responses, embeddings, image, audio, batch, and fine-tuning APIs.
- Workload identity, metadata-server token acquisition, and AWS credential
  provider chains. Customers can inject short-lived tokens through the current
  configuration surface until secret-provider integrations exist.

## Success criteria

- All ten canonical provider types construct a direct request with the correct
  endpoint and authentication mechanism.
- `grok` behaves as the `xai` alias.
- Compatible providers retain opaque response passthrough.
- Native buffered and supported streaming responses conform to the gateway's
  public OpenAI-compatible contract.
- The full test, race, vet, and build gates pass.
