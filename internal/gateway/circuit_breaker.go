package gateway

import (
	"net/http"
	"time"

	"nexoroute/internal/circuitbreaker"
	"nexoroute/internal/config"
)

func buildBreakers(models map[string]config.ModelConfig, policy config.CircuitBreakerConfig, now func() time.Time) map[targetKey]*circuitbreaker.Breaker {
	breakers := make(map[targetKey]*circuitbreaker.Breaker)
	for _, model := range models {
		for _, target := range model.Targets {
			key := targetKey{provider: target.Provider, model: target.Model}
			if _, exists := breakers[key]; exists {
				continue
			}
			breakers[key] = circuitbreaker.NewWithClock(circuitbreaker.Policy{
				FailureThreshold: policy.FailureThreshold,
				OpenDuration:     time.Duration(policy.OpenDuration),
			}, now)
		}
	}
	return breakers
}

func circuitFailure(status int, decision retryDecision) bool {
	return status != http.StatusTooManyRequests && decision.retryTarget
}

type ReadinessStatus struct {
	Ready     bool
	Total     int
	Available int
	Open      int
	HalfOpen  int
}

func (g *Gateway) Readiness() ReadinessStatus {
	status := ReadinessStatus{Total: len(g.breakers)}
	for _, breaker := range g.breakers {
		snapshot := breaker.Snapshot()
		switch snapshot.State {
		case circuitbreaker.StateOpen:
			status.Open++
		case circuitbreaker.StateHalfOpen:
			status.HalfOpen++
			if !snapshot.ProbeInFlight {
				status.Available++
			}
		default:
			status.Available++
		}
	}
	status.Ready = status.Available > 0
	return status
}
