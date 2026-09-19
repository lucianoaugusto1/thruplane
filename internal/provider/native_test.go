package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"nexoroute/internal/config"
)

func TestAnthropicAdapterTranslatesBufferedChat(t *testing.T) {
	t.Parallel()
	received := make(chan struct {
		request *http.Request
		body    []byte
	}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- struct {
			request *http.Request
			body    []byte
		}{r.Clone(r.Context()), body}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_1","model":"claude-test","content":[{"type":"text","text":"hello"},{"type":"text","text":" world"}],"stop_reason":"end_turn","usage":{"input_tokens":7,"output_tokens":2}}`))
	}))
	t.Cleanup(server.Close)

	client := testProviderClient(t, config.ProviderConfig{Type: "anthropic", BaseURL: server.URL, APIKey: "anthropic-secret"})
	body := []byte(`{"model":"alias","messages":[{"role":"system","content":"be concise"},{"role":"user","content":"hi"}],"max_completion_tokens":50,"temperature":0.2,"stop":"END"}`)
	response, err := client.Do(context.Background(), body, "claude-test")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	got := <-received
	if got.request.URL.Path != "/v1/messages" || got.request.Header.Get("x-api-key") != "anthropic-secret" || got.request.Header.Get("anthropic-version") != "2023-06-01" {
		t.Errorf("Anthropic request = path %q headers %#v", got.request.URL.Path, got.request.Header)
	}
	var upstream map[string]any
	if err := json.Unmarshal(got.body, &upstream); err != nil {
		t.Fatal(err)
	}
	if upstream["model"] != "claude-test" || upstream["system"] != "be concise" || upstream["max_tokens"] != float64(50) {
		t.Errorf("upstream body = %#v", upstream)
	}
	assertNormalizedCompletion(t, response, "msg_1", "claude-test", "hello world", "stop", 7, 2)
}

func TestAnthropicAdapterNormalizesStreaming(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream\",\"model\":\"claude-stream\",\"usage\":{\"input_tokens\":3}}}\n\n")
		_, _ = io.WriteString(w, "event: content_block_delta\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"hello\"}}\n\n")
		_, _ = io.WriteString(w, "event: message_delta\ndata: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"max_tokens\"},\"usage\":{\"output_tokens\":4}}\n\n")
		_, _ = io.WriteString(w, "event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "anthropic", BaseURL: server.URL})
	response, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":"hi"}],"stream":true}`), "claude-stream")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	text := string(data)
	for _, want := range []string{`"role":"assistant"`, `"content":"hello"`, `"finish_reason":"length"`, `"prompt_tokens":3`, `"completion_tokens":4`, "data: [DONE]"} {
		if !strings.Contains(text, want) {
			t.Errorf("stream %q does not contain %q", text, want)
		}
	}
}

func TestGoogleAdaptersTranslateBufferedChat(t *testing.T) {
	tests := []struct {
		name       string
		config     config.ProviderConfig
		wantPath   string
		wantHeader string
		wantValue  string
	}{
		{"gemini", config.ProviderConfig{Type: "gemini", APIKey: "gemini-key"}, "/v1beta/models/gemini-test:generateContent", "x-goog-api-key", "gemini-key"},
		{"vertex", config.ProviderConfig{Type: "vertex", Project: "customer-project", Location: "us-central1", AccessToken: "oauth-token"}, "/v1/projects/customer-project/locations/us-central1/publishers/google/models/gemini-test:generateContent", "Authorization", "Bearer oauth-token"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			received := make(chan struct {
				request *http.Request
				body    []byte
			}, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				received <- struct {
					request *http.Request
					body    []byte
				}{r.Clone(r.Context()), body}
				_, _ = w.Write([]byte(`{"responseId":"google-1","modelVersion":"gemini-test","candidates":[{"content":{"role":"model","parts":[{"text":"hello"},{"text":" google"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":5,"candidatesTokenCount":2,"totalTokenCount":7}}`))
			}))
			t.Cleanup(server.Close)
			tt.config.BaseURL = server.URL
			client := testProviderClient(t, tt.config)
			body := []byte(`{"messages":[{"role":"developer","content":"be concise"},{"role":"user","content":"hi"},{"role":"assistant","content":"hello"}],"max_tokens":32,"top_p":0.8,"stop":["END"]}`)
			response, err := client.Do(context.Background(), body, "gemini-test")
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			defer response.Body.Close()
			got := <-received
			if got.request.URL.Path != tt.wantPath || got.request.Header.Get(tt.wantHeader) != tt.wantValue {
				t.Errorf("request URL/header = %s %q, want %s %q", got.request.URL.String(), got.request.Header.Get(tt.wantHeader), tt.wantPath, tt.wantValue)
			}
			var upstream map[string]any
			if err := json.Unmarshal(got.body, &upstream); err != nil {
				t.Fatal(err)
			}
			if upstream["systemInstruction"] == nil || upstream["generationConfig"] == nil {
				t.Errorf("upstream body = %#v", upstream)
			}
			assertNormalizedCompletion(t, response, "google-1", "gemini-test", "hello google", "stop", 5, 2)
		})
	}
}

