# Community beta specification

## Problem statement

NexoRoute already routes real chat traffic, but it does not define a smaller
release gate between the internal gateway milestone and a public Community
launch. Beta testers need a build they can validate, identify, observe, and
roll back without depending on future Pro or Enterprise services.

## Goals

- [x] Make a self-hosted, BYOK deployment diagnosable before and after start.
- [x] Publish a repeatable beta qualification and operator workflow.
- [x] Keep request content, responses, credentials, and authorization headers
  out of metrics and logs.
- [x] Prove the beta candidate with deterministic tests and a short load run.

## Out of scope

| Capability | Reason |
| --- | --- |
| Multi-tenant keys and distributed quotas | A per-customer deployment can use the existing inbound key for beta. |
| Dynamic configuration reload | A documented restart and rollback workflow is sufficient for beta. |
| OpenTelemetry export | Prometheus-compatible local metrics cover the first operator loop. |
| Bedrock streaming and refreshed cloud credentials | These require provider-specific assurance and remain separate milestones. |
| Live certification of every provider | It requires capped customer credentials and cannot be deterministic CI. |
| Release signing and public registry publication | These belong to the public launch milestone after beta evidence. |

## User stories

### P1: Validate a deployment before start

**User story:** As an operator, I want to validate the complete configuration
without opening a listener so that a bad rollout fails before receiving
traffic.

**Acceptance criteria:**

1. WHEN an operator runs `nexoroute -check-config -config <path>` with a valid
   file THEN the command SHALL exit zero and print a confirmation.
2. WHEN the configuration is invalid THEN the command SHALL exit nonzero and
   report the validation error without printing secret values.
3. WHEN an operator runs `nexoroute -version` THEN the command SHALL identify
   the version, revision, and build date without loading configuration.

**Independent test:** Run the command against valid and invalid temporary YAML
files, then invoke `-version` without a config file.

### P1: Observe gateway health and traffic

**User story:** As an operator, I want an opt-in Prometheus endpoint so that I
can see request latency, status, route selection, fallbacks, and target health
without collecting model input or output.

**Acceptance criteria:**

1. WHEN metrics are disabled THEN `/metrics` SHALL not be registered and the
   request path SHALL add no metrics collection work.
2. WHEN metrics are enabled THEN `/metrics` SHALL expose valid Prometheus text
   for bounded HTTP routes, response status, in-flight requests, request
   duration, selected provider/model, request attempts/fallbacks, and readiness
   target states.
3. WHEN a chat request contains prompts, files, API keys, or authorization
   headers THEN metrics SHALL not contain those values.
4. WHEN multiple requests update metrics concurrently THEN collection and
   scraping SHALL remain race-free.

**Independent test:** Send authenticated chat and health requests through an
`httptest` server, scrape `/metrics`, and assert both required series and
sensitive-value absence.

### P1: Reproduce the beta gate

**User story:** As a maintainer, I want CI and a beta checklist so that every
candidate passes the same build, test, race, and smoke requirements.

**Acceptance criteria:**

1. WHEN a pull request or default-branch push runs THEN CI SHALL check
   formatting, unit/integration tests with the race detector, vet, and binary
   build.
2. WHEN an operator follows the beta runbook THEN they SHALL be able to
   install, preflight, start, check health/readiness/metrics, send buffered and
   streamed requests, and roll back.
3. WHEN the final beta gate runs THEN all deterministic tests, vet, build,
   race, and one-run performance scenarios SHALL pass.

**Independent test:** Execute the documented commands on a clean checkout and
verify that the workflow file contains the same mandatory gates.

## Edge cases

- WHEN `/metrics` is scraped before a chat request THEN all core metric
  families SHALL still have valid metadata and zero-safe output.
- WHEN a request uses an unknown path THEN the HTTP route label SHALL be a
  bounded fallback value rather than the raw path.
- WHEN provider or model labels contain quotes, backslashes, or newlines THEN
  exposition SHALL escape them according to the Prometheus text format.
- WHEN a streaming response remains open THEN the in-flight gauge SHALL remain
  incremented until the handler returns.

## Requirement traceability

| Requirement ID | Story | Status |
| --- | --- | --- |
| BETA-01 | Validate configuration | Verified |
| BETA-02 | Identify the build | Verified |
| BETA-03 | Opt-in metrics endpoint | Verified |
| BETA-04 | Privacy-safe bounded telemetry | Verified |
| BETA-05 | Continuous integration gate | Verified |
| BETA-06 | Operator runbook and smoke test | Verified |
| BETA-07 | Final deterministic qualification | Verified |

**Coverage:** 7 requirements, 7 mapped to tasks, 0 unmapped.

## Success criteria

- [x] A tester can validate and identify a build before it starts.
- [x] A tester can diagnose traffic and target health without sensitive
  telemetry.
- [x] The documented beta gate passes on the candidate revision.
- [x] The remaining provider-live evidence is labeled as unverified rather
  than implied by the beta designation.
