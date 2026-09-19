# Roadmap

**Current milestone:** Public Community launch
**Status:** In Progress

---

## Usable gateway MVP

**Goal:** Run one local binary that routes OpenAI-compatible chat requests to
OpenAI or Ollama with predictable failure behavior.

**Target:** Build, vet, and all automated tests pass.

### Features

**Gateway foundation** - COMPLETE

- Load and validate YAML configuration with environment expansion.
- Route model aliases to ordered upstream targets.
- Forward buffered and streaming chat completions.

**Operational API** - COMPLETE

- List configured model aliases.
- Expose health checks, request IDs, optional authentication, and structured
  request logs.

**Packaging and onboarding** - COMPLETE

- Provide a Dockerfile and example configuration.
- Document local, Ollama, and OpenAI usage.

---

## Public Community launch

**Goal:** Publish a credible open-source foundation under the NexoRoute working
brand without overstating commercial readiness.

### Features

**Commercial rebrand** - COMPLETE

- Rename the module, command, binary, image, configuration namespace, and docs.
- Establish Apache 2.0 licensing and contributor and security guidance.
- Document Community, Pro, and Enterprise boundaries.

**Product strategy** - COMPLETE

- Define the open data plane and commercial control-plane boundary.
- Prioritize Flight Recorder and SLO-based routing as strategic bets.
- Record the proposed Community, Pro, and Enterprise capabilities.

**Release hardening** - PLANNED

- Add CI, signed release artifacts, checksums, and a versioning policy.
- Complete legal, domain, social-handle, and package-registry clearance.
- Publish a public repository and first tagged release.

---

## Community production foundation

**Goal:** Make the open data plane secure, observable, and extensible enough
for broader production adoption.

### Features

**Protocol and provider expansion** - PLANNED

- Add Responses, embeddings, and priority provider adapters.
- Normalize streaming, tool calls, and structured outputs.

**Reliability and access controls** - PLANNED

- Add circuit breakers, weighted routing, multiple local keys, and local
  limits.

**Open observability and packaging** - PLANNED

- Add Prometheus, OpenTelemetry, cost estimates, hot reload, and Helm.

---

## Pro operations beta

**Goal:** Remove the repeated operational work required to manage gateway use
across a growing team.

### Features

**Projects, virtual keys, and distributed limits** - PLANNED

**Usage, cost, budgets, quotas, and alerts** - PLANNED

**Managed control plane and team dashboard** - PLANNED

---

## Safe model rollout

**Goal:** Let teams measure and control model changes before full production
rollout.

### Features

**Flight Recorder capture and replay** - PLANNED

**Shadow traffic, canary rollout, and rollback** - PLANNED

**Cost, latency, quality, schema, and tool-call comparisons** - PLANNED

---

## Adaptive routing

**Goal:** Select routes against explicit cost, latency, quality, availability,
and privacy objectives.

### Features

**SLO policy schema and routing receipts** - PLANNED

**Routing recommendations with approval** - PLANNED

**Opt-in NexoRoute Autopilot** - PLANNED

---

## Enterprise foundation

**Goal:** Add organization-wide controls without weakening the Community core.

### Features

**SSO, SCIM, and role-based access** - PLANNED

**Audit exports and policy governance** - PLANNED

**High availability and enterprise support** - PLANNED

**Agent and MCP registry, traces, budgets, and tool policies** - PLANNED

**Privacy-preserving hosted control plane** - PLANNED

---

## Broader provider surface

**Goal:** Extend compatibility without coupling the core to provider SDKs.

### Features

**Additional OpenAI-compatible providers** - PLANNED

**Embeddings and Responses API** - PLANNED

**Dynamic configuration reload** - PLANNED

---

## Future considerations

- Cost Autopilot with model cascades and explicit quality floors
- Agent Firewall with prompt-injection and data-exfiltration controls
- BYOC, on-premises, and air-gapped deployment patterns
- Provider-specific request transformations
