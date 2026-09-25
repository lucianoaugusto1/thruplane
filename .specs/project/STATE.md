# Project state

**Updated:** September 25, 2026
**Current work:** The gateway orchestration, native provider contract, and
configuration package have focused internal boundaries. Credentialed live
validation and performance baselines are next.

## Decisions

- NexoRoute has three product fronts: the open Gateway data plane, paid
  NexoRoute Inference, and NexoRoute Intelligence.
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
- Go 1.26 is the tested baseline; the implementation uses only standard HTTP
  features available since Go 1.22.
- The container uses a multi-stage Go build and a non-root distroless runtime.
- Provider/model targets share process-local request-rate, concurrency, queue,
  and adaptive cooldown state across aliases. Limits remain isolated between
  configured provider names.
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

## Blockers

- No blocker to local development. Live provider conformance requires scoped,
  capped test credentials and selected model/region pairs. Public release
  requires legal, brand, and security-contact decisions.

## Next implementation slice

- Select test models and regions, then run the opt-in native-provider smoke
  suite with scoped credentials and a spending cap.
- Complete the versioned normalized response compatibility matrix and add
  equivalent fixture evidence for OpenAI-compatible adapters.
- Establish end-to-end gateway overhead and failure baselines before claiming
  performance or implementing adaptive routing.
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
