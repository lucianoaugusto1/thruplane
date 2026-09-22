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

type anthropicBlock struct {
	Type      string           `json:"type"`
	Text      string           `json:"text,omitempty"`
	Source    *anthropicSource `json:"source,omitempty"`
	ID        string           `json:"id,omitempty"`
	Name      string           `json:"name,omitempty"`
	Input     json.RawMessage  `json:"input,omitempty"`
	ToolUseID string           `json:"tool_use_id,omitempty"`
	Content   string           `json:"content,omitempty"`
}

type anthropicSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Data      string `json:"data,omitempty"`
	URL       string `json:"url,omitempty"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicToolChoice struct {
	Type               string `json:"type"`
	Name               string `json:"name,omitempty"`
	DisableParallelUse bool   `json:"disable_parallel_tool_use,omitempty"`
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
	tools, err := nativeTools(common)
	if err != nil {
		return nil, err
	}
	payload := struct {
		Model         string               `json:"model"`
		Messages      []anthropicMessage   `json:"messages"`
		System        string               `json:"system,omitempty"`
		MaxTokens     int                  `json:"max_tokens"`
		Temperature   *float64             `json:"temperature,omitempty"`
		TopP          *float64             `json:"top_p,omitempty"`
		StopSequences []string             `json:"stop_sequences,omitempty"`
		Stream        bool                 `json:"stream,omitempty"`
		Tools         []anthropicTool      `json:"tools,omitempty"`
		ToolChoice    *anthropicToolChoice `json:"tool_choice,omitempty"`
	}{Model: model, MaxTokens: maxOutputTokens(common, 1024), Temperature: common.Temperature, TopP: common.TopP, Stream: common.Stream}
	var systems []string
	for _, item := range common.Messages {
		switch item.Role {
		case "system", "developer":
			text, err := textContent(item.Content)
			if err != nil {
				return nil, err
			}
			systems = append(systems, text)
		case "user":
			blocks, err := anthropicUserContent(item.Content)
			if err != nil {
				return nil, err
			}
			payload.Messages = append(payload.Messages, anthropicMessage{Role: "user", Content: blocks})
		case "assistant":
			text, err := textContent(item.Content)
			if err != nil {
				return nil, err
			}
			blocks := make([]anthropicBlock, 0, 1+len(item.ToolCalls))
			if text != "" {
				blocks = append(blocks, anthropicBlock{Type: "text", Text: text})
			}
			for _, call := range item.ToolCalls {
				arguments, err := toolArguments(call.Function.Arguments)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, anthropicBlock{Type: "tool_use", ID: call.ID, Name: call.Function.Name, Input: arguments})
			}
			payload.Messages = append(payload.Messages, anthropicMessage{Role: "assistant", Content: blocks})
		case "tool":
			text, err := textContent(item.Content)
			if err != nil {
				return nil, err
			}
			result := anthropicBlock{Type: "tool_result", ToolUseID: item.ToolCallID, Content: text}
			if last := len(payload.Messages) - 1; last >= 0 && anthropicToolResultMessage(payload.Messages[last]) {
				payload.Messages[last].Content = append(payload.Messages[last].Content, result)
			} else {
				payload.Messages = append(payload.Messages, anthropicMessage{Role: "user", Content: []anthropicBlock{result}})
			}
		default:
			return nil, &RequestError{Code: "unsupported_role", Message: "The Anthropic adapter supports system, developer, user, assistant, and tool messages."}
		}
	}
	payload.System = strings.Join(systems, "\n\n")
	payload.StopSequences, err = stopSequences(common.Stop)
	if err != nil {
		return nil, err
	}
	for _, definition := range tools.Definitions {
		payload.Tools = append(payload.Tools, anthropicTool{Name: definition.Function.Name, Description: definition.Function.Description, InputSchema: definition.Function.Parameters})
	}
	payload.ToolChoice = anthropicChoice(tools.Choice, len(payload.Tools) > 0)
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

func anthropicUserContent(raw json.RawMessage) ([]anthropicBlock, error) {
	parts, err := parseNativeContent(raw)
	if err != nil {
		return nil, err
	}
	blocks := make([]anthropicBlock, 0, len(parts))
	for _, part := range parts {
		switch part.Kind {
		case "text":
			blocks = append(blocks, anthropicBlock{Type: "text", Text: part.Text})
		case "image", "document":
			source := &anthropicSource{Type: "base64", MediaType: part.MIMEType, Data: part.Data}
			if part.URL != "" {
				source = &anthropicSource{Type: "url", URL: part.URL}
			}
			blocks = append(blocks, anthropicBlock{Type: part.Kind, Source: source})
		default:
			return nil, unsupportedContent("Anthropic Messages does not support this Chat Completions input modality.")
		}
	}
	return blocks, nil
}

func anthropicToolResultMessage(message anthropicMessage) bool {
	return message.Role == "user" && len(message.Content) > 0 && message.Content[0].Type == "tool_result"
}

func anthropicChoice(choice nativeToolChoice, hasTools bool) *anthropicToolChoice {
	result := &anthropicToolChoice{DisableParallelUse: choice.DisableParallel}
	switch choice.Mode {
	case "auto":
		result.Type = "auto"
	case "none":
		result.Type = "none"
	case "required":
		result.Type = "any"
	case "named":
		result.Type = "tool"
		result.Name = choice.Name
	default:
		if !choice.DisableParallel || !hasTools {
			return nil
		}
		result.Type = "auto"
	}
	return result
}

func (a *anthropicAdapter) normalizeResponse(response *http.Response, model string, stream bool) (*http.Response, error) {
	if stream {
		return streamResponse(response, func(reader io.Reader, writer io.Writer) error {
			return transformAnthropicStream(reader, writer, model)
		}), nil
	}
	var result struct {
		ID         string           `json:"id"`
		Model      string           `json:"model"`
		Content    []anthropicBlock `json:"content"`
		StopReason string           `json:"stop_reason"`
		Usage      struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if err := decodeResponseJSON(response, &result, "Anthropic"); err != nil {
		return nil, err
	}
	var text strings.Builder
	var calls []openAIToolCall
	for _, part := range result.Content {
		switch part.Type {
		case "text":
			text.WriteString(part.Text)
		case "tool_use":
			arguments := string(part.Input)
			if arguments == "" {
				arguments = "{}"
			}
			calls = append(calls, openAIToolCall{ID: part.ID, Type: "function", Function: openAIFunctionCall{Name: part.Name, Arguments: arguments}})
		}
	}
	usage := tokenUsage{PromptTokens: result.Usage.InputTokens, CompletionTokens: result.Usage.OutputTokens}
	usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	if result.Model != "" {
		model = result.Model
	}
	return normalizedResponse(response, result.ID, model, text.String(), anthropicFinishReason(result.StopReason), usage, calls)
}

func transformAnthropicStream(reader io.Reader, writer io.Writer, model string) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	id := "chatcmpl-anthropic"
	role := "assistant"
	started := false
	finish := "stop"
	usage := tokenUsage{}
	toolIndices := make(map[int]int)
	nextToolIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		var event struct {
			Type    string `json:"type"`
			Index   int    `json:"index"`
			Message struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage struct {
					InputTokens int `json:"input_tokens"`
				} `json:"usage"`
			} `json:"message"`
			ContentBlock anthropicBlock `json:"content_block"`
			Delta        struct {
				Type        string `json:"type"`
				Text        string `json:"text"`
				PartialJSON string `json:"partial_json"`
				StopReason  string `json:"stop_reason"`
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
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				toolIndices[event.Index] = nextToolIndex
				call := openAIStreamToolCall{Index: nextToolIndex, ID: event.ContentBlock.ID, Type: "function", Function: openAIFunctionCall{Name: event.ContentBlock.Name, Arguments: ""}}
				nextToolIndex++
				if err := writeToolChunk(writer, id, model, []openAIStreamToolCall{call}); err != nil {
					return err
				}
			}
		case "content_block_delta":
			if !started {
				if err := writeChunk(writer, id, model, &role, nil, nil, nil); err != nil {
					return err
				}
				started = true
			}
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != "" {
					if err := writeChunk(writer, id, model, nil, &event.Delta.Text, nil, nil); err != nil {
						return err
					}
				}
			case "input_json_delta":
				call := openAIStreamToolCall{Index: toolIndices[event.Index], Function: openAIFunctionCall{Arguments: event.Delta.PartialJSON}}
				if err := writeToolChunk(writer, id, model, []openAIStreamToolCall{call}); err != nil {
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
