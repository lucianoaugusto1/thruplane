package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"nexoroute/internal/config"
)

type anthropicAdapter struct {
	endpoint   string
	apiKey     string
	apiVersion string
}

func newAnthropicAdapter(cfg config.ProviderConfig) (adapter, error) {
	endpoint, err := providerEndpoint(cfg.BaseURL, "/v1/messages")
	if err != nil {
		return nil, err
	}
	version := cfg.APIVersion
	if version == "" {
		version = "2023-06-01"
	}
	return &anthropicAdapter{endpoint: endpoint, apiKey: cfg.APIKey, apiVersion: version}, nil
}

func (a *anthropicAdapter) buildRequest(ctx context.Context, body []byte, model string) (*http.Request, error) {
	common, err := decodeChatRequest(body)
	if err != nil {
		return nil, err
	}
	type message struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	payload := struct {
		Model         string    `json:"model"`
		Messages      []message `json:"messages"`
		System        string    `json:"system,omitempty"`
		MaxTokens     int       `json:"max_tokens"`
		Temperature   *float64  `json:"temperature,omitempty"`
		TopP          *float64  `json:"top_p,omitempty"`
		StopSequences []string  `json:"stop_sequences,omitempty"`
		Stream        bool      `json:"stream,omitempty"`
	}{Model: model, MaxTokens: maxOutputTokens(common, 1024), Temperature: common.Temperature, TopP: common.TopP, Stream: common.Stream}
	var systems []string
	for _, item := range common.Messages {
		text, err := textContent(item.Content)
		if err != nil {
			return nil, err
		}
		switch item.Role {
		case "system", "developer":
			systems = append(systems, text)
		case "user", "assistant":
			payload.Messages = append(payload.Messages, message{Role: item.Role, Content: text})
		default:
			return nil, &RequestError{Code: "unsupported_role", Message: "The Anthropic adapter supports system, developer, user, and assistant messages."}
		}
	}
	payload.System = strings.Join(systems, "\n\n")
	payload.StopSequences, err = stopSequences(common.Stop)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Anthropic request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create Anthropic request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("anthropic-version", a.apiVersion)
	if a.apiKey != "" {
		request.Header.Set("x-api-key", a.apiKey)
	}
	return request, nil
}

func (a *anthropicAdapter) normalizeResponse(response *http.Response, model string, stream bool) (*http.Response, error) {
	if stream {
		return streamResponse(response, func(reader io.Reader, writer io.Writer) error {
			return transformAnthropicStream(reader, writer, model)
		}), nil
	}
	var result struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		StopReason string `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Anthropic response: %w", err)
	}
	var text strings.Builder
	for _, part := range result.Content {
		if part.Type == "text" {
			text.WriteString(part.Text)
		}
	}
	usage := tokenUsage{PromptTokens: result.Usage.InputTokens, CompletionTokens: result.Usage.OutputTokens}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	if result.Model != "" {
		model = result.Model
	}
	return normalizedResponse(response, result.ID, model, text.String(), anthropicFinishReason(result.StopReason), usage)
}

func transformAnthropicStream(reader io.Reader, writer io.Writer, model string) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	id := "chatcmpl-anthropic"
	role := "assistant"
	started := false
	finish := "stop"
	usage := tokenUsage{}
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event struct {
			Type    string `json:"type"`
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			Delta struct {
				Type       string `json:"type"`
				Text       string `json:"text"`
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage struct {
				OutputTokens int `json:"output_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return fmt.Errorf("decode Anthropic stream event: %w", err)
		}
		switch event.Type {
		case "message_start":
			if event.Message.ID != "" {
				id = event.Message.ID
			}
			if event.Message.Model != "" {
				model = event.Message.Model
			}
			usage.PromptTokens = event.Message.Usage.InputTokens
			if err := writeChunk(writer, id, model, &role, nil, nil, nil); err != nil {
				return err
			}
			started = true
		case "content_block_delta":
			if !started {
				if err := writeChunk(writer, id, model, &role, nil, nil, nil); err != nil {
					return err
				}
				started = true
			}
			if event.Delta.Text != "" {
				if err := writeChunk(writer, id, model, nil, &event.Delta.Text, nil, nil); err != nil {
					return err
				}
			}
		case "message_delta":
			finish = anthropicFinishReason(event.Delta.StopReason)
			usage.CompletionTokens = event.Usage.OutputTokens
		case "message_stop":
			usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			if err := writeChunk(writer, id, model, nil, nil, &finish, &usage); err != nil {
				return err
			}
			_, err := io.WriteString(writer, "data: [DONE]\n\n")
			return err
		}
	}
	return scanner.Err()
}

func anthropicFinishReason(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	default:
		return "stop"
	}
}
