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

func TestGoogleAdaptersTranslateInlineImagePDFAudio(t *testing.T) {
	for _, adapterType := range []string{"gemini", "vertex"} {
		t.Run(adapterType, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			received := make(chan []byte, 1)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				body, _ := io.ReadAll(r.Body)
				received <- body
				_, _ = io.WriteString(w, `{"responseId":"google-media","candidates":[{"content":{"role":"model","parts":[{"text":"done"}]},"finishReason":"STOP"}]}`)
			}))
			t.Cleanup(server.Close)
			client := testProviderClient(t, config.ProviderConfig{
				Type: adapterType, BaseURL: server.URL, APIKey: "test", AccessToken: "test",
				Project: "project", Location: "us-central1",
			})
			body := []byte(`{"messages":[{"role":"user","content":[
				{"type":"text","text":"summarize"},
				{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,AQID"}},
				{"type":"file","file":{"file_data":"data:application/pdf;base64,JVBERg=="}},
				{"type":"input_audio","input_audio":{"data":"AQID","format":"mp3"}},
				{"type":"text","text":"now"}
			]}]}`)
			response, err := client.Do(context.Background(), body, "gemini-3.8-flash")
			if err != nil {
				t.Fatalf("Do() error = %v", err)
			}
			response.Body.Close()
			if calls.Load() != 1 {
				t.Fatalf("upstream calls = %d, want 1", calls.Load())
			}
			var upstream struct {
				Contents []struct {
					Parts []struct {
						Text       string `json:"text"`
						InlineData struct {
							MIMEType string `json:"mimeType"`
							Data     string `json:"data"`
						} `json:"inlineData"`
					} `json:"parts"`
				} `json:"contents"`
			}
			if err := json.Unmarshal(<-received, &upstream); err != nil {
				t.Fatalf("decode upstream: %v", err)
			}
			if len(upstream.Contents) != 1 || len(upstream.Contents[0].Parts) != 5 {
				t.Fatalf("contents = %#v", upstream.Contents)
			}
			parts := upstream.Contents[0].Parts
			if parts[0].Text != "summarize" || parts[4].Text != "now" {
				t.Fatalf("text order = %#v", parts)
			}
			for index, want := range []struct{ mime, data string }{{"image/jpeg", "AQID"}, {"application/pdf", "JVBERg=="}, {"audio/mp3", "AQID"}} {
				got := parts[index+1].InlineData
				if got.MIMEType != want.mime || got.Data != want.data {
					t.Fatalf("part %d inlineData = %#v, want %#v", index+1, got, want)
				}
			}
		})
	}
}

func TestGoogleAdaptersRejectImageURLBeforeNetwork(t *testing.T) {
	for _, adapterType := range []string{"gemini", "vertex"} {
		t.Run(adapterType, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
			t.Cleanup(server.Close)
			client := testProviderClient(t, config.ProviderConfig{Type: adapterType, BaseURL: server.URL, Project: "project", Location: "us-central1"})
			_, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"https://images.example.com/p.png"}}]}]}`), "gemini-3.8-flash")
			requestError, ok := err.(*RequestError)
			if !ok || requestError.Code != "unsupported_content" || calls.Load() != 0 {
				t.Fatalf("error=%v calls=%d, want unsupported_content before network", err, calls.Load())
			}
		})
	}
}
