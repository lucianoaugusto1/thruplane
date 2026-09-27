package gateway

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/lucianoaugusto1/thruplane/internal/circuitbreaker"
	"github.com/lucianoaugusto1/thruplane/internal/provider"
	"github.com/lucianoaugusto1/thruplane/internal/ratelimit"
)

type executionResult struct {
	response      *http.Response
	requestError  *provider.RequestError
	localDenial   *ratelimit.Denial
	circuitDenial *circuitbreaker.Denial
	provider      string
	model         string
	attempts      int
	fallbacks     int
}

func (g *Gateway) executeChat(ctx context.Context, body []byte, plan chatPlan) executionResult {
	var localDenial *ratelimit.Denial
	var circuitDenial *circuitbreaker.Denial
	attemptedUpstream := false
	upstreamAttempts := 0
	for targetIndex, target := range plan.targets {
		client := g.clients[target.Provider]
		key := targetKey{provider: target.Provider, model: target.Model}
		limiter := g.limiters[key]
		breaker := g.breakers[key]
		var targetStart time.Time

		for attempt := 0; attempt <= g.settings.routing.Retries; attempt++ {
			if ctx.Err() != nil {
				return executionResult{}
			}
			circuitPermit, deniedByCircuit := breaker.Acquire()
			if deniedByCircuit != nil {
				circuitDenial = earlierCircuitDenial(circuitDenial, deniedByCircuit)
				break
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
				circuitPermit.Cancel()
				return executionResult{}
			}
			if denied != nil {
				circuitPermit.Cancel()
				localDenial = earlierDenial(localDenial, denied)
				break
			}
			if targetStart.IsZero() {
				targetStart = g.now()
			} else if remaining, limited := remainingRetryBudget(targetStart, g.now(), time.Duration(g.settings.routing.Retry.Budget)); limited && remaining <= 0 {
				permit.Release()
				circuitPermit.Cancel()
				break
			}
			attemptedUpstream = true
			upstreamAttempts++

			response, err := client.Do(ctx, body, target.Model)
			if err != nil {
				permit.Release()
				if ctx.Err() != nil {
					circuitPermit.Cancel()
					return executionResult{}
				}
				var requestError *provider.RequestError
				if errors.As(err, &requestError) {
					circuitPermit.Cancel()
					return executionResult{requestError: requestError}
				}
				circuitPermit.Failure()
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
			if circuitFailure(response.StatusCode, decision) {
				circuitPermit.Failure()
			} else {
				circuitPermit.Success()
			}
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
	if !attemptedUpstream && circuitDenial != nil {
		return executionResult{circuitDenial: circuitDenial}
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

func earlierCircuitDenial(current, candidate *circuitbreaker.Denial) *circuitbreaker.Denial {
	if current == nil {
		return candidate
	}
	if candidate.RetryAfter > 0 && (current.RetryAfter <= 0 || candidate.RetryAfter < current.RetryAfter) {
		return candidate
	}
	return current
}
