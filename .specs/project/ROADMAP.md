# Roadmap

**Current milestone:** Usable gateway MVP
**Status:** In Progress

---

## Usable gateway MVP

**Goal:** Run one local binary that routes OpenAI-compatible chat requests to
OpenAI or Ollama with predictable failure behavior.

**Target:** Build, vet, and all automated tests pass.

### Features

**Gateway foundation** - IN PROGRESS

- Load and validate YAML configuration with environment expansion.
- Route model aliases to ordered upstream targets.
- Forward buffered and streaming chat completions.

**Operational API** - IN PROGRESS

- List configured model aliases.
- Expose health checks, request IDs, optional authentication, and structured
  request logs.

**Packaging and onboarding** - PLANNED

- Provide a Dockerfile and example configuration.
- Document local, Ollama, and OpenAI usage.

---

## Production controls

**Goal:** Add controls needed for multi-user deployments.

### Features

**Per-key policies and rate limits** - PLANNED

**Metrics and tracing** - PLANNED

**Usage and cost accounting** - PLANNED

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
