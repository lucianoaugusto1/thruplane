package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lucianoaugusto1/thruplane/internal/config"
)

func TestGoogleAdapterTranslatesToolRoundTrip(t *testing.T) {
	t.Parallel()
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		_, _ = io.WriteString(w, `{"responseId":"google-tool","modelVersion":"gemini-test","candidates":[{"content":{"role":"model","parts":[{"text":"checking"},{"functionCall":{"id":"call_2","name":"forecast","args":{"city":"Rio"}}}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":9,"candidatesTokenCount":4,"totalTokenCount":13}}`)
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "gemini", BaseURL: server.URL})
	body := []byte(`{
  "messages":[
    {"role":"user","content":"weather?"},
    {"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"forecast","arguments":"{\"city\":\"Sao Paulo\"}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"{\"temperature\":24}"}
  ],
  "tools":[{"type":"function","function":{"name":"forecast","description":"Weather forecast","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],
  "tool_choice":{"type":"function","function":{"name":"forecast"}}
}`)
	response, err := client.Do(context.Background(), body, "gemini-test")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	var upstream struct {
		Tools []struct {
			FunctionDeclarations []struct {
				Name       string          `json:"name"`
				Parameters json.RawMessage `json:"parameters"`
			} `json:"functionDeclarations"`
		} `json:"tools"`
		ToolConfig struct {
			FunctionCallingConfig struct {
				Mode                 string   `json:"mode"`
				AllowedFunctionNames []string `json:"allowedFunctionNames"`
			} `json:"functionCallingConfig"`
		} `json:"toolConfig"`
		Contents []googleContent `json:"contents"`
	}
	if err := json.Unmarshal(<-received, &upstream); err != nil {
		t.Fatalf("decode upstream: %v", err)
	}
	if len(upstream.Tools) != 1 || len(upstream.Tools[0].FunctionDeclarations) != 1 || upstream.Tools[0].FunctionDeclarations[0].Name != "forecast" {
		t.Errorf("tools = %#v", upstream.Tools)
	}
	config := upstream.ToolConfig.FunctionCallingConfig
	if config.Mode != "ANY" || len(config.AllowedFunctionNames) != 1 || config.AllowedFunctionNames[0] != "forecast" {
		t.Errorf("tool config = %#v", config)
	}
	encodedContents, _ := json.Marshal(upstream.Contents)
	for _, want := range []string{`"functionCall":{"id":"call_1","name":"forecast","args":{"city":"Sao Paulo"}}`, `"functionResponse":{"id":"call_1","name":"forecast","response":{"temperature":24}}`} {
		if !strings.Contains(string(encodedContents), want) {
			t.Errorf("contents = %s, want %s", encodedContents, want)
		}
	}
	assertNormalizedToolCompletion(t, response, "google-tool", "gemini-test", "checking", "call_2", "forecast", `{"city":"Rio"}`)
}

func TestGoogleAdapterStreamsToolCall(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"responseId\":\"google-stream\",\"modelVersion\":\"gemini-test\",\"candidates\":[{\"content\":{\"role\":\"model\",\"parts\":[{\"functionCall\":{\"id\":\"call_9\",\"name\":\"forecast\",\"args\":{\"city\":\"Rio\"}}}]},\"finishReason\":\"STOP\"}]}\n\n")
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "gemini", BaseURL: server.URL})
	response, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":"weather?"}],"tools":[{"type":"function","function":{"name":"forecast"}}],"stream":true}`), "gemini-test")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	stream := string(data)
	for _, want := range []string{`"id":"call_9"`, `"name":"forecast"`, `"arguments":"{\"city\":\"Rio\"}"`, `"finish_reason":"tool_calls"`, "data: [DONE]"} {
		if !strings.Contains(stream, want) {
			t.Errorf("stream = %q, want %q", stream, want)
		}
	}
}

func TestGoogleAdapterRejectsDisabledParallelCallsBeforeNetwork(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "gemini", BaseURL: server.URL})
	_, err := client.Do(context.Background(), []byte(`{"messages":[],"tools":[{"type":"function","function":{"name":"forecast"}}],"parallel_tool_calls":false}`), "gemini-test")
	requestError, ok := err.(*RequestError)
	if !ok || requestError.Code != "unsupported_parallel_tool_calls" {
		t.Fatalf("Do() error = %#v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}
