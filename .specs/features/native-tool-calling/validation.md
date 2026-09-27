# Native tool calling validation

**Date:** September 19, 2026
**Result:** Passed

## Coverage

- Shared validation covers unsupported tool types, strict schemas, malformed
  arguments, legacy fields, invalid choices, and unmatched results.
- Anthropic fixtures cover definitions, named choice, assistant calls, tool
  results, buffered normalization, and partial JSON streaming.
- Gemini fixtures cover definitions, function-call configuration, assistant
  calls, function responses, buffered normalization, and streaming.
- Bedrock fixtures cover signed Converse requests, tool configuration,
  assistant calls, results, and buffered normalization.
- Cross-provider fixtures verify that parallel tool results share one native
  user turn and retain every call identifier.
- Existing compatible-provider tests confirm opaque request and response
  behavior remains unchanged.

## Gates

```text
go test -race ./...
go vet ./...
go build ./cmd/thruplane
```

All gates pass without live provider credentials or network calls.
