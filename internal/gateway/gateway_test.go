package gateway

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gollm-gateway/internal/config"
	"gollm-gateway/internal/provider"
)

func TestChatCompletionsRewritesOnlyModelAndRelaysResponse(t *testing.T) {
	t.Parallel()

	received := make(chan []byte, 1)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		received <- body
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Retry-After", "3")
		w.Header().Set("X-RateLimit-Limit-Requests", "100")
		w.Header().Set("X-Request-Id", "upstream-request")
		w.Header().Set("X-Ignored", "secret-metadata")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"completion-1","custom":"response-field"}`))
	}))
	t.Cleanup(upstream.Close)

	gateway := newTestGateway(t, []testTarget{{name: "primary", url: upstream.URL, model: "provider-model"}}, 0, 1<<20)
	body := `{"model":"public-alias","messages":[{"role":"user","content":"hello"}],"temperature":0.25,"custom":{"future":true}}`
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	response := httptest.NewRecorder()

	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body = %s", response.Code, http.StatusCreated, response.Body.String())
	}
	if got := response.Body.String(); got != `{"id":"completion-1","custom":"response-field"}` {
		t.Errorf("body = %s, want opaque upstream body", got)
	}
	assertHeader(t, response.Header(), "Content-Type", "application/json; charset=utf-8")
	assertHeader(t, response.Header(), "Cache-Control", "no-cache")
	assertHeader(t, response.Header(), "Retry-After", "3")
	assertHeader(t, response.Header(), "X-RateLimit-Limit-Requests", "100")
	assertHeader(t, response.Header(), "X-Upstream-Request-Id", "upstream-request")
	if got := response.Header().Get("X-Ignored"); got != "" {
		t.Errorf("X-Ignored = %q, want omitted", got)
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(<-received, &fields); err != nil {
		t.Fatalf("decode upstream body: %v", err)
	}
	if got := string(fields["model"]); got != `"provider-model"` {
		t.Errorf("model = %s, want provider-model", got)
	}
	if got := string(fields["messages"]); got != `[{"role":"user","content":"hello"}]` {
		t.Errorf("messages = %s, want preserved", got)
	}
	if got := string(fields["temperature"]); got != "0.25" {
		t.Errorf("temperature = %s, want preserved", got)
	}
	if got := string(fields["custom"]); got != `{"future":true}` {
		t.Errorf("custom = %s, want preserved", got)
	}
}

func TestChatCompletionsRejectsInvalidRequestsWithoutUpstream(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	gateway := newTestGateway(t, []testTarget{{name: "primary", url: upstream.URL, model: "provider-model"}}, 0, 96)
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "invalid JSON", body: `{`, wantStatus: http.StatusBadRequest},
		{name: "non-object JSON", body: `[]`, wantStatus: http.StatusBadRequest},
		{name: "missing model", body: `{"messages":[]}`, wantStatus: http.StatusBadRequest},
		{name: "model is not string", body: `{"model":7,"messages":[]}`, wantStatus: http.StatusBadRequest},
		{name: "missing messages", body: `{"model":"public-alias"}`, wantStatus: http.StatusBadRequest},
		{name: "n is not one", body: `{"model":"public-alias","messages":[],"n":2}`, wantStatus: http.StatusBadRequest},
		{name: "unknown alias", body: `{"model":"unknown","messages":[]}`, wantStatus: http.StatusNotFound},
		{name: "body too large", body: `{"model":"public-alias","messages":[],"padding":"` + strings.Repeat("x", 96) + `"}`, wantStatus: http.StatusRequestEntityTooLarge},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
			response := httptest.NewRecorder()

			gateway.ChatCompletions(response, request)

			if response.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tt.wantStatus, response.Body.String())
			}
			assertOpenAIError(t, response)
		})
	}

	if got := calls.Load(); got != 0 {
		t.Fatalf("upstream calls = %d, want 0", got)
	}
}

func TestChatCompletionsRetriesTargetThenFallsBackInOrder(t *testing.T) {
	t.Parallel()

	var firstCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"temporarily unavailable"}}`))
	}))
	t.Cleanup(first.Close)

	var secondCalls atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secondCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"fallback"}`))
	}))
	t.Cleanup(second.Close)

	gateway := newTestGateway(t, []testTarget{
		{name: "first", url: first.URL, model: "model-a"},
		{name: "second", url: second.URL, model: "model-b"},
	}, 2, 1<<20)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))

	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"id":"fallback"}` {
		t.Fatalf("response = %d %s, want fallback success", response.Code, response.Body.String())
	}
	if got := firstCalls.Load(); got != 3 {
		t.Errorf("first target calls = %d, want 3 (initial + 2 retries)", got)
	}
	if got := secondCalls.Load(); got != 1 {
		t.Errorf("second target calls = %d, want 1", got)
	}
}

