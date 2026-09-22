package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"nexoroute/internal/config"
)

type googleAdapter struct {
	baseURL     string
	apiKey      string
	accessToken string
	project     string
	location    string
	vertex      bool
}

type googleFunctionCall struct {
	ID   string          `json:"id,omitempty"`
	Name string          `json:"name"`
	Args json.RawMessage `json:"args"`
}

type googleFunctionResponse struct {
	ID       string          `json:"id,omitempty"`
	Name     string          `json:"name"`
	Response json.RawMessage `json:"response"`
}

type googlePart struct {
	Text             string                  `json:"text,omitempty"`
	InlineData       *googleInlineData       `json:"inlineData,omitempty"`
	FunctionCall     *googleFunctionCall     `json:"functionCall,omitempty"`
	FunctionResponse *googleFunctionResponse `json:"functionResponse,omitempty"`
}

type googleInlineData struct {
	MIMEType string `json:"mimeType"`
	Data     string `json:"data"`
}

type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
}

type googleFunctionDeclaration struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type googleTool struct {
	FunctionDeclarations []googleFunctionDeclaration `json:"functionDeclarations"`
}

type googleToolConfig struct {
	FunctionCallingConfig struct {
		Mode                 string   `json:"mode"`
		AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
	} `json:"functionCallingConfig"`
}

type googleResponse struct {
	ResponseID string `json:"responseId"`
	Model      string `json:"modelVersion"`
	Candidates []struct {
		Content      googleContent `json:"content"`
		FinishReason string        `json:"finishReason"`
	} `json:"candidates"`
	Usage struct {
		PromptTokens     int `json:"promptTokenCount"`
		CompletionTokens int `json:"candidatesTokenCount"`
		TotalTokens      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

func newGoogleAdapter(cfg config.ProviderConfig, vertex bool) (adapter, error) {
	if _, err := providerEndpoint(cfg.BaseURL, ""); err != nil {
		return nil, err
	}
	return &googleAdapter{
		baseURL: cfg.BaseURL, apiKey: cfg.APIKey, accessToken: cfg.AccessToken,
		project: cfg.Project, location: cfg.Location, vertex: vertex,
	}, nil
}

func (a *googleAdapter) buildRequest(ctx context.Context, body []byte, model string) (*http.Request, error) {
	common, err := decodeChatRequest(body)
	if err != nil {
		return nil, err
	}
	tools, err := nativeTools(common)
	if err != nil {
		return nil, err
	}
	if tools.Choice.DisableParallel && len(tools.Definitions) > 0 {
		return nil, &RequestError{Code: "unsupported_parallel_tool_calls", Message: "Gemini and Vertex cannot portably enforce parallel_tool_calls=false."}
	}
	payload := struct {
		Contents          []googleContent `json:"contents"`
		SystemInstruction *googleContent  `json:"systemInstruction,omitempty"`
		GenerationConfig  struct {
			MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
			Temperature     *float64 `json:"temperature,omitempty"`
			TopP            *float64 `json:"topP,omitempty"`
			StopSequences   []string `json:"stopSequences,omitempty"`
		} `json:"generationConfig,omitempty"`
		Tools      []googleTool      `json:"tools,omitempty"`
		ToolConfig *googleToolConfig `json:"toolConfig,omitempty"`
	}{}
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
			parts, err := googleUserParts(item.Content)
			if err != nil {
				return nil, err
			}
			payload.Contents = append(payload.Contents, googleContent{Role: "user", Parts: parts})
		case "assistant":
			text, err := textContent(item.Content)
			if err != nil {
				return nil, err
			}
			parts := make([]googlePart, 0, 1+len(item.ToolCalls))
			if text != "" {
				parts = append(parts, googlePart{Text: text})
			}
			for _, call := range item.ToolCalls {
				arguments, err := toolArguments(call.Function.Arguments)
				if err != nil {
					return nil, err
				}
				parts = append(parts, googlePart{FunctionCall: &googleFunctionCall{ID: call.ID, Name: call.Function.Name, Args: arguments}})
			}
			payload.Contents = append(payload.Contents, googleContent{Role: "model", Parts: parts})
		case "tool":
			result, err := googleToolResult(item.Content)
			if err != nil {
				return nil, err
			}
			part := googlePart{FunctionResponse: &googleFunctionResponse{ID: item.ToolCallID, Name: tools.CallNames[item.ToolCallID], Response: result}}
			if last := len(payload.Contents) - 1; last >= 0 && googleToolResultContent(payload.Contents[last]) {
				payload.Contents[last].Parts = append(payload.Contents[last].Parts, part)
			} else {
				payload.Contents = append(payload.Contents, googleContent{Role: "user", Parts: []googlePart{part}})
			}
		default:
			return nil, &RequestError{Code: "unsupported_role", Message: "The Google adapters support system, developer, user, assistant, and tool messages."}
		}
	}
	if len(systems) > 0 {
		payload.SystemInstruction = &googleContent{Parts: []googlePart{{Text: strings.Join(systems, "\n\n")}}}
	}
	payload.GenerationConfig.MaxOutputTokens = maxOutputTokens(common, 0)
	payload.GenerationConfig.Temperature = common.Temperature
	payload.GenerationConfig.TopP = common.TopP
	payload.GenerationConfig.StopSequences, err = stopSequences(common.Stop)
	if err != nil {
		return nil, err
	}
	if len(tools.Definitions) > 0 {
		declarations := make([]googleFunctionDeclaration, 0, len(tools.Definitions))
		for _, definition := range tools.Definitions {
			declarations = append(declarations, googleFunctionDeclaration{Name: definition.Function.Name, Description: definition.Function.Description, Parameters: definition.Function.Parameters})
		}
		payload.Tools = []googleTool{{FunctionDeclarations: declarations}}
	}
	payload.ToolConfig = googleChoice(tools.Choice, len(payload.Tools) > 0)
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Google request: %w", err)
	}
	endpoint := a.endpoint(model, common.Stream)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create Google request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if a.vertex {
		request.Header.Set("Authorization", "Bearer "+a.accessToken)
	} else if a.apiKey != "" {
		request.Header.Set("x-goog-api-key", a.apiKey)
	}
	return request, nil
}

