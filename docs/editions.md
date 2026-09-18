# NexoRoute editions

NexoRoute uses an open-core model. Community provides the production-usable
gateway. Planned paid editions add organizational workflows, governance, and
assurance around that open core.

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
| Prometheus and OpenTelemetry integration | Planned open source | Included | Included |
| Virtual keys and team workspaces | Not planned | Planned | Planned |
| Usage and cost analytics | Not planned | Planned | Planned |
| Budgets, quotas, and alerting | Not planned | Planned | Planned |
| Advanced routing policies | Basic rules | Planned | Planned |
| Web management console | Not planned | Planned | Planned |
| SSO with SAML or OIDC | Not planned | Not planned | Planned |
| RBAC and immutable audit logs | Not planned | Limited roles | Planned |
| Multi-tenancy and policy hierarchy | Not planned | Not planned | Planned |
| High-availability control plane | Self-managed | Optional | Planned |
| Secret-manager and KMS integrations | Environment variables | Limited | Planned |
| Support | Community | Priority | Contracted with SLA |

## NexoRoute Community

Community targets individual developers, startups, and platform teams that can
operate their own gateway. Apache License 2.0 allows commercial use,
modification, and redistribution under its terms.

Community must remain useful without a commercial subscription. Future open
work includes additional providers, standard metrics and traces, more endpoint
families, and stronger routing primitives.

## NexoRoute Pro

Pro is intended for teams that want to operate NexoRoute without building their
own management layer. Candidate capabilities include:

- Team workspaces and virtual API keys
- Usage and cost dashboards
- Budgets, quotas, alerts, and provider health views
- Managed configuration and safer rollout workflows
- Advanced routing and policy templates
- Priority support

The first Pro release must be informed by customer discovery. This repository
does not publish prices or release dates yet.

## NexoRoute Enterprise

Enterprise is intended for organizations with governance, security, and
availability requirements. Candidate capabilities include:

- SAML or OIDC single sign-on and granular RBAC
- Immutable audit records and organization-wide policy controls
- Multi-tenant hierarchy and delegated administration
- High-availability control and data planes
- Private networking, secret-manager integrations, and deployment review
- Contracted support, SLAs, upgrade planning, and security response

## Commercial packaging

A practical launch sequence is:

1. Grow Community adoption and validate production workloads.
2. Interview operators about cost, access, and policy pain.
3. Launch Pro around the smallest repeated operational workflow.
4. Sell Enterprise only after security, support, and HA commitments are real.
5. Consider NexoRoute Cloud after the self-hosted operational model is stable.

Avoid pricing by feature count alone. Price paid editions around managed model
spend, team scale, support expectations, and deployment complexity after the
project has real usage data.
