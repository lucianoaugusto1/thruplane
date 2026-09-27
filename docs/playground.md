# Developer playground

The Thruplane Developer Playground lets you exercise the gateway's public API
from a browser. It uses the same model-discovery and Chat Completions routes as
an application, so it is useful for local evaluation and beta onboarding.

The playground is part of the Community binary. It is disabled by default and
doesn't add a database, a frontend build step, or an external service.

## Enable the playground

1. Enable the interface in your gateway configuration.

   ```yaml
   server:
     address: ":8080"
     api_key: "${THRUPLANE_API_KEY}"
     playground:
       enabled: true
   ```

2. Start Thruplane.

   ```sh
   go run ./cmd/thruplane -config config.yaml
   ```

3. Open `http://localhost:8080/playground/`.

4. If `server.api_key` is set, enter the Thruplane API key and select
   **Connect**. This is the inbound gateway key, not a provider credential.

When `server.playground.enabled` is false or omitted, Thruplane returns `404`
for playground paths. Enabling the static page doesn't bypass authentication
on `/v1/models` or `/v1/chat/completions`.

## Test a provider credential

Credential testing is a separate opt-in for onboarding. It lets you validate a
provider key and physical model before adding them to the gateway's durable
YAML configuration.

```yaml
server:
  api_key: "${THRUPLANE_API_KEY}"
  playground:
    enabled: true
    credential_testing:
      enabled: true
      allowed_base_urls:
        - http://localhost:11434
        - https://models.example.com
```

Both `playground.enabled` and `credential_testing.enabled` must be true, and
`server.api_key` must be non-empty. Restart Thruplane after changing the file.
The page then shows **Request source** with these choices:

- **Configured aliases** uses the normal `/v1` routes and credentials already
  present in server configuration.
- **Provider credential** constructs a transient provider setup for OpenAI,
  Anthropic, Gemini, Vertex AI, Bedrock, Azure OpenAI, Ollama,
  OpenAI-compatible APIs, Thruplane Inference, or xAI/Grok.

To use provider credential mode:

1. Enter the inbound Thruplane API key at the top of the page.
2. Select **Provider credential** and the provider adapter.
3. Enter its physical model ID and the fields shown for that provider.
4. Leave **Base URL** empty to use a built-in official endpoint. Azure OpenAI,
   OpenAI-compatible APIs, and Thruplane Inference require a base URL.
5. Select **Test connection**. This sends a real buffered request with an
   eight-token output limit and may create a provider charge.
6. After the status reads **Connection verified**, use text, tools, streaming,
   images, PDFs, or audio through the normal composer.

Any base URL typed in the page must exactly match a normalized
`allowed_base_urls` entry. This includes official URLs entered explicitly.
Leaving an official provider URL blank uses the adapter's built-in default and
doesn't require an allowlist entry. The allowlist prevents this developer
endpoint from becoming an arbitrary network proxy.

Changing any provider field invalidates the successful test. Thruplane uses
one upstream attempt, no fallback, and no circuit breaker for each transient
request, which avoids silent duplicate billable calls. The request-scoped
client is intentionally isolated from configured routing state.

## Send a request

1. Select a public model alias.
2. Optional: set a system prompt, temperature, and maximum output tokens.
3. Optional: enable **Stream response** to inspect SSE delivery and time to
   first token.
4. Enter a message and select **Send request**. You can also use
   <kbd>Command</kbd>+<kbd>Enter</kbd> on macOS or
   <kbd>Control</kbd>+<kbd>Enter</kbd> on other platforms.
5. Select **Stop** to cancel an active browser request.

The browser requests model aliases from `GET /v1/models` and target details
from `GET /v1/models/{model}`. The model card shows the effective capabilities
reported by the gateway when the target matches the embedded catalog.

## Test tools

Expand **Function tools** and enter an OpenAI-compatible tools array. The
playground validates that the top-level value is an array and includes it in
the Chat Completions request.

```json
[
  {
    "type": "function",
    "function": {
      "name": "get_weather",
      "description": "Get weather for a city.",
      "parameters": {
        "type": "object",
        "properties": {
          "city": { "type": "string" }
        },
        "required": ["city"]
      }
    }
  }
]
```

The playground displays tool calls returned by a model. It never executes a
tool. Add a tool result through application code when you need to test a full
tool loop.

## Attach media

Select **Attach** to add supported files to the next user message.

| Input | Browser encoding | Supported file types |
| --- | --- | --- |
| Image | `image_url` data URL | PNG, JPEG, GIF, and WebP |
| Document | `file_data` data URL | PDF |
| Audio | `input_audio` base64 | WAV and MP3 |

The browser limits each file to 4 MiB before base64 encoding. Base64 increases
the request size, so configure `server.max_body_bytes` above the encoded total.
The default gateway limit is 1 MiB.

Support still depends on the selected model and provider adapter. Review
`effective_capabilities` in the model card and the
[provider adapter guide](providers.md) before treating one successful model as
proof that every target accepts the same input.

## Inspect routing

The **Route Inspector** displays the completed request's:

- HTTP status;
- first-token time and total browser-observed latency;
- selected provider and physical model;
- upstream attempt and fallback counts;
- token usage, when the upstream response includes it; and
- gateway and upstream request identifiers, when available.

Thruplane exposes route metadata through `X-Thruplane-Provider`,
`X-Thruplane-Model`, `X-Thruplane-Attempts`, and
`X-Thruplane-Fallbacks`. These headers don't contain prompts or credentials.

Expand **Request JSON** and **Latest raw response** to inspect the wire shape.
Select **Copy curl** to copy a request that references
`${THRUPLANE_API_KEY}`. In provider credential mode it also substitutes
`${PROVIDER_API_KEY}`, `${GOOGLE_ACCESS_TOKEN}`, or the relevant `${AWS_*}`
variables. The copied command never includes a key entered in the page.

## Privacy and security

The playground has the following boundaries:

- It keeps the gateway key, messages, files, and conversation history only in
  the current page's JavaScript memory.
- When credential testing is enabled, it also keeps provider credentials only
  in current-tab memory and the active request. The server constructs a
  request-scoped client and never writes those values to YAML or a database.
- It doesn't use cookies, browser storage, analytics, external fonts, or
  third-party assets.
- Configured alias credentials remain server-side and are never sent to the
  browser. Only credentials typed by the user enter the transient test flow.
- It doesn't store server-side playground history.
- It doesn't place credential values in access logs, metrics, the Route
  Inspector, error responses, or copied commands. Upstream error bodies are
  defensively redacted against the submitted secret values.
- It serves a restrictive Content Security Policy and disables framing,
  MIME-type sniffing, caching, and referrer forwarding.

The page itself is public when enabled. Protect the gateway with an inbound API
key and your normal network controls. Use TLS at the deployment edge before
entering any key through a remote browser. Do not enable credential testing on
an untrusted network, and prefer scoped, capped test credentials.

## Current limits

- The playground targets Chat Completions. It doesn't expose Responses,
  embeddings, image-generation, audio-output, or batch APIs.
- Tool calls are visible but aren't executed.
- Files remain inline in the request. There is no separate upload service.
- Conversation history disappears when the page reloads.
- Timings are browser observations, not durable traces or benchmark results.
- Route Inspector describes the current response and isn't an audit log.
- The interface has no tenant, project, budget, or policy administration.
- Credential testing is not a credential vault and doesn't implement OAuth,
  cloud identity discovery, token refresh, or connection pooling across
  requests.

Use the [performance testing guide](performance.md) for repeatable latency,
throughput, allocation, connection-reuse, and failure measurements.
