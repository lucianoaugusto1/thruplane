# Native Chat Completions contract design

**Spec:** `.specs/features/native-contract/spec.md`

`decodeChatRequest` is the shared entry point for Anthropic, Gemini, Vertex,
and Bedrock. Use `json.Decoder.DisallowUnknownFields` on the existing typed
request, message, tool, and tool-call structures. Explicitly model only
no-op-equivalent root fields (`n`, text-only `modalities`, and text-only
`response_format`), then validate their values. Keep JSON Schema parameters
and function arguments opaque; those are intentionally provider payloads.

The content parser receives `json.RawMessage`, so it validates each part and
its nested object separately. Text-only content in system, developer,
assistant, and tool messages gets equivalent shape validation. Unknown fields
are errors rather than being dropped. `filename` is rejected until a portable
translation exists; the Bedrock neutral document name remains unchanged.

Compatible adapters still use `transformCompatibleBody` and retain arbitrary
JSON fields. The gateway's catalog filter can skip known incapable models;
native preflight is the final guard for all selected models, including unknown
ones.

Error codes: `unsupported_field` for root/message/tool fields,
`unsupported_content` for content part fields, and existing dedicated codes
for legacy functions, tool choice, media format, and other known violations.

This is a narrowing of the previously permissive native input contract.
Document the accepted subset and migration guidance before release.
