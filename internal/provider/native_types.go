package provider

import "encoding/json"

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
