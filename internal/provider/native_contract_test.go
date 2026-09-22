package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"nexoroute/internal/config"
)

func TestNativeDecoderRejectsUntranslatedFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		code string
	}{
		{"top level", `{"messages":[{"role":"user","content":"hi"}],"frequency_penalty":0.5}`, "unsupported_field"},
		{"nested message", `{"messages":[{"role":"user","content":"hi","name":"alice"}]}`, "unsupported_field"},
		{"nested tool definition", `{"tools":[{"type":"function","function":{"name":"save","extra":true}}]}`, "unsupported_field"},
		{"nested tool call", `{"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"save","arguments":"{}","extra":true}}]}]}`, "unsupported_field"},
		{"unsupported sample count", `{"n":2}`, "unsupported_n"},
		{"audio output", `{"modalities":["audio"]}`, "unsupported_field"},
		{"structured output", `{"response_format":{"type":"json_schema","json_schema":{"name":"answer"}}}`, "unsupported_field"},
		{"extra text format setting", `{"response_format":{"type":"text","extra":true}}`, "unsupported_field"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := decodeChatRequest([]byte(test.body))
			assertNativeContractError(t, err, test.code)
		})
	}
}

func TestNativeDecoderAcceptsEquivalentTextDefaults(t *testing.T) {
	t.Parallel()
	_, err := decodeChatRequest([]byte(`{"model":"alias","messages":[{"role":"user","content":"hi"}],"n":1,"modalities":["text"],"response_format":{"type":"text"}}`))
	if err != nil {
		t.Fatalf("decodeChatRequest() error = %v", err)
	}
}

func TestNativeToolsRejectIgnoredToolCallAndChoiceFields(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		code string
	}{
		{"custom assistant call", `{"messages":[{"role":"assistant","tool_calls":[{"id":"call_1","type":"custom","function":{"name":"save","arguments":"{}"}}]}]}`, "unsupported_tool_type"},
		{"extra named choice", `{"tools":[{"type":"function","function":{"name":"save"}}],"tool_choice":{"type":"function","function":{"name":"save","extra":true}}}`, "unsupported_tool_choice"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, err := decodeChatRequest([]byte(test.body))
			if err != nil {
				t.Fatalf("decodeChatRequest() error = %v", err)
			}
			_, err = nativeTools(request)
			assertNativeContractError(t, err, test.code)
		})
	}
}

func TestNativeAdaptersRejectUnknownRootFieldBeforeNetwork(t *testing.T) {
	t.Parallel()
	for _, providerType := range []string{"anthropic", "gemini", "vertex", "bedrock"} {
		t.Run(providerType, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
			t.Cleanup(server.Close)
			client := testProviderClient(t, config.ProviderConfig{
				Type: providerType, BaseURL: server.URL, Project: "project", Location: "us-central1",
				Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret",
			})
			_, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":"hi"}],"seed":42}`), "uncataloged-model")
			assertNativeContractError(t, err, "unsupported_field")
			if calls.Load() != 0 {
				t.Fatalf("upstream calls = %d, want 0", calls.Load())
			}
		})
	}
}

func TestNativeAdaptersRejectUntranslatedContentBeforeNetwork(t *testing.T) {
	t.Parallel()
	for _, providerType := range []string{"anthropic", "gemini", "vertex", "bedrock"} {
		t.Run(providerType, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
			t.Cleanup(server.Close)
			client := testProviderClient(t, config.ProviderConfig{
				Type: providerType, BaseURL: server.URL, Project: "project", Location: "us-central1",
				Region: "us-east-1", AccessKeyID: "key", SecretAccessKey: "secret",
			})
			for _, content := range []string{
				`[{"type":"image_url","image_url":{"url":"data:image/png;base64,AQID","detail":"high"}}]`,
				`[{"type":"file","file":{"filename":"report.pdf","file_data":"data:application/pdf;base64,JVBERg=="}}]`,
				`[{"type":"text","text":"hi","prompt_cache_breakpoint":true}]`,
			} {
				body := []byte(`{"messages":[{"role":"user","content":` + content + `}]}`)
				_, err := client.Do(context.Background(), body, "uncataloged-model")
				assertNativeContractError(t, err, "unsupported_content")
			}
			if calls.Load() != 0 {
				t.Fatalf("upstream calls = %d, want 0", calls.Load())
			}
		})
	}
}

func assertNativeContractError(t *testing.T, err error, code string) {
	t.Helper()
	requestError, ok := err.(*RequestError)
	if !ok || requestError.Code != code {
		t.Fatalf("error = %v, want RequestError %q", err, code)
	}
}
