# Project state

**Updated:** September 22, 2026
**Current work:** Planning the next delivery sequence before implementing more
gateway features.

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
- Request bodies are minimally inspected so unknown fields pass through.
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
- Go 1.26 is the tested baseline; the implementation uses only standard HTTP
  features available since Go 1.22.
- The container uses a multi-stage Go build and a non-root distroless runtime.

## Blockers

- No blocker to local development. Live provider conformance requires scoped,
  capped test credentials and selected model/region pairs. Public release
  requires legal, brand, and security-contact decisions.

## Next implementation slice

- Version the Chat Completions compatibility contract for native versus
  passthrough adapters.
- Reject untranslated native fields even for unknown catalog models.
- Build deterministic provider fixtures, then opt-in live smoke tests.
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

## Deferred ideas

- NexoRoute Inference aliases and shared Pro compute credits
- Enterprise reserved, dedicated, private, and BYOC inference
- Semantic and selective LLM routing after sufficient evaluation data exists
- Cost Autopilot with model cascades and an explicit quality floor
- Agent and MCP governance with tool policies and human approval
- Provider compatibility monitoring and drift alerts
- Bedrock binary event-stream decoding and normalization
- Native media output normalization and provider-hosted tool adapters
- Pro implementation: cost tracking, budgets, rate limiting, analytics, and
  managed operations
- Enterprise implementation: SSO, SCIM, RBAC, audit exports, policy controls,
  high availability, and support workflows
- Responses API and embeddings
- Hot reload and distributed configuration
- Full observability stack integration

## Preferences

- Keep dependencies minimal and code idiomatic.
