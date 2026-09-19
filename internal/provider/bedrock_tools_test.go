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

	"nexoroute/internal/config"
)

func TestBedrockAdapterTranslatesToolRoundTrip(t *testing.T) {
	t.Parallel()
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		_, _ = io.WriteString(w, `{"output":{"message":{"role":"assistant","content":[{"text":"checking"},{"toolUse":{"toolUseId":"call_2","name":"forecast","input":{"city":"Rio"}}}]}},"stopReason":"tool_use","usage":{"inputTokens":10,"outputTokens":4,"totalTokens":14}}`)
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "bedrock", BaseURL: server.URL, Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret"})
	body := []byte(`{
  "messages":[
    {"role":"user","content":"weather?"},
    {"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"forecast","arguments":"{\"city\":\"Sao Paulo\"}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"{\"temperature\":24}"}
  ],
  "tools":[{"type":"function","function":{"name":"forecast","description":"Weather forecast","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],
  "tool_choice":{"type":"function","function":{"name":"forecast"}}
}`)
	response, err := client.Do(context.Background(), body, "amazon.test-model")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	var upstream map[string]any
	if err := json.Unmarshal(<-received, &upstream); err != nil {
		t.Fatalf("decode upstream: %v", err)
	}
	encoded, _ := json.Marshal(upstream)
	for _, want := range []string{
		`"toolSpec":{"description":"Weather forecast","inputSchema":{"json":{"properties":{"city":{"type":"string"}},"type":"object"}},"name":"forecast"}`,
		`"toolChoice":{"tool":{"name":"forecast"}}`,
		`"toolUse":{"input":{"city":"Sao Paulo"},"name":"forecast","toolUseId":"call_1"}`,
		`"toolResult":{"content":[{"json":{"temperature":24}}],"toolUseId":"call_1"}`,
	} {
		if !strings.Contains(string(encoded), want) {
			t.Errorf("upstream = %s, want %s", encoded, want)
		}
	}
	assertNormalizedToolCompletion(t, response, "chatcmpl-bedrock", "amazon.test-model", "checking", "call_2", "forecast", `{"city":"Rio"}`)
}

func TestBedrockAdapterRejectsDisabledParallelCallsBeforeNetwork(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "bedrock", BaseURL: server.URL, Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret"})
	_, err := client.Do(context.Background(), []byte(`{"messages":[],"tools":[{"type":"function","function":{"name":"forecast"}}],"parallel_tool_calls":false}`), "model")
	requestError, ok := err.(*RequestError)
	if !ok || requestError.Code != "unsupported_parallel_tool_calls" {
		t.Fatalf("Do() error = %#v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}
