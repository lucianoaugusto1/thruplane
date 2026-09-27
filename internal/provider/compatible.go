package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/lucianoaugusto1/thruplane/internal/config"
)

type compatibleAdapter struct {
	endpoint     string
	apiKey       string
	authScheme   string
	translateMax bool
}

func newCompatibleAdapter(cfg config.ProviderConfig, path, authScheme string, translateMax bool) (*compatibleAdapter, error) {
	endpoint, err := providerEndpoint(cfg.BaseURL, path)
	if err != nil {
		return nil, err
	}
	if cfg.Type == "azure-openai" && cfg.APIVersion != "" {
		parsed, _ := url.Parse(endpoint)
		query := parsed.Query()
		query.Set("api-version", cfg.APIVersion)
		parsed.RawQuery = query.Encode()
		endpoint = parsed.String()
	}
	return &compatibleAdapter{
		endpoint:     endpoint,
		apiKey:       cfg.APIKey,
		authScheme:   authScheme,
		translateMax: translateMax,
	}, nil
}

func (a *compatibleAdapter) buildRequest(ctx context.Context, body []byte, model string) (*http.Request, error) {
	transformedBody, err := transformCompatibleBody(body, model, a.translateMax)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, a.endpoint, bytes.NewReader(transformedBody))
	if err != nil {
		return nil, fmt.Errorf("create provider request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if a.apiKey != "" {
		if a.authScheme == "api-key" {
			request.Header.Set("api-key", a.apiKey)
		} else {
			request.Header.Set("Authorization", "Bearer "+a.apiKey)
		}
	}
	return request, nil
}

func (a *compatibleAdapter) normalizeResponse(response *http.Response, _ string, _ bool) (*http.Response, error) {
	return response, nil
}

func transformCompatibleBody(body []byte, model string, translateMax bool) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return nil, &RequestError{Code: "invalid_json", Message: "The request body must be a JSON object."}
	}

	encodedModel, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("encode provider model: %w", err)
	}
	fields["model"] = encodedModel
	if translateMax {
		if value, ok := fields["max_completion_tokens"]; ok {
			fields["max_tokens"] = value
			delete(fields, "max_completion_tokens")
		}
	}

	transformed, err := json.Marshal(fields)
	if err != nil {
		return nil, fmt.Errorf("encode provider request: %w", err)
	}
	return transformed, nil
}

func providerEndpoint(baseURL, path string) (string, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	endpoint := baseURL + path
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("invalid base URL %q", baseURL)
	}
	return endpoint, nil
}
