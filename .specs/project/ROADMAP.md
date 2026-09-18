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

**Release hardening** - PLANNED

- Add CI, signed release artifacts, checksums, and a versioning policy.
- Complete legal, domain, social-handle, and package-registry clearance.
- Publish a public repository and first tagged release.

---

## Production controls

**Goal:** Validate and build the planned Pro team-operations offering.

### Features

**Per-key policies and rate limits** - PLANNED

**Metrics and tracing** - PLANNED

**Usage and cost accounting** - PLANNED

**Managed control plane and team dashboard** - PLANNED

---

## Enterprise foundation

**Goal:** Add organization-wide controls without weakening the Community core.

### Features

**SSO, SCIM, and role-based access** - PLANNED

**Audit exports and policy governance** - PLANNED

**High availability and enterprise support** - PLANNED

---

## Broader provider surface

**Goal:** Extend compatibility without coupling the core to provider SDKs.

### Features

**Additional OpenAI-compatible providers** - PLANNED

**Embeddings and Responses API** - PLANNED

**Dynamic configuration reload** - PLANNED

---

## Future considerations

- Semantic routing and load balancing
- Redis-backed distributed quotas
- Administrative API and dashboard
- Provider-specific request transformations
