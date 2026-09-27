package provider

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lucianoaugusto1/thruplane/internal/config"
)

type bedrockAdapter struct {
	baseURL         string
	region          string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	now             func() time.Time
}

type bedrockToolUse struct {
	ToolUseID string          `json:"toolUseId"`
	Name      string          `json:"name"`
	Input     json.RawMessage `json:"input"`
}

type bedrockToolResultContent struct {
	Text string          `json:"text,omitempty"`
	JSON json.RawMessage `json:"json,omitempty"`
}

type bedrockToolResult struct {
	ToolUseID string                     `json:"toolUseId"`
	Content   []bedrockToolResultContent `json:"content"`
}

type bedrockContent struct {
	Text       string             `json:"text,omitempty"`
	Image      *bedrockImage      `json:"image,omitempty"`
	Document   *bedrockDocument   `json:"document,omitempty"`
	ToolUse    *bedrockToolUse    `json:"toolUse,omitempty"`
	ToolResult *bedrockToolResult `json:"toolResult,omitempty"`
}

type bedrockImage struct {
	Format string `json:"format"`
	Source struct {
		Bytes string `json:"bytes"`
	} `json:"source"`
}

type bedrockDocument struct {
	Format string `json:"format"`
	Name   string `json:"name"`
	Source struct {
		Bytes string `json:"bytes"`
	} `json:"source"`
}

type bedrockMessage struct {
	Role    string           `json:"role"`
	Content []bedrockContent `json:"content"`
}

type bedrockToolSpec struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema struct {
		JSON json.RawMessage `json:"json"`
	} `json:"inputSchema"`
}

type bedrockTool struct {
	ToolSpec bedrockToolSpec `json:"toolSpec"`
}

type bedrockToolChoice struct {
	Auto *struct{} `json:"auto,omitempty"`
	Any  *struct{} `json:"any,omitempty"`
	Tool *struct {
		Name string `json:"name"`
	} `json:"tool,omitempty"`
}

type bedrockToolConfig struct {
	Tools      []bedrockTool      `json:"tools"`
	ToolChoice *bedrockToolChoice `json:"toolChoice,omitempty"`
}

func newBedrockAdapter(cfg config.ProviderConfig) (adapter, error) {
	if _, err := providerEndpoint(cfg.BaseURL, ""); err != nil {
		return nil, err
	}
	return &bedrockAdapter{
		baseURL: cfg.BaseURL, region: cfg.Region, accessKeyID: cfg.AccessKeyID,
		secretAccessKey: cfg.SecretAccessKey, sessionToken: cfg.SessionToken,
		now: time.Now,
	}, nil
}

