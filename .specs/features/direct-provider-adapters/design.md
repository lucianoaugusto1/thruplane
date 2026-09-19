# Direct provider adapters design

**Specification:** `spec.md`

## Architecture

```text
OpenAI-compatible request
          |
          v
  gateway routing/retry
          |
          v
 provider.Client ---- shared tuned http.Transport
          |
          +-- OpenAI-compatible adapter -> opaque response passthrough
          |
          +-- native adapter -> request codec -> provider API
                                     |
                                     +-> response/SSE normalizer
```

`provider.Client` owns one immutable adapter. Adapters build direct HTTP
requests and normalize successful responses. The gateway continues to own
model aliases, retries, fallback, and public error rendering.

## Fast path

The OpenAI-compatible family uses a raw JSON object only to replace `model`
and the small number of provider-specific fields. It does not decode response
bodies. This path covers OpenAI, Azure OpenAI, Ollama, xAI,
OpenAI-compatible endpoints, and NexoRoute Inference.

## Native codecs

- Anthropic maps system messages to `system`, chat messages to `messages`, and
  token and sampling fields to the Messages API. The response normalizer maps
  text content and named SSE events.
- Gemini maps system messages to `systemInstruction`, assistant messages to
  the `model` role, and generation fields to `generationConfig`.
- Vertex reuses the Gemini codec with the Vertex regional resource endpoint
  and OAuth bearer authentication.
- Bedrock maps the common request to Converse, signs the request with AWS
  Signature Version 4, and normalizes a buffered Converse response.

## Transport and performance

All clients share one cloned `http.Transport`. The transport enables HTTP/2,
raises idle connection limits, keeps connections alive, and applies only a
response-header timeout. Response bodies remain governed by request context so
long streams are not cut off by a global client timeout.

Native response conversion uses streaming decoders and pipes for SSE rather
than buffering an entire stream. Compatible responses remain zero-conversion
passthrough.

## Errors

An adapter can return `provider.RequestError` for deterministic client errors,
such as unsupported Bedrock streaming or unsupported native message content.
The gateway returns these as OpenAI-shaped `400` responses and does not retry
or fail over. Network errors and provider HTTP statuses retain current routing
semantics.

## Security

- Provider credentials overwrite, and never inherit, inbound credentials.
- AWS signatures include the exact host, payload hash, date, and optional
  session token.
- Configuration validation rejects credentials embedded in URLs.
- Tests use synthetic credentials and local servers only.

## Trade-offs

The first native codec deliberately supports text content only. Silently
dropping tool or multimodal blocks would be dangerous, so unsupported shapes
return a deterministic client error. Bedrock streaming is also explicit rather
than approximated because its binary event-stream protocol requires a separate
CRC-validated decoder.
