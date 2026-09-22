# Native multimodal chat design

**Spec**: `.specs/features/native-multimodal/spec.md`
**Status**: Approved by the user's implementation request

## Approach

The existing native request decoder retains the original message content as
`json.RawMessage`. A small parser in `internal/provider` will turn each content
array into ordered text, image, PDF, or audio parts. It will validate media
types and base64 without downloading remote content. Each native adapter will
map those parts directly to its provider's request shape. Existing tool-call
and text-only flows remain intact.

## Provider mapping

| Public content part | Anthropic Messages | Gemini and Vertex | Bedrock Converse |
| --- | --- | --- | --- |
| `text` | `text` block | `text` part | `text` block |
| `image_url` data URI | `image` base64 source | `inlineData` | `image.source.bytes` |
| `image_url` HTTPS URL | `image` URL source | Reject | Reject |
| `file.file_data` PDF | `document` base64 source | PDF `inlineData` | `document.source.bytes` |
| `input_audio` WAV/MP3 | Reject | Audio `inlineData` | Reject |

The gateway's default body limit bounds inline payloads. Adapter-specific
limits remain enforced by providers; catalog limits are reference data.
OpenAI-compatible adapters retain their existing passthrough behavior.

## Interfaces

- `parseNativeContent(raw json.RawMessage) ([]nativeContentPart, error)`:
  preserve order and validate supported public content shapes.
- Adapter-specific conversion functions: consume parsed parts, return native
  request blocks or a `RequestError` before network I/O.
- `EffectiveCapabilities`: intersect model metadata with implemented adapter
  modalities.

## Error behavior

Malformed or unsupported media returns `400` with `unsupported_content` or
`invalid_media`. Media never falls back to text. A known target whose adapter
lacks the required modality is skipped by capability-aware routing.

## Sources

- [OpenAI Chat content parts](https://developers.openai.com/api/reference/cli/resources/chat)
- [Anthropic vision](https://platform.claude.com/docs/en/build-with-claude/vision)
- [Anthropic PDF support](https://platform.claude.com/docs/en/build-with-claude/pdf-support)
- [Gemini file inputs](https://ai.google.dev/gemini-api/docs/generate-content/file-input-methods)
- [Gemini audio](https://ai.google.dev/gemini-api/docs/generate-content/audio)
- [Bedrock content blocks](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ContentBlock.html)
