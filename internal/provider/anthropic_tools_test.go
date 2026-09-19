package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"nexoroute/internal/config"
)

func TestAnthropicAdapterTranslatesToolRoundTrip(t *testing.T) {
	t.Parallel()
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		_, _ = io.WriteString(w, `{"id":"msg_tool","model":"claude-test","content":[{"type":"text","text":"checking"},{"type":"tool_use","id":"call_2","name":"forecast","input":{"city":"Rio"}}],"stop_reason":"tool_use","usage":{"input_tokens":11,"output_tokens":5}}`)
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "anthropic", BaseURL: server.URL})
	body := []byte(`{
  "messages":[
    {"role":"user","content":"weather?"},
    {"role":"assistant","content":null,"tool_calls":[{"id":"call_1","type":"function","function":{"name":"forecast","arguments":"{\"city\":\"Sao Paulo\"}"}}]},
    {"role":"tool","tool_call_id":"call_1","content":"{\"temperature\":24}"}
  ],
  "tools":[{"type":"function","function":{"name":"forecast","description":"Weather forecast","parameters":{"type":"object","properties":{"city":{"type":"string"}}}}}],
  "tool_choice":{"type":"function","function":{"name":"forecast"}},
  "parallel_tool_calls":false
}`)
	response, err := client.Do(context.Background(), body, "claude-test")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	var upstream struct {
		Tools []struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"input_schema"`
		} `json:"tools"`
		ToolChoice struct {
			Type               string `json:"type"`
			Name               string `json:"name"`
			DisableParallelUse bool   `json:"disable_parallel_tool_use"`
		} `json:"tool_choice"`
		Messages []struct {
			Role    string            `json:"role"`
			Content []json.RawMessage `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(<-received, &upstream); err != nil {
		t.Fatalf("decode upstream: %v", err)
	}
	if len(upstream.Tools) != 1 || upstream.Tools[0].Name != "forecast" || !strings.Contains(string(upstream.Tools[0].InputSchema), `"city"`) {
		t.Errorf("tools = %#v", upstream.Tools)
	}
	if upstream.ToolChoice.Type != "tool" || upstream.ToolChoice.Name != "forecast" || !upstream.ToolChoice.DisableParallelUse {
		t.Errorf("tool_choice = %#v", upstream.ToolChoice)
	}
	encodedMessages, _ := json.Marshal(upstream.Messages)
	for _, want := range []string{`"type":"tool_use"`, `"id":"call_1"`, `"input":{"city":"Sao Paulo"}`, `"type":"tool_result"`, `"tool_use_id":"call_1"`} {
		if !strings.Contains(string(encodedMessages), want) {
			t.Errorf("messages = %s, want %s", encodedMessages, want)
		}
	}

	assertNormalizedToolCompletion(t, response, "msg_tool", "claude-test", "checking", "call_2", "forecast", `{"city":"Rio"}`)
}

func TestAnthropicAdapterStreamsToolArguments(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream\",\"model\":\"claude-test\",\"usage\":{\"input_tokens\":3}}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_start\",\"index\":1,\"content_block\":{\"type\":\"tool_use\",\"id\":\"call_7\",\"name\":\"forecast\",\"input\":{}}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"{\\\"city\\\":\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"content_block_delta\",\"index\":1,\"delta\":{\"type\":\"input_json_delta\",\"partial_json\":\"\\\"Rio\\\"}\"}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"tool_use\"},\"usage\":{\"output_tokens\":4}}\n\n")
		_, _ = io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "anthropic", BaseURL: server.URL})
	response, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":"weather?"}],"tools":[{"type":"function","function":{"name":"forecast"}}],"stream":true}`), "claude-test")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	stream := string(data)
	for _, want := range []string{`"id":"call_7"`, `"name":"forecast"`, `"arguments":"{\"city\":"`, `"arguments":"\"Rio\"}"`, `"finish_reason":"tool_calls"`, "data: [DONE]"} {
		if !strings.Contains(stream, want) {
			t.Errorf("stream = %q, want %q", stream, want)
		}
	}
}

func assertNormalizedToolCompletion(t *testing.T, response *http.Response, id, model, text, callID, name, arguments string) {
	t.Helper()
	var got struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			FinishReason string `json:"finish_reason"`
			Message      struct {
				Content   *string          `json:"content"`
				ToolCalls []openAIToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(response.Body).Decode(&got); err != nil {
		t.Fatalf("decode normalized response: %v", err)
	}
	if got.ID != id || got.Model != model || len(got.Choices) != 1 || got.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("normalized response = %#v", got)
		return
	}
	message := got.Choices[0].Message
	if message.Content == nil || *message.Content != text || len(message.ToolCalls) != 1 {
		t.Errorf("message = %#v", message)
		return
	}
	call := message.ToolCalls[0]
	if call.ID != callID || call.Type != "function" || call.Function.Name != name || call.Function.Arguments != arguments {
		t.Errorf("tool call = %#v", call)
	}
}
