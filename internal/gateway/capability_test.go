package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lucianoaugusto1/thruplane/internal/config"
	"github.com/lucianoaugusto1/thruplane/internal/provider"
)

func TestCapabilityRoutingSkipsGoogleForRemoteImage(t *testing.T) {
	t.Parallel()
	var nativeCalls, compatibleCalls atomic.Int32
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nativeCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(native.Close)
	compatible := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compatibleCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(compatible.Close)

	cfg := capabilityTestConfig(native.URL, compatible.URL)
	cfg.Providers["native"] = config.ProviderConfig{Type: "gemini", BaseURL: native.URL, APIKey: "test"}
	cfg.Models["multimodal"] = config.ModelConfig{Targets: []config.TargetConfig{
		{Provider: "native", Model: "gemini-3.8-flash"},
		{Provider: "compatible", Model: "gpt-4o-mini"},
	}}
	g := capabilityTestGateway(t, cfg)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"multimodal","messages":[{"role":"user","content":[
			{"type":"text","text":"Describe"},
			{"type":"image_url","image_url":{"url":"https://images.example.com/photo.png"}}
		]}]}`))
	g.ChatCompletions(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204; body = %s", response.Code, response.Body.String())
	}
	if nativeCalls.Load() != 0 || compatibleCalls.Load() != 1 {
		t.Fatalf("upstream calls: native=%d compatible=%d, want 0 and 1", nativeCalls.Load(), compatibleCalls.Load())
	}
}

func TestCapabilityRoutingRejectsUnsupportedContentBeforeUpstream(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(native.Close)
	cfg := capabilityTestConfig(native.URL, native.URL)
	cfg.Models["native-only"] = config.ModelConfig{Targets: []config.TargetConfig{{Provider: "native", Model: "claude-sonnet-5"}}}
	g := capabilityTestGateway(t, cfg)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"native-only","messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"AQID","format":"wav"}}]}]}`))
	response := httptest.NewRecorder()
	g.ChatCompletions(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "unsupported_capability") {
		t.Fatalf("response = %d %s, want unsupported_capability", response.Code, response.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}

func TestCapabilityRoutingUsesAnthropicForInlineImage(t *testing.T) {
	t.Parallel()
	var nativeCalls, compatibleCalls atomic.Int32
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nativeCalls.Add(1)
		_, _ = w.Write([]byte(`{"id":"msg_1","content":[{"type":"text","text":"image"}],"stop_reason":"end_turn","usage":{"input_tokens":4,"output_tokens":1}}`))
	}))
	t.Cleanup(native.Close)
	compatible := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compatibleCalls.Add(1)
	}))
	t.Cleanup(compatible.Close)
	g := capabilityTestGateway(t, capabilityTestConfig(native.URL, compatible.URL))
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"multimodal","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AQID"}}]}]}`))
	response := httptest.NewRecorder()
	g.ChatCompletions(response, request)
	if response.Code != http.StatusOK || nativeCalls.Load() != 1 || compatibleCalls.Load() != 0 {
		t.Fatalf("response=%d native=%d compatible=%d", response.Code, nativeCalls.Load(), compatibleCalls.Load())
	}
}

func TestCapabilityRoutingUsesGeminiForInlineAudio(t *testing.T) {
	t.Parallel()
	var nativeCalls, compatibleCalls atomic.Int32
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nativeCalls.Add(1)
		_, _ = w.Write([]byte(`{"responseId":"audio_1","candidates":[{"content":{"role":"model","parts":[{"text":"heard"}]},"finishReason":"STOP"}]}`))
	}))
	t.Cleanup(native.Close)
	compatible := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compatibleCalls.Add(1)
	}))
	t.Cleanup(compatible.Close)
	cfg := capabilityTestConfig(native.URL, compatible.URL)
	cfg.Providers["native"] = config.ProviderConfig{Type: "gemini", BaseURL: native.URL, APIKey: "test"}
	cfg.Models["audio"] = config.ModelConfig{Targets: []config.TargetConfig{
		{Provider: "native", Model: "gemini-3.8-flash"},
		{Provider: "compatible", Model: "gpt-4o-mini"},
	}}
	g := capabilityTestGateway(t, cfg)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"audio","messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"AQID","format":"wav"}}]}]}`))
	response := httptest.NewRecorder()
	g.ChatCompletions(response, request)
	if response.Code != http.StatusOK || nativeCalls.Load() != 1 || compatibleCalls.Load() != 0 {
		t.Fatalf("response=%d native=%d compatible=%d", response.Code, nativeCalls.Load(), compatibleCalls.Load())
	}
}

