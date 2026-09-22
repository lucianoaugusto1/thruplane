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

func TestAnthropicAdapterTranslatesImageAndPDFInOrder(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		received <- body
		_, _ = io.WriteString(w, `{"id":"msg_media","content":[{"type":"text","text":"done"}],"stop_reason":"end_turn","usage":{"input_tokens":12,"output_tokens":2}}`)
	}))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "anthropic", BaseURL: server.URL})
	body := []byte(`{"messages":[{"role":"user","content":[
		{"type":"text","text":"compare"},
		{"type":"image_url","image_url":{"url":"data:image/png;base64,AQID"}},
		{"type":"image_url","image_url":{"url":"https://images.example.com/photo.png"}},
		{"type":"file","file":{"file_data":"data:application/pdf;base64,JVBERg=="}},
		{"type":"text","text":"please"}
	]}]}`)
	response, err := client.Do(context.Background(), body, "claude-sonnet-5")
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
				Type   string          `json:"type"`
				Text   string          `json:"text"`
				Source json.RawMessage `json:"source"`
			} `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(<-received, &upstream); err != nil {
		t.Fatalf("decode upstream: %v", err)
	}
	if len(upstream.Messages) != 1 || len(upstream.Messages[0].Content) != 5 {
		t.Fatalf("messages = %#v", upstream.Messages)
	}
	parts := upstream.Messages[0].Content
	if parts[0].Type != "text" || parts[0].Text != "compare" || parts[4].Text != "please" {
		t.Fatalf("text part order = %#v", parts)
	}
	assertJSONEqual(t, parts[1].Source, `{"type":"base64","media_type":"image/png","data":"AQID"}`)
	assertJSONEqual(t, parts[2].Source, `{"type":"url","url":"https://images.example.com/photo.png"}`)
	assertJSONEqual(t, parts[3].Source, `{"type":"base64","media_type":"application/pdf","data":"JVBERg=="}`)
	if parts[1].Type != "image" || parts[2].Type != "image" || parts[3].Type != "document" {
		t.Fatalf("media types = %#v", parts)
	}
}

func TestAnthropicAdapterRejectsAudioBeforeNetwork(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	t.Cleanup(server.Close)
	client := testProviderClient(t, config.ProviderConfig{Type: "anthropic", BaseURL: server.URL})
	_, err := client.Do(context.Background(), []byte(`{"messages":[{"role":"user","content":[{"type":"input_audio","input_audio":{"data":"AQID","format":"wav"}}]}]}`), "claude-sonnet-5")
	requestError, ok := err.(*RequestError)
	if !ok || requestError.Code != "unsupported_content" || calls.Load() != 0 {
		t.Fatalf("error=%v calls=%d, want unsupported_content before network", err, calls.Load())
	}
}

func assertJSONEqual(t *testing.T, got json.RawMessage, want string) {
	t.Helper()
	var actual, expected any
	if err := json.Unmarshal(got, &actual); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &expected); err != nil {
		t.Fatal(err)
	}
	gotEncoded, _ := json.Marshal(actual)
	wantEncoded, _ := json.Marshal(expected)
	if string(gotEncoded) != string(wantEncoded) {
		t.Fatalf("JSON = %s, want %s", gotEncoded, wantEncoded)
	}
}
