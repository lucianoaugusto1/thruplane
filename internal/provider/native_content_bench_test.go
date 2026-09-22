package provider

import (
	"encoding/json"
	"strings"
	"testing"
)

func BenchmarkNativeInlineImageTranslation(b *testing.B) {
	// 64 KiB of base64 represents 48 KiB of image bytes. The payload is fixed
	// so the benchmark measures local validation and block translation only.
	raw := json.RawMessage(`[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + strings.Repeat("AQID", 16*1024) + `"}}]`)
	cases := []struct {
		name      string
		translate func(json.RawMessage) error
	}{
		{"anthropic", func(raw json.RawMessage) error { _, err := anthropicUserContent(raw); return err }},
		{"gemini_vertex", func(raw json.RawMessage) error { _, err := googleUserParts(raw); return err }},
		{"bedrock", func(raw json.RawMessage) error { _, err := bedrockUserContent(raw); return err }},
	}
	for _, test := range cases {
		b.Run(test.name, func(b *testing.B) {
			b.SetBytes(int64(len(raw)))
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if err := test.translate(raw); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
