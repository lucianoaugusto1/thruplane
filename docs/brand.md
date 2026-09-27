# Thruplane brand guide

## Brand status

Thruplane is the project's commercial brand. A preliminary search completed on
September 27, 2026, found no exact-name AI gateway, software product, indexed
GitHub project, npm package, or PyPI package. The `thruplane.com` domain also
had no registration record at the time of the search.

This check is not legal clearance. Before a commercial launch, verify similar
trademarks, company names, domains, and social handles in every target market.
Domain and registry availability can change at any time.

## Positioning

**Category:** Open-source AI gateway

**Tagline:** The open control plane for AI traffic.

**One-line description:** Thruplane routes, secures, observes, and governs model
calls through one OpenAI-compatible endpoint.

**Short pitch:** Thruplane gives engineering teams one stable API for hosted and
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

- **Thruplane Community:** Apache-licensed, self-hosted open-source gateway
- **Thruplane Pro:** Planned team operations and cost-control edition
- **Thruplane Enterprise:** Planned governance, scale, security, and support
- **Thruplane Cloud:** Reserved name for a future managed service

Use **Thruplane** on first reference and in headings. Use `thruplane` for the
binary, Go module, container image, and command examples.

## Visual identity

The visual system combines a dark infrastructure foundation with two routing
signals. Flux cyan represents the control plane. Route mint represents traffic
that successfully passes through it.

| Token | Hex | Use |
| --- | --- | --- |
| Obsidian | `#070B12` | Primary background and icon tile |
| Graphite | `#0E1621` | Panels and navigation |
| Elevated | `#14202D` | Raised surfaces and controls |
| Steel | `#20303F` | Borders and dividers |
| Ice | `#F4F7FA` | Primary text |
| Mist | `#8FA0B3` | Secondary text |
| Flux cyan | `#35D8FF` | Primary actions, focus, and control-plane signals |
| Route mint | `#2EE6A6` | Active routes, success, and healthy state |
| Signal amber | `#F5BD4F` | Warnings and degraded state |
| Fault coral | `#FF6B78` | Errors and destructive actions |

Use cyan as the primary brand color. Reserve mint for movement, selection,
success, and live traffic. Do not use both colors decoratively when no routing
or state relationship exists.

### Icon

The [Thruplane mark](../assets/brand/thruplane-mark.svg) shows three traffic
paths meeting a tilted control plane. The highlighted center route passes
through the plane and changes from cyan to mint.

- Use the complete square mark at 20 pixels or larger.
- Keep clear space equal to one-eighth of the mark's width.
- Use the supplied dark tile on both light and dark backgrounds.
- Do not rotate the mark, change the route count, or add letters inside it.
- Use the product name beside the mark when the audience may not know it yet.

## Messaging pillars

### Open by default

The protocol layer and core routing engine are open source. Teams can self-host,
inspect behavior, and leave without exporting from a proprietary format.

### Operationally serious

Thruplane treats retries, cancellation, streaming, credentials, errors, and
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

> Thruplane is an open-source AI gateway for routing production model traffic
> through one OpenAI-compatible endpoint. Run it yourself today, then add team
> operations or enterprise governance when you need them.
