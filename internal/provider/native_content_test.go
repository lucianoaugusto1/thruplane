package provider

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestParseNativeContentPreservesOrderedMedia(t *testing.T) {
	got, err := parseNativeContent(json.RawMessage(`[
		{"type":"text","text":"first"},
		{"type":"image_url","image_url":{"url":"data:image/png;base64,AQID"}},
		{"type":"file","file":{"filename":"report.pdf","file_data":"data:application/pdf;base64,JVBERg=="}},
		{"type":"input_audio","input_audio":{"data":"AQID","format":"wav"}},
		{"type":"text","text":"last"}
	]`))
	if err != nil {
		t.Fatalf("parseNativeContent() error = %v", err)
	}
	want := []nativeContentPart{
		{Kind: "text", Text: "first"},
		{Kind: "image", MIMEType: "image/png", Data: "AQID"},
		{Kind: "document", MIMEType: "application/pdf", Data: "JVBERg=="},
		{Kind: "audio", MIMEType: "audio/wav", Data: "AQID"},
		{Kind: "text", Text: "last"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parts = %#v, want %#v", got, want)
	}
}

func TestParseNativeContentAcceptsImageHTTPSURL(t *testing.T) {
	got, err := parseNativeContent(json.RawMessage(`[{"type":"image_url","image_url":{"url":"https://images.example.com/photo.png"}}]`))
	if err != nil {
		t.Fatalf("parseNativeContent() error = %v", err)
	}
	if len(got) != 1 || got[0].Kind != "image" || got[0].URL != "https://images.example.com/photo.png" {
		t.Fatalf("parts = %#v", got)
	}
}

func TestParseNativeContentRejectsInvalidMedia(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{"invalid base64", `[{"type":"image_url","image_url":{"url":"data:image/png;base64,%%%"}}]`, "invalid_media"},
		{"unsupported MIME", `[{"type":"image_url","image_url":{"url":"data:image/svg+xml;base64,AQID"}}]`, "unsupported_content"},
		{"insecure URL", `[{"type":"image_url","image_url":{"url":"http://example.com/image.png"}}]`, "unsupported_content"},
		{"file ID", `[{"type":"file","file":{"file_id":"file_123"}}]`, "unsupported_content"},
		{"non PDF file", `[{"type":"file","file":{"file_data":"data:text/plain;base64,AQID"}}]`, "unsupported_content"},
		{"unknown part", `[{"type":"video_url","video_url":"https://example.com/a.mp4"}]`, "unsupported_content"},
		{"unsupported audio format", `[{"type":"input_audio","input_audio":{"data":"AQID","format":"flac"}}]`, "unsupported_content"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseNativeContent(json.RawMessage(test.body))
			requestErr, ok := err.(*RequestError)
			if !ok || requestErr.Code != test.code {
				t.Fatalf("error = %v, want RequestError %q", err, test.code)
			}
		})
	}
}
