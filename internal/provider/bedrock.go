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

	"nexoroute/internal/config"
)

type bedrockAdapter struct {
	baseURL         string
	region          string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	now             func() time.Time
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
	if common.Stream {
		return nil, &RequestError{
			Code:    "unsupported_streaming",
			Message: "The Bedrock adapter does not support streaming yet; send stream=false.",
		}
	}
	type content struct {
		Text string `json:"text"`
	}
	type message struct {
		Role    string    `json:"role"`
		Content []content `json:"content"`
	}
	payload := struct {
		Messages        []message `json:"messages"`
		System          []content `json:"system,omitempty"`
		InferenceConfig struct {
			MaxTokens     int      `json:"maxTokens,omitempty"`
			Temperature   *float64 `json:"temperature,omitempty"`
			TopP          *float64 `json:"topP,omitempty"`
			StopSequences []string `json:"stopSequences,omitempty"`
		} `json:"inferenceConfig,omitempty"`
	}{}
	for _, item := range common.Messages {
		text, err := textContent(item.Content)
		if err != nil {
			return nil, err
		}
		switch item.Role {
		case "system", "developer":
			payload.System = append(payload.System, content{Text: text})
		case "user", "assistant":
			payload.Messages = append(payload.Messages, message{Role: item.Role, Content: []content{{Text: text}}})
		default:
			return nil, &RequestError{Code: "unsupported_role", Message: "The Bedrock adapter supports system, developer, user, and assistant messages."}
		}
	}
	payload.InferenceConfig.MaxTokens = maxOutputTokens(common, 0)
	payload.InferenceConfig.Temperature = common.Temperature
	payload.InferenceConfig.TopP = common.TopP
	payload.InferenceConfig.StopSequences, err = stopSequences(common.Stop)
	if err != nil {
		return nil, err
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

func (a *bedrockAdapter) normalizeResponse(response *http.Response, model string, _ bool) (*http.Response, error) {
	var result struct {
		Output struct {
			Message struct {
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		} `json:"output"`
		StopReason string `json:"stopReason"`
		Usage      struct {
			InputTokens  int `json:"inputTokens"`
			OutputTokens int `json:"outputTokens"`
			TotalTokens  int `json:"totalTokens"`
		} `json:"usage"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode Bedrock response: %w", err)
	}
	var text strings.Builder
	for _, part := range result.Output.Message.Content {
		text.WriteString(part.Text)
	}
	usage := tokenUsage{PromptTokens: result.Usage.InputTokens, CompletionTokens: result.Usage.OutputTokens, TotalTokens: result.Usage.TotalTokens}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	return normalizedResponse(response, "chatcmpl-bedrock", model, text.String(), bedrockFinishReason(result.StopReason), usage)
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
