# Developer playground validation

**Status:** Passed
**Date:** September 25, 2026

## Coverage

| Requirement | Evidence | Result |
| --- | --- | --- |
| PLG-01 | Disabled `404`, enabled redirect, and allowlisted assets | Passed |
| PLG-02 | Existing `/v1` authentication remains enforced | Passed |
| PLG-03 | Browser loads aliases and target details from public routes | Passed |
| PLG-04 | Model, system, temperature, token, and stream controls | Passed |
| PLG-05 | Incremental SSE, first-token timing, and abort control | Passed |
| PLG-06 | Tool JSON validation and streamed tool-call assembly | Passed |
| PLG-07 | Image, PDF, WAV, and MP3 request-part encoding | Passed |
| PLG-08 | Safe route headers and Route Inspector values | Passed |
| PLG-09 | `curl` export uses an environment-variable placeholder | Passed |
| PLG-10 | Security headers and no browser persistence or remote assets | Passed |
| PLG-11 | Semantic contract and desktop/mobile browser checks | Passed |
| PLG-12 | JavaScript, test, race, vet, build, and diff gates | Passed |

## Browser smoke

The embedded server used a local OpenAI-compatible synthetic upstream. A
1440×900 browser completed an SSE request and displayed the physical provider,
model, HTTP status, attempt count, fallback count, first-token time, total
latency, usage, and raw response. A 390×844 check confirmed sequential panels
without horizontal overflow.

The smoke test found an invisible file input overlapping the **Send request**
button. The final CSS constrains that input to a clipped one-pixel target, and a
second browser run verified that the button receives the click and completes
the streamed request.

## Final gates

```text
node --check internal/httpapi/playground/app.js
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/thruplane
git diff --check
```

All final gates passed. The cancellation scenario also passed ten consecutive
isolated runs after a contention-sensitive failure while the normal, race,
vet, and build gates initially ran in parallel.

The browser smoke uses only loopback traffic and synthetic content. It doesn't
make a live provider request or validate provider credentials.
