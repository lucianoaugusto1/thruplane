package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nexoroute/internal/catalog"
	"nexoroute/internal/config"
	"nexoroute/internal/provider"
)

func TestCircuitBreakerOpensAndSkipsPrimaryTarget(t *testing.T) {
	var primaryCalls atomic.Int32
	primary := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		primaryCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(primary.Close)
	var fallbackCalls atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fallbackCalls.Add(1)
		_, _ = io.WriteString(w, `{"id":"fallback"}`)
	}))
	t.Cleanup(fallback.Close)

	gateway := newCircuitTestGateway(t, []testTarget{
		{name: "primary", url: primary.URL, model: "model-a"},
		{name: "fallback", url: fallback.URL, model: "model-b"},
	}, 2, time.Minute, time.Now)
	for requestIndex := 0; requestIndex < 3; requestIndex++ {
		response := performChat(t, gateway, "public-alias")
		if response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200; body = %s", requestIndex+1, response.Code, response.Body.String())
		}
		if requestIndex == 2 {
			assertHeader(t, response.Header(), "X-NexoRoute-Attempts", "1")
			assertHeader(t, response.Header(), "X-NexoRoute-Fallbacks", "1")
		}
	}
	if got := primaryCalls.Load(); got != 2 {
		t.Fatalf("primary calls = %d, want 2 before open", got)
	}
	if got := fallbackCalls.Load(); got != 3 {
		t.Fatalf("fallback calls = %d, want 3", got)
	}
}

func TestCircuitBreakerReturnsServiceUnavailableWhenAllTargetsOpen(t *testing.T) {
	now := time.Unix(100, 0)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(upstream.Close)
	gateway := newCircuitTestGateway(t,
		[]testTarget{{name: "primary", url: upstream.URL, model: "model-a"}},
		1, time.Minute, func() time.Time { return now },
	)

	first := performChat(t, gateway, "public-alias")
	if first.Code != http.StatusBadGateway {
		t.Fatalf("first status = %d, want upstream 502", first.Code)
	}
	second := performChat(t, gateway, "public-alias")
	if second.Code != http.StatusServiceUnavailable {
		t.Fatalf("second status = %d, want 503; body = %s", second.Code, second.Body.String())
	}
	assertHeader(t, second.Header(), "Retry-After", "60")
	assertErrorCode(t, second, "circuit_open")
	if got := calls.Load(); got != 1 {
		t.Fatalf("upstream calls = %d, want one before circuit opened", got)
	}
}

func TestCircuitBreakerStateIsSharedAcrossAliases(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(upstream.Close)
	cfg := circuitTestConfig([]testTarget{{name: "shared", url: upstream.URL, model: "physical-model"}}, 1, time.Minute)
	target := cfg.Models["public-alias"].Targets[0]
	cfg.Models["second-alias"] = config.ModelConfig{Targets: []config.TargetConfig{target}}
	gateway := circuitGatewayFromConfig(t, cfg, time.Now)

	if response := performChat(t, gateway, "public-alias"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("first alias status = %d, want 503", response.Code)
	}
	response := performChat(t, gateway, "second-alias")
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("second alias status = %d, want local 503", response.Code)
	}
	assertErrorCode(t, response, "circuit_open")
	if got := calls.Load(); got != 1 {
		t.Fatalf("shared target calls = %d, want 1", got)
	}
}

func TestCircuitBreakerHalfOpenProbeRecoversTarget(t *testing.T) {
	now := time.Unix(100, 0)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"id":"recovered"}`)
	}))
	t.Cleanup(upstream.Close)
	gateway := newCircuitTestGateway(t,
		[]testTarget{{name: "primary", url: upstream.URL, model: "model-a"}},
		1, 10*time.Second, func() time.Time { return now },
	)

	if response := performChat(t, gateway, "public-alias"); response.Code != http.StatusServiceUnavailable {
		t.Fatalf("failure status = %d, want 503", response.Code)
	}
	now = now.Add(10 * time.Second)
	if response := performChat(t, gateway, "public-alias"); response.Code != http.StatusOK {
		t.Fatalf("probe status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if response := performChat(t, gateway, "public-alias"); response.Code != http.StatusOK {
		t.Fatalf("closed status = %d, want 200", response.Code)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("upstream calls = %d, want failure, probe, and closed call", got)
	}
}

func TestRateLimitResponseDoesNotOpenCircuit(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(upstream.Close)
	gateway := newCircuitTestGateway(t,
		[]testTarget{{name: "primary", url: upstream.URL, model: "model-a"}},
		1, time.Minute, time.Now,
	)

	for requestIndex := 0; requestIndex < 2; requestIndex++ {
		if response := performChat(t, gateway, "public-alias"); response.Code != http.StatusTooManyRequests {
			t.Fatalf("request %d status = %d, want 429", requestIndex+1, response.Code)
		}
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("upstream calls = %d, want circuit to remain closed", got)
	}
}

func newCircuitTestGateway(t *testing.T, targets []testTarget, threshold int, openDuration time.Duration, now func() time.Time) *Gateway {
	t.Helper()
	return circuitGatewayFromConfig(t, circuitTestConfig(targets, threshold, openDuration), now)
}

func circuitTestConfig(targets []testTarget, threshold int, openDuration time.Duration) config.Config {
	providers := make(map[string]config.ProviderConfig, len(targets))
	configuredTargets := make([]config.TargetConfig, 0, len(targets))
	for _, target := range targets {
		providers[target.name] = config.ProviderConfig{Type: "openai", BaseURL: target.url}
		configuredTargets = append(configuredTargets, config.TargetConfig{Provider: target.name, Model: target.model})
	}
	return config.Config{
		Server:    config.ServerConfig{MaxBodyBytes: 1 << 20},
		Providers: providers,
		Models: map[string]config.ModelConfig{
			"public-alias": {Targets: configuredTargets},
		},
		Routing: config.RoutingConfig{
			ResponseHeaderTimeout: config.Duration(2 * time.Second),
			CircuitBreaker: config.CircuitBreakerConfig{
				FailureThreshold: threshold,
				OpenDuration:     config.Duration(openDuration),
			},
		},
	}
}

func circuitGatewayFromConfig(t *testing.T, cfg config.Config, now func() time.Time) *Gateway {
	t.Helper()
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	registry, err := catalog.BuiltIn()
	if err != nil {
		t.Fatalf("catalog.BuiltIn() error = %v", err)
	}
	return newWithCatalogAndClock(cfg, clients, registry, now)
}

func performChat(t *testing.T, gateway *Gateway, alias string) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"`+alias+`","messages":[{"role":"user","content":"hello"}]}`,
	))
	gateway.ChatCompletions(response, request)
	return response
}

func assertErrorCode(t *testing.T, response *httptest.ResponseRecorder, want string) {
	t.Helper()
	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if payload.Error.Code != want {
		t.Fatalf("error code = %q, want %q; body = %s", payload.Error.Code, want, response.Body.String())
	}
}
