# Playground provider credentials validation

**Status:** Passed
**Date:** September 26, 2026

## Automated coverage

| Requirement | Evidence | Result |
| --- | --- | --- |
| PCR-01 | Disabled route, config dependencies, and public feature flag tests | Passed |
| PCR-02 | Provider selector contract and server provider validation | Passed |
| PCR-03 | No browser storage plus response, log, metric, and export redaction tests | Passed |
| PCR-04 | Missing inbound bearer token returns `401` | Passed |
| PCR-05 | Exact normalized allowlist and pre-network rejection tests | Passed |
| PCR-06 | One request-scoped target with retries and breaker disabled | Passed |
| PCR-07 | UI revision invalidation and small buffered test request contract | Passed |
| PCR-08 | Existing normalized gateway path for buffered and SSE requests | Passed |
| PCR-09 | Fixed HTTP metric route and no dynamic route-selection labels | Passed |
| PCR-10 | Environment placeholders for provider and gateway secrets | Passed |
| PCR-11 | Strict envelope and stable OpenAI-shaped errors | Passed |
| PCR-12 | Final quality and browser gates | Passed |

## Security observations

- Tests use synthetic secrets and loopback upstreams only.
- A submitted base URL outside the operator allowlist is rejected before a
  network call.
- Upstream error content is buffered and scrubbed against every submitted
  provider secret before it reaches the browser.
- Successful and streaming responses preserve the existing relay path.
- Credential requests are deliberately excluded from dynamic provider/model
  metrics because the values are user controlled.

## Browser smoke

An in-app browser used a local OpenAI-compatible synthetic upstream. In a
1440×900 viewport, the playground accepted the inbound gateway key, switched
to provider credential mode, tested a synthetic provider key, enabled chat,
streamed a response, and displayed provider, physical model, status, TTFT,
total latency, attempts, fallbacks, usage, and request identifiers.

The copied credential-mode command contained `${THRUPLANE_API_KEY}` and
`${PROVIDER_API_KEY}` and contained neither entered synthetic key. Provider
selection also showed Bedrock region, access-key, secret-key, and session-token
fields while hiding the generic API-key field.

At 390×844, browser measurement reported `scrollWidth` 388 for `innerWidth`
390. The first mobile run exposed an invisible radio input inheriting the text
field width and causing horizontal overflow; the final CSS excludes radios
from that selector, and the repeated measurement passed.

## Final gates

```text
node --check internal/httpapi/playground/app.js
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/thruplane
git diff --check
```

All gates passed. No live provider, public network request, or real credential
was used.
