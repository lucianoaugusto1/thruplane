# Roadmap

**Current milestone:** Gateway assurance
**Status:** Planning
**Updated:** September 22, 2026

This page tracks shippable outcomes. The detailed
[delivery plan](../../docs/next-steps.md) lists required work, artifacts,
dependencies, and acceptance gates. Planned
capabilities are not available product claims or release-date commitments.

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

## Gateway assurance

**Goal:** Verify the initial provider adapters and the public compatibility
contract before making broad production claims.

### Features

**Initial direct-provider adapters and native tools/media input** - COMPLETE

- OpenAI, Anthropic, Gemini, Vertex AI, Bedrock, Azure OpenAI, Ollama,
  xAI/Grok, and configurable compatible protocols.
- Native function tools; native image and PDF input; Gemini and Vertex
  inline audio input; capability-aware routing.

**Contract enforcement and provider conformance** - PLANNED

- Version accepted fields and reject untranslated native fields explicitly.
- Add golden fixtures and opt-in live smoke tests per provider and modality.
- Record model, region, auth mode, and verification date in a report.

**Performance and failure baseline** - PLANNED

- Measure added p50/p95/p99 latency, first SSE chunk, throughput,
  allocations, memory, and behavior under retries and disconnects.
- Add refreshable credentials and complete Bedrock streaming.

**Exit gate:** Deterministic and opt-in live conformance evidence, documented
exceptions, repeatable benchmarks, and passing Go build/vet/race checks.

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
- Document catalog maintenance, upgrades, rollback, and security contact.
- Complete legal, domain, social-handle, and package-registry clearance.
- Publish a public repository and first tagged release.
- Complete the Gateway assurance exit gate first.

---

## Community production foundation

**Goal:** Make the open data plane secure, observable, and extensible enough
for broader production adoption.

### Features

**API and protocol expansion** - PLANNED

- Add Responses and embeddings through separate direct-API contracts.
- Extend structured outputs, strict tools, and media output only where
  semantics can be preserved and verified.

**Reliability and access controls** - PLANNED

- Add circuit breakers, retry budgets, weighted routing, readiness, multiple
  local keys, local limits, and secure secret sources.

**Open observability and packaging** - PLANNED

- Add Prometheus, OpenTelemetry, cost estimates, hot reload, and Helm.
- Prove privacy and redaction defaults under load and failure.

---

## Pro operations beta

**Goal:** Remove the repeated operational work required to manage gateway use
across a growing team.

### Features

**Projects, virtual keys, and distributed limits** - PLANNED

**Usage, cost, budgets, quotas, and alerts** - PLANNED

**Managed control plane and team dashboard** - PLANNED

---

## Hosted inference

**Goal:** Include useful model capacity in Pro and Enterprise without creating
unbounded cost or coupling applications to physical model names.

### Features

**Curated model aliases** - PLANNED

- Add `nexoroute/fast`, `nexoroute/smart`, `nexoroute/embed`,
  `nexoroute/guard`, and `nexoroute/auto`.

**Pro shared inference** - PLANNED

- Add monthly compute credits, plan limits, explicit overage, and BYOK fallback.

**Enterprise inference deployment** - PLANNED

- Add reserved, dedicated, BYOC, private, and approved-region options.

**Capacity and margin controls** - PLANNED

- Measure utilization, inference cost, gross margin, concurrency, and cache
  benefit before operating dedicated GPU infrastructure.

---

## Safe model rollout

**Goal:** Let teams measure and control model changes before full production
rollout.

### Features

**Flight Recorder capture and replay** - PLANNED

- Require tenant identity, usage accounting, opt-in capture, and redaction.
- Prevent replay from duplicating side-effecting tool calls by default.

**Shadow traffic, canary rollout, and rollback** - PLANNED

**Cost, latency, quality, schema, and tool-call comparisons** - PLANNED

---

## Adaptive routing

**Goal:** Select routes against explicit cost, latency, quality, availability,
and privacy objectives.

### Features

**Hard constraints, heuristic routing, and routing receipts** - PLANNED

**Semantic routing and evaluated workload profiles** - PLANNED

**Selective LLM router for ambiguous decisions** - PLANNED

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
- Dedicated inference infrastructure after demand validation
- Agent Firewall with prompt-injection and data-exfiltration controls
- BYOC, on-premises, and air-gapped deployment patterns
- Provider-specific request transformations
