package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"nexoroute/internal/config"
)

func TestBedrockAdapterTranslatesInlineImageAndPDF(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		received <- body
		if r.Header.Get("X-Amz-Content-Sha256") != sha256Hex(body) {
			t.Error("signature payload hash differs from media body")
		}
		_, _ = io.WriteString(w, `{"output":{"message":{"content":[{"text":"done"}]}},"stopReason":"end_turn","usage":{"inputTokens":9,"outputTokens":1}}`)
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{
		Type: "bedrock", BaseURL: server.URL, Region: "us-east-1",
		AccessKeyID: "key", SecretAccessKey: "secret",
	})
	body := []byte(`{"messages":[{"role":"user","content":[
		{"type":"text","text":"summarize"},
		{"type":"image_url","image_url":{"url":"data:image/webp;base64,AQID"}},
		{"type":"file","file":{"filename":"unsafe-name.pdf","file_data":"data:application/pdf;base64,JVBERg=="}},
		{"type":"text","text":"now"}
	]}]}`)
	response, err := client.Do(context.Background(), body, "amazon.nova-2-lite-v1:0")
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	response.Body.Close()
	if calls.Load() != 1 {
		t.Fatalf("upstream calls = %d, want 1", calls.Load())
	}
	var upstream struct {
		Messages []struct {
			Content []struct {
				Text  string `json:"text"`
				Image struct {
					Format string `json:"format"`
					Source struct {
						Bytes string `json:"bytes"`
					} `json:"source"`
				} `json:"image"`
				Document struct {
					Format string `json:"format"`
					Name   string `json:"name"`
					Source struct {
						Bytes string `json:"bytes"`
					} `json:"source"`
				} `json:"document"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(<-received, &upstream); err != nil {
		t.Fatalf("decode upstream: %v", err)
	}
	if len(upstream.Messages) != 1 || len(upstream.Messages[0].Content) != 4 {
		t.Fatalf("messages = %#v", upstream.Messages)
	}
	parts := upstream.Messages[0].Content
	if parts[0].Text != "summarize" || parts[3].Text != "now" || parts[1].Image.Format != "webp" || parts[1].Image.Source.Bytes != "AQID" {
		t.Fatalf("image or text parts = %#v", parts)
	}
	if parts[2].Document.Format != "pdf" || parts[2].Document.Name != "document.pdf" || parts[2].Document.Source.Bytes != "JVBERg==" {
		t.Fatalf("document part = %#v", parts[2].Document)
	}
}

func TestBedrockAdapterRejectsAudioAndURLBeforeNetwork(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{
		Type: "bedrock", BaseURL: server.URL, Region: "us-east-1",
		AccessKeyID: "key", SecretAccessKey: "secret",
	})
	for _, body := range []string{
		`{"messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"AQID","format":"wav"}}]}]}`,
		`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://images.example.com/p.png"}}]}]}`,
	} {
		_, err := client.Do(context.Background(), []byte(body), "amazon.nova-2-lite-v1:0")
		requestError, ok := err.(*RequestError)
		if !ok || requestError.Code != "unsupported_content" {
			t.Fatalf("error = %v, want unsupported_content", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatalf("upstream calls = %d, want 0", calls.Load())
	}
}
