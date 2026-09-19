package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type chatRequest struct {
	Messages            []chatMessage   `json:"messages"`
	MaxCompletionTokens *int            `json:"max_completion_tokens"`
	MaxTokens           *int            `json:"max_tokens"`
	Temperature         *float64        `json:"temperature"`
	TopP                *float64        `json:"top_p"`
	Stop                json.RawMessage `json:"stop"`
	Stream              bool            `json:"stream"`
}

type chatMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type tokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func decodeChatRequest(body []byte) (chatRequest, error) {
	var request chatRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return chatRequest{}, &RequestError{Code: "invalid_json", Message: "The request body must be a JSON object."}
	}
	return request, nil
}

func textContent(raw json.RawMessage) (string, error) {
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", &RequestError{Code: "unsupported_content", Message: "Native provider adapters currently support text message content only."}
	}
	var builder strings.Builder
	for _, part := range parts {
		if part.Type != "text" && part.Type != "input_text" {
			return "", &RequestError{Code: "unsupported_content", Message: "Native provider adapters currently support text message content only."}
		}
		builder.WriteString(part.Text)
	}
	return builder.String(), nil
}

func stopSequences(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		return []string{one}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, &RequestError{Code: "invalid_stop", Message: "The stop field must be a string or an array of strings."}
	}
	return many, nil
}

func maxOutputTokens(request chatRequest, fallback int) int {
	if request.MaxCompletionTokens != nil {
		return *request.MaxCompletionTokens
	}
	if request.MaxTokens != nil {
		return *request.MaxTokens
	}
	return fallback
}

func normalizedResponse(response *http.Response, id, model, text, finish string, usage tokenUsage) (*http.Response, error) {
	if id == "" {
		id = "chatcmpl-nexoroute"
	}
	payload := struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage tokenUsage `json:"usage"`
	}{ID: id, Object: "chat.completion", Created: time.Now().Unix(), Model: model, Usage: usage}
	payload.Choices = make([]struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	}, 1)
	payload.Choices[0].Message.Role = "assistant"
	payload.Choices[0].Message.Content = text
	payload.Choices[0].FinishReason = finish
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode normalized response: %w", err)
	}
	response.Body = io.NopCloser(bytes.NewReader(encoded))
	response.ContentLength = int64(len(encoded))
	response.Header.Set("Content-Type", "application/json")
	response.Header.Del("Content-Encoding")
	return response, nil
}

func writeChunk(w io.Writer, id, model string, role, content *string, finish *string, usage *tokenUsage) error {
	delta := make(map[string]string, 2)
	if role != nil {
		delta["role"] = *role
	}
	if content != nil {
		delta["content"] = *content
	}
	payload := struct {
		ID      string `json:"id"`
		Object  string `json:"object"`
		Created int64  `json:"created"`
		Model   string `json:"model"`
		Choices []struct {
			Index        int               `json:"index"`
			Delta        map[string]string `json:"delta"`
			FinishReason *string           `json:"finish_reason"`
		} `json:"choices"`
		Usage *tokenUsage `json:"usage,omitempty"`
	}{ID: id, Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: model, Usage: usage}
	payload.Choices = make([]struct {
		Index        int               `json:"index"`
		Delta        map[string]string `json:"delta"`
		FinishReason *string           `json:"finish_reason"`
	}, 1)
	payload.Choices[0].Delta = delta
	payload.Choices[0].FinishReason = finish
	encoded, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", encoded)
	return err
}

func streamResponse(response *http.Response, transform func(io.Reader, io.Writer) error) *http.Response {
	upstream := response.Body
	reader, writer := io.Pipe()
	go func() {
		defer upstream.Close()
		err := transform(upstream, writer)
		_ = writer.CloseWithError(err)
	}()
	response.Body = reader
	response.ContentLength = -1
	response.Header.Set("Content-Type", "text/event-stream")
	response.Header.Set("Cache-Control", "no-cache")
	response.Header.Del("Content-Encoding")
	return response
}
