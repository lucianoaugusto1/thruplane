# Thruplane editions

Thruplane uses an open-core model. Community provides the self-hosted gateway
and is currently in beta. Planned paid editions add organizational workflows,
governance, and assurance around that open core.

Only Community exists today. Pro and Enterprise describe product direction,
not generally available features or contractual commitments.

## Edition principles

- Keep protocols, provider adapters, core routing, retries, streaming, health,
  and baseline observability open source.
- Let Community users self-host without request, model, or provider limits.
- Sell operational leverage and organizational controls, not basic access.
- Keep configuration portable and avoid proprietary application contracts.
- Publish clear compatibility and migration information before paid launches.

## Capability direction

| Capability | Community | Pro, planned | Enterprise, planned |
| --- | --- | --- | --- |
| OpenAI-compatible API | Included | Included | Included |
| OpenAI and Ollama adapters | Included | Included | Included |
| Streaming, retries, and fallback | Included | Included | Included |
| YAML configuration | Included | Included | Included |
| Basic bearer authentication | Included | Included | Included |
| Structured logs and health checks | Included | Included | Included |
| Prometheus-compatible metrics | Included | Included | Included |
| OpenTelemetry traces | Planned open source | Included | Included |
| Multiple local keys and simple scopes | Planned open source | Managed | Managed |
| Local provider/model rate and concurrency limits | Included | Distributed | Distributed |
| Per-request token and cost estimates | Planned open source | Included | Included |
| Persistent usage and cost analytics | Not planned | Planned | Planned |
| Budgets, distributed quotas, and alerts | Not planned | Planned | Planned |
| Weighted and policy-based routing | Planned open source | SLO automation | Governed |
| Guardrail rules and webhooks | Planned open source | Managed catalog | Organization policy |
| Replay, shadow traffic, and canary rollout | Not planned | Planned | Planned |
| Thruplane-hosted inference | Not included | Shared credits | Reserved or dedicated |
| AutoRouter | Static local policy | Heuristic and semantic | Custom and governed |
| LLM routing layer | Not included | Selective and metered | Custom and governed |
| Web management console | Not planned | Planned | Planned |
| SSO with SAML or OIDC | Not planned | Not planned | Planned |
| RBAC and immutable audit logs | Not planned | Limited roles | Planned |
| Multi-tenancy and policy hierarchy | Not planned | Not planned | Planned |
| High-availability control plane | Self-managed | Optional | Planned |
| Secret-manager and KMS integrations | Environment variables | Limited | Planned |
| Support | Community | Priority | Contracted with SLA |

## Thruplane Community

Community targets individual developers, startups, and platform teams that can
operate their own gateway. Apache License 2.0 allows commercial use,
modification, and redistribution under its terms.

Community must remain useful without a commercial subscription. Future open
work includes additional providers, OpenTelemetry traces, multiple local keys,
token-aware local quotas, more endpoint families, and stronger routing
primitives.

## Thruplane Pro

Pro is intended for teams that want to operate Thruplane without building their
own management layer. Candidate capabilities include:

- Team workspaces and virtual API keys
- Usage and cost dashboards
- Budgets, quotas, alerts, and provider health views
- Managed configuration and safer rollout workflows
- Flight Recorder for replay, shadow traffic, canary rollout, and rollback
- SLO-based routing recommendations and opt-in Autopilot
- Advanced routing and policy templates
- A privacy-preserving hosted control plane
- Monthly compute credits for shared Thruplane Inference
- AutoRouter with heuristic, semantic, and selective LLM routing
- Priority support

The first Pro release must be informed by customer discovery. This repository
does not publish prices or release dates yet.

## Thruplane Enterprise

Enterprise is intended for organizations with governance, security, and
availability requirements. Candidate capabilities include:

- SAML or OIDC single sign-on and granular RBAC
- Immutable audit records and organization-wide policy controls
- Multi-tenant hierarchy and delegated administration
- High-availability control and data planes
- Private networking, secret-manager integrations, and deployment review
- Reserved, dedicated, BYOC, or private inference capacity
- Custom routing policies and routing models
- Contracted support, SLAs, upgrade planning, and security response

The longer product thesis, strategic bets, and proposed delivery sequence live
in [the product strategy](product-strategy.md).

## Commercial packaging

A practical launch sequence is:

1. Grow Community adoption and validate production workloads.
2. Interview operators about cost, access, and policy pain.
3. Launch Pro around the smallest repeated operational workflow.
4. Sell Enterprise only after security, support, and HA commitments are real.
5. Consider Thruplane Cloud after the self-hosted operational model is stable.

Describe paid-plan model usage as included inference backed by compute credits,
limits, and explicit overage. Do not market inference as unlimited or free.

Avoid pricing by feature count alone. Price paid editions around managed model
spend, team scale, support expectations, and deployment complexity after the
project has real usage data.
