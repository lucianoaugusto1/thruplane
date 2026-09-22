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
	Messages            []chatMessage          `json:"messages"`
	Model               string                 `json:"model"`
	N                   *int                   `json:"n"`
	MaxCompletionTokens *int                   `json:"max_completion_tokens"`
	MaxTokens           *int                   `json:"max_tokens"`
	Temperature         *float64               `json:"temperature"`
	TopP                *float64               `json:"top_p"`
	Stop                json.RawMessage        `json:"stop"`
	Stream              bool                   `json:"stream"`
	Tools               []openAIToolDefinition `json:"tools"`
	ToolChoice          json.RawMessage        `json:"tool_choice"`
	ParallelToolCalls   *bool                  `json:"parallel_tool_calls"`
	LegacyFunctions     json.RawMessage        `json:"functions"`
	LegacyFunctionCall  json.RawMessage        `json:"function_call"`
	Modalities          json.RawMessage        `json:"modalities"`
	ResponseFormat      json.RawMessage        `json:"response_format"`
}

type chatMessage struct {
	Role       string           `json:"role"`
	Content    json.RawMessage  `json:"content"`
	ToolCalls  []openAIToolCall `json:"tool_calls"`
	ToolCallID string           `json:"tool_call_id"`
}

type openAIFunctionDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
	Strict      *bool           `json:"strict,omitempty"`
}

type openAIToolDefinition struct {
	Type     string                   `json:"type"`
	Function openAIFunctionDefinition `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments"`
}

type openAIToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openAIFunctionCall `json:"function"`
}

type openAIStreamToolCall struct {
	Index    int                `json:"index"`
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function openAIFunctionCall `json:"function"`
}

type nativeToolChoice struct {
	Mode            string
	Name            string
	DisableParallel bool
}

type nativeToolContract struct {
	Definitions []openAIToolDefinition
	Choice      nativeToolChoice
	CallNames   map[string]string
}

type tokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

func decodeChatRequest(body []byte) (chatRequest, error) {
	var request chatRequest
	if trimmed := bytes.TrimSpace(body); len(trimmed) == 0 || trimmed[0] != '{' {
		return chatRequest{}, &RequestError{Code: "invalid_json", Message: "The request body must be a JSON object."}
	}
	if err := decodeStrictJSON(body, &request); err != nil {
		if isUnknownJSONField(err) {
			return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters do not support " + strings.TrimPrefix(err.Error(), "json: unknown field ") + "."}
		}
		return chatRequest{}, &RequestError{Code: "invalid_json", Message: "The request body must be a JSON object with supported field types."}
	}
	if request.N != nil && *request.N != 1 {
		return chatRequest{}, &RequestError{Code: "unsupported_n", Message: "Native adapters support n=1 only."}
	}
	if request.MaxTokens != nil && request.MaxCompletionTokens != nil {
		return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters accept max_tokens or max_completion_tokens, not both."}
	}
	if (request.MaxTokens != nil && *request.MaxTokens <= 0) || (request.MaxCompletionTokens != nil && *request.MaxCompletionTokens <= 0) {
		return chatRequest{}, &RequestError{Code: "invalid_max_tokens", Message: "The output token limit must be greater than zero."}
	}
	if hasJSONValue(request.Modalities) {
		var modalities []string
		if err := json.Unmarshal(request.Modalities, &modalities); err != nil || len(modalities) != 1 || modalities[0] != "text" {
			return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters support text-only output modalities."}
		}
	}
	if hasJSONValue(request.ResponseFormat) {
		var format struct {
			Type string `json:"type"`
		}
		if err := decodeStrictJSON(request.ResponseFormat, &format); err != nil || format.Type != "text" {
			return chatRequest{}, &RequestError{Code: "unsupported_field", Message: "Native adapters support response_format type text only."}
		}
	}
	return request, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err != nil {
			return err
		}
		return fmt.Errorf("unexpected extra JSON value")
	}
	return nil
}

func isUnknownJSONField(err error) bool {
	return strings.HasPrefix(err.Error(), "json: unknown field ")
}

