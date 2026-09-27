# Delivery plan after the first multimodal adapters

**Status:** Active, not a release commitment
**Updated:** September 26, 2026

This plan turns the existing product strategy into work that can be verified.
The sequence matters: prove the direct-provider gateway before adding a
commercial control plane, hosted inference, or an LLM-based router.

## What exists today

- A Go binary with Chat Completions, model discovery, SSE streaming, retries,
  ordered fallbacks, optional single-key authentication, and YAML config.
- Direct native adapters for Anthropic, Gemini, Vertex AI, and Bedrock.
  OpenAI, Azure OpenAI, xAI/Grok, Ollama, and configurable compatible endpoints
  use OpenAI-compatible HTTP contracts. No aggregator is in the request path.
- Native function-tool translation. Native user input supports inline images
  and PDFs; Gemini and Vertex also support inline WAV/MP3 audio. Source and
  model restrictions still apply. Native media output is not normalized.
- An embedded, sourced model catalog with capability-aware routing and local
  `httptest` coverage. One inline-image translation benchmark exists.

These are implemented behaviors, not a claim that every provider/model
combination has passed a live compatibility or performance test.

## Priority 0: Make the current contract trustworthy

### 1. Define and enforce API compatibility

**Progress:** Native request-field enforcement, dated request and response
contracts, deterministic native and compatible-provider fixtures, and an
opt-in live runner are implemented. Recorded live evidence remains open.

**Why:** Native adapters decode selected Chat Completions fields. An unknown
model in `allow` mode bypasses catalog filtering, so an unsupported field can
reach an adapter that does not translate it. Passthrough adapters have a
different contract from native adapters.

**Create:** A versioned request/response compatibility matrix, provider-field
fixtures, and a shared native preflight validator. Cover roles, text, media,
tools, `tool_choice`, `response_format`, usage, finish reasons, streaming, and
unknown fields. Explicitly define the behavior of `file_id`, media output,
strict schemas, legacy function fields, and provider-hosted tools.

**Done when:** Every accepted native field is translated and tested, and every
untranslated field is rejected before network I/O. Compatible passthrough
remains transparent. Error codes and migration notes are documented.

**Candidate artifacts:** `docs/api-compatibility.md`,
`internal/provider/contract_test.go`, and shared validation code in
`internal/provider/`.

### 2. Prove each initial provider with a conformance suite

**Why:** Local `httptest` tests prove our request shapes, not a real provider's
current API behavior, account permissions, region availability, or model ID.

**Create:** Table-driven golden fixtures for buffered/streamed text, images,
PDFs, supported audio, multiple tool calls and results, cancellation, timeout,
rate-limit, malformed media, and upstream errors. Add opt-in live smoke tests
with a small spending cap, redacted logs, pinned test model IDs, and explicit
credentials. Record provider, model, region, date, capability, and outcome.

**Done when:** Each advertised native capability passes a fixture test and a
live smoke test for at least one supported model, or is clearly marked
unverified. A failing live test blocks that capability claim, not all local
development. CI runs deterministic fixtures; live tests never run by default.

**Candidate artifacts:** `internal/provider/testdata/`,
`tests/provider-live/`, `docs/provider-validation.md`, and a capability
report generated from test results.

### 3. Close authentication and protocol gaps

**Why:** Vertex currently requires a supplied access token; Bedrock streaming
is not implemented. Static credentials and incomplete streaming reduce the
number of real deployments that can use the gateway safely.

**Create:** Refreshable Vertex credentials through an explicit token source,
short-lived AWS credential support, and a Bedrock `ConverseStream` decoder with
event framing, integrity checks, cancellation, and SSE normalization. Keep
credential acquisition off the per-request hot path when practical.

**Done when:** Expired credentials refresh without restarting the gateway;
secret material never appears in logs; Bedrock streaming handles text, tool
events, usage, errors, and client disconnects under deterministic tests and
one opt-in live test. Do not advertise it before that gate passes.

