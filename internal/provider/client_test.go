package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nexoroute/internal/config"
)

func TestNewClientsReuseTransportWithResponseHeaderTimeout(t *testing.T) {
	t.Parallel()

	wantTimeout := 1750 * time.Millisecond
	clients, err := NewClients(config.Config{
		Providers: map[string]config.ProviderConfig{
			"first":  {Type: "openai", BaseURL: "https://first.example"},
			"second": {Type: "ollama", BaseURL: "https://second.example"},
		},
		Routing: config.RoutingConfig{
			ResponseHeaderTimeout: config.Duration(wantTimeout),
		},
	})
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}

	first := clients["first"]
	second := clients["second"]
	if first == nil || second == nil {
		t.Fatalf("NewClients() = %#v, want both configured clients", clients)
	}
	if first.httpClient != second.httpClient {
		t.Fatal("configured providers do not reuse the same HTTP client")
	}
	if first.httpClient.Timeout != 0 {
		t.Errorf("http.Client.Timeout = %s, want 0", first.httpClient.Timeout)
	}

	transport, ok := first.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("HTTP transport type = %T, want *http.Transport", first.httpClient.Transport)
	}
	if transport.ResponseHeaderTimeout != wantTimeout {
		t.Errorf("ResponseHeaderTimeout = %s, want %s", transport.ResponseHeaderTimeout, wantTimeout)
	}
	if transport == http.DefaultTransport {
		t.Fatal("provider client mutated or reused http.DefaultTransport instead of a clone")
	}
	if transport.MaxIdleConns != 512 || transport.MaxIdleConnsPerHost != 64 {
		t.Errorf("idle connection limits = %d/%d, want 512/64", transport.MaxIdleConns, transport.MaxIdleConnsPerHost)
	}
	if !transport.ForceAttemptHTTP2 {
		t.Error("ForceAttemptHTTP2 = false, want true")
	}
}

func TestCompatibleProviderEndpointsAndAuthentication(t *testing.T) {
	tests := []struct {
		name       string
		typeName   string
		apiVersion string
		wantPath   string
		wantQuery  string
		wantAuth   string
		wantAPIKey string
	}{
		{"openai", "openai", "", "/v1/chat/completions", "", "Bearer secret", ""},
		{"compatible", "openai-compatible", "", "/v1/chat/completions", "", "Bearer secret", ""},
		{"inference", "nexoroute-inference", "", "/v1/chat/completions", "", "Bearer secret", ""},
		{"xai", "xai", "", "/v1/chat/completions", "", "Bearer secret", ""},
		{"ollama", "ollama", "", "/v1/chat/completions", "", "Bearer secret", ""},
		{"azure", "azure-openai", "2025-01-01-preview", "/openai/v1/chat/completions", "api-version=2025-01-01-preview", "", "secret"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			received := make(chan *http.Request, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received <- r.Clone(r.Context())
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"id":"direct"}`))
			}))
			t.Cleanup(server.Close)

			clients, err := NewClients(config.Config{
				Providers: map[string]config.ProviderConfig{"tested": {
					Type: tt.typeName, BaseURL: server.URL + "/root", APIKey: "secret", APIVersion: tt.apiVersion,
				}},
				Routing: config.RoutingConfig{ResponseHeaderTimeout: config.Duration(time.Second)},
			})
			if err != nil {
				t.Fatalf("NewClients() error = %v", err)
			}
			response, err := clients["tested"].Do(context.Background(), []byte(`{"model":"alias","messages":[]}`), "upstream")
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			response.Body.Close()
			got := <-received
			if got.URL.Path != "/root"+tt.wantPath || got.URL.RawQuery != tt.wantQuery {
				t.Errorf("URL = %s, want path %s query %s", got.URL.String(), "/root"+tt.wantPath, tt.wantQuery)
			}
			if got.Header.Get("Authorization") != tt.wantAuth || got.Header.Get("api-key") != tt.wantAPIKey {
				t.Errorf("auth headers = Authorization %q api-key %q", got.Header.Get("Authorization"), got.Header.Get("api-key"))
			}
		})
	}
}

func TestClientDoUsesConfiguredEndpointCredentialAndModel(t *testing.T) {
	t.Parallel()

	type receivedRequest struct {
		method        string
		path          string
		authorization string
		contentType   string
		body          []byte
	}
	received := make(chan receivedRequest, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		received <- receivedRequest{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
			contentType:   r.Header.Get("Content-Type"),
			body:          body,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"upstream-response"}`))
	}))
	t.Cleanup(server.Close)

	clients, err := NewClients(config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {
				Type:    "openai",
				BaseURL: server.URL + "/proxy-root/",
				APIKey:  "configured-secret",
			},
		},
		Routing: config.RoutingConfig{ResponseHeaderTimeout: config.Duration(time.Second)},
	})
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}

	body := []byte(`{"model":"public-alias","messages":[{"role":"user","content":"hello"}],"n":3,"custom":{"future":true}}`)
	response, err := clients["openai"].Do(context.Background(), body, "provider-model")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if response.StatusCode != http.StatusCreated {
		t.Errorf("response status = %d, want %d", response.StatusCode, http.StatusCreated)
	}
	if string(responseBody) != `{"id":"upstream-response"}` {
		t.Errorf("response body = %s, want upstream body", responseBody)
	}

	got := <-received
	if got.method != http.MethodPost {
		t.Errorf("method = %q, want POST", got.method)
	}
	if got.path != "/proxy-root/v1/chat/completions" {
		t.Errorf("path = %q, want /proxy-root/v1/chat/completions", got.path)
	}
	if got.authorization != "Bearer configured-secret" {
		t.Errorf("Authorization = %q, want configured provider credential", got.authorization)
	}
	if got.contentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got.contentType)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(got.body, &fields); err != nil {
		t.Fatalf("decode upstream body: %v", err)
	}
	if string(fields["model"]) != `"provider-model"` {
		t.Errorf("model = %s, want provider-model", fields["model"])
	}
	if string(fields["n"]) != "3" {
		t.Errorf("n = %s, want preserved value 3", fields["n"])
	}
	if string(fields["custom"]) != `{"future":true}` {
		t.Errorf("custom = %s, want preserved unknown object", fields["custom"])
	}
	if string(fields["messages"]) != `[{"role":"user","content":"hello"}]` {
		t.Errorf("messages = %s, want preserved messages", fields["messages"])
	}
}

