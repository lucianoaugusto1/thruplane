# Roadmap

**Current milestone:** Community beta
**Status:** Complete
**Updated:** September 26, 2026

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

**Contract enforcement and provider conformance** - IN PROGRESS

- Versioned request and response contracts reject untranslated native fields
  and document compatible passthrough boundaries.
- Golden fixtures cover native and OpenAI-compatible adapters, modalities,
  tools, streaming or explicit rejection, and upstream errors.
- The opt-in live smoke runner exists; recorded model, region, auth mode, and
  verification-date evidence remains pending.
- Refreshable credentials and Bedrock streaming remain planned.

**Performance and failure baseline** - COMPLETE

- The local harness measures added p50/p95/p99 latency, first SSE chunk,
  throughput, allocations, memory, connection reuse, modalities, retries,
  fallbacks, throttling, slow clients, and cancellation.
- Versioned artifacts retain five raw runs and comparable environment data.
- Pull requests compare base and candidate on one runner with configurable
  regression thresholds and retained evidence.

**Exit gate:** Deterministic and opt-in live conformance evidence, documented
exceptions, repeatable benchmarks, and passing Go build/vet/race checks.

---

## Community beta

**Goal:** Give design partners a functional self-hosted/BYOK candidate with a
repeatable operator and maintainer gate.

### Features

**Runtime and operator diagnostics** - COMPLETE

- Validate configuration without opening a listener.
- Identify version, revision, and build date.
- Expose opt-in Prometheus-compatible metrics with bounded labels and no
  prompt, response, credential, file, or authorization-header content.

**Qualification and operations** - COMPLETE

- Run formatting, race-enabled tests, vet, and build in standard CI.
- Document install, preflight, probes, metrics, buffered and streaming smoke
  tests, rollout, rollback, privacy, and beta boundaries.
- Run the deterministic build, race, and short performance gates on the
  candidate revision.

**Developer onboarding** - COMPLETE

- Use the embedded playground with configured aliases or an optional ephemeral
  provider credential.
- Test all initial adapter credential shapes without persisting secrets.
- Require inbound authentication and exact allowlisting for custom upstream
  destinations; keep logs, metrics, errors, inspection, and export secret-safe.

**Exit gate:** All local qualification commands pass. Live provider evidence
remains explicit and does not block the deterministic BYOK beta contract.

---

## Public Community launch

**Goal:** Publish a credible open-source foundation under the Thruplane working
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

**Developer playground** - COMPLETE

- Serve an opt-in interface from the Community binary with no frontend runtime
  dependency.
- Exercise the public API for streaming, tools, images, PDFs, and audio.
- Show safe route, timing, usage, attempt, fallback, and request-ID details.
- Keep API keys, prompts, and files in browser memory without server history.

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

**Reliability and access controls** - IN PROGRESS

- Retry budgets, jittered backoff, provider cooldown, bounded queues, and local
  provider/model request and concurrency limits are complete.
- Process-local per-target circuit breakers and public aggregate readiness are
  complete.
- Weighted routing, multiple local keys, token-aware quotas, and secure secret
  sources remain planned.

**Open observability and packaging** - IN PROGRESS

- Prometheus-compatible process metrics are complete.
- Add OpenTelemetry, cost estimates, hot reload, and Helm.
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

- Add `thruplane/fast`, `thruplane/smart`, `thruplane/embed`,
  `thruplane/guard`, and `thruplane/auto`.

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

**Opt-in Thruplane Autopilot** - PLANNED

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
