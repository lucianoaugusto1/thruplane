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

## Three product fronts

NexoRoute develops as three connected products with separate responsibilities:

| Front | Product | Responsibility |
| --- | --- | --- |
| 1 | NexoRoute Gateway | Open data plane, protocols, BYOK, security, and resilience |
| 2 | NexoRoute Inference | Curated hosted open-weight models with paid-plan usage allowances |
| 3 | NexoRoute Intelligence | AutoRouter, Flight Recorder, evaluations, and optimization |

Gateway drives adoption and remains independently useful. Inference creates a
recurring usage business. Intelligence differentiates the platform through
measured routing decisions rather than provider aggregation alone.

Each front must remain independently observable and replaceable. A customer
can use Gateway with only bring-your-own-key providers, use NexoRoute Inference
without enabling automated routing, or enable Intelligence across both hosted
and external models.

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
- [Cloudflare Workers AI pricing](https://developers.cloudflare.com/workers-ai/platform/pricing/)
- [OpenRouter free model collection](https://openrouter.ai/collections/free-models)

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
- In-memory provider/model request and concurrency limits (implemented);
  token-aware and tenant quotas remain planned.
- Per-request token and cost estimates.
- Prometheus metrics (implemented) and OpenTelemetry traces (planned).
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
- Shared NexoRoute Inference usage included through monthly compute credits.
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
- Reserved or dedicated NexoRoute Inference capacity.
- Private and customer-specific hosted models.
- Contracted support, security response, upgrade planning, and SLAs.

Enterprise claims must follow working implementations, operational evidence,
and support readiness.

## NexoRoute Inference

NexoRoute Inference is the second product front. It provides curated hosted
open-weight models through the same API as customer-managed providers.

Commercial material must describe this benefit as **included inference** or
**included model usage**, not free or unlimited inference. NexoRoute still pays
for compute, and an unlimited promise would create unpredictable margins and an
abuse surface.

### Pro packaging

Pro uses a shared inference pool with:

- Monthly compute credits included in the subscription.
- Per-model request, concurrency, context, and output limits.
- Optional metered overage with an explicit customer opt-in.
- Fallback to the customer's BYOK providers.
- No dedicated-capacity guarantee.

Compute credits normalize different input, output, cache, and model costs. They
avoid presenting one token as economically equivalent across every model.

### Enterprise packaging

Enterprise adds deployment and capacity choices:

- Reserved or dedicated inference capacity.
- Private or customer-specific models.
- Customer VPC, BYOC, on-premises, or approved-region deployment.
- Negotiated limits, availability, throughput, and support commitments.
- Customer-managed keys and network controls.

Enterprise sells predictable capacity, privacy, and control rather than a
larger pool of nominally free tokens.

### Initial model aliases

Start with a small catalog based on stable capabilities:

- `nexoroute/fast` for chat, classification, and simple tasks.
- `nexoroute/smart` for higher-quality general work.
- `nexoroute/embed` for embeddings and semantic routing.
- `nexoroute/guard` for moderation, PII, and safety checks.
- `nexoroute/auto` as the virtual alias controlled by AutoRouter.

Aliases decouple the application contract from a physical model. NexoRoute can
change the serving model only within published compatibility, quality, and
change-management rules.

### Capacity strategy

Validate demand before operating a fixed GPU fleet:

1. Start with metered serverless inference providers behind NexoRoute.
2. Measure utilization, gross margin, latency, concurrency, and cache benefit.
3. Move stable workloads to reserved capacity when utilization justifies it.
4. Operate dedicated infrastructure only where it improves economics or meets
   customer isolation requirements.

The gateway preserves the model alias and policy contract while the underlying
capacity changes.

### Economic controls

- Set monthly compute-credit allowances instead of unlimited usage.
- Limit concurrency, context size, output size, and batch behavior by plan.
- Use prefix and response caching when policy allows it.
- Deduplicate equivalent in-flight requests when safe.
- Enforce internal budgets and a kill switch per workspace and model.
- Maintain a minimum gross-margin threshold for automated route selection.
- Require authorization before overage billing begins.

## Strategic product bets

### NexoRoute AutoRouter and Autopilot

AutoRouter is the decision engine in the third product front. Autopilot is the
opt-in control loop that applies AutoRouter recommendations automatically.
Flight Recorder supplies the evaluation and production evidence used by both.

AutoRouter routes requests according to outcomes instead of a fixed provider
list. A policy can express constraints and inference preferences such as:

```yaml
policy:
  max_cost_per_request: 0.02
  latency_p95: 1500ms
  minimum_quality: 0.87
  data_classification: confidential
  allowed_regions: [br, us]
  provider_preference:
    - nexoroute_included
    - customer_byok
  maximum_escalation_levels: 2
```

The decision pipeline has four layers:

1. **Hard constraints:** Filter by modality, tools, structured output, context,
   region, data classification, provider policy, budget, health, and capacity.
2. **Heuristics:** Score token count, code signals, tool count, schema
   complexity, language, priority, price, latency, errors, and fallback history.
3. **Semantic routing:** Classify task, domain, complexity, and similarity to
   workloads with known evaluation results.
4. **LLM routing:** Resolve only ambiguous or high-value decisions with a small
   structured-output routing model.

```text
Request
  -> hard constraints
  -> heuristic router when confidence is high
  -> semantic router when heuristics are uncertain
  -> LLM router only when the decision remains ambiguous
  -> selected model or escalation cascade
```

The router combines model price, observed latency, availability, evaluated
quality, capabilities, and data restrictions. Each decision produces a routing
receipt that records the selected route, confidence, policy inputs, escalation,
and fallback.

The LLM routing layer must not run on every request. It adds latency, cost, and
another failure dependency. NexoRoute must cache eligible routing decisions and
fall back to deterministic policy whenever the semantic or LLM layer fails.

Community receives the policy schema and deterministic local rules. Pro adds
heuristic and semantic routing, learned performance profiles, and opt-in
automation. Enterprise adds custom routing models, organization-wide
constraints, regional enforcement, and approval workflows.

The intended request lifecycle is:

1. Prefer an eligible NexoRoute model covered by included usage.
2. Validate the quality floor, budget, latency goal, and policy constraints.
3. Escalate to a stronger included or BYOK model only when necessary.
4. Record the decision and outcome through Flight Recorder.
5. Use evaluated outcomes to improve future recommendations.

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
- Add circuit breakers, weighted routing, virtual keys, and token-aware local
  quotas; provider/model request and concurrency limits are implemented.
- Prometheus metrics are implemented; add OpenTelemetry, cost estimates, and
  hot reload.
- Add release automation, compatibility tests, and a Helm chart.

### v0.3: Pro operations beta

- Add PostgreSQL and Redis state.
- Add projects, keys, usage, costs, budgets, quotas, and alerts.
- Add the first web console and hosted control-plane boundary.
- Pilot NexoRoute Inference with compute credits and a small alias catalog.

### v0.4: Safe model rollout

- Add sanitized capture, replay, shadow traffic, and canary rollout.
- Add regression comparisons and explicit promotion criteria.
- Ship Flight Recorder as the first differentiated Pro workflow.

### v0.5: Adaptive routing

- Add hard constraints, heuristic routing, and routing receipts.
- Add semantic routing after representative evaluation data exists.
- Use an LLM router only for ambiguous or high-value requests.
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
- Never advertise included inference as unlimited or cost-free infrastructure.

## Discovery questions

Validate these questions before fixing packaging or prices:

- Which model change caused the most recent production incident?
- How does the team decide whether a cheaper model is good enough?
- Who owns AI spend, provider keys, and routing policy?
- Which request data may leave the customer's network or region?
- Which audit evidence does security or compliance require?
- Would the team pay first for cost control, safe rollout, or governance?
- Which included model tasks create enough value without unacceptable subsidy?
- Does the customer prefer shared usage, reserved capacity, or BYOC?

Track activation, gateways in production, weekly routed requests, retained
projects, inference cost per active workspace, gross margin, escalation rate,
prevented regressions, measured savings, and time to approve a model change.
These outcomes matter more than the number of supported providers.
