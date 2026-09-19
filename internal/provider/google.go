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

type googlePart struct {
	Text string `json:"text"`
}

type googleContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []googlePart `json:"parts"`
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
	payload := struct {
		Contents          []googleContent `json:"contents"`
		SystemInstruction *googleContent  `json:"systemInstruction,omitempty"`
		GenerationConfig  struct {
			MaxOutputTokens int      `json:"maxOutputTokens,omitempty"`
			Temperature     *float64 `json:"temperature,omitempty"`
			TopP            *float64 `json:"topP,omitempty"`
			StopSequences   []string `json:"stopSequences,omitempty"`
		} `json:"generationConfig,omitempty"`
	}{}
	var systems []string
	for _, item := range common.Messages {
		text, err := textContent(item.Content)
		if err != nil {
			return nil, err
		}
		switch item.Role {
		case "system", "developer":
			systems = append(systems, text)
		case "user":
			payload.Contents = append(payload.Contents, googleContent{Role: "user", Parts: []googlePart{{Text: text}}})
		case "assistant":
			payload.Contents = append(payload.Contents, googleContent{Role: "model", Parts: []googlePart{{Text: text}}})
		default:
			return nil, &RequestError{Code: "unsupported_role", Message: "The Google adapters support system, developer, user, and assistant messages."}
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
	usage := tokenUsage{PromptTokens: result.Usage.PromptTokens, CompletionTokens: result.Usage.CompletionTokens, TotalTokens: result.Usage.TotalTokens}
	if usage.TotalTokens == 0 {
		usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
	}
	if result.Model != "" {
		model = result.Model
	}
	return normalizedResponse(response, result.ResponseID, model, text, finish, usage)
}

func transformGoogleStream(reader io.Reader, writer io.Writer, model string) error {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 4096), 4<<20)
	id := "chatcmpl-google"
	role := "assistant"
	started := false
	finished := false
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

func googleTextAndFinish(result googleResponse) (string, string) {
	if len(result.Candidates) == 0 {
		return "", "stop"
	}
	var text strings.Builder
	for _, part := range result.Candidates[0].Content.Parts {
		text.WriteString(part.Text)
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
