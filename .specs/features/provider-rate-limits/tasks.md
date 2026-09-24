# Provider rate-limit tasks

**Design:** `design.md`
**Status:** Complete

```text
T1 -> T2 -> T3 -> T4 -> T5
```

## T1: Define and validate configuration — Complete

**What:** Add retry timing and per-target limiter settings with strict
validation and safe defaults.
**Where:** `internal/config`, `config.example.yaml`.
**Requirements:** RL-02, RL-04, RL-08.
**Tests:** Configuration unit tests. **Gate:** Quick.

## T2: Build deterministic target admission — Complete

**What:** Implement request token bucket, concurrency permits, queue timeout,
cooldown observation, and snapshots.
**Where:** `internal/ratelimit`.
**Depends on:** T1. **Requirements:** RL-04, RL-05, RL-06.
**Tests:** Unit and race tests. **Gate:** Quick.

## T3: Integrate retry policy and classification — Complete

**What:** Add `Retry-After`, jittered backoff, retry budget, permanent quota
classification, shared limiter registry, and fallback semantics.
**Where:** `internal/gateway`.
**Depends on:** T2. **Requirements:** RL-01 through RL-07.
**Tests:** `httptest` integration and cancellation tests. **Gate:** Full.

## T4: Verify streaming and public contract — Complete

**What:** Prove permits live through response-body completion, headers are
relayed, and local exhaustion returns the documented error contract.
**Where:** `internal/gateway`, `internal/httpapi`.
**Depends on:** T3. **Requirements:** RL-05 through RL-09.
**Tests:** End-to-end, streaming, and race tests. **Gate:** Full.

## T5: Publish operator guidance and validation — Complete

**What:** Document configuration, semantics, provider caveats, and project
state; record verification results.
**Where:** README, `docs/`, `.specs/project`, feature validation.
**Depends on:** T4. **Requirements:** RL-01 through RL-09.
**Tests:** Documentation review, race, vet, build, diff check. **Gate:** Build.
