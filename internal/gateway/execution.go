package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"nexoroute/internal/provider"
	"nexoroute/internal/ratelimit"
)

type executionResult struct {
	response     *http.Response
	requestError *provider.RequestError
	localDenial  *ratelimit.Denial
	provider     string
	model        string
	attempts     int
	fallbacks    int
}

func (g *Gateway) executeChat(ctx context.Context, body []byte, plan chatPlan) executionResult {
	var localDenial *ratelimit.Denial
	attemptedUpstream := false
	upstreamAttempts := 0
	for targetIndex, target := range plan.targets {
		client := g.clients[target.Provider]
		limiter := g.limiters[targetKey{provider: target.Provider, model: target.Model}]
		var targetStart time.Time

		for attempt := 0; attempt <= g.settings.routing.Retries; attempt++ {
			if ctx.Err() != nil {
				return executionResult{}
			}
			queueTimeout := time.Duration(target.RateLimit.QueueTimeout)
			if attempt > 0 {
				remaining, limited := remainingRetryBudget(targetStart, g.now(), time.Duration(g.settings.routing.Retry.Budget))
				if limited && remaining <= 0 {
					break
				}
				if limited && queueTimeout > 0 && remaining < queueTimeout {
					queueTimeout = remaining
				}
			}
			permit, denied, err := limiter.Acquire(ctx, queueTimeout)
			if err != nil {
				return executionResult{}
			}
			if denied != nil {
				localDenial = earlierDenial(localDenial, denied)
				break
			}
			if targetStart.IsZero() {
				targetStart = g.now()
			} else if remaining, limited := remainingRetryBudget(targetStart, g.now(), time.Duration(g.settings.routing.Retry.Budget)); limited && remaining <= 0 {
				permit.Release()
				break
			}
			attemptedUpstream = true
			upstreamAttempts++

			response, err := client.Do(ctx, body, target.Model)
			if err != nil {
				permit.Release()
				if ctx.Err() != nil {
					return executionResult{}
				}
				var requestError *provider.RequestError
				if errors.As(err, &requestError) {
					return executionResult{requestError: requestError}
				}
				if attempt == g.settings.routing.Retries {
					break
				}
				delay := retryDelay(g.settings.routing.Retry, attempt, nil, g.now(), g.jitter)
				if !retryFitsBudget(targetStart, g.now(), time.Duration(g.settings.routing.Retry.Budget), delay) {
					break
				}
				if err := g.wait(ctx, delay); err != nil {
					return executionResult{}
				}
				continue
			}
			wrapPermit(response, permit)
			providerType := g.settings.providerType(target.Provider)
			limiter.Observe(providerType, response.StatusCode, response.Header)

			decision := classifyResponse(providerType, response)
			if !decision.fallback {
				return executionResult{
					response: response, provider: target.Provider, model: target.Model,
					attempts: upstreamAttempts, fallbacks: targetIndex,
				}
			}

			lastAttempt := attempt == g.settings.routing.Retries
			lastTarget := targetIndex == len(plan.targets)-1
			if !decision.retryTarget || lastAttempt {
				if lastTarget {
					return executionResult{
						response: response, provider: target.Provider, model: target.Model,
						attempts: upstreamAttempts, fallbacks: targetIndex,
					}
				}
				drainAndClose(response)
				break
			}

			delay := retryDelay(g.settings.routing.Retry, attempt, response, g.now(), g.jitter)
			if !retryFitsBudget(targetStart, g.now(), time.Duration(g.settings.routing.Retry.Budget), delay) {
				if lastTarget {
					return executionResult{
						response: response, provider: target.Provider, model: target.Model,
						attempts: upstreamAttempts, fallbacks: targetIndex,
					}
				}
				drainAndClose(response)
				break
			}

			drainAndClose(response)
			if err := g.wait(ctx, delay); err != nil {
				return executionResult{}
			}
		}
	}

	if !attemptedUpstream && localDenial != nil {
		return executionResult{localDenial: localDenial}
	}
	return executionResult{}
}

func earlierDenial(current, candidate *ratelimit.Denial) *ratelimit.Denial {
	if current == nil {
		return candidate
	}
	if candidate.RetryAfter > 0 && (current.RetryAfter <= 0 || candidate.RetryAfter < current.RetryAfter) {
		return candidate
	}
	return current
}
