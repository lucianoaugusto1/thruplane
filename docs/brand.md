# NexoRoute brand guide

## Brand status

NexoRoute is the project's working commercial brand. A preliminary web search
did not find another AI gateway using this name. It did find an unrelated
transportation business in France.

This check is not legal clearance. Before public launch, verify trademarks,
company names, domains, social handles, and package registries in every target
market. Replace the working brand if that review finds a material conflict.

## Positioning

**Category:** Open-source AI gateway

**Tagline:** The open control plane for AI traffic.

**One-line description:** NexoRoute routes, secures, observes, and governs model
calls through one OpenAI-compatible endpoint.

**Short pitch:** NexoRoute gives engineering teams one stable API for hosted and
local models. The open-source gateway handles credentials, aliases, streaming,
retries, and failover. Paid editions add team operations and enterprise
governance without changing application code.

## Audience

### Primary

- Platform engineers standardizing how applications access models
- AI product teams reducing provider lock-in and integration duplication
- Infrastructure teams operating local and hosted models together

### Commercial buyers

- Engineering leaders who need usage visibility and cost controls
- Security teams that need identity, policy, and auditability
- Enterprises that need high availability, support, and deployment assurance

## Product names

- **NexoRoute Community:** Apache-licensed, self-hosted open-source gateway
- **NexoRoute Pro:** Planned team operations and cost-control edition
- **NexoRoute Enterprise:** Planned governance, scale, security, and support
- **NexoRoute Cloud:** Reserved name for a future managed service

Use **NexoRoute** on first reference and in headings. Use `nexoroute` for the
binary, Go module, container image, and command examples.

## Messaging pillars

### Open by default

The protocol layer and core routing engine are open source. Teams can self-host,
inspect behavior, and leave without exporting from a proprietary format.

### Operationally serious

NexoRoute treats retries, cancellation, streaming, credentials, errors, and
startup validation as production concerns rather than integration details.

### Provider-neutral

Applications depend on one public contract. Providers, models, and fallback
order remain deployment choices.

### Clear commercial value

Paid editions sell collaboration, governance, assurance, and operational
leverage. They do not sell access to the open protocol or hold configuration
hostage.

## Voice

- Direct, technical, and calm
- Specific about what exists today
- Explicit when a capability is planned
- Confident without claiming complete provider compatibility
- Helpful to operators, not only application developers

Avoid superlatives such as "fastest," "complete," and "enterprise-grade"
unless current evidence supports the claim.

## Launch description

> NexoRoute is an open-source AI gateway for routing production model traffic
> through one OpenAI-compatible endpoint. Run it yourself today, then add team
> operations or enterprise governance when you need them.