func TestChatCompletionsFallsBackForEveryRetryableStatus(t *testing.T) {
	t.Parallel()

	statuses := []int{
		http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}
	for _, status := range statuses {
		status := status
		t.Run(http.StatusText(status), func(t *testing.T) {
			t.Parallel()
			var fallbackCalls atomic.Int32
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
			}))
			t.Cleanup(first.Close)
			second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fallbackCalls.Add(1)
				w.WriteHeader(http.StatusNoContent)
			}))
			t.Cleanup(second.Close)

			gateway := newTestGateway(t, []testTarget{
				{name: "first", url: first.URL, model: "model-a"},
				{name: "second", url: second.URL, model: "model-b"},
			}, 0, 1<<20)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))

			gateway.ChatCompletions(response, request)

			if response.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want fallback status %d", response.Code, http.StatusNoContent)
			}
			if got := fallbackCalls.Load(); got != 1 {
				t.Fatalf("fallback calls = %d, want 1", got)
			}
		})
	}
}

func TestChatCompletionsDoesNotFallbackForNonRetryableStatus(t *testing.T) {
	t.Parallel()

	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"provider rejected request","type":"invalid_request_error"}}`))
	}))
	t.Cleanup(first.Close)
	var fallbackCalls atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(second.Close)

	gateway := newTestGateway(t, []testTarget{
		{name: "first", url: first.URL, model: "model-a"},
		{name: "second", url: second.URL, model: "model-b"},
	}, 3, 1<<20)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))

	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if !strings.Contains(response.Body.String(), "provider rejected request") {
		t.Errorf("body = %s, want relayed provider error", response.Body.String())
	}
	if got := fallbackCalls.Load(); got != 0 {
		t.Fatalf("fallback calls = %d, want 0", got)
	}
}

func TestChatCompletionsFallsBackAfterTransportErrors(t *testing.T) {
	t.Parallel()

	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unavailableURL := unavailable.URL
	unavailable.Close()

	var fallbackCalls atomic.Int32
	fallback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		_, _ = w.Write([]byte(`{"id":"recovered"}`))
	}))
	t.Cleanup(fallback.Close)

	gateway := newTestGateway(t, []testTarget{
		{name: "unavailable", url: unavailableURL, model: "model-a"},
		{name: "fallback", url: fallback.URL, model: "model-b"},
	}, 1, 1<<20)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))

	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusOK || response.Body.String() != `{"id":"recovered"}` {
		t.Fatalf("response = %d %s, want recovered response", response.Code, response.Body.String())
	}
	if got := fallbackCalls.Load(); got != 1 {
		t.Fatalf("fallback calls = %d, want 1", got)
	}
}

func TestChatCompletionsStopsUpstreamWorkWhenContextIsCanceled(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	releaseFirst := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(releaseFirst) }) })
	var startOnce sync.Once
	var firstCalls atomic.Int32
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		firstCalls.Add(1)
		startOnce.Do(func() { close(started) })
		select {
		case <-r.Context().Done():
		case <-releaseFirst:
		}
	}))
	t.Cleanup(first.Close)
	var fallbackCalls atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(second.Close)

	gateway := newTestGateway(t, []testTarget{
		{name: "first", url: first.URL, model: "model-a"},
		{name: "second", url: second.URL, model: "model-b"},
	}, 2, 1<<20)
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false))).WithContext(ctx)
	done := make(chan struct{})
	go func() {
		gateway.ChatCompletions(httptest.NewRecorder(), request)
		close(done)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("upstream request did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handler did not stop after cancellation")
	}
	releaseOnce.Do(func() { close(releaseFirst) })
	if got := firstCalls.Load(); got != 1 {
		t.Errorf("first target calls = %d, want 1", got)
	}
	if got := fallbackCalls.Load(); got != 0 {
		t.Errorf("fallback calls = %d, want 0", got)
	}
}

func TestChatCompletionsStreamsIncrementallyWithoutInventingDone(t *testing.T) {
	t.Parallel()

	release := make(chan struct{})
	var releaseOnce sync.Once
	t.Cleanup(func() { releaseOnce.Do(func() { close(release) }) })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("upstream ResponseWriter does not implement http.Flusher")
			return
		}
		_, _ = io.WriteString(w, "data: first\n\n")
		flusher.Flush()
		<-release
		_, _ = io.WriteString(w, "data: second\n\n")
		flusher.Flush()
	}))
	t.Cleanup(upstream.Close)

	gateway := newTestGateway(t, []testTarget{{name: "primary", url: upstream.URL, model: "provider-model"}}, 0, 1<<20)
	server := httptest.NewServer(http.HandlerFunc(gateway.ChatCompletions))
	t.Cleanup(server.Close)
	request, err := http.NewRequest(http.MethodPost, server.URL, strings.NewReader(validChatBody(true)))
	if err != nil {
		t.Fatalf("create request: %v", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatalf("stream request: %v", err)
	}
	defer response.Body.Close()
	reader := bufio.NewReader(response.Body)
	firstLine := make(chan string, 1)
	readErr := make(chan error, 1)
	go func() {
		line, err := reader.ReadString('\n')
		if err != nil {
			readErr <- err
			return
		}
		firstLine <- line
	}()

	select {
	case line := <-firstLine:
		if line != "data: first\n" {
			t.Fatalf("first streamed line = %q, want first event", line)
		}
	case err := <-readErr:
		t.Fatalf("read first streamed line: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("first SSE event was buffered until upstream completion")
	}

	releaseOnce.Do(func() { close(release) })
	remainder, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read remaining stream: %v", err)
	}
	fullStream := "data: first\n" + string(remainder)
	if !strings.Contains(fullStream, "data: second\n\n") {
		t.Errorf("stream = %q, want second event", fullStream)
	}
	if strings.Contains(fullStream, "[DONE]") {
		t.Errorf("stream = %q, gateway invented [DONE]", fullStream)
	}
}

func TestChatCompletionsReturnsOpenAIErrorWhenAllTransportsFail(t *testing.T) {
	t.Parallel()

	unavailable := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	unavailableURL := unavailable.URL
	unavailable.Close()
	gateway := newTestGateway(t, []testTarget{{name: "unavailable", url: unavailableURL, model: "model-a"}}, 1, 1<<20)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(validChatBody(false)))

	gateway.ChatCompletions(response, request)

	if response.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadGateway)
	}
	assertOpenAIError(t, response)
}

type testTarget struct {
	name  string
	url   string
	model string
}

func newTestGateway(t *testing.T, targets []testTarget, retries int, maxBodyBytes int64) *Gateway {
	t.Helper()
	providers := make(map[string]config.ProviderConfig, len(targets))
	configuredTargets := make([]config.TargetConfig, 0, len(targets))
	for _, target := range targets {
		providers[target.name] = config.ProviderConfig{Type: "openai", BaseURL: target.url}
		configuredTargets = append(configuredTargets, config.TargetConfig{Provider: target.name, Model: target.model})
	}
	cfg := config.Config{
		Server:    config.ServerConfig{MaxBodyBytes: maxBodyBytes},
		Providers: providers,
		Models: map[string]config.ModelConfig{
			"public-alias": {Targets: configuredTargets},
		},
		Routing: config.RoutingConfig{
			Retries:               retries,
			ResponseHeaderTimeout: config.Duration(2 * time.Second),
		},
	}
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	return New(cfg, clients)
}

func validChatBody(stream bool) string {
	if stream {
		return `{"model":"public-alias","messages":[{"role":"user","content":"hello"}],"stream":true}`
	}
	return `{"model":"public-alias","messages":[{"role":"user","content":"hello"}]}`
}

func assertHeader(t *testing.T, headers http.Header, name, want string) {
	t.Helper()
	if got := headers.Get(name); got != want {
		t.Errorf("%s = %q, want %q", name, got, want)
	}
}

func assertOpenAIError(t *testing.T, response *httptest.ResponseRecorder) {
	t.Helper()
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var payload struct {
		Error struct {
			Message string          `json:"message"`
			Type    string          `json:"type"`
			Param   json.RawMessage `json:"param"`
			Code    string          `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error response %q: %v", response.Body.String(), err)
	}
	if payload.Error.Message == "" || payload.Error.Type == "" || payload.Error.Code == "" {
		t.Errorf("error response = %#v, want non-empty OpenAI error fields", payload.Error)
	}
	if len(payload.Error.Param) == 0 {
		t.Errorf("error response = %#v, want param field", payload.Error)
	}
}