func nativeTools(request chatRequest) (nativeToolContract, error) {
	if hasJSONValue(request.LegacyFunctions) || hasJSONValue(request.LegacyFunctionCall) {
		return nativeToolContract{}, &RequestError{Code: "unsupported_legacy_functions", Message: "Native adapters support tools and tool_choice, not the legacy functions fields."}
	}
	contract := nativeToolContract{CallNames: make(map[string]string)}
	for _, tool := range request.Tools {
		if tool.Type != "function" {
			return nativeToolContract{}, &RequestError{Code: "unsupported_tool_type", Message: "Native adapters currently support function tools only."}
		}
		if tool.Function.Name == "" {
			return nativeToolContract{}, &RequestError{Code: "invalid_tool", Message: "Every function tool must have a name."}
		}
		if tool.Function.Strict != nil && *tool.Function.Strict {
			return nativeToolContract{}, &RequestError{Code: "unsupported_tool_option", Message: "Native adapters do not yet provide portable strict function schemas."}
		}
		if len(tool.Function.Parameters) == 0 {
			tool.Function.Parameters = json.RawMessage(`{"type":"object","properties":{}}`)
		}
		var schema map[string]any
		if err := json.Unmarshal(tool.Function.Parameters, &schema); err != nil || schema == nil {
			return nativeToolContract{}, &RequestError{Code: "invalid_tool_schema", Message: "Function parameters must be a JSON Schema object."}
		}
		contract.Definitions = append(contract.Definitions, tool)
	}

	for _, message := range request.Messages {
		if message.Role != "tool" && message.ToolCallID != "" {
			return nativeToolContract{}, &RequestError{Code: "unsupported_field", Message: "tool_call_id is supported only on tool messages."}
		}
		for _, call := range message.ToolCalls {
			if call.Type != "function" {
				return nativeToolContract{}, &RequestError{Code: "unsupported_tool_type", Message: "Native adapters support function tool calls only."}
			}
			if message.Role != "assistant" || call.ID == "" || call.Function.Name == "" {
				return nativeToolContract{}, &RequestError{Code: "invalid_tool_call", Message: "Assistant tool calls require an id and function name."}
			}
			if _, err := toolArguments(call.Function.Arguments); err != nil {
				return nativeToolContract{}, err
			}
			contract.CallNames[call.ID] = call.Function.Name
		}
	}
	for _, message := range request.Messages {
		if message.Role == "tool" && (message.ToolCallID == "" || contract.CallNames[message.ToolCallID] == "") {
			return nativeToolContract{}, &RequestError{Code: "unmatched_tool_result", Message: "Every tool result must match an earlier assistant tool call."}
		}
	}

	choice, err := parseNativeToolChoice(request.ToolChoice)
	if err != nil {
		return nativeToolContract{}, err
	}
	if (choice.Mode == "required" || choice.Mode == "named") && len(contract.Definitions) == 0 {
		return nativeToolContract{}, &RequestError{Code: "unsupported_tool_choice", Message: "tool_choice requires at least one function tool."}
	}
	if choice.Mode == "named" {
		found := false
		for _, definition := range contract.Definitions {
			if definition.Function.Name == choice.Name {
				found = true
				break
			}
		}
		if !found {
			return nativeToolContract{}, &RequestError{Code: "unsupported_tool_choice", Message: "The named tool_choice must reference a configured function tool."}
		}
	}
	if request.ParallelToolCalls != nil && !*request.ParallelToolCalls {
		choice.DisableParallel = true
	}
	contract.Choice = choice
	return contract, nil
}

func parseNativeToolChoice(raw json.RawMessage) (nativeToolChoice, error) {
	if !hasJSONValue(raw) {
		return nativeToolChoice{}, nil
	}
	var mode string
	if err := json.Unmarshal(raw, &mode); err == nil {
		switch mode {
		case "auto", "none", "required":
			return nativeToolChoice{Mode: mode}, nil
		default:
			return nativeToolChoice{}, &RequestError{Code: "unsupported_tool_choice", Message: "tool_choice must be auto, none, required, or a named function."}
		}
	}
	var named struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := decodeStrictJSON(raw, &named); err != nil || named.Type != "function" || named.Function.Name == "" {
		return nativeToolChoice{}, &RequestError{Code: "unsupported_tool_choice", Message: "tool_choice must be auto, none, required, or a named function."}
	}
	return nativeToolChoice{Mode: "named", Name: named.Function.Name}, nil
}

func hasJSONValue(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) > 0 && !bytes.Equal(trimmed, []byte("null"))
}

func toolArguments(arguments string) (json.RawMessage, error) {
	if strings.TrimSpace(arguments) == "" {
		return json.RawMessage(`{}`), nil
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(arguments), &object); err != nil || object == nil {
		return nil, &RequestError{Code: "invalid_tool_arguments", Message: "Function arguments must encode a JSON object."}
	}
	return json.RawMessage(arguments), nil
}

func textContent(raw json.RawMessage) (string, error) {
	if !hasJSONValue(raw) {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text, nil
	}
	var parts []struct {
		Type string  `json:"type"`
		Text *string `json:"text"`
	}
	if err := decodeStrictJSON(raw, &parts); err != nil {
		return "", &RequestError{Code: "unsupported_content", Message: "Native provider adapters currently support text message content only."}
	}
	var builder strings.Builder
	for _, part := range parts {
		if (part.Type != "text" && part.Type != "input_text") || part.Text == nil {
			return "", &RequestError{Code: "unsupported_content", Message: "Native provider adapters currently support text message content only."}
		}
		builder.WriteString(*part.Text)
	}
	return builder.String(), nil
}

func stopSequences(raw json.RawMessage) ([]string, error) {
	if !hasJSONValue(raw) {
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

func normalizedResponse(response *http.Response, id, model, text, finish string, usage tokenUsage, toolCallSets ...[]openAIToolCall) (*http.Response, error) {
	if id == "" {
		id = "chatcmpl-nexoroute"
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
