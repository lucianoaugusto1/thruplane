# Native multimodal chat specification

## Problem

The model catalog records image, document, and audio support, but native
adapters currently reject every non-text content part. Customers cannot use
these model capabilities through a direct NexoRoute integration.

## Goals

- [x] Preserve ordered text and media parts in native requests.
- [x] Translate supported media without another service or gateway-side fetch.
- [x] Reject unsupported media and formats before contacting the provider.
- [x] Expose only translated modalities as effective gateway capabilities.
- [x] Verify transformations and measure local adapter overhead.

## Out of scope

| Feature | Reason |
| --- | --- |
| Media output normalization | Chat audio and image output need separate response contracts. |
| Gateway file uploads or remote fetching | They add state, latency, and SSRF risk. |
| Provider Files APIs | They require lifecycle management and extra requests. |
| Bedrock streaming | Binary event-stream decoding is separate work. |
| Video | The public Chat Completions contract doesn't define a portable video part. |

## Acceptance criteria

### P1: Image and PDF input

1. WHEN a user message contains `image_url` as a supported data URI THEN the
   native adapter SHALL preserve its position and bytes in a provider image
   block.
2. WHEN a user message contains a PDF in `file.file_data` THEN the native
   adapter SHALL preserve its position and bytes in a provider document block.
3. WHEN Anthropic receives an HTTPS image URL THEN it SHALL pass the URL to
   Anthropic directly without fetching it in the gateway.
4. WHEN an adapter cannot represent a source or format THEN it SHALL return a
   structured 400 error before any upstream call.

### P2: Audio input

1. WHEN a Gemini or Vertex user message contains `input_audio` in WAV or MP3
   format THEN the adapter SHALL create an inline audio part.
2. WHEN Anthropic or Bedrock receives this audio contract THEN the gateway
   SHALL reject it explicitly before any upstream call.

### P2: Honest discovery and performance

1. WHEN model metadata is requested THEN effective capabilities SHALL include
   only the modalities supported by the target adapter.
2. WHEN a known model lacks the requested modality THEN routing SHALL skip it
   before contacting the provider.
3. WHEN translation is benchmarked THEN the repository SHALL include a local
   reproducible benchmark with allocations and runtime per operation.

## Edges

- Inline media must have valid base64 and a supported MIME type.
- A provider-specific file ID is not portable and must not be forwarded.
- Media is accepted in user messages only; tool results remain text-only.
- The gateway validates base64 syntax but must not interpret or log media
  contents.

## Traceability

| ID | Requirement | Status |
| --- | --- | --- |
| MM-01 | Parse ordered, validated Chat Completions content parts. | Done |
| MM-02 | Translate Anthropic image and PDF input. | Done |
| MM-03 | Translate Gemini and Vertex image, PDF, and audio input. | Done |
| MM-04 | Translate Bedrock Converse image and PDF input. | Done |
| MM-05 | Publish accurate effective capabilities and rejection behavior. | Done |
| MM-06 | Document supported shapes and benchmark local overhead. | Done |
