package gateway

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"nexoroute/internal/catalog"
	"nexoroute/internal/config"
	"nexoroute/internal/provider"
)

func TestRetryDelayPrefersRetryAfterAndFallsBackToExponentialJitter(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	policy := config.RetryConfig{
		BaseDelay: config.Duration(200 * time.Millisecond),
		MaxDelay:  config.Duration(5 * time.Second),
		Budget:    config.Duration(15 * time.Second),
	}
	response := &http.Response{Header: http.Header{"Retry-After": []string{"3"}}}
	if got := retryDelay(policy, 4, response, now, func(time.Duration) time.Duration {
		t.Fatal("jitter called for valid Retry-After")
		return 0
	}); got != 3*time.Second {
		t.Fatalf("retry delay = %s, want 3s", got)
	}

	response.Header.Set("Retry-After", "invalid")
	if got := retryDelay(policy, 2, response, now, func(delay time.Duration) time.Duration {
		if delay != 800*time.Millisecond {
			t.Fatalf("unjittered delay = %s, want 800ms", delay)
		}
		return 700 * time.Millisecond
	}); got != 700*time.Millisecond {
		t.Fatalf("retry delay = %s, want injected jitter result", got)
	}
}

func TestRetryBudgetDoesNotStartAttemptAtDeadline(t *testing.T) {
	start := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if retryFitsBudget(start, start.Add(4*time.Second), 5*time.Second, time.Second) {
		t.Fatal("retry delay equal to remaining budget must not start another attempt")
	}
	if !retryFitsBudget(start, start.Add(3*time.Second), 5*time.Second, time.Second) {
		t.Fatal("retry delay shorter than remaining budget should fit")
	}
}

func TestRetryWaitStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- waitContext(ctx, time.Minute) }()
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("waitContext() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("retry wait did not stop after cancellation")
	}
}

func TestClassifyResponseSkipsRetryForPermanentQuotaAndPreservesBody(t *testing.T) {
	tests := []struct {
		provider string
		body     string
	}{
		{provider: "openai", body: `{"error":{"type":"insufficient_quota","code":"insufficient_quota"}}`},
		{provider: "anthropic", body: `{"error":{"type":"rate_limit_error","details":{"error_code":"enforced_spend_limit_reached"}}}`},
		{provider: "gemini", body: `{"error":{"status":"RESOURCE_EXHAUSTED","details":[{"reason":"quota_exceeded"}]}}`},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			response := &http.Response{
				StatusCode: http.StatusTooManyRequests,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
			}
			decision := classifyResponse(tt.provider, response)
			if decision.retryTarget || !decision.fallback {
				t.Fatalf("decision = %#v, want fallback without same-target retry", decision)
			}
			got, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read reconstructed body: %v", err)
			}
			if string(got) != tt.body {
				t.Fatalf("body = %q, want byte-preserved %q", got, tt.body)
			}
		})
	}
}

func TestClassifyResponseRetriesUnknownRateLimit(t *testing.T) {
	response := &http.Response{
		StatusCode: http.StatusTooManyRequests,
		Body:       io.NopCloser(strings.NewReader(`{"error":{"code":"slow_down"}}`)),
	}
	decision := classifyResponse("openai", response)
	if !decision.retryTarget || !decision.fallback {
		t.Fatalf("decision = %#v, want temporary retry and fallback", decision)
	}
}

