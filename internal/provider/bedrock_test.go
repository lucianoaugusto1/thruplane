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
	"time"

	"nexoroute/internal/config"
)

func TestBedrockAdapterSignsAndTranslatesBufferedChat(t *testing.T) {
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
		_, _ = w.Write([]byte(`{"output":{"message":{"role":"assistant","content":[{"text":"hello"},{"text":" bedrock"}]}},"stopReason":"max_tokens","usage":{"inputTokens":6,"outputTokens":3,"totalTokens":9}}`))
	}))
	t.Cleanup(server.Close)

	client := testProviderClient(t, config.ProviderConfig{
		Type: "bedrock", BaseURL: server.URL, Region: "us-east-1",
		AccessKeyID: "AKIDEXAMPLE", SecretAccessKey: "secret", SessionToken: "session",
	})
	adapter := client.adapter.(*bedrockAdapter)
	adapter.now = func() time.Time { return time.Date(2026, 9, 19, 12, 34, 56, 0, time.UTC) }
	body := []byte(`{"messages":[{"role":"system","content":"safe"},{"role":"user","content":"hi"}],"max_completion_tokens":64,"temperature":0.3,"stop":"END"}`)
	response, err := client.Do(context.Background(), body, "amazon.test-model-v1:0")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer response.Body.Close()
	got := <-received
	if got.request.URL.EscapedPath() != "/model/amazon.test-model-v1:0/converse" {
		t.Errorf("path = %q", got.request.URL.EscapedPath())
	}
	if got.request.Header.Get("X-Amz-Date") != "20260919T123456Z" || got.request.Header.Get("X-Amz-Security-Token") != "session" {
		t.Errorf("AWS headers = %#v", got.request.Header)
	}
	authorization := got.request.Header.Get("Authorization")
	for _, want := range []string{"AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/20260919/us-east-1/bedrock/aws4_request", "SignedHeaders=content-type;host;x-amz-content-sha256;x-amz-date;x-amz-security-token", "Signature="} {
		if !strings.Contains(authorization, want) {
			t.Errorf("Authorization = %q, want %q", authorization, want)
		}
	}
	if got.request.Header.Get("X-Amz-Content-Sha256") != sha256Hex(got.body) {
		t.Error("payload hash does not match request body")
	}
	var upstream map[string]any
	if err := json.Unmarshal(got.body, &upstream); err != nil {
		t.Fatal(err)
	}
	if upstream["messages"] == nil || upstream["system"] == nil || upstream["inferenceConfig"] == nil {
		t.Errorf("upstream body = %#v", upstream)
	}
	assertNormalizedCompletion(t, response, "chatcmpl-bedrock", "amazon.test-model-v1:0", "hello bedrock", "length", 6, 3)
}

func TestBedrockAdapterRejectsStreamingBeforeNetwork(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{
		Type: "bedrock", BaseURL: server.URL, Region: "us-east-1",
		AccessKeyID: "key", SecretAccessKey: "secret",
	})
	_, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":"hi"}],"stream":true}`), "model")
	if err == nil || !strings.Contains(err.Error(), "does not support streaming") {
		t.Fatalf("Do() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}
