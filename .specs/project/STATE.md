# Project state

**Updated:** September 18, 2026

## Decisions

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
- Standard Go tests and `httptest` provide unit and end-to-end coverage.
- Go 1.26 is the tested baseline; the implementation uses only standard HTTP
  features available since Go 1.22.
- The container uses a multi-stage Go build and a non-root distroless runtime.

## Blockers

- None.

## Launch prerequisites

- Legal and trademark review of the NexoRoute working name
- Domain, social-handle, and package-registry availability checks
- Public repository, release automation, and versioning policy
- Customer interviews that rank cost control, safe rollout, and governance pain

## Lessons learned

- Root binary ignore patterns must be anchored so they do not hide Go package
  directories with the same name.
- Streaming middleware must expose its wrapped writer through `Unwrap` so
  `http.ResponseController` can flush SSE chunks.

## Deferred ideas

- Cost Autopilot with model cascades and an explicit quality floor
- Agent and MCP governance with tool policies and human approval
- Provider compatibility monitoring and drift alerts
- Pro implementation: cost tracking, budgets, rate limiting, analytics, and
  managed operations
- Enterprise implementation: SSO, SCIM, RBAC, audit exports, policy controls,
  high availability, and support workflows
- Responses API and embeddings
- Hot reload and distributed configuration
- Full observability stack integration

## Preferences

- Keep dependencies minimal and code idiomatic.
