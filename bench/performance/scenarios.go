package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	benchmarkModelAlias = "benchmark-model"
	minimumMediaBytes   = 48 << 10
	benchmarkIDMarker   = "{{benchmark_id}}"
)

type upstreamMode string

const (
	upstreamSuccess            upstreamMode = "success"
	upstreamRetryOnce          upstreamMode = "retry_once"
	upstreamUnavailable        upstreamMode = "unavailable"
	upstreamPermanentRateLimit upstreamMode = "permanent_rate_limit"
	upstreamDelayed            upstreamMode = "delayed"
)

type targetScenario struct {
	Name         string
	ProviderType string
	Model        string
	Mode         upstreamMode
}

type performanceScenario struct {
	Name               string
	BodyTemplate       string
	Targets            []targetScenario
	Retries            int
	Stream             bool
	MediaBytes         int
	SlowReadDelay      time.Duration
	CancelAfter        time.Duration
	ExpectCancellation bool
}

var scenarioCatalog = buildPerformanceScenarios()

func scenarioNames() []string {
	names := make([]string, 0, len(scenarioCatalog))
	for name := range scenarioCatalog {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func scenarioByName(name string) (performanceScenario, error) {
	scenario, ok := scenarioCatalog[name]
	if !ok {
		return performanceScenario{}, fmt.Errorf("unknown performance scenario %q", name)
	}
	return scenario, nil
}

func buildPerformanceScenarios() map[string]performanceScenario {
	media := syntheticMedia(minimumMediaBytes)
	compatible := targetScenario{Name: "compatible", ProviderType: "openai-compatible", Model: "mock-compatible", Mode: upstreamSuccess}
	anthropic := targetScenario{Name: "anthropic", ProviderType: "anthropic", Model: "claude-benchmark", Mode: upstreamSuccess}
	gemini := targetScenario{Name: "gemini", ProviderType: "gemini", Model: "gemini-benchmark", Mode: upstreamSuccess}

	return map[string]performanceScenario{
		"text": {
			Name: "text", Targets: []targetScenario{compatible},
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":"hello"}]}`,
		},
		"tools": {
			Name: "tools", Targets: []targetScenario{anthropic},
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":"weather in Rio"}],"tools":[{"type":"function","function":{"name":"forecast","description":"Get a forecast","parameters":{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}}}],"tool_choice":"auto"}`,
		},
		"image": {
			Name: "image", Targets: []targetScenario{anthropic}, MediaBytes: minimumMediaBytes,
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":[{"type":"text","text":"describe"},{"type":"image_url","image_url":{"url":"data:image/png;base64,` + media + `"}}]}]}`,
		},
		"pdf": {
			Name: "pdf", Targets: []targetScenario{anthropic}, MediaBytes: minimumMediaBytes,
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":[{"type":"text","text":"summarize"},{"type":"file","file":{"file_data":"data:application/pdf;base64,` + media + `"}}]}]}`,
		},
		"audio": {
			Name: "audio", Targets: []targetScenario{gemini}, MediaBytes: minimumMediaBytes,
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":[{"type":"text","text":"transcribe"},{"type":"input_audio","input_audio":{"data":"` + media + `","format":"wav"}}]}]}`,
		},
		"sse": {
			Name: "sse", Targets: []targetScenario{anthropic}, Stream: true,
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":"stream"}],"stream":true}`,
		},
		"retry": {
			Name: "retry", Retries: 1,
			Targets:      []targetScenario{{Name: "retry", ProviderType: "openai-compatible", Model: "mock-retry", Mode: upstreamRetryOnce}},
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":"retry"}],"benchmark_id":"` + benchmarkIDMarker + `"}`,
		},
		"fallback": {
			Name: "fallback", Retries: 0,
			Targets: []targetScenario{
				{Name: "primary", ProviderType: "openai-compatible", Model: "mock-primary", Mode: upstreamUnavailable},
				{Name: "secondary", ProviderType: "openai-compatible", Model: "mock-secondary", Mode: upstreamSuccess},
			},
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":"fallback"}]}`,
		},
		"rate-limit-429": {
			Name: "rate-limit-429", Retries: 1,
			Targets: []targetScenario{
				{Name: "quota", ProviderType: "openai-compatible", Model: "mock-quota", Mode: upstreamPermanentRateLimit},
				{Name: "secondary", ProviderType: "openai-compatible", Model: "mock-secondary", Mode: upstreamSuccess},
			},
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":"quota"}]}`,
		},
		"slow-client": {
			Name: "slow-client", Targets: []targetScenario{anthropic}, Stream: true,
			SlowReadDelay: 250 * time.Microsecond,
			BodyTemplate:  `{"model":"benchmark-model","messages":[{"role":"user","content":"stream slowly"}],"stream":true}`,
		},
		"cancellation": {
			Name: "cancellation", Retries: 1, ExpectCancellation: true,
			CancelAfter:  5 * time.Millisecond,
			Targets:      []targetScenario{{Name: "delayed", ProviderType: "openai-compatible", Model: "mock-delayed", Mode: upstreamDelayed}},
			BodyTemplate: `{"model":"benchmark-model","messages":[{"role":"user","content":"cancel"}]}`,
		},
	}
}

func (s performanceScenario) requestBody(id uint64) []byte {
	body := s.BodyTemplate
	if strings.Contains(body, benchmarkIDMarker) {
		body = strings.ReplaceAll(body, benchmarkIDMarker, strconv.FormatUint(id, 10))
	}
	return []byte(body)
}

func (s performanceScenario) directTarget() targetScenario {
	return s.Targets[len(s.Targets)-1]
}

func syntheticMedia(size int) string {
	data := make([]byte, size)
	for index := range data {
		data[index] = byte(index % 251)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func writeSuccessfulResponse(w http.ResponseWriter, r *http.Request, scenario performanceScenario, target targetScenario) {
	if scenario.Stream {
		writeStreamResponse(w, r, scenario, target)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	switch target.ProviderType {
	case "anthropic":
		_, _ = io.WriteString(w, `{"id":"msg_benchmark","model":"claude-benchmark","content":[{"type":"text","text":"ok"}],"stop_reason":"end_turn","usage":{"input_tokens":8,"output_tokens":2}}`)
	case "gemini", "vertex":
		_, _ = io.WriteString(w, `{"responseId":"gemini-benchmark","modelVersion":"gemini-benchmark","candidates":[{"content":{"role":"model","parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":8,"candidatesTokenCount":2,"totalTokenCount":10}}`)
	default:
		_, _ = io.WriteString(w, `{"id":"chatcmpl-benchmark","object":"chat.completion","model":"mock-compatible","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{"prompt_tokens":8,"completion_tokens":2,"total_tokens":10}}`)
	}
}

func writeStreamResponse(w http.ResponseWriter, r *http.Request, scenario performanceScenario, target targetScenario) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	flusher, _ := w.(http.Flusher)
	if !waitForRequest(r, 2*time.Millisecond) {
		return
	}

	if target.ProviderType != "anthropic" {
		_, _ = io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"ok\"}}]}\n\n")
		if flusher != nil {
			flusher.Flush()
		}
		_, _ = io.WriteString(w, "data: [DONE]\n\n")
		return
	}

	_, _ = io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_stream\",\"model\":\"claude-benchmark\",\"usage\":{\"input_tokens\":8}}}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
	chunks := 2
	if scenario.SlowReadDelay > 0 {
		chunks = 32
	}
	for index := 0; index < chunks; index++ {
		_, _ = fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"index\":0,\"delta\":{\"type\":\"text_delta\",\"text\":\"chunk-%d\"}}\n\n", index)
		if flusher != nil {
			flusher.Flush()
		}
	}
	_, _ = io.WriteString(w, "data: {\"type\":\"message_delta\",\"delta\":{\"stop_reason\":\"end_turn\"},\"usage\":{\"output_tokens\":2}}\n\n")
	_, _ = io.WriteString(w, "data: {\"type\":\"message_stop\"}\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func waitForRequest(r *http.Request, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-r.Context().Done():
		return false
	case <-timer.C:
		return true
	}
}
