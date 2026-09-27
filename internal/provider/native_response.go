package provider

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

func normalizedResponse(response *http.Response, id, model, text, finish string, usage tokenUsage, toolCallSets ...[]openAIToolCall) (*http.Response, error) {
	if id == "" {
		id = "chatcmpl-thruplane"
	}
	type message struct {
		Role      string           `json:"role"`
		Content   *string          `json:"content"`
		ToolCalls []openAIToolCall `json:"tool_calls,omitempty"`
	}
	type choice struct {
		Index        int     `json:"index"`
		Message      message `json:"message"`
		FinishReason string  `json:"finish_reason"`
	}
	payload := struct {
		ID      string     `json:"id"`
		Object  string     `json:"object"`
		Created int64      `json:"created"`
		Model   string     `json:"model"`
		Choices []choice   `json:"choices"`
		Usage   tokenUsage `json:"usage"`
	}{ID: id, Object: "chat.completion", Created: time.Now().Unix(), Model: model, Usage: usage, Choices: make([]choice, 1)}
	payload.Choices[0].Message.Role = "assistant"
	var toolCalls []openAIToolCall
	if len(toolCallSets) > 0 {
		toolCalls = toolCallSets[0]
	}
	if text != "" || len(toolCalls) == 0 {
		payload.Choices[0].Message.Content = &text
	}
	payload.Choices[0].Message.ToolCalls = toolCalls
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

func decodeResponseJSON(response *http.Response, target any, providerName string) error {
	upstream := response.Body
	defer upstream.Close()
	if err := json.NewDecoder(upstream).Decode(target); err != nil {
		return fmt.Errorf("decode %s response: %w", providerName, err)
	}
	_, _ = io.Copy(io.Discard, upstream)
	return nil
}

func writeChunk(w io.Writer, id, model string, role, content *string, finish *string, usage *tokenUsage) error {
	delta := make(map[string]any, 2)
	if role != nil {
		delta["role"] = *role
	}
	if content != nil {
		delta["content"] = *content
	}
	return writeDeltaChunk(w, id, model, delta, finish, usage)
}

func writeToolChunk(w io.Writer, id, model string, calls []openAIStreamToolCall) error {
	return writeDeltaChunk(w, id, model, map[string]any{"tool_calls": calls}, nil, nil)
}

func writeDeltaChunk(w io.Writer, id, model string, delta map[string]any, finish *string, usage *tokenUsage) error {
	type choice struct {
		Index        int            `json:"index"`
		Delta        map[string]any `json:"delta"`
		FinishReason *string        `json:"finish_reason"`
	}
	payload := struct {
		ID      string      `json:"id"`
		Object  string      `json:"object"`
		Created int64       `json:"created"`
		Model   string      `json:"model"`
		Choices []choice    `json:"choices"`
		Usage   *tokenUsage `json:"usage,omitempty"`
	}{ID: id, Object: "chat.completion.chunk", Created: time.Now().Unix(), Model: model, Usage: usage, Choices: make([]choice, 1)}
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
