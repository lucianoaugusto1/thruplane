# NexoRoute product strategy

**Status:** Working direction
**Updated:** September 19, 2026

This document records the current product thesis and the capabilities proposed
for Community, Pro, and Enterprise. Only the Community gateway described in the
README exists today. Every other capability remains planned until implemented
and verified.

## Product thesis

NexoRoute must become more than a Go implementation of an existing LLM
gateway. Its strongest position is:

> The gateway that finds the lowest-cost model that meets your quality,
> latency, privacy, and availability goals, and proves the result before
> rollout.

The short product promise is:

> NexoRoute makes every model replaceable, measurable, and governed.

NexoRoute combines an open, portable Go data plane with optional commercial
operations and governance. Teams keep control of their traffic and
configuration while paid editions reduce the work required to operate the
gateway across teams and regulated environments.

## Market baseline

Provider abstraction alone is not a durable differentiator. Current gateways
already offer many of the following capabilities:

- LiteLLM documents virtual keys, multi-tenant cost tracking, budgets,
  authentication, routing, and an administration dashboard.
- Cloudflare AI Gateway documents caching, spend limits, rate limiting,
  dynamic routing, guardrails, DLP, analytics, and logging.
- Portkey documents load balancing, conditional routing, fallbacks,
  observability, and an emerging agent gateway.

References:

