//go:build live

package providerlive

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lucianoaugusto1/thruplane/internal/config"
	"github.com/lucianoaugusto1/thruplane/internal/provider"
)

const maxLiveResponseBytes = 4 << 20

type liveSettings struct {
	provider  string
	model     string
	scenarios []string
	maxTokens int
	timeout   time.Duration
}

type liveScenario struct {
	body     []byte
	toolName string
	stream   bool
}

func TestNativeProviderLive(t *testing.T) {
	settings := loadLiveSettings(t)
	providerConfig := liveProviderConfig(t, settings.provider)
	clients, err := provider.NewClients(config.Config{
		Providers: map[string]config.ProviderConfig{"live": providerConfig},
		Routing: config.RoutingConfig{
			ResponseHeaderTimeout: config.Duration(settings.timeout),
		},
	})
	if err != nil {
		t.Fatalf("create provider client: %v", err)
	}
	client := clients["live"]
	for _, name := range settings.scenarios {
		name := name
		t.Run(name, func(t *testing.T) {
			scenario, err := buildLiveScenario(settings, name)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), settings.timeout)
			defer cancel()
			response, err := client.Do(ctx, scenario.body, settings.model)
			if err != nil {
				t.Fatalf("provider request failed: %v", err)
			}
			defer response.Body.Close()
			if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
				t.Fatalf("provider status = %d, request_id = %q", response.StatusCode, providerRequestID(response.Header))
			}
			body, err := io.ReadAll(io.LimitReader(response.Body, maxLiveResponseBytes+1))
			if err != nil {
				t.Fatalf("read provider response: %v", err)
			}
			if len(body) > maxLiveResponseBytes {
				t.Fatalf("provider response exceeds %d bytes", maxLiveResponseBytes)
			}
			if scenario.stream {
				if !bytes.Contains(body, []byte("data:")) || !bytes.Contains(body, []byte("data: [DONE]")) {
					t.Fatalf("normalized stream is incomplete")
				}
				return
			}
			assertLiveCompletion(t, body, scenario.toolName)
		})
	}
}

func loadLiveSettings(t *testing.T) liveSettings {
	t.Helper()
	settings := liveSettings{
		provider: strings.ToLower(requiredLiveEnv(t, "THRUPLANE_LIVE_PROVIDER")),
		model:    requiredLiveEnv(t, "THRUPLANE_LIVE_MODEL"),
	}
	scenarioList := requiredLiveEnv(t, "THRUPLANE_LIVE_SCENARIOS")
	seen := make(map[string]bool)
	for _, value := range strings.Split(scenarioList, ",") {
		name := strings.ToLower(strings.TrimSpace(value))
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		settings.scenarios = append(settings.scenarios, name)
	}
	if len(settings.scenarios) == 0 {
		t.Fatal("THRUPLANE_LIVE_SCENARIOS must select at least one scenario")
	}
	maxTokens, err := strconv.Atoi(requiredLiveEnv(t, "THRUPLANE_LIVE_MAX_TOKENS"))
	if err != nil || maxTokens < 1 || maxTokens > 128 {
		t.Fatal("THRUPLANE_LIVE_MAX_TOKENS must be an integer from 1 to 128")
	}
	settings.maxTokens = maxTokens
	settings.timeout = 90 * time.Second
	if value := strings.TrimSpace(os.Getenv("THRUPLANE_LIVE_TIMEOUT")); value != "" {
		settings.timeout, err = time.ParseDuration(value)
		if err != nil || settings.timeout <= 0 || settings.timeout > 10*time.Minute {
			t.Fatal("THRUPLANE_LIVE_TIMEOUT must be greater than zero and at most 10m")
		}
	}
	return settings
}

func requiredLiveEnv(t *testing.T, name string) string {
	t.Helper()
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		t.Fatalf("%s is required for live provider tests", name)
	}
	return value
}

