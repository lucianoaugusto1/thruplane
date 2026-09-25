package gateway

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"nexoroute/internal/catalog"
	"nexoroute/internal/circuitbreaker"
	"nexoroute/internal/config"
	"nexoroute/internal/provider"
	"nexoroute/internal/ratelimit"
)

type Gateway struct {
	settings gatewaySettings
	clients  map[string]*provider.Client
	catalog  *catalog.Registry
	limiters map[targetKey]*ratelimit.Limiter
	breakers map[targetKey]*circuitbreaker.Breaker
	now      func() time.Time
	jitter   func(time.Duration) time.Duration
	wait     func(context.Context, time.Duration) error
}

func New(cfg config.Config, clients map[string]*provider.Client) *Gateway {
	registry, err := catalog.BuiltIn()
	if err != nil {
		panic("invalid embedded model catalog: " + err.Error())
	}
	return NewWithCatalog(cfg, clients, registry)
}

func NewWithCatalog(cfg config.Config, clients map[string]*provider.Client, registry *catalog.Registry) *Gateway {
	return newWithCatalogAndClock(cfg, clients, registry, time.Now)
}

func newWithCatalogAndClock(cfg config.Config, clients map[string]*provider.Client, registry *catalog.Registry, now func() time.Time) *Gateway {
	settings := newGatewaySettings(cfg)
	return &Gateway{
		settings: settings,
		clients:  clients,
		catalog:  registry,
		limiters: buildLimiters(settings.models, now),
		breakers: buildBreakers(settings.models, settings.routing.CircuitBreaker, now),
		now:      now,
		jitter:   equalJitter,
		wait:     waitContext,
	}
}

func (g *Gateway) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	body, requestError := readBody(r, g.settings.maxBodyBytes)
	if requestError != nil {
		writeGatewayError(w, requestError)
		return
	}

	plan, requestError := g.planChat(body)
	if requestError != nil {
		writeGatewayError(w, requestError)
		return
	}

	result := g.executeChat(r.Context(), body, plan)
	if result.response != nil {
		setRouteHeaders(w.Header(), result)
		relayResponse(w, result.response, plan.stream)
		return
	}
	if r.Context().Err() != nil {
		return
	}
	if result.requestError != nil {
		writeError(w, http.StatusBadRequest, result.requestError.Message, "invalid_request_error", result.requestError.Code)
		return
	}
	if result.localDenial != nil {
		writeRateLimitError(w, result.localDenial)
		return
	}
	if result.circuitDenial != nil {
		writeCircuitOpenError(w, result.circuitDenial)
		return
	}
	writeError(w, http.StatusBadGateway, "The gateway could not reach an upstream provider.", "api_error", "upstream_unavailable")
}

func setRouteHeaders(header http.Header, result executionResult) {
	if safeRouteHeaderValue(result.provider) {
		header.Set("X-NexoRoute-Provider", result.provider)
	}
	if safeRouteHeaderValue(result.model) {
		header.Set("X-NexoRoute-Model", result.model)
	}
	header.Set("X-NexoRoute-Attempts", strconv.Itoa(result.attempts))
	header.Set("X-NexoRoute-Fallbacks", strconv.Itoa(result.fallbacks))
}

func safeRouteHeaderValue(value string) bool {
	if value == "" || len(value) > 512 {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character > 0x7e {
			return false
		}
	}
	return true
}
