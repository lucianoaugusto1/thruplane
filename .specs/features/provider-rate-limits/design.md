# Provider rate-limit design

**Specification:** `spec.md`

## Architecture

```text
request -> capability filter -> target limiter -> provider.Client
                                  |                  |
                                  |                  +-> response/error
                                  |                           |
                                  +<- cooldown/header observer+
```

The gateway owns retry and fallback policy. A new `internal/ratelimit` package
owns in-process target admission with no provider or HTTP dependencies beyond
header observation helpers. Limiters are keyed by configured provider name and
upstream model ID, so aliases share the quota while distinct credentials or
provider configurations remain isolated.

## Local admission

The limiter combines:

- a token bucket for requests per minute and explicit burst;
- an in-flight counter for maximum concurrency;
- a provider cooldown timestamp learned from upstream headers;
- a broadcast notification channel for releases and cooldown changes.

Acquisition waits for the earliest of capacity, a calculated rate/cooldown
timer, request cancellation, or queue timeout. A permit is idempotent and is
released by a response-body wrapper on EOF or Close.

## Retry policy

`Retry-After` accepts integer seconds and HTTP-date values. Without a valid
hint, delay is exponential from `base_delay`, capped at `max_delay`, with equal
jitter in the upper half of the delay range. Tests inject deterministic jitter
and time sources.

The budget measures wall-clock time from the first attempt on a target. A
server hint larger than the remaining budget is never truncated. The target is
abandoned so the next independent fallback can be attempted immediately.

## Classification

Temporary transport and HTTP errors are retryable. For `429`, a bounded prefix
of the error body is inspected while the body is reconstructed for later relay.
Known OpenAI quota/billing codes, Anthropic enforced spend-limit codes, and
Gemini quota-exhaustion codes skip same-target retries but remain eligible for
fallback.

## Header adaptation

OpenAI duration reset headers and Anthropic RFC3339 reset headers can extend a
target cooldown when the corresponding remaining count is zero. A direct
`Retry-After` on `429` or `503` also extends the cooldown. Provider headers are
relayed to callers; local rejections use Thruplane-specific reason metadata.

## Compatibility and performance

Limits default to disabled. Retry defaults add a small delay only on failures;
the success path adds one mutex acquisition and one response-body wrapper.
There are no background goroutines, tickers, external services, or new module
dependencies.