func (a *bedrockAdapter) buildRequest(ctx context.Context, body []byte, model string) (*http.Request, error) {
	common, err := decodeChatRequest(body)
	if err != nil {
		return nil, err
	}
	tools, err := nativeTools(common)
	if err != nil {
		return nil, err
	}
	if common.Stream {
		return nil, &RequestError{
			Code:    "unsupported_streaming",
			Message: "The Bedrock adapter does not support streaming yet; send stream=false.",
		}
	}
	if tools.Choice.DisableParallel && len(tools.Definitions) > 0 {
		return nil, &RequestError{Code: "unsupported_parallel_tool_calls", Message: "Bedrock Converse cannot portably enforce parallel_tool_calls=false."}
	}
	type content struct {
		Text string `json:"text"`
	}
	payload := struct {
		Messages        []bedrockMessage `json:"messages"`
		System          []content        `json:"system,omitempty"`
		InferenceConfig struct {
			MaxTokens     int      `json:"maxTokens,omitempty"`
			Temperature   *float64 `json:"temperature,omitempty"`
			TopP          *float64 `json:"topP,omitempty"`
			StopSequences []string `json:"stopSequences,omitempty"`
		} `json:"inferenceConfig,omitempty"`
		ToolConfig *bedrockToolConfig `json:"toolConfig,omitempty"`
	}{}
	for _, item := range common.Messages {
		switch item.Role {
		case "system", "developer":
			text, err := textContent(item.Content)
			if err != nil {
				return nil, err
			}
			payload.System = append(payload.System, content{Text: text})
		case "user":
			blocks, err := bedrockUserContent(item.Content)
			if err != nil {
				return nil, err
			}
			payload.Messages = append(payload.Messages, bedrockMessage{Role: "user", Content: blocks})
		case "assistant":
			text, err := textContent(item.Content)
			if err != nil {
				return nil, err
			}
			blocks := make([]bedrockContent, 0, 1+len(item.ToolCalls))
			if text != "" {
				blocks = append(blocks, bedrockContent{Text: text})
			}
			for _, call := range item.ToolCalls {
				arguments, err := toolArguments(call.Function.Arguments)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, bedrockContent{ToolUse: &bedrockToolUse{ToolUseID: call.ID, Name: call.Function.Name, Input: arguments}})
			}
			payload.Messages = append(payload.Messages, bedrockMessage{Role: "assistant", Content: blocks})
		case "tool":
			result, err := bedrockResultContent(item.Content)
			if err != nil {
				return nil, err
			}
			block := bedrockContent{ToolResult: &bedrockToolResult{ToolUseID: item.ToolCallID, Content: result}}
			if last := len(payload.Messages) - 1; last >= 0 && bedrockToolResultMessage(payload.Messages[last]) {
				payload.Messages[last].Content = append(payload.Messages[last].Content, block)
			} else {
				payload.Messages = append(payload.Messages, bedrockMessage{Role: "user", Content: []bedrockContent{block}})
			}
		default:
			return nil, &RequestError{Code: "unsupported_role", Message: "The Bedrock adapter supports system, developer, user, assistant, and tool messages."}
		}
	}
	payload.InferenceConfig.MaxTokens = maxOutputTokens(common, 0)
	payload.InferenceConfig.Temperature = common.Temperature
	payload.InferenceConfig.TopP = common.TopP
	payload.InferenceConfig.StopSequences, err = stopSequences(common.Stop)
	if err != nil {
		return nil, err
	}
	if len(tools.Definitions) > 0 && tools.Choice.Mode != "none" {
		payload.ToolConfig = &bedrockToolConfig{Tools: make([]bedrockTool, 0, len(tools.Definitions))}
		for _, definition := range tools.Definitions {
			spec := bedrockToolSpec{Name: definition.Function.Name, Description: definition.Function.Description}
			spec.InputSchema.JSON = definition.Function.Parameters
			payload.ToolConfig.Tools = append(payload.ToolConfig.Tools, bedrockTool{ToolSpec: spec})
		}
		payload.ToolConfig.ToolChoice = bedrockChoice(tools.Choice)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("encode Bedrock request: %w", err)
	}
	endpoint := strings.TrimRight(a.baseURL, "/") + "/model/" + url.PathEscape(model) + "/converse"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create Bedrock request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	a.sign(request, encoded, a.now().UTC())
	return request, nil
}

func bedrockUserContent(raw json.RawMessage) ([]bedrockContent, error) {
	parts, err := parseNativeContent(raw)
	if err != nil {
		return nil, err
	}
	blocks := make([]bedrockContent, 0, len(parts))
	for _, part := range parts {
		switch part.Kind {
		case "text":
			blocks = append(blocks, bedrockContent{Text: part.Text})
		case "image":
			if part.URL != "" {
				return nil, unsupportedContent("Bedrock Converse requires inline image bytes in this Chat Completions adapter.")
			}
			image := &bedrockImage{Format: strings.TrimPrefix(part.MIMEType, "image/")}
			image.Source.Bytes = part.Data
			blocks = append(blocks, bedrockContent{Image: image})
		case "document":
			document := &bedrockDocument{Format: "pdf", Name: "document.pdf"}
			document.Source.Bytes = part.Data
			blocks = append(blocks, bedrockContent{Document: document})
		default:
			return nil, unsupportedContent("Bedrock Converse does not support this Chat Completions input modality.")
		}
	}
	return blocks, nil
}

func bedrockToolResultMessage(message bedrockMessage) bool {
	return message.Role == "user" && len(message.Content) > 0 && message.Content[0].ToolResult != nil
}