- [LiteLLM documentation](https://docs.litellm.ai/)
- [Cloudflare AI Gateway features](https://developers.cloudflare.com/ai-gateway/features/)
- [Portkey conditional routing](https://docs1.portkey.ai/docs/product/ai-gateway/conditional-routing)
- [Portkey Agent Gateway announcement](https://portkey.ai/blog/agent-gateway/)

NexoRoute needs these table-stakes features, but it must differentiate through
safe model change, outcome-based routing, private deployment, and agent
governance.

## Packaging principles

- Keep the production data plane, protocols, provider adapters, baseline
  security, routing, and observability open source.
- Do not put minimum production security behind a paid plan.
- Sell operational leverage, shared state, automation, governance, assurance,
  and support.
- Keep configuration and telemetry portable.
- Make commercial automation opt-in, explainable, and reversible.
- Label roadmap items as planned until they ship with tests and documentation.

## Community direction

Community drives adoption and must remain suitable for real self-hosted
workloads. Planned capabilities include:

- Chat Completions, Responses, embeddings, and streaming.
- OpenAI, Anthropic, Gemini, Azure OpenAI, Bedrock, Ollama, and custom
  OpenAI-compatible endpoints.
- Normalized streaming, tool calls, and structured outputs.
- Retries, fallbacks, timeouts, circuit breakers, health checks, and weighted
  load balancing.
- Multiple local API keys with simple scopes and expiration.
- In-memory rate limits and quotas.
- Per-request token and cost estimates.
- Prometheus metrics and OpenTelemetry traces.
- Validated YAML or GitOps configuration with hot reload.
- Guardrail webhooks and deterministic local rules.
- A Docker image, Helm chart, and portable single binary.
- Compatibility tests that detect provider and protocol drift.

Community contains the core policy schema and static routing engine. A user
must not need a paid subscription to run a secure gateway for one team.

## Pro direction

Pro removes the operational work that growing teams would otherwise build
themselves. Candidate capabilities include:

- A web control plane with projects, environments, and team workspaces.
- Managed virtual keys with rotation, expiration, and scoped permissions.
- Persistent request metadata with configurable prompt and response redaction.
- Cost, token, latency, cache, reliability, and provider-health dashboards.
- Budgets, distributed quotas, spend limits, alerts, and chargeback reports.
- Distributed rate limiting and shared state through PostgreSQL and Redis.
- Exact and semantic caching with policy-controlled eligibility.
- Managed configuration, validation, versioning, and rollback.
- Shadow traffic, replay, canary rollout, A/B tests, and regression reports.
- SLO-based routing recommendations and opt-in automation.
- A privacy-preserving hosted control plane.
- Priority support.

Pro sells faster, safer operations rather than access to basic gateway
protocols.

## Enterprise direction

Enterprise adds organization-wide governance, security, scale, and contractual
assurance. Candidate capabilities include:

- SAML or OIDC single sign-on, SCIM, and granular RBAC.
- Organization, business-unit, team, project, and environment hierarchy.
- Policy as code with approvals, ownership, and separation of duties.
- Immutable audit logs and SIEM export.
- Vault, KMS, and cloud secret-manager integrations.
- Private networking, egress controls, and customer-managed encryption keys.
- Data classification, DLP, PII tokenization, and zero-retention policies.
- Geographic routing and data-residency enforcement.
- High-availability, multi-region, disaster-recovery, and backup options.
- BYOC, on-premises, and air-gapped deployment patterns.
- Contracted support, security response, upgrade planning, and SLAs.

Enterprise claims must follow working implementations, operational evidence,
and support readiness.

## Strategic product bets

### NexoRoute Autopilot

Autopilot routes requests according to outcomes instead of a fixed provider
list. A policy can express constraints such as:

```yaml
policy:
  max_cost_per_request: 0.02
  latency_p95: 1500ms
  minimum_quality: 0.87
  data_classification: confidential
  allowed_regions: [br, us]
```

The router combines model price, observed latency, availability, evaluated
quality, capabilities, and data restrictions. Each decision produces a routing
receipt that explains the selected route and any fallback.

Community receives the policy schema and deterministic local rules. Pro adds
recommendations, learned performance profiles, and opt-in automation.
Enterprise adds organization-wide constraints, regional enforcement, and
approval workflows.

### Flight Recorder

Flight Recorder makes model changes measurable before they affect all users.
It supports:

- Sanitized traffic capture and replay.
- Shadow requests that do not change the user response.
- Cost, latency, schema, tool-call, and quality comparisons.
- Canary releases with explicit promotion criteria.
- Automatic rollback when an approved SLO regresses.
- A clear go or no-go report for model and provider changes.

Flight Recorder is the recommended first commercial wedge because it solves an
immediate production risk and creates the evidence required by Autopilot.

### Cost Autopilot

Cost Autopilot uses model cascades to reduce spend while maintaining a quality
floor. It can try an economical model first and escalate only when deterministic
validation, confidence signals, or sampled evaluation indicates that a stronger
model is needed.

The product must report measured savings together with quality and latency
impact. It must never optimize cost without an explicit quality constraint.

### Agent Firewall

Agent Firewall extends governance from model calls to agent and MCP tool calls.
The direction includes:

- Agent and MCP server registration.
- Tool allowlists and argument policies.
- Per-run cost, token, step, and duration limits.
- Prompt-injection and data-exfiltration checks before tool execution.
- Human approval for sensitive actions.
- One trace across model calls, tool calls, retries, and fallbacks.

This capability can become an Enterprise differentiator after the LLM gateway
data plane and tracing model are stable.

### Privacy-preserving control plane

The NexoRoute data plane runs inside the customer's infrastructure. A hosted
control plane can receive configuration, aggregate metrics, sanitized audit
events, costs, and health signals without receiving prompts or responses by
default.

This split offers SaaS-like operations without forcing sensitive AI traffic
through NexoRoute infrastructure.

## Delivery sequence

### v0.2: Community production foundation

- Add priority providers and the Responses and embeddings APIs.
- Add circuit breakers, weighted routing, virtual keys, and local limits.
- Add Prometheus, OpenTelemetry, cost estimates, and hot reload.
- Add release automation, compatibility tests, and a Helm chart.

### v0.3: Pro operations beta

- Add PostgreSQL and Redis state.
- Add projects, keys, usage, costs, budgets, quotas, and alerts.
- Add the first web console and hosted control-plane boundary.

### v0.4: Safe model rollout

- Add sanitized capture, replay, shadow traffic, and canary rollout.
- Add regression comparisons and explicit promotion criteria.
- Ship Flight Recorder as the first differentiated Pro workflow.

### v0.5: Adaptive routing

- Add SLO policies and routing receipts.
- Start with recommendations and human-approved policy changes.
- Add opt-in Autopilot only after replay and evaluation prove its decisions.

### Later: governed agents

- Add agent and MCP traces, registries, budgets, and tool policies.
- Add Enterprise Agent Firewall workflows after customer discovery.

## Product safety constraints

- Do not run an LLM judge synchronously on every production request by default.
- Base quality profiles on representative evaluations and sampled monitoring.
- Require explicit opt-in before the gateway changes a production route.
- Redact sensitive request data before persistence or replay.
- Treat provider costs as estimates unless reconciled with provider billing.
- Preserve a deterministic manual route and immediate rollback path.
- Explain why each automated route or policy action occurred.

## Discovery questions

Validate these questions before fixing packaging or prices:

- Which model change caused the most recent production incident?
- How does the team decide whether a cheaper model is good enough?
- Who owns AI spend, provider keys, and routing policy?
- Which request data may leave the customer's network or region?
- Which audit evidence does security or compliance require?
- Would the team pay first for cost control, safe rollout, or governance?

Track activation, gateways in production, weekly routed requests, retained
projects, prevented regressions, measured savings, and time to approve a model
change. These outcomes matter more than the number of supported providers.