func TestChatCompletionsHonorsRetryAfterBeforeRetry(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = io.WriteString(w, `{"error":{"code":"slow_down"}}`)
			return
		}
		_, _ = io.WriteString(w, `{"id":"recovered"}`)
	}))
	t.Cleanup(upstream.Close)

	cfg := rateLimitTestConfig(upstream.URL)
	cfg.Routing.Retries = 1
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	registry, err := catalog.BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn() error = %v", err)
	}
	gateway := newWithCatalogAndClock(cfg, clients, registry, func() time.Time { return now })
	var waited time.Duration
	gateway.wait = func(ctx context.Context, delay time.Duration) error {
		waited += delay
		now = now.Add(delay)
		return ctx.Err()
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))
	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"id":"recovered"}` {
		t.Fatalf("response = %d %s, want recovered response", response.Code, response.Body.String())
	}
	if waited != 2*time.Second || calls.Load() != 2 {
		t.Fatalf("waited %s with %d calls, want 2s and 2 calls", waited, calls.Load())
	}
}

func TestChatCompletionsDoesNotTruncateRetryAfterToBudget(t *testing.T) {
	var firstCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		w.Header().Set("Retry-After", "10")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"code":"slow_down"}}`)
	}))
	t.Cleanup(first.Close)
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"fallback"}`)
	}))
	t.Cleanup(second.Close)

	gateway := newTestGateway(t, []testTarget{
		{name: "first", url: first.URL, model: "model-a"},
		{name: "second", url: second.URL, model: "model-b"},
	}, 2, 1<<20)
	gateway.settings.routing.Retry = config.RetryConfig{
		BaseDelay: config.Duration(100 * time.Millisecond),
		MaxDelay:  config.Duration(time.Second),
		Budget:    config.Duration(5 * time.Second),
	}
	gateway.wait = func(context.Context, time.Duration) error {
		t.Fatal("gateway waited after Retry-After exceeded retry budget")
		return nil
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))
	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"id":"fallback"}` {
		t.Fatalf("response = %d %s, want fallback success", response.Code, response.Body.String())
	}
	if firstCalls.Load() != 1 {
		t.Fatalf("first calls = %d, want no retry", firstCalls.Load())
	}
}

func TestChatCompletionsSkipsSameTargetRetryForPermanentQuota(t *testing.T) {
	var firstCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = io.WriteString(w, `{"error":{"type":"insufficient_quota","code":"insufficient_quota"}}`)
	}))
	t.Cleanup(first.Close)
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"fallback"}`)
	}))
	t.Cleanup(second.Close)

	gateway := newTestGateway(t, []testTarget{
		{name: "first", url: first.URL, model: "model-a"},
		{name: "second", url: second.URL, model: "model-b"},
	}, 3, 1<<20)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))
	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"id":"fallback"}` {
		t.Fatalf("response = %d %s, want fallback success", response.Code, response.Body.String())
	}
	if firstCalls.Load() != 1 {
		t.Fatalf("permanent quota target calls = %d, want 1", firstCalls.Load())
	}
}