func TestCapabilityRoutingRejectsUnknownModelInStrictMode(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)
	cfg := capabilityTestConfig(server.URL, server.URL)
	cfg.Catalog.UnknownModels = "reject"
	cfg.Models["unknown"] = config.ModelConfig{Targets: []config.TargetConfig{{Provider: "compatible", Model: "future-model"}}}
	g := capabilityTestGateway(t, cfg)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"unknown","messages":[{"role":"user","content":"hi"}]}`))
	response := httptest.NewRecorder()
	g.ChatCompletions(response, request)
	if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), "model_not_cataloged") {
		t.Fatalf("response = %d %s, want model_not_cataloged", response.Code, response.Body.String())
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}

func TestUnknownNativeModelRejectsUntranslatedFieldsBeforeUpstream(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)
	cfg := capabilityTestConfig(server.URL, server.URL)
	cfg.Models["unknown-native"] = config.ModelConfig{Targets: []config.TargetConfig{{Provider: "native", Model: "future-native-model"}}}
	g := capabilityTestGateway(t, cfg)
	for _, test := range []struct {
		name, body, code string
	}{
		{"root option", `{"model":"unknown-native","messages":[{"role":"user","content":"hi"}],"seed":42}`, "unsupported_field"},
		{"nested media option", `{"model":"unknown-native","messages":[{"role":"user","content":[{"type":"file","file":{"filename":"report.pdf","file_data":"data:application/pdf;base64,JVBERg=="}}]}]}`, "unsupported_content"},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(test.body))
			g.ChatCompletions(response, request)
			if response.Code != http.StatusBadRequest || !strings.Contains(response.Body.String(), `"code":"`+test.code+`"`) {
				t.Fatalf("response = %d %s, want %s", response.Code, response.Body.String(), test.code)
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}

func TestUnknownCompatibleModelPreservesExtraFields(t *testing.T) {
	t.Parallel()
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	cfg := capabilityTestConfig(server.URL, server.URL)
	cfg.Models["unknown-compatible"] = config.ModelConfig{Targets: []config.TargetConfig{{Provider: "compatible", Model: "future-compatible-model"}}}
	g := capabilityTestGateway(t, cfg)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"unknown-compatible","messages":[{"role":"user","content":"hi","name":"alice"}],
		"seed":42,"vendor_option":{"enabled":true}
	}`))
	g.ChatCompletions(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("response = %d %s, want 204", response.Code, response.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(<-received, &body); err != nil {
		t.Fatalf("upstream body: %v", err)
	}
	if string(body["model"]) != `"future-compatible-model"` || string(body["seed"]) != "42" || string(body["vendor_option"]) != `{"enabled":true}` || !strings.Contains(string(body["messages"]), `"name":"alice"`) {
		t.Fatalf("upstream body lost fields: %s", body)
	}
}

func TestCapabilityRoutingSkipsNativeStrictToolTarget(t *testing.T) {
	t.Parallel()
	var nativeCalls, compatibleCalls atomic.Int32
	native := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nativeCalls.Add(1)
	}))
	t.Cleanup(native.Close)
	compatible := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compatibleCalls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(compatible.Close)
	g := capabilityTestGateway(t, capabilityTestConfig(native.URL, compatible.URL))
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{
		"model":"multimodal","messages":[{"role":"user","content":"hello"}],
		"tools":[{"type":"function","function":{"name":"save","strict":true,"parameters":{"type":"object"}}}]
	}`))
	response := httptest.NewRecorder()
	g.ChatCompletions(response, request)
	if response.Code != http.StatusNoContent || nativeCalls.Load() != 0 || compatibleCalls.Load() != 1 {
		t.Fatalf("response=%d native=%d compatible=%d", response.Code, nativeCalls.Load(), compatibleCalls.Load())
	}
}

func TestCapabilityRoutingMapsDeploymentToCatalogModel(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	cfg := capabilityTestConfig(server.URL, server.URL)
	cfg.Catalog.UnknownModels = "reject"
	cfg.Models["deployment"] = config.ModelConfig{Targets: []config.TargetConfig{{
		Provider: "compatible", Model: "prod-deployment", CatalogModel: "gpt-4o-mini",
	}}}
	g := capabilityTestGateway(t, cfg)
	request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"deployment","messages":[{"role":"user","content":"hello"}]}`))
	response := httptest.NewRecorder()
	g.ChatCompletions(response, request)
	if response.Code != http.StatusNoContent || calls.Load() != 1 {
		t.Fatalf("response=%d calls=%d, want 204 and one call", response.Code, calls.Load())
	}
}

func capabilityTestConfig(nativeURL, compatibleURL string) config.Config {
	return config.Config{
		Server: config.ServerConfig{MaxBodyBytes: 1 << 20},
		Providers: map[string]config.ProviderConfig{
			"native":     {Type: "anthropic", BaseURL: nativeURL, APIKey: "test"},
			"compatible": {Type: "openai", BaseURL: compatibleURL, APIKey: "test"},
		},
		Models: map[string]config.ModelConfig{
			"multimodal": {Targets: []config.TargetConfig{
				{Provider: "native", Model: "claude-sonnet-5"},
				{Provider: "compatible", Model: "gpt-4o-mini"},
			}},
		},
		Routing: config.RoutingConfig{ResponseHeaderTimeout: config.Duration(time.Second)},
	}
}

func capabilityTestGateway(t *testing.T, cfg config.Config) *Gateway {
	t.Helper()
	clients, err := provider.NewClients(cfg)
	if err != nil {
		t.Fatalf("NewClients() error = %v", err)
	}
	return New(cfg, clients)
}
