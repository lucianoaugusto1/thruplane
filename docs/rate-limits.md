# Rate limits, retries, and circuit breakers

Thruplane protects provider capacity in two layers: local admission before a
request leaves the process, and adaptive retry behavior after an upstream
response. Both layers run in the customer-owned data plane and add no external
service to the request path.

## Configure a target

```yaml
models:
  fast:
    targets:
      - provider: openai
        model: gpt-4o-mini
        rate_limit:
          requests_per_minute: 500
          burst: 10
          max_concurrency: 32
          queue_timeout: 250ms
```

Use quota values from the actual provider account, model, and region. Do not
copy the example values into production without checking them.

| Field | Behavior |
| --- | --- |
| `requests_per_minute` | Token-bucket request rate. `0` disables request-rate admission. |
| `burst` | Requests available immediately. It defaults to `1` when RPM is configured. |
| `max_concurrency` | Responses and streams allowed in flight. `0` disables the ceiling. |
| `queue_timeout` | Maximum local admission wait. `0s` fails over without waiting. |

The limiter key is the configured provider name plus upstream model ID. Two
public aliases that use the same pair share capacity. Different configured
provider names stay isolated, even if they use the same provider type, because
they may represent different credentials or accounts. Conflicting policies for
the same pair are rejected during configuration loading.

Concurrency remains occupied until the upstream response body reaches EOF or
is closed. This includes SSE streams, so a long-lived stream consumes one slot
for its lifetime. Admission waits stop immediately when the client cancels.

## Configure retries

```yaml
routing:
  retries: 1
  retry:
    base_delay: 200ms
    max_delay: 5s
    budget: 15s
```

`retries` counts extra attempts for each target. `budget` starts immediately
before the first upstream attempt and includes attempt duration plus later
backoff waits. It does not include the target's initial admission queue.

For temporary `429` and `503` responses, a valid `Retry-After` is the minimum
delay and is never shortened to fit `max_delay`. If it does not fit in the
remaining budget, Thruplane moves to the next target or returns the original
upstream error. Without a valid hint, the gateway uses exponential backoff
capped by `max_delay` and adds equal jitter. `408`, `500`, `502`, `503`, `504`,
and transport failures follow the same bounded fallback policy.

Known quota, billing, and spend-limit errors that require operator action are
not repeated against the same target. They remain eligible for fallback to a
different provider/model. Thruplane does not retry a stream after public output
has begun.

This follows the [official OpenAI rate-limit guidance](https://developers.openai.com/api/docs/guides/rate-limits):
honor `Retry-After`, use jittered exponential backoff when it is unavailable,
and bound both attempts and total retry time.

## Adaptive provider cooldown

Thruplane extends a shared target cooldown when it receives:

- `Retry-After` on `429` or `503`;
- OpenAI-style `x-ratelimit-remaining-*` equal to zero with a valid duration in
  the corresponding `x-ratelimit-reset-*` header;
- Anthropic `anthropic-ratelimit-*-remaining` equal to zero with a valid
  RFC3339 timestamp in the corresponding reset header.

Cooldowns only move forward; a delayed response cannot shorten a newer limit.
OpenAI `x-ratelimit-*`, Anthropic `anthropic-ratelimit-*`, and `Retry-After`
headers are relayed to the caller.

Gemini and Bedrock throttling currently adapt through HTTP status and
`Retry-After`. For Bedrock, set a realistic `max_tokens`: Bedrock reserves
token quota using input plus requested output capacity, so an unnecessarily
large output bound can cause throttling even at a modest request rate. See the
[Amazon Bedrock quota documentation](https://docs.aws.amazon.com/bedrock/latest/userguide/quotas.html).

## Local rejection

When every eligible target is locally unavailable before any upstream attempt,
the gateway returns:

```http
HTTP/1.1 429 Too Many Requests
X-Thruplane-RateLimit-Reason: request_rate
Retry-After: 1
Content-Type: application/json
```

```json
{
  "error": {
    "message": "All eligible upstream targets are currently rate limited.",
    "type": "rate_limit_error",
    "param": null,
    "code": "gateway_rate_limited"
  }
}
```

The reason is `request_rate`, `concurrency`, `provider_cooldown`, or
`queue_timeout`. `Retry-After` is included only when a positive wait can be
calculated and is rounded up to a whole second.

## Circuit breakers

Enable target circuit breakers under `routing`:

```yaml
routing:
  circuit_breaker:
    failure_threshold: 5
    open_duration: 30s
```

The breaker is disabled when `failure_threshold` is `0`, which is the default.
Start with a threshold that tolerates occasional provider errors but opens
before repeated retries create avoidable latency.

Circuit breakers share the same physical target identity as local limits: the
configured provider name plus upstream model ID. Aliases that use the same
pair share one circuit.

The breaker records these failures:

- non-canceled transport failures; and
- retryable `408`, `500`, `502`, `503`, and `504` responses.

A `429` doesn't open the circuit because request admission and adaptive
cooldown already handle provider capacity. Other HTTP responses reset the
consecutive-failure count because the upstream was reachable. Client
cancellation, local admission rejection, and adapter validation don't count.

After the threshold, new requests skip the target and continue to the next
ordered fallback. After `open_duration`, one request becomes the half-open
probe. A successful response closes the circuit. A health failure reopens it
for the full duration. Other concurrent requests continue fallback while that
probe is active.

If every eligible target is already circuit-open, Thruplane returns:

```http
HTTP/1.1 503 Service Unavailable
Retry-After: 30
Content-Type: application/json
```

```json
{
  "error": {
    "message": "All eligible upstream targets have an open circuit.",
    "type": "api_error",
    "param": null,
    "code": "circuit_open"
  }
}
```

`Retry-After` is present when Thruplane can calculate a positive delay.

## Liveness and readiness

Use `GET /healthz` as a process-liveness check. It remains `200` while the
process can serve HTTP, even when upstream circuits are open.

Use `GET /readyz` to inspect route readiness. It returns only aggregate target
counts:

```json
{
  "status": "ready",
  "targets": {
    "total": 2,
    "available": 1,
    "open": 1,
    "half_open": 0
  }
}
```

The endpoint returns `503` with `status: "not_ready"` when every physical
target is circuit-open or a half-open probe is already using the only target.
It is public like `/healthz` and never exposes provider names, model IDs, URLs,
credentials, or request content.

Provider cooldown and local rate limits don't change readiness. Restarting or
removing a healthy gateway process doesn't restore external provider quota.

## Current boundary

The Community limiter and circuit breaker are deliberately process-local and
target-level. They don't provide tenant identity, distributed state across
replicas, persistent usage, or exact local token-per-minute accounting. Those
capabilities require request identity, provider-specific usage reconciliation,
and shared state.
