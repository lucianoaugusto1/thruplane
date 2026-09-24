# Provider rate-limit specification

**Status:** Verified
**Date:** September 24, 2026

## Problem

NexoRoute retries throttled and transient upstream responses immediately. That
can amplify provider overload, consume quota with unsuccessful requests, and
create a retry storm across concurrent gateway requests. Operators also cannot
bound concurrency or request rate for a provider/model target before traffic is
sent upstream.

## Goals

- Respect provider retry hints without sleeping past a configured retry budget.
- Add cancelable exponential backoff with jitter when no valid hint exists.
- Bound request rate, concurrency, and queue time per physical provider/model.
- Share cooldown and capacity across aliases that use the same physical target.
- Preserve streaming and release concurrency only when the response body ends.
- Fail over when another target can serve traffic instead of violating a limit.
- Return an explicit OpenAI-shaped `429` when every eligible target is locally
  limited.

## Requirements

### RL-01: Retry timing

WHEN an eligible temporary response contains a valid `Retry-After` header THEN
the gateway SHALL treat it as the minimum retry delay. WHEN the header is absent
or invalid THEN the gateway SHALL use bounded exponential backoff with jitter.
All waits SHALL stop on request cancellation.

### RL-02: Retry budget

Retries SHALL be bounded by both `routing.retries` and
`routing.retry.budget`. A delay that does not fit in the remaining budget SHALL
not be shortened; the gateway SHALL move to an independent fallback or return
the original upstream response.

### RL-03: Error classification

The gateway SHALL retry temporary `408`, `429`, `500`, `502`, `503`, and `504`
responses and transport failures. It SHALL not repeat provider quota, billing,
or spend-limit errors that require operator action. A permanent `429` MAY still
fail over to another target.

### RL-04: Local target limits

Each target MAY configure requests per minute, burst, maximum concurrency, and
queue timeout. The limiter SHALL be in-process, dependency-free, and shared by
all aliases that resolve to the same configured provider name and upstream
model ID.

### RL-05: Adaptive cooldown

The limiter SHALL learn temporary cooldowns from `Retry-After` and from
supported provider remaining/reset headers. It SHALL preserve OpenAI
`x-ratelimit-*` and Anthropic `anthropic-ratelimit-*` response headers for the
caller.

### RL-06: Streaming safety

The gateway SHALL acquire capacity before each upstream attempt and release it
after an error or after the response body reaches EOF or is closed. It SHALL
never replay a request after public streaming output has begun.

### RL-07: Local rejection contract

WHEN all eligible targets are locally unavailable before an upstream call THEN
the gateway SHALL return status `429`, code `gateway_rate_limited`, a valid
`Retry-After` value when known, and an `X-NexoRoute-RateLimit-Reason` header.

### RL-08: Configuration safety

The strict YAML loader SHALL reject negative values, invalid burst settings,
invalid retry bounds, and conflicting limiter settings for the same physical
target. Existing configurations without limits SHALL remain valid.

### RL-09: Verification

Deterministic tests SHALL cover timing calculation, limiter sharing,
concurrency release, queue timeout, cancellation, header observation,
permanent quota classification, fallback, and streaming behavior. The complete
race, vet, and build gates SHALL pass.

## Out of scope

- Distributed or tenant-level limits; these require shared state and identity.
- Exact local token-per-minute accounting; tokenization and reservation differ
  by provider and model. Provider token reset headers are still observed.
- Usage billing, persistent counters, Prometheus, and OpenTelemetry exporters.
- Provider quota discovery APIs or automatic quota mutation.

## Success criteria

- No retry is sent earlier than a valid provider hint.
- A canceled request exits every limiter or retry wait promptly.
- Concurrent aliases cannot bypass a configured target limiter.
- A slow downstream stream continues to occupy its concurrency permit.
- Permanent quota errors skip same-target retries without disabling fallback.
- Existing configurations and successful fast paths remain compatible.
