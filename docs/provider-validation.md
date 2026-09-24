# Provider validation

**Updated:** September 24, 2026  
**Live provider calls:** Not run

NexoRoute separates deterministic protocol evidence from live provider
evidence. A fixture proves that the adapter translates and normalizes a
recorded protocol shape. It doesn't prove that a provider currently accepts a
specific model, account, region, or capability.

## Evidence levels

| Level | Meaning |
| --- | --- |
| Fixture verified | A local `httptest` fixture asserts the request and normalized response without network access. |
| Live verified | The opt-in runner passed against the listed provider, model, region, date, and commit. |
| Unsupported | The adapter rejects the capability before network I/O. |
| Unverified | No current fixture or recorded live result supports the claim. |

## Native adapter snapshot

| Provider | Protocol | Buffered evidence | Media evidence | Tool evidence | Streaming evidence | Live evidence |
| --- | --- | --- | --- | --- | --- | --- |
| Anthropic | Messages | Text and upstream errors | Inline/URL image and inline PDF | Request, result, and normalized call | Text and tool-call SSE | Not run |
| Gemini | `generateContent` | Text and upstream errors | Inline image, PDF, and WAV/MP3 audio | Request, result, and normalized call | Text and tool-call SSE | Not run |
| Vertex AI | Vertex `generateContent` | Text and upstream errors | Inline image, PDF, and WAV/MP3 audio | Request and normalized call | Text and tool-call SSE | Not run |
| Amazon Bedrock | Converse | Text, SigV4 body hash, and upstream errors | Inline image and PDF | Request, result, and normalized call | Explicitly unsupported before network I/O | Not run |

Each provider file in `internal/provider/testdata/conformance/` covers buffered
text, media, function tools, a non-2xx provider response, and streaming or its
documented rejection. The harness asserts HTTP method, path, query,
authentication shape, translated JSON, normalized response fields, usage, and
finish reasons. Bedrock fixtures use a fixed clock and verify the
`X-Amz-Content-Sha256` value.

Run the deterministic suite with no credentials:

```sh
go test ./internal/provider -run TestNativeProviderConformanceFixtures -count=1
```

The normal project gate also runs these fixtures and never contacts a public
provider.

## Run opt-in live smoke tests

Live tests can generate provider charges. Use scoped credentials, provider
budgets, and models approved for testing. The runner makes one request per
selected scenario and requires an output limit from 1 to 128 tokens. It never
prints credentials, prompts, or response bodies.

Set the common variables:

```sh
export NEXOROUTE_LIVE_PROVIDER="anthropic"
export NEXOROUTE_LIVE_MODEL="your-approved-model-id"
export NEXOROUTE_LIVE_SCENARIOS="text,tools,image,pdf,stream"
export NEXOROUTE_LIVE_MAX_TOKENS="32"
export NEXOROUTE_LIVE_TIMEOUT="90s"
```

Set the provider-specific variables:

| Provider | Required variables | Optional variables |
| --- | --- | --- |
| Anthropic | `ANTHROPIC_API_KEY` | `NEXOROUTE_LIVE_BASE_URL` |
| Gemini | `GEMINI_API_KEY` | `NEXOROUTE_LIVE_BASE_URL` |
| Vertex AI | `GOOGLE_CLOUD_PROJECT`, `GOOGLE_CLOUD_LOCATION`, `GOOGLE_ACCESS_TOKEN` | `NEXOROUTE_LIVE_BASE_URL` |
| Bedrock | `AWS_REGION`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY` | `AWS_SESSION_TOKEN`, `NEXOROUTE_LIVE_BASE_URL` |

For Bedrock, use short-lived credentials obtained through an IAM role and set
`AWS_SESSION_TOKEN` when applicable. The current adapter receives credentials
from configuration; it doesn't implement the AWS default credential chain.

Run only the live package with the build tag:

```sh
go test -tags=live -count=1 \
  -run TestNativeProviderLive ./tests/provider-live
```

Available scenarios are:

| Scenario | Anthropic | Gemini | Vertex AI | Bedrock |
| --- | --- | --- | --- | --- |
| `text` | Yes | Yes | Yes | Yes |
| `tools` | Yes | Yes | Yes | Yes |
| `image` | Yes | Yes | Yes | Yes |
| `pdf` | Yes | Yes | Yes | Yes |
| `audio` | No | Yes | Yes | No |
| `stream` | Yes | Yes | Yes | No |

The runner rejects unsupported provider/scenario combinations before making a
request. The selected model can still reject a capability even when the
adapter supports its protocol shape.

## Record live evidence

Record each approved run in this table. Never infer success from a fixture.

| Date | Commit | Provider | Model | Region | Scenarios | Result |
| --- | --- | --- | --- | --- | --- | --- |
| Not run | — | — | — | — | — | Unverified |

Include the provider request ID in a private test report when available. Don't
record credentials, prompts, responses, or customer data.

## Remaining gaps

- Run the smoke suite with capped credentials and selected model/region pairs.
- Add fixtures for OpenAI-compatible adapters to the same evidence model.
- Version the normalized response compatibility matrix, including cache usage
  and provider-specific finish reasons.
- Implement and validate Bedrock `ConverseStream` before advertising Bedrock
  streaming.
- Add provider drift checks only after live test accounts and spending policy
  are established.

The fixture shapes follow the
[Anthropic Messages API](https://platform.claude.com/docs/en/api/messages/create),
[Gemini generateContent API](https://ai.google.dev/api/generate-content),
[Vertex AI REST reference](https://cloud.google.com/vertex-ai/generative-ai/docs/reference/rest/v1beta1/projects.locations.publishers.models),
and [Amazon Bedrock Converse API](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_Converse.html).
