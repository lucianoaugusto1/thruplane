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
	"testing"
	"time"

	"nexoroute/internal/config"
	"nexoroute/internal/gateway"
	"nexoroute/internal/provider"
)

func TestHealthIsPublicAndHasRequestID(t *testing.T) {
	t.Parallel()

	handler := New(config.Config{Server: config.ServerConfig{APIKey: "nexoroute-secret"}}, nil, discardLogger())
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

func TestProtectedRoutesRequireConfiguredBearerToken(t *testing.T) {
	t.Parallel()

	cfg := config.Config{
		Server: config.ServerConfig{APIKey: "nexoroute-secret"},
		Models: map[string]config.ModelConfig{"fast": {}},
	}
	handler := New(cfg, gateway.New(cfg, nil), discardLogger())

	for _, authorization := range []string{"", "Bearer wrong", "Basic nexoroute-secret"} {
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
	request.Header.Set("Authorization", "bearer nexoroute-secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("valid bearer status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
}

func TestPlaygroundIsDisabledByDefault(t *testing.T) {
	t.Parallel()

	handler := New(config.Config{}, nil, discardLogger())
	for _, path := range []string{"/playground", "/playground/", "/playground/app.js"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want 404", path, response.Code)
		}
	}
}

func TestPlaygroundServesOnlyEmbeddedAssetsWithSecurityHeaders(t *testing.T) {
	t.Parallel()

	cfg := config.Config{Server: config.ServerConfig{
		APIKey:     "nexoroute-secret",
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
		{path: "/playground/", contentType: "text/html; charset=utf-8", contains: "NexoRoute Playground"},
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