func bedrockResultContent(content json.RawMessage) ([]bedrockToolResultContent, error) {
	text, err := textContent(content)
	if err != nil {
		return nil, err
	}
	var object map[string]any
	if json.Unmarshal([]byte(text), &object) == nil && object != nil {
		return []bedrockToolResultContent{{JSON: json.RawMessage(text)}}, nil
	}
	return []bedrockToolResultContent{{Text: text}}, nil
}

func bedrockChoice(choice nativeToolChoice) *bedrockToolChoice {
	result := &bedrockToolChoice{}
	switch choice.Mode {
	case "auto":
		result.Auto = &struct{}{}
	case "required":
		result.Any = &struct{}{}
	case "named":
		result.Tool = &struct {
			Name string `json:"name"`
		}{Name: choice.Name}
	case "none", "":
		return nil
	}
	return result
}

func (a *bedrockAdapter) normalizeResponse(response *http.Response, model string, _ bool) (*http.Response, error) {
	var result struct {
		Output struct {
			Message struct {
				Content []bedrockContent `json:"content"`
			} `json:"message"`
		} `json:"output"`
		StopReason string `json:"stopReason"`
		Usage      struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			TotalTokens  int `json:"totalTokens"`
		} `json:"usage"`
	}
	if err := decodeResponseJSON(response, &result, "Bedrock"); err != nil {
		return nil, err
	}
	var text strings.Builder
	var calls []openAIToolCall
	for _, part := range result.Output.Message.Content {
		text.WriteString(part.Text)
		if part.ToolUse != nil {
			arguments := string(part.ToolUse.Input)
			if arguments == "" {
				arguments = "{}"
			}
			calls = append(calls, openAIToolCall{ID: part.ToolUse.ToolUseID, Type: "function", Function: openAIFunctionCall{Name: part.ToolUse.Name, Arguments: arguments}})
		}
	}
	usage := tokenUsage{PromptTokens: result.Usage.InputTokens, CompletionTokens: result.Usage.OutputTokens, TotalTokens: result.Usage.TotalTokens}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return normalizedResponse(response, "chatcmpl-bedrock", model, text.String(), bedrockFinishReason(result.StopReason), usage, calls)
}

func (a *bedrockAdapter) sign(request *http.Request, payload []byte, now time.Time) {
	payloadHash := sha256Hex(payload)
	amzDate := now.Format("20060102T150405Z")
	shortDate := now.Format("20060102")
	request.Header.Set("X-Amz-Date", amzDate)
	request.Header.Set("X-Amz-Content-Sha256", payloadHash)
	if a.sessionToken != "" {
		request.Header.Set("X-Amz-Security-Token", a.sessionToken)
	}

	canonicalHeaders := "content-type:" + strings.TrimSpace(request.Header.Get("Content-Type")) + "\n" +
		"host:" + request.URL.Host + "\n" +
		"x-amz-content-sha256:" + payloadHash + "\n" +
		"x-amz-date:" + amzDate + "\n"
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	if a.sessionToken != "" {
		canonicalHeaders += "x-amz-security-token:" + strings.TrimSpace(a.sessionToken) + "\n"
		signedHeaders += ";x-amz-security-token"
	}
	canonicalRequest := request.Method + "\n" + request.URL.EscapedPath() + "\n" + request.URL.RawQuery + "\n" +
		canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	scope := shortDate + "/" + a.region + "/bedrock/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + sha256Hex([]byte(canonicalRequest))
	dateKey := hmacSHA256([]byte("AWS4"+a.secretAccessKey), shortDate)
	regionKey := hmacSHA256(dateKey, a.region)
	serviceKey := hmacSHA256(regionKey, "bedrock")
	signingKey := hmacSHA256(serviceKey, "aws4_request")
	signature := hex.EncodeToString(hmacSHA256(signingKey, stringToSign))
	request.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+a.accessKeyID+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func hmacSHA256(key []byte, value string) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write([]byte(value))
	return mac.Sum(nil)
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func bedrockFinishReason(reason string) string {
	switch reason {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "content_filtered", "guardrail_intervened":
		return "content_filter"
	default:
		return "stop"
	}
}