func TestOllamaClientTranslatesCompletionTokenField(t *testing.T) {
	t.Parallel()

	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	clients, err := NewClients(config.Config{
		Providers: map[string]config.ProviderConfig{
			"ollama": {Type: "ollama", BaseURL: server.URL},
		},
		Routing: config.RoutingConfig{ResponseHeaderTimeout: config.Duration(time.Second)},
	})
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}

	body := []byte(`{"model":"alias","max_tokens":10,"max_completion_tokens":42,"metadata":{"trace":"keep"}}`)
	response, err := clients["ollama"].Do(context.Background(), body, "llama3.2")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	response.Body.Close()

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(<-received, &fields); err != nil {
		t.Fatalf("decode upstream body: %v", err)
	}
	if string(fields["model"]) != `"llama3.2"` {
		t.Errorf("model = %s, want llama3.2", fields["model"])
	}
	if _, ok := fields["max_completion_tokens"]; ok {
		t.Error("max_completion_tokens was not removed for Ollama")
	}
	if string(fields["max_tokens"]) != "42" {
		t.Errorf("max_tokens = %s, want translated value 42", fields["max_tokens"])
	}
	if string(fields["metadata"]) != `{"trace":"keep"}` {
		t.Errorf("metadata = %s, want preserved unknown object", fields["metadata"])
	}
}

func TestClientDoUsesRequestContext(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	t.Cleanup(server.Close)

	clients, err := NewClients(config.Config{
		Providers: map[string]config.ProviderConfig{
			"openai": {Type: "openai", BaseURL: server.URL},
		},
		Routing: config.RoutingConfig{ResponseHeaderTimeout: config.Duration(time.Second)},
	})
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	response, err := clients["openai"].Do(ctx, []byte(`{"model":"alias"}`), "provider-model")
	if response != nil {
		response.Body.Close()
		t.Fatal("Do() response is non-nil for a canceled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Do() error = %v, want context.Canceled", err)
	}
}