func liveProviderConfig(t *testing.T, providerType string) config.ProviderConfig {
	t.Helper()
	cfg := config.ProviderConfig{Type: providerType}
	switch providerType {
	case "anthropic":
		cfg.BaseURL = "https://api.anthropic.com"
		cfg.APIKey = requiredLiveEnv(t, "ANTHROPIC_API_KEY")
	case "gemini":
		cfg.BaseURL = "https://generativelanguage.googleapis.com"
		cfg.APIKey = requiredLiveEnv(t, "GEMINI_API_KEY")
	case "vertex":
		cfg.Project = requiredLiveEnv(t, "GOOGLE_CLOUD_PROJECT")
		cfg.Location = requiredLiveEnv(t, "GOOGLE_CLOUD_LOCATION")
		cfg.AccessToken = requiredLiveEnv(t, "GOOGLE_ACCESS_TOKEN")
		cfg.BaseURL = "https://" + cfg.Location + "-aiplatform.googleapis.com"
	case "bedrock":
		cfg.Region = requiredLiveEnv(t, "AWS_REGION")
		cfg.AccessKeyID = requiredLiveEnv(t, "AWS_ACCESS_KEY_ID")
		cfg.SecretAccessKey = requiredLiveEnv(t, "AWS_SECRET_ACCESS_KEY")
		cfg.SessionToken = strings.TrimSpace(os.Getenv("AWS_SESSION_TOKEN"))
		cfg.BaseURL = "https://bedrock-runtime." + cfg.Region + ".amazonaws.com"
	default:
		t.Fatalf("THRUPLANE_LIVE_PROVIDER must be anthropic, gemini, vertex, or bedrock")
	}
	if override := strings.TrimSpace(os.Getenv("THRUPLANE_LIVE_BASE_URL")); override != "" {
		cfg.BaseURL = strings.TrimRight(override, "/")
	}
	return cfg
}

func buildLiveScenario(settings liveSettings, name string) (liveScenario, error) {
	supported := map[string]map[string]bool{
		"anthropic": {"text": true, "tools": true, "image": true, "pdf": true, "stream": true},
		"gemini":    {"text": true, "tools": true, "image": true, "pdf": true, "audio": true, "stream": true},
		"vertex":    {"text": true, "tools": true, "image": true, "pdf": true, "audio": true, "stream": true},
		"bedrock":   {"text": true, "tools": true, "image": true, "pdf": true},
	}
	if !supported[settings.provider][name] {
		return liveScenario{}, fmt.Errorf("scenario %q is not supported by the %s adapter", name, settings.provider)
	}
	request := map[string]any{
		"model":                 "live",
		"max_completion_tokens": settings.maxTokens,
	}
	scenario := liveScenario{}
	switch name {
	case "text":
		request["messages"] = []any{map[string]any{"role": "user", "content": "Reply with one short sentence."}}
	case "tools":
		scenario.toolName = "thruplane_probe"
		request["messages"] = []any{map[string]any{"role": "user", "content": "Call thruplane_probe with value ready."}}
		request["tools"] = []any{map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": scenario.toolName, "description": "Return the supplied probe value.",
				"parameters": map[string]any{
					"type": "object", "properties": map[string]any{"value": map[string]any{"type": "string"}},
					"required": []string{"value"},
				},
			},
		}}
		request["tool_choice"] = map[string]any{"type": "function", "function": map[string]any{"name": scenario.toolName}}
	case "image":
		uri, err := livePNGDataURI()
		if err != nil {
			return liveScenario{}, err
		}
		request["messages"] = []any{liveMediaMessage("Describe this image briefly.", "image_url", map[string]any{"url": uri})}
	case "pdf":
		request["messages"] = []any{liveMediaMessage("Read this PDF briefly.", "file", map[string]any{"file_data": livePDFDataURI()})}
	case "audio":
		request["messages"] = []any{liveMediaMessage("Describe this audio briefly.", "input_audio", map[string]any{
			"data": base64.StdEncoding.EncodeToString(liveWAV()), "format": "wav",
		})}
	case "stream":
		request["messages"] = []any{map[string]any{"role": "user", "content": "Reply with one short sentence."}}
		request["stream"] = true
		scenario.stream = true
	default:
		return liveScenario{}, fmt.Errorf("unknown live scenario %q", name)
	}
	body, err := json.Marshal(request)
	if err != nil {
		return liveScenario{}, fmt.Errorf("encode live scenario: %w", err)
	}
	scenario.body = body
	return scenario, nil
}