func googleUserParts(raw json.RawMessage) ([]googlePart, error) {
	parts, err := parseNativeContent(raw)
	if err != nil {
		return nil, err
	}
	result := make([]googlePart, 0, len(parts))
	for _, part := range parts {
		switch part.Kind {
		case "text":
			result = append(result, googlePart{Text: part.Text})
		case "image", "document", "audio":
			if part.URL != "" {
				return nil, unsupportedContent("Gemini and Vertex require inline media in this Chat Completions adapter.")
			}
			result = append(result, googlePart{InlineData: &googleInlineData{MIMEType: part.MIMEType, Data: part.Data}})
		default:
			return nil, unsupportedContent("Gemini and Vertex do not support this Chat Completions input modality.")
		}
	}
	return result, nil
}

func googleToolResultContent(content googleContent) bool {
	return content.Role == "user" && len(content.Parts) > 0 && content.Parts[0].FunctionResponse != nil
}

func googleToolResult(content json.RawMessage) (json.RawMessage, error) {
	text, err := textContent(content)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if json.Unmarshal([]byte(text), &object) == nil && object != nil {
		return json.RawMessage(text), nil
	}
	encoded, err := json.Marshal(map[string]any{"result": text})
	if err != nil {
		return nil, fmt.Errorf("encode Google tool result: %w", err)
	}
	return encoded, nil
}

func googleChoice(choice nativeToolChoice, hasTools bool) *googleToolConfig {
	if !hasTools && choice.Mode == "none" {
		return nil
	}
	result := &googleToolConfig{}
	switch choice.Mode {
	case "auto":
		result.FunctionCallingConfig.Mode = "AUTO"
	case "none":
		result.FunctionCallingConfig.Mode = "NONE"
	case "required":
		result.FunctionCallingConfig.Mode = "ANY"
	case "named":
		result.FunctionCallingConfig.Mode = "ANY"
		result.FunctionCallingConfig.AllowedFunctionNames = []string{choice.Name}
	default:
		return nil
	}
	return result
}

