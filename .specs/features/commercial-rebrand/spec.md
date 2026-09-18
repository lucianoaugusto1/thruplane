# Commercial rebrand specification

## Problem statement

The project uses a generic name that conflicts with existing LLM products and
does not explain how the open-source project relates to future paid editions.
It needs a distinct working identity, credible community foundations, and clear
commercial boundaries before public launch.

## Brand decision

**Working brand:** NexoRoute
**Category:** Open-source AI gateway
**Tagline:** The open control plane for AI traffic.

The name is provisional. A preliminary web search found no competing AI gateway
under this name, but it did find an unrelated French transportation business.
Trademark, company-name, domain, and package-registry clearance remain required
before a public commercial launch.

## Goals

- [x] Present one consistent NexoRoute identity across code and documentation.
- [x] Keep a genuinely useful Community edition under Apache License 2.0.
- [x] Explain planned Pro and Enterprise value without claiming it exists today.
- [x] Prepare the repository for external contributors and security reports.
- [ ] Move the completed repository to `/Users/lucianobr01/go-llm-gateway`.

## Out of scope

| Feature | Reason |
| --- | --- |
| Implement Pro or Enterprise features | This change defines the offering only |
| Publish packages, images, or a GitHub repository | No registry or remote was authorized |
| Set prices | Pricing needs customer discovery and cost data |
| Create a legal entity or register a trademark | Requires professional legal work |
| Build a logo or website | Product identity comes before visual production |

## Requirements

| ID | Requirement | Verification |
| --- | --- | --- |
| BR-01 | Product-facing text uses NexoRoute consistently | Repository search |
| BR-02 | Module, imports, binary, command, and image use `nexoroute` | Build and Docker gate |
| BR-03 | Community source has an Apache 2.0 license | License review |
| BR-04 | Community, Pro, and Enterprise boundaries are explicit | Editions document review |
| BR-05 | Contribution and security paths are documented | File and link review |
| BR-06 | Paid features are labeled planned, not available | Documentation review |
| BR-07 | Repository works from the requested destination | Full gate after move |

## Edition principles

- Community must remain production-usable for self-hosted single-team use.
- Open protocols, core routing, provider adapters, retries, streaming, and basic
  observability stay open source.
- Pro monetizes team operations, cost controls, and a managed experience.
- Enterprise monetizes organization-wide governance, security, scale, support,
  and deployment assurance.
- Paid plans must not create protocol lock-in or prevent Community users from
  exporting their configuration and data.

## Success criteria

- [x] Repository search finds no obsolete product or module identity.
- [x] Existing 29 top-level tests and 16 subtests remain intact.
- [x] Race detector, vet, binary build, and Docker build pass.
- [ ] Git worktree is clean at the destination path.