func TestGeminiAdapterNormalizesStreaming(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("alt") != "sse" || !strings.Contains(r.URL.Path, ":streamGenerateContent") {
			t.Errorf("stream URL = %s", r.URL.String())
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"responseId\":\"g-stream\",\"modelVersion\":\"gemini-test\",\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hello\"}]}}]}\n\n")
		_, _ = io.WriteString(w, "data: {\"responseId\":\"g-stream\",\"candidates\":[{\"content\":{\"parts\":[{\"text\":\" world\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":2,\"candidatesTokenCount\":2,\"totalTokenCount\":4}}\n\n")
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "gemini", BaseURL: server.URL, APIKey: "key"})
	response, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":"hi"}],"stream":true}`), "gemini-test")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	text := string(data)
	for _, want := range []string{`"content":"hello"`, `"content":" world"`, `"finish_reason":"stop"`, "data: [DONE]"} {
		if !strings.Contains(text, want) {
			t.Errorf("stream %q does not contain %q", text, want)
		}
	}
}

func TestNativeAdaptersRejectUnsupportedContentBeforeNetwork(t *testing.T) {
	t.Parallel()
	client := testProviderClient(t, config.ProviderConfig{Type: "anthropic", BaseURL: "http://127.0.0.1:1"})
	_, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]}]}`), "model")
	var requestError *RequestError
	if err == nil || !strings.Contains(err.Error(), "text message content") || !errorAs(err, &requestError) {
		t.Fatalf("Do() error = %v, want RequestError", err)
	}
}

func testProviderClient(t *testing.T, providerConfig config.ProviderConfig) *Client {
	t.Helper()
	clients, err := NewClients(config.Config{
		Providers: map[string]config.ProviderConfig{"tested": providerConfig},
		Routing:   config.RoutingConfig{ResponseHeaderTimeout: config.Duration(2 * time.Second)},
	})
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	return clients["tested"]
}

func assertNormalizedCompletion(t *testing.T, response *http.Response, id, model, content, finish string, prompt, completion int) {
	t.Helper()
	var got struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage tokenUsage `json:"usage"`
	}
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode normalized response: %v", err)
	}
	if got.ID != id || got.Model != model || len(got.Choices) != 1 || got.Choices[0].Message.Content != content || got.Choices[0].FinishReason != finish {
		t.Errorf("normalized response = %#v", got)
	}
	if got.Usage.PromptTokens != prompt || got.Usage.CompletionTokens != completion || got.Usage.TotalTokens != prompt+completion {
		t.Errorf("normalized usage = %#v", got.Usage)
	}
}

func errorAs(err error, target any) bool {
	switch value := target.(type) {
	case **RequestError:
		if typed, ok := err.(*RequestError); ok {
			*value = typed
			return true
		}
	}
	return false
}
