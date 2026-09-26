# Project state

**Updated:** September 26, 2026
**Current work:** The Community beta includes an opt-in provider-credential
playground flow for onboarding. It uses request-scoped clients, exact base-URL
allowlisting, one upstream attempt, transient browser memory, and secret-safe
logs, metrics, errors, inspection, and export. Credentialed live provider
validation remains the next external assurance step.

## Decisions

- NexoRoute has three product fronts: the open Gateway data plane, paid
  NexoRoute Inference, and NexoRoute Intelligence.
- The Community binary includes an optional developer playground. It remains
  disabled by default and adds no frontend runtime dependency or privileged
  internal API.
- Playground API keys, prompts, files, and history stay in page memory. The
  optional credential-testing mode also keeps user-entered provider secrets in
  tab memory and the active request only; configured credentials remain
  server-side. The page never executes model-requested tools.
- Provider credential testing requires a gateway API key, is separately
  disabled by default, accepts custom base URLs only through an exact
  server-side allowlist, and never mutates durable gateway configuration.
- Ephemeral credential requests create an isolated one-target provider client
  with no retry, fallback, rate-limit state, or circuit state. Dynamic model
  IDs are excluded from route-selection metric labels.
- Safe response headers expose the selected provider and model, attempt count,
  and fallback count for request-level inspection.
- NexoRoute Inference is described as included usage backed by compute credits,
  limits, and explicit overage. It is never marketed as unlimited or free.
- Pro uses a shared inference pool with monthly credits and BYOK fallback.
  Enterprise can use reserved, dedicated, private, BYOC, or approved-region
  capacity.
- Start hosted inference through metered external capacity. Operate reserved or
  dedicated infrastructure only after utilization and margin justify it.
- AutoRouter is the decision engine; Autopilot is its opt-in automated mode;
  Flight Recorder provides evaluation and production evidence.
- AutoRouter evaluates hard constraints first, then heuristics, then semantic
  classification, and uses an LLM only for ambiguous or high-value decisions.
- NexoRoute's product thesis is to make models replaceable, measurable, and
  governed rather than compete only as a Go implementation of LiteLLM.
- The commercial boundary follows operational scale and governance, not minimum
  production safety. Basic multiple keys, local limits, metrics, and traces
  belong in Community.
- The first differentiated Pro workflow is Flight Recorder: sanitized replay,
  shadow traffic, canary rollout, comparisons, and rollback.
- The longer-term product identity is SLO-based routing across cost, latency,
  quality, availability, privacy, and region constraints.
- Automated routing must be opt-in, explainable, reversible, and backed by
  representative evaluations.
- The hosted control plane must avoid receiving prompts and responses by
  default; the Go data plane remains inside the customer's infrastructure.
- Agent Firewall is a later strategic direction after gateway tracing and
  customer discovery validate the need.
- NexoRoute is the working product brand and must complete legal, domain, and
  registry clearance before public commercial launch.
- The Community edition is licensed under Apache License 2.0 and remains useful
  for production self-hosting.
- Pro and Enterprise are planned commercial editions; their capabilities are
  roadmap statements, not currently available product claims.
- Pro focuses on team operations, cost controls, and managed convenience.
- Enterprise focuses on governance, security, scale, assurance, and support.
- The MVP is stateless and uses one YAML file as its source of truth.
- The public API is an OpenAI-compatible subset centered on chat completions.
- OpenAI-compatible request bodies retain unknown fields. Native adapters
  strictly reject fields they cannot translate, including for uncataloged
  models; a native contract error does not trigger cross-target fallback.
- Provider support begins with OpenAI-compatible HTTP APIs, specifically
  OpenAI and Ollama.
- Direct provider calls are the data-plane rule: customer traffic must not
  require an aggregation gateway or hosted control plane.
- The first adapter set covers OpenAI, Anthropic, Gemini, Vertex AI, Bedrock,
  Azure OpenAI, Ollama, configurable OpenAI-compatible APIs, NexoRoute
  Inference, and xAI. `grok` is an alias for xAI.
- Compatible adapters use response passthrough. Native adapters translate
  text, image, and PDF input; Gemini and Vertex also translate inline audio.
  Bedrock streaming and native media output remain follow-up work.
- Native adapters translate client-executed function definitions, tool choice,
  assistant tool calls, parallel tool results, and normalized buffered
  responses. Anthropic, Gemini, and Vertex also normalize streamed tool calls.
- Portable native tools intentionally exclude provider-hosted tools,
  `strict: true`, legacy function fields, and multimodal tool results.
- Standard Go tests and `httptest` provide unit and end-to-end coverage.
- External JSON fixtures now verify native adapter text, media, tools,
  streaming or explicit rejection, upstream errors, and response
  normalization. Live evidence remains unverified until credentialed runs are
  recorded.
- One shared OpenAI-compatible fixture verifies all six canonical compatible
  provider types against opaque buffered, SSE, and error responses. It also
  covers multimodal fields, tools, structured output, future fields, and the
  Ollama output-token rename.
