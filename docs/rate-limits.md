# Rate limits and retry operations

NexoRoute protects provider capacity in two layers: local admission before a
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
remaining budget, NexoRoute moves to the next target or returns the original
upstream error. Without a valid hint, the gateway uses exponential backoff
capped by `max_delay` and adds equal jitter. `408`, `500`, `502`, `503`, `504`,
and transport failures follow the same bounded fallback policy.

Known quota, billing, and spend-limit errors that require operator action are
not repeated against the same target. They remain eligible for fallback to a
different provider/model. NexoRoute does not retry a stream after public output
has begun.

This follows the [official OpenAI rate-limit guidance](https://developers.openai.com/api/docs/guides/rate-limits):
honor `Retry-After`, use jittered exponential backoff when it is unavailable,
and bound both attempts and total retry time.

## Adaptive provider cooldown

NexoRoute extends a shared target cooldown when it receives:

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
X-NexoRoute-RateLimit-Reason: request_rate
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

## Current boundary

The Community limiter is deliberately process-local and target-level. It does
not provide tenant identity, a distributed counter across replicas, persistent
usage, or exact local token-per-minute accounting. Those capabilities require
request identity, provider-specific usage reconciliation, and shared state.
