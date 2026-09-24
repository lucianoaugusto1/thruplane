# Native provider conformance specification

## Problem

The native adapters have focused `httptest` coverage, but their protocol
examples live inside test code. That makes drift hard to review, coverage hard
to summarize, and live validation hard to run consistently.

## Goals

- [ ] Store deterministic protocol fixtures outside Go source for Anthropic,
  Gemini, Vertex AI, and Amazon Bedrock.
- [ ] Verify request method, path, authentication shape, translated JSON,
  normalized buffered responses, streaming, and upstream errors.
- [ ] Keep live provider tests opt-in, bounded, and absent from the default
  test path.
- [ ] Publish a capability matrix that distinguishes fixture coverage from
  live verification.

## Out of scope

- Claiming that a provider/model combination works before a live test passes.
- Adding Bedrock streaming or new request fields.
- Testing OpenAI-compatible adapters in this first native-adapter slice.
- Running paid provider calls without an explicit live-test command.

## Acceptance criteria

1. WHEN the default Go test suite runs without credentials, THEN all protocol
   fixtures SHALL run locally and no public provider SHALL be contacted.
2. WHEN a fixture runs, THEN it SHALL assert the translated request and the
   normalized response or explicit adapter error.
3. WHEN streaming is supported, THEN fixtures SHALL cover normalized text and
   tool-call output; Bedrock SHALL record its current pre-network rejection.
4. WHEN a live build is requested, THEN the runner SHALL require an explicit
   provider, model, scenario list, credentials, and a bounded output limit.
5. WHEN validation status is documented, THEN fixture and live evidence SHALL
   be reported separately with provider, protocol, and verification date.
6. WHEN a Bedrock request is generated, THEN `inferenceConfig.maxTokens` SHALL
   be explicit in fixtures and live requests.

## Sources

- [Anthropic Messages API](https://platform.claude.com/docs/en/api/messages/create)
- [Anthropic streaming](https://platform.claude.com/docs/en/build-with-claude/streaming)
- [Gemini generateContent API](https://ai.google.dev/api/generate-content)
- [Vertex AI generative model REST API](https://cloud.google.com/vertex-ai/generative-ai/docs/reference/rest/v1beta1/projects.locations.publishers.models)
- [Amazon Bedrock Converse API](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_Converse.html)

## Traceability

| ID | Requirement | Status |
| --- | --- | --- |
| PC-01 | External deterministic native fixtures | Pending |
| PC-02 | Buffered, media, tool, stream, and error assertions | Pending |
| PC-03 | Explicit opt-in live runner | Pending |
| PC-04 | Evidence-based validation matrix | Pending |