func liveMediaMessage(prompt, mediaType string, media map[string]any) map[string]any {
	return map[string]any{
		"role": "user",
		"content": []any{
			map[string]any{"type": "text", "text": prompt},
			map[string]any{"type": mediaType, mediaType: media},
		},
	}
}

func livePNGDataURI() (string, error) {
	var encoded bytes.Buffer
	pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
	pixel.Set(0, 0, color.White)
	if err := png.Encode(&encoded, pixel); err != nil {
		return "", fmt.Errorf("encode live PNG: %w", err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(encoded.Bytes()), nil
}

func livePDFDataURI() string {
	content := "BT /F1 12 Tf 20 100 Td (Thruplane live PDF) Tj ET"
	objects := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 200 200] /Resources << /Font << /F1 5 0 R >> >> /Contents 4 0 R >>",
		fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(content), content),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
	}
	var document strings.Builder
	document.WriteString("%PDF-1.4\n")
	offsets := make([]int, len(objects)+1)
	for index, object := range objects {
		offsets[index+1] = document.Len()
		fmt.Fprintf(&document, "%d 0 obj\n%s\nendobj\n", index+1, object)
	}
	xref := document.Len()
	fmt.Fprintf(&document, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, offset := range offsets[1:] {
		fmt.Fprintf(&document, "%010d 00000 n \n", offset)
	}
	fmt.Fprintf(&document, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	return "data:application/pdf;base64," + base64.StdEncoding.EncodeToString([]byte(document.String()))
}

func liveWAV() []byte {
	const sampleRate = 8000
	const samples = 800
	dataSize := samples * 2
	result := make([]byte, 44+dataSize)
	copy(result[0:4], "RIFF")
	binary.LittleEndian.PutUint32(result[4:8], uint32(36+dataSize))
	copy(result[8:12], "WAVE")
	copy(result[12:16], "fmt ")
	binary.LittleEndian.PutUint32(result[16:20], 16)
	binary.LittleEndian.PutUint16(result[20:22], 1)
	binary.LittleEndian.PutUint16(result[22:24], 1)
	binary.LittleEndian.PutUint32(result[24:28], sampleRate)
	binary.LittleEndian.PutUint32(result[28:32], sampleRate*2)
	binary.LittleEndian.PutUint16(result[32:34], 2)
	binary.LittleEndian.PutUint16(result[34:36], 16)
	copy(result[36:40], "data")
	binary.LittleEndian.PutUint32(result[40:44], uint32(dataSize))
	return result
}

func assertLiveCompletion(t *testing.T, body []byte, toolName string) {
	t.Helper()
	var result struct {
		Choices []struct {
			Message struct {
				Content   *string `json:"content"`
				ToolCalls []struct {
					Type     string `json:"type"`
					Function struct {
						Name string `json:"name"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		t.Fatalf("decode normalized response: %v", err)
	}
	if len(result.Choices) == 0 {
		t.Fatal("normalized response has no choices")
	}
	message := result.Choices[0].Message
	if toolName == "" {
		if message.Content == nil || strings.TrimSpace(*message.Content) == "" {
			t.Fatal("normalized response has no text content")
		}
		return
	}
	if len(message.ToolCalls) == 0 || message.ToolCalls[0].Type != "function" || message.ToolCalls[0].Function.Name != toolName {
		t.Fatalf("normalized response did not call %s", toolName)
	}
}

func providerRequestID(headers http.Header) string {
	for _, name := range []string{"request-id", "x-request-id", "x-amzn-requestid", "x-goog-request-id"} {
		if value := headers.Get(name); value != "" {
			return value
		}
	}
	return ""
}