func (a *googleAdapter) endpoint(model string, stream bool) string {
	model = strings.TrimPrefix(model, "models/")
	action := "generateContent"
	if stream {
		action = "streamGenerateContent"
	}
	var path string
	if a.vertex {
		path = "/v1/projects/" + url.PathEscape(a.project) + "/locations/" + url.PathEscape(a.location) + "/publishers/google/models/" + url.PathEscape(model) + ":" + action
	} else {
		path = "/v1beta/models/" + url.PathEscape(model) + ":" + action
	}
	endpoint := strings.TrimRight(a.baseURL, "/") + path
	if stream {
		endpoint += "?alt=sse"
	}
	return endpoint
}

func (a *googleAdapter) normalizeResponse(response *http.Response, model string, stream bool) (*http.Response, error) {
	if stream {
		return streamResponse(response, func(reader io.Reader, writer io.Writer) error {
			return transformGoogleStream(reader, writer, model)
		}), nil
	}
	var result googleResponse
	if err := decodeResponseJSON(response, &result, "Google"); err != nil {
		return nil, err
	}
	text, finish := googleTextAndFinish(result)
	calls := googleToolCalls(result)
	usage := tokenUsage{PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if result.Model != "" {
		model = result.Model
	}
	return normalizedResponse(response, result.ResponseID, model, text, finish, usage, calls)
}

func transformGoogleStream(reader io.Reader, writer io.Writer, model string) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	id := "chatcmpl-google"
	role := "assistant"
	started := false
	finished := false
	toolIndices := make(map[string]int)
	nextToolIndex := 0
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var result googleResponse
		if err := json.Unmarshal([]byte(data), &result); err != nil {
			return fmt.Errorf("decode Google stream event: %w", err)
		}
		if result.ResponseID != "" {
			id = result.ResponseID
		}
		if result.Model != "" {
			model = result.Model
		}
		if !started {
			if err := writeChunk(writer, id, model, &role, nil, nil, nil); err != nil {
				return err
			}
			started = true
		}
		text, finish := googleTextAndFinish(result)
		if text != "" {
			if err := writeChunk(writer, id, model, nil, &text, nil, nil); err != nil {
				return err
			}
		}
		calls := googleToolCalls(result)
		for _, call := range calls {
			index, ok := toolIndices[call.ID]
			if !ok {
				index = nextToolIndex
				toolIndices[call.ID] = index
				nextToolIndex++
			}
			streamCall := openAIStreamToolCall{Index: index, ID: call.ID, Type: "function", Function: call.Function}
			if err := writeToolChunk(writer, id, model, []openAIStreamToolCall{streamCall}); err != nil {
				return err
			}
		}
		if len(result.Candidates) > 0 && result.Candidates[0].FinishReason != "" {
			usage := tokenUsage{PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens}
			if usage.TotalTokens == 0 {
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
			if err := writeChunk(writer, id, model, nil, nil, &finish, &usage); err != nil {
				return err
			}
			finished = true
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if !finished {
		finish := "stop"
		if err := writeChunk(writer, id, model, nil, nil, &finish, nil); err != nil {
			return err
		}
	}
	_, err := io.WriteString(writer, "data: [DONE]\n\n")
	return err
}

func googleToolCalls(result googleResponse) []openAIToolCall {
	if len(result.Candidates) == 0 {
		return nil
	}
	var calls []openAIToolCall
	for index, part := range result.Candidates[0].Content.Parts {
		if part.FunctionCall == nil {
			continue
		}
		callID := part.FunctionCall.ID
		if callID == "" {
			callID = fmt.Sprintf("call_google_%d", index)
		}
		arguments := string(part.FunctionCall.Args)
		if arguments == "" {
			arguments = "{}"
		}
		calls = append(calls, openAIToolCall{ID: callID, Type: "function", Function: openAIFunctionCall{Name: part.FunctionCall.Name, Arguments: arguments}})
	}
	return calls
}

func googleTextAndFinish(result googleResponse) (string, string) {
	if len(result.Candidates) == 0 {
		return "", "stop"
	}
	var text strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		text.WriteString(part.Text)
	}
	if len(googleToolCalls(result)) > 0 {
		return text.String(), "tool_calls"
	}
	switch result.Candidates[0].FinishReason {
	case "MAX_TOKENS":
		return text.String(), "length"
	case "STOP", "":
		return text.String(), "stop"
	default:
		return text.String(), "content_filter"
	}
}
