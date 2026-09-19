# Direct provider adapters tasks

**Design:** `design.md`
**Status:** In progress

## Execution plan

```text
T1 -> T2 -> T3 -> T4 -> T5
```

### T1: Extend and validate provider configuration

**What:** Add all provider types and their type-specific settings.
**Where:** `internal/config/config.go`, `internal/config/config_test.go`
**Requirement:** PA-01, PA-06, PA-07
**Tests:** Unit
**Gate:** Quick

**Done when:**

- [ ] Every canonical type and the `grok` alias normalize correctly.
- [ ] Required URL, Google, Azure, and AWS fields are validated.
- [ ] Existing OpenAI and Ollama configurations remain valid.

### T2: Build the adapter transport and compatible family

**What:** Introduce the adapter contract, tune the shared transport, and add
direct adapters for OpenAI, Azure OpenAI, Ollama, xAI,
OpenAI-compatible endpoints, and NexoRoute Inference.
**Where:** `internal/provider/client.go`, `internal/provider/compatible.go`,
`internal/provider/client_test.go`
**Requirement:** PA-01, PA-02, PA-03, PA-06
**Tests:** Integration with `httptest`
**Gate:** Full

### T3: Add Anthropic and Google native codecs

**What:** Translate requests and normalize buffered and SSE responses for
Anthropic, Gemini, and Vertex.
**Where:** `internal/provider/anthropic.go`, `internal/provider/google.go`,
provider tests
**Requirement:** PA-04, PA-05, PA-08, PA-09
**Tests:** Unit and integration
**Gate:** Full

### T4: Add direct Bedrock Converse support

**What:** Translate buffered text chat, sign it with AWS Signature Version 4,
and normalize the response. Reject streaming before network I/O.
**Where:** `internal/provider/bedrock.go`, provider tests
**Requirement:** PA-04, PA-05, PA-06, PA-08, PA-09
**Tests:** Unit and integration
**Gate:** Full

### T5: Integrate errors and document operators' configuration

**What:** Render adapter request errors, add provider examples and support
matrix, and update persistent project state.
**Where:** `internal/gateway`, `README.md`, `config.example.yaml`, `docs`,
`.specs/project/STATE.md`
**Requirement:** PA-01 through PA-09
**Tests:** End-to-end, documentation review, and build
**Gate:** Build

## Definition and dependency cross-check

| Task | One concern | Depends on | Tests colocated | Status |
| --- | --- | --- | --- | --- |
| T1 | Configuration contract | None | Yes | Ready |
| T2 | Compatible transport | T1 | Yes | Ready |
| T3 | Native SSE codecs | T2 | Yes | Ready |
| T4 | Bedrock Converse | T2 | Yes | Ready |
| T5 | Gateway and docs | T3, T4 | Yes | Ready |
