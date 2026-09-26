# Playground provider credentials design

**Specification:** `spec.md`
**Status:** Approved

## Architecture

```text
browser playground
  |  gateway bearer key + ephemeral credential envelope
  v
POST /playground/api/credentials/chat/completions
  |
  +-- validate provider fields and explicit base URL allowlist
  +-- create one request-scoped provider client
  +-- create one request-scoped, one-target gateway
  +-- execute existing native adapter and response normalization
  v
official or explicitly allowed provider endpoint
```

The configured `/v1/chat/completions` path is unchanged. Credential mode is a
separate developer-only entry point whose handler adapts an ephemeral provider
description into the existing provider and gateway layers.

## Configuration

```yaml
server:
  playground:
    enabled: true
    credential_testing:
      enabled: true
      allowed_base_urls:
        - http://127.0.0.1:11434
        - https://company-model.example.com
```

The feature is disabled by zero value. Enablement requires both the playground
and a non-empty inbound gateway API key. Allowed URLs are normalized by
trimming whitespace and a trailing slash, then validated as absolute HTTP(S)
URLs without userinfo, query, or fragment.

Built-in official endpoints need no allowlist entry when the request omits
`base_url`. OpenAI-compatible, Azure OpenAI, and NexoRoute Inference require an
explicit allowed URL because they have no safe universal default. Ollama can
use its built-in loopback default.

## API contract

```json
{
  "provider": {
    "type": "openai",
    "base_url": "",
    "api_key": "ephemeral-secret"
  },
  "model": "physical-model-id",
  "request": {
    "model": "physical-model-id",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": false
  }
}
```

The handler uses strict JSON decoding, one document only, and the configured
request-size limit plus a small bounded allowance for the envelope. It validates
the physical model separately and overwrites `request.model` with an internal
alias before invoking the request-scoped gateway. Provider validation reuses
the configuration package instead of duplicating adapter rules.

The response is the existing normalized OpenAI-compatible response, including
streaming and safe `X-NexoRoute-*` headers. Errors use the same OpenAI shape.

## Execution policy

The ephemeral configuration contains exactly one provider and one target:

- provider name: `playground-credential`
- internal alias: `playground-credential`
- physical model: the submitted model
- catalog policy: allow unknown models
- retries: zero
- circuit breaker: disabled
- rate limit: disabled
- response-header timeout: inherited from the server configuration

Provider clients and transports exist only for the request lifetime. This
trades connection reuse for a smaller secret lifetime and clear isolation. The
feature is for onboarding and testing, not latency-sensitive production use.

## Authentication and SSRF boundary

The authentication middleware protects both `/v1/` and `/playground/api/`.
Static playground assets remain public when enabled so the page can render and
prompt for the inbound key.

Only an omitted upstream base URL may select a built-in default. Any submitted
URL must exactly equal a normalized allowlist item. This makes the endpoint
unsuitable as an arbitrary network proxy and allows local synthetic endpoints
only when the operator explicitly opts in.

## UI state

The configuration rail has two modes:

1. **Configured aliases** — the existing model discovery flow.
2. **Provider credential** — provider form, physical model, test status, and
   activation for conversation requests.

Credential fields use password inputs where appropriate and `autocomplete=off`.
No values are stored. A successful test records only an in-memory fingerprint
of the current non-exported form state; any input change clears readiness.

The request inspector continues to show only the normalized chat payload. It
never receives or renders the envelope. Copy-as-curl uses placeholders such as
`${PROVIDER_API_KEY}`, `${AWS_ACCESS_KEY_ID}`, and `${AWS_SECRET_ACCESS_KEY}`.

## Observability

The HTTP metric route is the fixed string
`/playground/api/credentials/chat/completions`. Route-selection metrics are
skipped for that endpoint because submitted provider types and physical model
IDs are user-controlled and could create unbounded labels. Access logs already
exclude headers and bodies and therefore remain secret-safe.

## Testing strategy

- Configuration unit tests cover defaults, cross-field requirements, and URL
  normalization/validation.
- Handler tests use local synthetic upstreams and fake keys to prove auth,
  allowlisting, provider auth/model forwarding, buffered output, streaming,
  cancellation, and redaction.
- Asset contract tests cover provider choices, field IDs, no browser storage,
  no embedded remote resources, and placeholder-only export.
- Browser smoke covers configured and credential modes at desktop and mobile
  widths without contacting a real provider.

## Security review checklist

- No persistence API or config mutation.
- No secret values in errors, logs, metrics, inspector, or clipboard output.
- No dynamic outbound URL without exact allowlist approval.
- No credential endpoint without an inbound gateway key.
- No retry/fallback duplicate billing in credential mode.
- Documentation requires TLS for non-loopback use.
