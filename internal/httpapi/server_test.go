package httpapi

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lucianoaugusto1/thruplane/internal/config"
	"github.com/lucianoaugusto1/thruplane/internal/gateway"
	"github.com/lucianoaugusto1/thruplane/internal/provider"
	"github.com/lucianoaugusto1/thruplane/internal/telemetry"
)

func TestHealthIsPublicAndHasRequestID(t *testing.T) {
	t.Parallel()

	handler := New(config.Config{Server: config.ServerConfig{APIKey: "thruplane-secret"}}, nil, discardLogger())
	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("X-Request-Id"); got == "" {
		t.Error("X-Request-Id is empty")
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := strings.TrimSpace(response.Body.String()); got != `{"status":"ok"}` {
		t.Errorf("body = %q, want health response", got)
	}
}

func TestReadinessIsPublicAndReflectsOpenTargets(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	t.Cleanup(upstream.Close)
	cfg := testConfig(upstream.URL, "thruplane-secret")
	cfg.Routing.CircuitBreaker = config.CircuitBreakerConfig{
		FailureThreshold: 1,
		OpenDuration:     config.Duration(time.Minute),
	}
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	handler := New(cfg, gateway.New(cfg, clients), discardLogger())

	ready := httptest.NewRecorder()
	handler.ServeHTTP(ready, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assertReadinessResponse(t, ready, http.StatusOK, "ready", 1, 1, 0)

	chatRequest := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"fast","messages":[{"role":"user","content":"hello"}]}`,
	))
	chatRequest.Header.Set("Authorization", "Bearer thruplane-secret")
	handler.ServeHTTP(httptest.NewRecorder(), chatRequest)

	notReady := httptest.NewRecorder()
	handler.ServeHTTP(notReady, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	assertReadinessResponse(t, notReady, http.StatusServiceUnavailable, "not_ready", 1, 0, 1)
	for _, sensitive := range []string{"provider", "provider-model", upstream.URL, "thruplane-secret"} {
		if strings.Contains(notReady.Body.String(), sensitive) {
			t.Errorf("readiness body contains sensitive target detail %q: %s", sensitive, notReady.Body.String())
		}
	}

	health := httptest.NewRecorder()
	handler.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if health.Code != http.StatusOK {
		t.Fatalf("health status after open circuit = %d, want 200", health.Code)
	}
}

func TestProtectedRoutesRequireConfiguredBearerToken(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Server: config.ServerConfig{APIKey: "thruplane-secret"},
		Models: map[string]config.ModelConfig{"fast": {}},
	}
	handler := New(cfg, gateway.New(cfg, nil), discardLogger())

	for _, authorization := range []string{"", "Bearer wrong", "Basic thruplane-secret"} {
		request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
		request.Header.Set("Authorization", authorization)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusUnauthorized {
			t.Errorf("Authorization %q status = %d, want 401", authorization, response.Code)
		}
		assertAPIError(t, response.Body.Bytes())
	}

	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "bearer thruplane-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("valid bearer status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}

func TestPlaygroundIsDisabledByDefault(t *testing.T) {
	t.Parallel()

	handler := New(config.Config{}, nil, discardLogger())
	for _, path := range []string{"/playground", "/playground/", "/playground/app.js", "/playground/config.json", credentialChatPath} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, response.Code)
		}
	}
}

func TestCredentialTestingRouteRequiresExplicitEnablementAndAuthentication(t *testing.T) {
	t.Parallel()

	cfg := credentialTestingConfig("http://localhost:11434", "http://localhost:11434")
	payload := credentialEnvelope(t, "http://localhost:11434", "provider-secret", false)

	disabled := cfg
	disabled.Server.Playground.CredentialTesting.Enabled = false
	disabledHandler := New(disabled, nil, discardLogger())
	disabledResponse := httptest.NewRecorder()
	disabledHandler.ServeHTTP(disabledResponse, httptest.NewRequest(http.MethodPost, credentialChatPath, strings.NewReader(payload)))
	if disabledResponse.Code != http.StatusNotFound {
		t.Fatalf("disabled route status = %d, want 404", disabledResponse.Code)
	}

	handler := New(cfg, nil, discardLogger())
	unauthorized := httptest.NewRecorder()
	handler.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodPost, credentialChatPath, strings.NewReader(payload)))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want 401", unauthorized.Code)
	}

	configResponse := httptest.NewRecorder()
	handler.ServeHTTP(configResponse, httptest.NewRequest(http.MethodGet, "/playground/config.json", nil))
	if configResponse.Code != http.StatusOK || strings.TrimSpace(configResponse.Body.String()) != `{"credential_testing":true}` {
		t.Fatalf("playground config = %d %s, want enabled", configResponse.Code, configResponse.Body.String())
	}
}

func TestCredentialTestingRouteUsesFixedMetricsAndRedactedLogs(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-safe","choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	t.Cleanup(upstream.Close)

	cfg := credentialTestingConfig(upstream.URL, upstream.URL)
	cfg.Server.Metrics.Enabled = true
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	handler := New(cfg, nil, logger)
	payload := credentialEnvelope(t, upstream.URL, "provider-secret-never-log", false)
	payload = strings.Replace(payload, "physical-model", "customer-private-model-id", 1)
	request := httptest.NewRequest(http.MethodPost, credentialChatPath, strings.NewReader(payload))
	request.Header.Set("Authorization", "Bearer gateway-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || calls.Load() != 1 {
		t.Fatalf("credential response = %d %s; calls = %d", response.Code, response.Body.String(), calls.Load())
	}

	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK {
		t.Fatalf("metrics status = %d", metrics.Code)
	}
	if !strings.Contains(metrics.Body.String(), `route="/playground/api/credentials/chat/completions",status="200"`) {
		t.Errorf("metrics missing fixed credential route: %s", metrics.Body.String())
	}
	combined := logs.String() + metrics.Body.String()
	for _, sensitive := range []string{"provider-secret-never-log", "customer-private-model-id", upstream.URL} {
		if strings.Contains(combined, sensitive) {
			t.Errorf("logs or metrics contain sensitive value %q", sensitive)
		}
	}
}

func TestMetricsAreDisabledByDefault(t *testing.T) {
	t.Parallel()

	handler := New(config.Config{}, nil, discardLogger())
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("GET /metrics status = %d, want 404", response.Code)
	}
}

func TestMetricsExposeBoundedOperationalDataWithoutSecrets(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-metrics"}`)
	}))
	t.Cleanup(upstream.Close)

	cfg := testConfig(upstream.URL, "gateway-secret-value")
	cfg.Server.Metrics.Enabled = true
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	handler := NewWithBuildInfo(cfg, gateway.New(cfg, clients), discardLogger(), telemetry.BuildInfo{
		Version:  "v0.1.0-beta.1",
		Revision: "abc123",
		Date:     "2026-09-26T00:00:00Z",
	})

	chat := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"fast","messages":[{"role":"user","content":"prompt-secret-value"}]}`,
	))
	chat.Header.Set("Authorization", "Bearer gateway-secret-value")
	chat.Header.Set("X-Private-Header", "header-secret-value")
	chatResponse := httptest.NewRecorder()
	handler.ServeHTTP(chatResponse, chat)
	if chatResponse.Code != http.StatusOK {
		t.Fatalf("chat status = %d, want 200; body = %s", chatResponse.Code, chatResponse.Body.String())
	}

	unknown := httptest.NewRecorder()
	handler.ServeHTTP(unknown, httptest.NewRequest(http.MethodGet, "/private-path-value", nil))

	metrics := httptest.NewRecorder()
	handler.ServeHTTP(metrics, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if metrics.Code != http.StatusOK {
		t.Fatalf("metrics status = %d, want 200; body = %s", metrics.Code, metrics.Body.String())
	}
	assertContainsAll(t, metrics.Body.String(),
		`thruplane_build_info{build_date="2026-09-26T00:00:00Z",revision="abc123",version="v0.1.0-beta.1"} 1`,
		`thruplane_http_requests_total{method="GET",route="unmatched",status="404"} 1`,
		`thruplane_http_requests_total{method="POST",route="/v1/chat/completions",status="200"} 1`,
		`thruplane_route_selections_total{model="provider-model",provider="provider"} 1`,
		`thruplane_request_attempts_sum{model="provider-model",provider="provider"} 1`,
		`thruplane_request_fallbacks_sum{model="provider-model",provider="provider"} 0`,
		`thruplane_targets{state="available"} 1`,
		`thruplane_targets{state="total"} 1`,
	)
	for _, sensitive := range []string{
		"prompt-secret-value", "gateway-secret-value", "header-secret-value",
		"private-path-value", "Authorization", "X-Private-Header",
	} {
		if strings.Contains(metrics.Body.String(), sensitive) {
			t.Errorf("metrics contain sensitive value %q", sensitive)
		}
	}
}

func TestPlaygroundServesOnlyEmbeddedAssetsWithSecurityHeaders(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Server: config.ServerConfig{
		APIKey:     "thruplane-secret",
		Playground: config.PlaygroundConfig{Enabled: true},
	}}
	handler := New(cfg, nil, discardLogger())

	redirect := httptest.NewRecorder()
	handler.ServeHTTP(redirect, httptest.NewRequest(http.MethodGet, "/playground", nil))
	if redirect.Code != http.StatusPermanentRedirect || redirect.Header().Get("Location") != "/playground/" {
		t.Fatalf("redirect = %d %q", redirect.Code, redirect.Header().Get("Location"))
	}

	tests := []struct {
		path        string
		contentType string
		contains    string
	}{
		{path: "/playground/", contentType: "text/html; charset=utf-8", contains: "Thruplane Playground"},
		{path: "/playground/app.js", contentType: "text/javascript; charset=utf-8", contains: "use strict"},
		{path: "/playground/styles.css", contentType: "text/css; charset=utf-8", contains: ":root"},
	}
	for _, test := range tests {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, test.path, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want 200", test.path, response.Code)
		}
		if got := response.Header().Get("Content-Type"); got != test.contentType {
			t.Errorf("GET %s Content-Type = %q, want %q", test.path, got, test.contentType)
		}
		if !strings.Contains(response.Body.String(), test.contains) {
			t.Errorf("GET %s body missing %q", test.path, test.contains)
		}
		for name, want := range map[string]string{
			"Cache-Control":           "no-store",
			"Content-Security-Policy": "default-src 'self'",
			"Referrer-Policy":         "no-referrer",
			"X-Content-Type-Options":  "nosniff",
			"X-Frame-Options":         "DENY",
		} {
			if got := response.Header().Get(name); !strings.Contains(got, want) {
				t.Errorf("GET %s %s = %q, want %q", test.path, name, got, want)
			}
		}
	}

	missing := httptest.NewRecorder()
	handler.ServeHTTP(missing, httptest.NewRequest(http.MethodGet, "/playground/private.txt", nil))
	if missing.Code != http.StatusNotFound {
		t.Fatalf("unknown playground asset status = %d, want 404", missing.Code)
	}

	models := httptest.NewRecorder()
	handler.ServeHTTP(models, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if models.Code != http.StatusUnauthorized {
		t.Fatalf("models status = %d, want 401", models.Code)
	}
}

func TestServerRoutesChatCompletions(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream request: %v", err)
		}
		if !bytes.Contains(body, []byte(`"model":"provider-model"`)) {
			t.Errorf("upstream body = %s, want provider model", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-1"}`)
	}))
	t.Cleanup(upstream.Close)

	cfg := testConfig(upstream.URL, "")
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	handler := New(cfg, gateway.New(cfg, clients), discardLogger())
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(
		`{"model":"fast","messages":[{"role":"user","content":"hello"}]}`,
	))
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"id":"chatcmpl-1"}` {
		t.Fatalf("response = %d %s, want upstream response", response.Code, response.Body.String())
	}
}

func TestAccessLogHasOperationalFieldsWithoutSecrets(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	cfg := config.Config{
		Server: config.ServerConfig{APIKey: "top-secret"},
		Models: map[string]config.ModelConfig{"fast": {}},
	}
	handler := New(cfg, gateway.New(cfg, nil), logger)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer top-secret")
	request.Header.Set("X-Prompt", "private prompt")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	line := logs.String()
	for _, field := range []string{`"request_id":`, `"method":"GET"`, `"path":"/v1/models"`, `"status":200`, `"duration_ms":`} {
		if !strings.Contains(line, field) {
			t.Errorf("log = %s, want field %s", line, field)
		}
	}
	for _, secret := range []string{"top-secret", "private prompt", "Authorization", "X-Prompt"} {
		if strings.Contains(line, secret) {
			t.Errorf("log contains sensitive value %q: %s", secret, line)
		}
	}
}

func TestServerRecoversPanicsAsOpenAIError(t *testing.T) {
	t.Parallel()

	handler := New(config.Config{}, nil, discardLogger())
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", response.Code, response.Body.String())
	}
	assertAPIError(t, response.Body.Bytes())
}

func TestServerPreservesIncrementalSSEFlush(t *testing.T) {
	release := make(chan struct{})
	var once sync.Once
	t.Cleanup(func() { once.Do(func() { close(release) }) })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-release
		_, _ = io.WriteString(w, "data: second\n\n")
		w.(http.Flusher).Flush()
	}))
	t.Cleanup(upstream.Close)

	cfg := testConfig(upstream.URL, "")
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	server := httptest.NewServer(New(cfg, gateway.New(cfg, clients), discardLogger()))
	t.Cleanup(server.Close)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/chat/completions", strings.NewReader(
		`{"model":"fast","messages":[],"stream":true}`,
	))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("send request: %v", err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	first := make(chan string, 1)
	go func() {
		line, _ := reader.ReadString('\n')
		first <- line
	}()
	select {
	case line := <-first:
		if line != "data: first\n" {
			t.Fatalf("first line = %q, want first SSE event", line)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("middleware buffered the first SSE event")
	}
	once.Do(func() { close(release) })
}

func testConfig(baseURL, apiKey string) config.Config {
	return config.Config{
		Server: config.ServerConfig{APIKey: apiKey, MaxBodyBytes: 1 << 20},
		Providers: map[string]config.ProviderConfig{
			"provider": {Type: "openai", BaseURL: baseURL},
		},
		Models: map[string]config.ModelConfig{
			"fast": {Targets: []config.TargetConfig{{Provider: "provider", Model: "provider-model"}}},
		},
		Routing: config.RoutingConfig{ResponseHeaderTimeout: config.Duration(2 * time.Second)},
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func assertAPIError(t *testing.T, body []byte) {
	t.Helper()
	var payload struct {
		Error struct {
			Message string          `json:"message"`
			Type    string          `json:"type"`
			Param   json.RawMessage `json:"param"`
			Code    string          `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode error response %q: %v", body, err)
	}
	if payload.Error.Message == "" || payload.Error.Type == "" || payload.Error.Code == "" {
		t.Errorf("error response = %#v, want populated fields", payload.Error)
	}
	if len(payload.Error.Param) == 0 {
		t.Errorf("error response = %#v, want param field", payload.Error)
	}
}

func assertReadinessResponse(t *testing.T, response *httptest.ResponseRecorder, wantStatus int, wantState string, total, available, open int) {
	t.Helper()
	if response.Code != wantStatus {
		t.Fatalf("readiness status = %d, want %d; body = %s", response.Code, wantStatus, response.Body.String())
	}
	var payload struct {
		Status  string `json:"status"`
		Targets struct {
			Total     int `json:"total"`
			Available int `json:"available"`
			Open      int `json:"open"`
			HalfOpen  int `json:"half_open"`
		} `json:"targets"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode readiness response: %v", err)
	}
	if payload.Status != wantState || payload.Targets.Total != total || payload.Targets.Available != available || payload.Targets.Open != open {
		t.Fatalf("readiness payload = %#v, want status %q total %d available %d open %d", payload, wantState, total, available, open)
	}
}

func assertContainsAll(t *testing.T, value string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			t.Errorf("value missing %q:\n%s", fragment, value)
		}
	}
}