- Go 1.26 is the tested baseline; the implementation uses only standard HTTP
  features available since Go 1.22.
- The container uses a multi-stage Go build and a non-root distroless runtime.
- Provider/model targets share process-local request-rate, concurrency, queue,
  and adaptive cooldown state across aliases. Limits remain isolated between
  configured provider names.
- Provider/model targets also share process-local circuit state across aliases.
  Retryable transport and server failures open a configured circuit; one
  half-open request probes recovery after the open duration.
- `/healthz` remains process liveness. `/readyz` reports only aggregate target
  counts and ignores provider rate-limit cooldown to avoid orchestrator churn.
- `/metrics` is opt-in and public for Prometheus-style scraping. It uses only
  bounded route/status and configured provider/model labels; it never receives
  prompts, responses, files, credentials, authorization headers, or raw
  unknown paths.
- `-check-config` validates strict YAML, the embedded catalog, and provider
  initialization without opening a listener. `-version` works without loading
  configuration.
- The Community beta is a deterministic self-hosted/BYOK contract. It does not
  imply live certification for every provider, model, account, or region.
- Retry behavior honors `Retry-After`, otherwise uses cancelable exponential
  backoff with jitter, and never shortens a provider hint to fit the target
  retry budget.
- Known provider quota, billing, and spend-limit errors skip same-target
  retries but can still use an independent fallback.
- The gateway retains a focused runtime settings snapshot and does not retain
  provider credentials or unrelated server configuration.
- Chat request planning, upstream execution, and HTTP transport remain in one
  cohesive package but live in separate files with explicit internal results.
- Native provider and configuration responsibilities remain in their existing
  packages to avoid exporting implementation details solely for subpackages.
- Local performance evidence uses the real gateway and adapter paths with
  synthetic `httptest` upstreams. Direct-versus-gateway deltas are local
  comparison data, not universal provider or production claims.
- Performance coverage includes text, tools, 48 KiB image/PDF/audio payloads,
  SSE, retry, fallback, permanent `429`, slow readers, and cancellation.
- Performance artifacts preserve raw reports from at least three runs. The
  default CI gate uses five-run medians, rejects incompatible environments,
  and compares a pull request with its base commit on the same runner.
- Latency and TTFT regressions must cross both percentage and absolute limits;
  this reduces false positives from small loopback measurements.

## Blockers

- No blocker to local development. Live provider conformance requires scoped,
  capped test credentials and selected model/region pairs. Public release
  requires legal, brand, and security-contact decisions.

## Next implementation slice

- Select test models and regions, then run the opt-in native-provider smoke
  suite with scoped credentials and a spending cap.
- Review early CI variance and tune performance thresholds only from retained
  base/candidate evidence.
- Add multiple scoped local API keys and OpenTelemetry only after beta feedback
  confirms the operator requirements.
- Use [the delivery plan](../../docs/next-steps.md) for dependencies, artifacts,
  and acceptance gates. The plan is proposed until prioritized with the user.

## Launch prerequisites

- Legal and trademark review of the NexoRoute working name
- Domain, social-handle, and package-registry availability checks
- Public repository, release automation, and versioning policy
- Customer interviews that rank cost control, safe rollout, and governance pain
- A unit-economic model for compute credits, overage, and capacity commitments

## Lessons learned

- Root binary ignore patterns must be anchored so they do not hide Go package
  directories with the same name.
- Streaming middleware must expose its wrapped writer through `Unwrap` so
  `http.ResponseController` can flush SSE chunks.
- Native `file.filename` was previously accepted but discarded; reject it
  explicitly until a portable translation exists.
- A concurrency permit must live until response EOF or Close; releasing at
  response headers would allow long SSE streams to bypass admission limits.
- In Go, splitting a cohesive internal package into subpackages can increase
  coupling by forcing private wire types to become exported. Focused files are
  the safer boundary until the contracts stabilize.
- Visually hidden file inputs need a selector at least as specific as the
  shared form-control selector; otherwise an invisible input can cover nearby
  controls even when its local rule sets a one-pixel size.

## Deferred ideas

- NexoRoute Inference aliases and shared Pro compute credits
- Enterprise reserved, dedicated, private, and BYOC inference
- Semantic and selective LLM routing after sufficient evaluation data exists
- Cost Autopilot with model cascades and an explicit quality floor
- Agent and MCP governance with tool policies and human approval
- Provider compatibility monitoring and drift alerts
- Bedrock binary event-stream decoding and normalization
- Native media output normalization and provider-hosted tool adapters
- Pro implementation: cost tracking, budgets, distributed tenant rate limits,
  analytics, and managed operations
- Enterprise implementation: SSO, SCIM, RBAC, audit exports, policy controls,
  high availability, and support workflows
- Responses API and embeddings
- Hot reload and distributed configuration
- Full observability stack integration

## Preferences

- Keep dependencies minimal and code idiomatic.