func TestChatCompletionsSharesConcurrencyAcrossAliases(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		once.Do(func() { close(started) })
		<-release
		_, _ = io.WriteString(w, `{"id":"first"}`)
	}))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() {
		select {
		case <-release:
		default:
			close(release)
		}
	})

	cfg := rateLimitTestConfig(upstream.URL)
	target := cfg.Models["public-alias"].Targets[0]
	target.RateLimit.MaxConcurrency = 1
	cfg.Models["public-alias"] = config.ModelConfig{Targets: []config.TargetConfig{target}}
	cfg.Models["second-alias"] = config.ModelConfig{Targets: []config.TargetConfig{target}}
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	gateway := New(cfg, clients)

	firstDone := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))
		gateway.ChatCompletions(response, request)
		firstDone <- response
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first upstream request did not start")
	}

	secondResponse := httptest.NewRecorder()
	secondRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"second-alias","messages":[{"role":"user","content":"hello"}]}`,
	))
	gateway.ChatCompletions(secondResponse, secondRequest)
	if secondResponse.Code != http.StatusTooManyRequests {
		t.Fatalf("second status = %d, want 429; body = %s", secondResponse.Code, secondResponse.Body.String())
	}
	assertHeader(t, secondResponse.Header(), "X-NexoRoute-RateLimit-Reason", "concurrency")
	if !strings.Contains(secondResponse.Body.String(), "gateway_rate_limited") {
		t.Fatalf("second body = %s, want gateway rate-limit code", secondResponse.Body.String())
	}

	close(release)
	select {
	case response := <-firstDone:
		if response.Code != http.StatusOK {
			t.Fatalf("first status = %d, want 200", response.Code)
		}
	case <-time.After(time.Second):
		t.Fatal("first request did not finish")
	}
}

func TestChatCompletionsHoldsConcurrencyPermitUntilStreamEnds(t *testing.T) {
	release := make(chan struct{})
	var releaseOnce sync.Once
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "data: second\n\n")
	}))
	t.Cleanup(upstream.Close)
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })

	cfg := rateLimitTestConfig(upstream.URL)
	target := cfg.Models["public-alias"].Targets[0]
	target.RateLimit.MaxConcurrency = 1
	cfg.Models["public-alias"] = config.ModelConfig{Targets: []config.TargetConfig{target}}
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(New(cfg, clients).ChatCompletions))
	t.Cleanup(server.Close)

	firstRequest, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(validChatBody(true)))
	if err != nil {
		t.Fatalf("create first request: %v", err)
	}
	firstResponse, err := server.Client().Do(firstRequest)
	if err != nil {
		t.Fatalf("first request: %v", err)
	}
	defer firstResponse.Body.Close()
	line, err := bufio.NewReader(firstResponse.Body).ReadString('\n')
	if err != nil || line != "data: first\n" {
		t.Fatalf("first stream line = %q, error %v", line, err)
	}

	secondResponse, err := server.Client().Post(server.URL, "application/json", strings.NewReader(validChatBody(false)))
	if err != nil {
		t.Fatalf("second request: %v", err)
	}
	secondBody, readErr := io.ReadAll(secondResponse.Body)
	secondResponse.Body.Close()
	if readErr != nil {
		t.Fatalf("read second response: %v", readErr)
	}
	if secondResponse.StatusCode != http.StatusTooManyRequests || !strings.Contains(string(secondBody), "gateway_rate_limited") {
		t.Fatalf("second response = %d %s, want local 429 while stream is open", secondResponse.StatusCode, secondBody)
	}

	releaseOnce.Do(func() { close(release) })
}

func TestChatCompletionsEnforcesLocalRequestRate(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"id":"ok"}`)
	}))
	t.Cleanup(upstream.Close)
	cfg := rateLimitTestConfig(upstream.URL)
	target := cfg.Models["public-alias"].Targets[0]
	target.RateLimit = config.RateLimitConfig{RequestsPerMinute: 60, Burst: 1}
	cfg.Models["public-alias"] = config.ModelConfig{Targets: []config.TargetConfig{target}}
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	gateway := New(cfg, clients)

	for attempt := 0; attempt < 2; attempt++ {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))
		gateway.ChatCompletions(response, request)
		if attempt == 0 && response.Code != http.StatusOK {
			t.Fatalf("first status = %d, want 200", response.Code)
		}
		if attempt == 1 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("second status = %d, want 429", response.Code)
			}
			assertHeader(t, response.Header(), "X-NexoRoute-RateLimit-Reason", "request_rate")
			assertHeader(t, response.Header(), "Retry-After", "1")
		}
	}
}

func rateLimitTestConfig(baseURL string) config.Config {
	return config.Config{
		Server: config.ServerConfig{MaxBodyBytes: 1 << 20},
		Providers: map[string]config.ProviderConfig{
			"provider": {Type: "openai", BaseURL: baseURL},
		},
		Models: map[string]config.ModelConfig{
			"public-alias": {Targets: []config.TargetConfig{{Provider: "provider", Model: "provider-model"}}},
		},
		Routing: config.RoutingConfig{
			ResponseHeaderTimeout: config.Duration(2 * time.Second),
			Retry: config.RetryConfig{
				BaseDelay: config.Duration(200 * time.Millisecond),
				MaxDelay:  config.Duration(5 * time.Second),
				Budget:    config.Duration(15 * time.Second),
			},
		},
	}
}
