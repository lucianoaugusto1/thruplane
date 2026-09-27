# Community beta operations guide

Thruplane Community Beta is a self-hosted, bring-your-own-key gateway for
development teams and design partners. It is useful for OpenAI-compatible Chat
Completions, model aliases, direct provider adapters, streaming, retries,
fallbacks, local admission control, and operational diagnosis.

The beta designation covers the deterministic gateway contract. It does not
mean that every model, account, and region has passed a live provider test.
Check the [provider validation report](provider-validation.md) before making a
provider-specific compatibility claim.

## Beta acceptance gate

A revision qualifies as a beta candidate when all of these commands pass:

```sh
gofmt -l ./cmd ./internal ./bench ./tests
go test ./...
go test -race ./...
go vet ./...
go build ./cmd/thruplane
go test -run '^$' -bench . -benchtime=1x -benchmem ./bench/performance
```

The first command must produce no output. CI runs formatting, race-enabled
tests, vet, and build. The performance workflow separately compares repeated
base and candidate runs on the same runner.

## Build and identify the candidate

For a local development build, run:

```sh
go build -o thruplane ./cmd/thruplane
./thruplane -version
```

For a distributable beta build, inject the version, revision, and UTC build
date:

```sh
VERSION=v0.1.0-beta.1
REVISION="$(git rev-parse --short=12 HEAD)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
LDFLAGS="-X main.version=${VERSION}"
LDFLAGS="${LDFLAGS} -X main.revision=${REVISION}"
LDFLAGS="${LDFLAGS} -X main.buildDate=${BUILD_DATE}"
go build -trimpath \
  -ldflags "${LDFLAGS}" \
  -o thruplane ./cmd/thruplane
./thruplane -version
```

Development builds report `dev` and `unknown` values unless you inject build
metadata. The CI candidate build always injects metadata.

## Configure and preflight

1. Copy the example configuration.

   ```sh
   cp config.example.yaml config.yaml
   ```

2. Set one inbound gateway key and the provider credentials you use.

   ```sh
   export THRUPLANE_API_KEY="replace-with-a-long-random-value"
   export OPENAI_API_KEY="replace-with-a-scoped-provider-key"
   ```

   You can use the `local` alias with Ollama without an OpenAI key. Keep an
   inbound key for any listener that other machines can reach.

3. Enable the features needed by the beta tester.

   ```yaml
   server:
     playground:
       enabled: true
     metrics:
       enabled: true
   ```

4. Validate the exact file and environment before opening a listener.

   ```sh
   ./thruplane -check-config -config config.yaml
   ```

   A valid file prints `configuration valid: config.yaml` and exits with code
   zero. Invalid YAML, unknown fields, missing target references, invalid URLs,
   and conflicting shared-target limits fail before start.

## Start and probe

Start the gateway in the foreground for the first smoke test:

```sh
./thruplane -config config.yaml
```

From another terminal, verify liveness and route readiness:

```sh
curl --fail http://localhost:8080/healthz
curl --fail http://localhost:8080/readyz
```

`/healthz` reports process liveness. `/readyz` reports whether at least one
configured target is available according to the process-local circuit
breakers. A healthy process can be temporarily not ready when every target
circuit is open.

If you enabled metrics, scrape the public endpoint:

```sh
curl --fail http://localhost:8080/metrics
```

The endpoint includes bounded HTTP route/status series, handler latency,
in-flight requests, final route selection, per-request attempts and fallbacks,
aggregate target state, and build identity. It does not include prompt or
response content, request IDs, file bytes, client identity, provider
credentials, or authorization headers.

The endpoint is intentionally unauthenticated for Prometheus-style scraping,
like the health probes. Restrict the listener or `/metrics` path at your
network boundary. Configured provider names and model IDs appear as labels.

## Run the API smoke test

List the configured aliases:

```sh
curl --fail http://localhost:8080/v1/models \
  -H "Authorization: Bearer ${THRUPLANE_API_KEY}"
```

Send a buffered request. Replace `local` with the alias under test:

```sh
curl --fail http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ${THRUPLANE_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local",
    "messages": [{"role": "user", "content": "Reply with: beta-ok"}]
  }'
```

Send a streaming request and confirm that output arrives incrementally:

```sh
curl --fail --no-buffer http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ${THRUPLANE_API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "local",
    "messages": [{"role": "user", "content": "Count from one to three."}],
    "stream": true
  }'
```

Inspect these response headers during diagnosis:

- `X-Request-Id`
- `X-Thruplane-Provider`
- `X-Thruplane-Model`
- `X-Thruplane-Attempts`
- `X-Thruplane-Fallbacks`

Use the [developer playground](playground.md) to exercise tools, images, PDFs,
and supported audio inputs without building a test client. The browser does
not execute model-requested tools.

## Roll out and roll back

Use an immutable binary and configuration pair for each beta revision.

1. Keep the current binary and config under their existing revision name.
2. Validate the new config with the new binary.
3. Stop accepting new traffic at the load balancer or process supervisor.
4. Send `SIGTERM` and allow `server.shutdown_timeout` for active requests.
5. Start the new binary, then require `/healthz` and `/readyz` to pass.
6. Run the buffered and streaming smoke requests.
7. Restore the previous binary and config pair if probes or requests fail.

Thruplane does not hot reload configuration in this beta. A process restart is
the configuration transaction boundary. Circuit-breaker, rate-limit, and
metrics state are process-local and reset on restart.

## Current beta boundaries

- Chat Completions is the only public inference operation. Responses,
  embeddings, image generation, audio generation, and batch APIs are not
  implemented.
- One inbound API key protects `/v1` routes. Multi-key projects, scopes,
  expiry, tenant quotas, and persistent usage belong to later milestones.
- Metrics are process-local. OpenTelemetry traces and distributed state are
  not implemented.
- Bedrock supports buffered Converse responses, not streaming.
- Vertex and Bedrock credentials are configured explicitly; automatic refresh
  remains planned.
- Compatible adapters preserve unknown fields. Native adapters reject fields
  they cannot translate.
- Live provider behavior varies by model, account, region, and API version.
  Run the opt-in provider smoke suite with scoped, capped credentials before
  enabling a capability in production.

## Report beta findings

Include the Thruplane version output, provider type, upstream model ID, region
when relevant, request ID, HTTP status, and a sanitized request shape. Remove
prompts, response content, files, API keys, access tokens, account IDs, and
authorization headers.

Report suspected vulnerabilities through the private process in
[`SECURITY.md`](../SECURITY.md), not a public issue.
