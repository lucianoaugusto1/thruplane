package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"nexoroute/internal/config"
)

func TestCredentialTesterExecutesRequestScopedProvider(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer provider-secret-value" {
			t.Errorf("Authorization = %q, want provider bearer token", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte(`"model":"physical-model"`)) {
			t.Errorf("upstream body = %s, want physical model", body)
		}
		if bytes.Contains(body, []byte(`ignored-browser-model`)) {
			t.Errorf("upstream body retained untrusted inner model: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"chatcmpl-credential","choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	t.Cleanup(upstream.Close)

	cfg := credentialTestingConfig(upstream.URL, upstream.URL)

	payload := credentialEnvelope(t, upstream.URL, "provider-secret-value", false)
	request := httptest.NewRequest(http.MethodPost, "/playground/api/credentials/chat/completions", strings.NewReader(payload))
	response := httptest.NewRecorder()
	newCredentialTester(cfg).ChatCompletions(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}
	if got := response.Header().Get("X-NexoRoute-Provider"); got != "openai" {
		t.Errorf("X-NexoRoute-Provider = %q, want openai", got)
	}
	if got := response.Header().Get("X-NexoRoute-Model"); got != "physical-model" {
		t.Errorf("X-NexoRoute-Model = %q, want physical-model", got)
	}
}

func TestCredentialTesterRejectsDestinationBeforeNetwork(t *testing.T) {
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(upstream.Close)

	cfg := credentialTestingConfig(upstream.URL, "https://allowed.example.com")
	request := httptest.NewRequest(http.MethodPost, "/playground/api/credentials/chat/completions", strings.NewReader(
		credentialEnvelope(t, upstream.URL, "do-not-leak-this-secret", false),
	))
	response := httptest.NewRecorder()
	newCredentialTester(cfg).ChatCompletions(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", response.Code, response.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
	if !strings.Contains(response.Body.String(), `"code":"base_url_not_allowed"`) {
		t.Errorf("body = %s, want stable error code", response.Body.String())
	}
	if strings.Contains(response.Body.String(), "do-not-leak-this-secret") || strings.Contains(response.Body.String(), upstream.URL) {
		t.Errorf("body disclosed submitted credential or URL: %s", response.Body.String())
	}
}

func TestCredentialTesterValidatesProviderContract(t *testing.T) {
	cfg := testConfig("http://localhost:11434", "gateway-secret")
	tester := newCredentialTester(cfg)
	tests := []struct {
		name     string
		provider credentialProvider
		wantCode string
	}{
		{name: "unsupported", provider: credentialProvider{Type: "unknown"}, wantCode: "unsupported_provider"},
		{name: "API key", provider: credentialProvider{Type: "openai"}, wantCode: "provider_api_key_required"},
		{name: "custom URL", provider: credentialProvider{Type: "openai-compatible", APIKey: "secret"}, wantCode: "base_url_required"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tester.providerConfig(tt.provider)
			if err == nil || err.code != tt.wantCode {
				t.Fatalf("providerConfig() error = %#v, want code %q", err, tt.wantCode)
			}
		})
	}
}

func TestCredentialTesterRejectsUnknownEnvelopeFieldsWithoutEchoingSecrets(t *testing.T) {
	cfg := testConfig("http://localhost:11434", "gateway-secret")
	request := httptest.NewRequest(http.MethodPost, "/playground/api/credentials/chat/completions", strings.NewReader(
		`{"provider":{"type":"openai","api_key":"secret-value","unexpected":"private-value"},"model":"model","request":{}}`,
	))
	response := httptest.NewRecorder()
	newCredentialTester(cfg).ChatCompletions(response, request)

	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"invalid_json"`) {
		t.Fatalf("response = %d %s, want invalid_json", response.Code, response.Body.String())
	}
	for _, secret := range []string{"secret-value", "private-value", "unexpected"} {
		if strings.Contains(response.Body.String(), secret) {
			t.Errorf("response disclosed %q: %s", secret, response.Body.String())
		}
	}
}

func credentialEnvelope(t *testing.T, baseURL, providerKey string, stream bool) string {
	t.Helper()
	payload := map[string]any{
		"provider": map[string]any{
			"type":     "openai",
			"base_url": baseURL,
			"api_key":  providerKey,
		},
		"model": "physical-model",
		"request": map[string]any{
			"model":    "ignored-browser-model",
			"messages": []map[string]any{{"role": "user", "content": "hello"}},
			"stream":   stream,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func credentialTestingConfig(providerBaseURL string, allowedBaseURLs ...string) config.Config {
	cfg := testConfig(providerBaseURL, "gateway-secret")
	cfg.Server.Address = ":8080"
	cfg.Server.ReadHeaderTimeout = config.Duration(5 * time.Second)
	cfg.Server.ShutdownTimeout = config.Duration(10 * time.Second)
	cfg.Server.Playground.Enabled = true
	cfg.Server.Playground.CredentialTesting.Enabled = true
	cfg.Server.Playground.CredentialTesting.AllowedBaseURLs = allowedBaseURLs
	cfg.Routing.Retry.BaseDelay = config.Duration(200 * time.Millisecond)
	cfg.Routing.Retry.MaxDelay = config.Duration(5 * time.Second)
	cfg.Routing.Retry.Budget = config.Duration(15 * time.Second)
	cfg.Routing.CircuitBreaker.OpenDuration = config.Duration(30 * time.Second)
	return cfg
}
