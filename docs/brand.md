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

The visual system combines warm, paper-like surfaces with a precise industrial
orange. Cream makes the product approachable and premium; Thruplane orange
identifies routes, focus, and primary actions.

| Token | Hex | Use |
| --- | --- | --- |
| Paper | `#F7F0E6` | Page background |
| Porcelain | `#FFF9F2` | Panels and navigation |
| Elevated | `#FFFCF7` | Raised surfaces and controls |
| Sand | `#F2E7DA` | Secondary surfaces |
| Clay line | `#DCCBB9` | Borders and dividers |
| Ink | `#241A14` | Primary text |
| Umber | `#79695C` | Secondary text |
| Thruplane orange | `#F26632` | Primary actions, focus, and routes |
| Tangerine | `#FF8A4C` | Hover, highlights, and route origin |
| Route green | `#397A61` | Success and healthy state |
| Signal amber | `#B97822` | Warnings and degraded state |
| Fault clay | `#C94F45` | Errors and destructive actions |

Use orange as the primary brand color and let cream occupy most of the visual
field. Reserve green for success and healthy traffic. Avoid large solid orange
surfaces; the color works best as a route, focus signal, or decisive action.

### Icon

The [Thruplane mark](../assets/brand/thruplane-mark.svg) shows three traffic
paths meeting a tilted control plane. The highlighted center route passes
through the plane and deepens from tangerine to terracotta.

- Use the complete square mark at 20 pixels or larger.
- Keep clear space equal to one-eighth of the mark's width.
- Use the supplied cream tile on both light and dark backgrounds.
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
