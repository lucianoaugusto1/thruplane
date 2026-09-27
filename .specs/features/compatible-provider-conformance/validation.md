# OpenAI-compatible provider conformance validation

**Status:** Passed
**Date:** September 25, 2026

## Coverage

| Requirement | Evidence | Result |
| --- | --- | --- |
| CPC-01 | Six canonical provider variants plus existing `grok` normalization test | Passed |
| CPC-02 | Opaque text, media, tool, schema, and unknown-field request fixture | Passed |
| CPC-03 | Exact buffered, SSE, and upstream-error status/header/body checks | Passed |
| CPC-04 | Invalid-object fixture and canceled-context integration test | Passed |
| CPC-05 | Versioned response matrix and complete project gate | Passed |

The shared fixture runs four cases against each of the six compatible provider
types. It uses local `httptest` servers and synthetic credentials. It doesn't
contact OpenAI, Azure OpenAI, Ollama, xAI, or another external endpoint.

## Gates

```text
go test -count=1 ./...
go test -race -count=1 ./...
go vet ./...
go build ./cmd/thruplane
git diff --check
```

All gates passed. No test was skipped or removed, and the adapter required no
production change to satisfy the consolidated conformance contract.

## Claim boundary

The fixtures prove Thruplane's endpoint, authentication, request rewrite, and
response-passthrough behavior. They don't prove that every provider or model
accepts every forwarded OpenAI field. Live evidence remains unverified.