**Candidate artifacts:** Credential-source interfaces, protocol tests, and
deployment examples. The [Google Cloud ADC guide](https://docs.cloud.google.com/docs/authentication/application-default-credentials)
and [Bedrock ConverseStream reference](https://docs.aws.amazon.com/bedrock/latest/APIReference/API_runtime_ConverseStream.html)
are source material, not proof of implementation.

### 4. Establish a performance and failure baseline

**Progress:** A deterministic local load runner and Go benchmark matrix now
cover latency percentiles, SSE TTFT, throughput, runtime cost, connection
reuse, modalities, retry, fallback, `429`, slow clients, and cancellation.
Versioned five-run artifacts, configurable regression thresholds, and a
same-runner pull-request gate are implemented. Live-provider and long-running
capacity evidence remain separate follow-up work.

**Why:** A sub-millisecond local adapter benchmark does not measure gateway
overhead under concurrency, first-token latency, network reuse, or failure
recovery. Optimization without a baseline risks adding complexity for no gain.

**Create:** A reproducible load harness with an in-process mock upstream and
optional live mode. Measure gateway-added p50/p95/p99 latency, time to first
SSE chunk, throughput, allocations, memory, CPU, connection reuse, and error
rate for text, tools, and 48 KiB or larger media. Include cancellation, slow
readers, retries, and fallback storms. Establish load profiles and a
published baseline per Go version and hardware class.

**Done when:** Every optimization has before/after measurements and a
regression threshold. The gateway enforces an end-to-end deadline budget and
does not retry work after client cancellation. Do not publish a universal
latency or TPS claim based on one laptop benchmark.

**Candidate artifacts:** `bench/`, `docs/performance.md`, and CI benchmarks.

## Priority 1: Production-ready Community release

### 5. Reliability, safety, and observability

**Progress:** Bounded per-target retry budgets, cancelable jittered backoff,
`Retry-After`, permanent quota classification, process-local request and
concurrency admission, bounded queues, shared provider cooldown, per-target
circuit breakers, aggregate readiness, and opt-in Prometheus-compatible HTTP,
route, fallback, attempt, readiness, and build metrics are implemented.
OpenTelemetry exporters, multiple local keys, token-aware quotas, and weighted
routing remain open.

**Create:** Per-target health and circuit breakers, weighted routing, readiness
checks, multiple local keys with scopes and expiry, token-aware local quotas,
and clear fail-open versus fail-closed policies. Maintain
Prometheus-compatible metrics and add OpenTelemetry traces without prompt,
response, or credential content by default. Define retention, redaction, and
high-cardinality label rules.

**Done when:** Chaos tests cover a slow provider, retryable errors, partial
streams, client disconnects, and mixed healthy/unhealthy targets. Operators
can answer which target was used, why fallback occurred, and where latency was
spent without exposing sensitive content.

**Candidate artifacts:** `internal/routing/`, `internal/telemetry/`,
`docs/operations.md`, and threat-model and runbook documents. Use the
[OpenTelemetry Go guidance](https://opentelemetry.io/docs/languages/go/) when
choosing instrumentation.

### 6. Catalog and release lifecycle

**Create:** Catalog update policy, expiry/staleness warnings, versioned price
sources, deprecated-model policy, and tests for aliases and region-specific
availability. Add CI, vulnerability/dependency checks, release notes,
reproducible multi-platform builds, checksums, signatures, and a versioning
policy. Review the working brand, domain, license notices, and public security
contact before publishing.

**Done when:** A release can be reproduced and verified; stale model facts
cannot silently become routing or billing guarantees; installation, upgrade,
rollback, and support boundaries are documented. Public repository creation
and legal/trademark clearance are explicit launch prerequisites.

**Candidate artifacts:** `.github/workflows/`, `docs/releases.md`,
`docs/catalog-maintenance.md`, and a release manifest.

### 7. Extend the public API deliberately

**Create:** Separate contracts and capability matrices for Responses and
embeddings, then implement direct adapters one operation at a time. Add
structured-output parity, strict tool schemas where semantics truly match,
and separate audio/image-output contracts rather than hiding them inside text
chat. Provider-hosted tools and multimodal tool results require their own
security and compatibility design.

**Done when:** Each operation has documented request/response behavior,
streaming tests where applicable, usage normalization, unsupported-field
rejection, and provider conformance. Existing Chat Completions clients do not
change. OpenAI's [Responses API reference](https://developers.openai.com/api/reference/cli/resources/responses/methods/create)
is one input to that contract; it is not automatically portable to all
providers.

## Priority 2: Pro operational workflows

### 8. Control plane, usage accounting, and Flight Recorder

**Create:** A tenant/project/key identity model, persistent usage and
cost ledger, price versioning, distributed quotas and budgets, alerts, and
auditable configuration changes. Keep this state outside the fast data path
where possible. Build a managed dashboard after the underlying APIs and
permissions work. For Flight Recorder, add opt-in sanitized capture, replay,
shadow traffic, evaluation datasets, canary rules, and rollback reports.

**Done when:** Usage reconciles with provider-reported tokens and cache data;
limits remain safe during control-plane outages; replay cannot duplicate
side-effecting tools without explicit controls; a model change has a clear
go/no-go report. Prompt and response collection is off by default.

**Candidate artifacts:** Separate control-plane service and schema,
`docs/privacy.md`, usage-ledger tests, replay fixtures, and rollout runbooks.

## Priority 3: Inference and Intelligence

### 9. Included inference, not unlimited free models

**Create:** A metered Thruplane Inference backend, curated stable aliases,
model-specific limits, admission control, abuse prevention, a compute-credit
ledger, explicit overage consent, BYOK fallback policy, and margin dashboards.
Start with metered external capacity and validate demand before reserving or
operating GPUs. Define data processing, region, and incident responsibilities.

**Done when:** Every request has an attributable cost and limit decision;
customers can predict included usage and overage; failure of hosted capacity
does not block independent BYOK routes. Dedicated or private capacity remains
an Enterprise option only after economics and operations are proven.

**Candidate artifacts:** Inference service contract, metering and billing
schema, plan limits, capacity runbook, and unit-economics model.

### 10. AutoRouter in measured layers

**Create:** Hard-constraint filtering first, then deterministic scoring and
routing receipts. Use Flight Recorder data to evaluate workload-specific
quality, cost, and latency. Add semantic classification only after labeled
evaluations show benefit; add an LLM router only for ambiguous/high-value
decisions with a strict latency and cost budget. Autopilot remains opt-in.

**Done when:** Every choice is explainable and reversible; each added layer
beats the previous baseline on a held-out evaluation set without violating
privacy, cost, or latency guardrails. A deterministic route survives failure
of semantic or LLM routing.

**Candidate artifacts:** Versioned policy schema, routing-receipt format,
offline evaluation harness, quality datasets, and rollout controls.

## Priority 4: Enterprise assurance

**Create:** SSO, SCIM, RBAC, policy approvals, immutable audit export,
secret-manager and KMS integration, private networking, region/data-residency
controls, HA and disaster-recovery plans, BYOC/private inference, and a
contracted support and incident process. Implement controls only against
validated customer requirements and a reviewed threat model.

**Done when:** Tenant isolation, backup restore, failover, audit integrity,
access revocation, and support response have tested evidence. Do not sell an
SLA or compliance claim before operational and legal review.

## Decisions to make before implementation

1. Choose the first production reference deployment and obtain test accounts,
   regions, and capped credentials for each provider.
2. Define the first release's support promise: specific models and modalities,
   or a narrower documented compatibility tier per adapter.
3. Set measurable gateway-added latency, memory, availability, and cost
   budgets from the baseline, not from a marketing target.
4. Decide whether the first Pro workflow is Flight Recorder or usage/budget
   management after customer interviews; both require identity and metering.
5. Decide the hosted control-plane deployment and data-retention model before
   collecting any customer prompts or responses.

The Community beta gate, privacy-safe Prometheus metrics, configuration
preflight, build identity, CI, and operator runbook are complete. The next
external validation slice is **recorded live provider conformance**. The next
credential-free production slice is **multiple scoped local keys and
OpenTelemetry traces**. Live evidence and release hardening remain
prerequisites for a public production claim.
