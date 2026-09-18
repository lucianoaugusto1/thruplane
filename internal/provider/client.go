package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nexoroute/internal/config"
)

const chatCompletionsPath = "/v1/chat/completions"

type Client struct {
	httpClient *http.Client
	endpoint   string
	apiKey     string
	provider   string
}

func NewClients(cfg config.Config) (map[string]*Client, error) {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default HTTP transport has type %T, want *http.Transport", http.DefaultTransport)
	}

	transport := defaultTransport.Clone()
	transport.ResponseHeaderTimeout = time.Duration(cfg.Routing.ResponseHeaderTimeout)
	httpClient := &http.Client{Transport: transport}
	clients := make(map[string]*Client, len(cfg.Providers))

	for name, providerConfig := range cfg.Providers {
		baseURL := strings.TrimRight(providerConfig.BaseURL, "/")
		endpoint := baseURL + chatCompletionsPath
		parsed, err := url.Parse(endpoint)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, fmt.Errorf("provider %q has invalid base URL %q", name, providerConfig.BaseURL)
		}

		providerType := providerConfig.Type
		if providerType == "" {
			providerType = "openai"
		}
		clients[name] = &Client{
			httpClient: httpClient,
			endpoint:   endpoint,
			apiKey:     providerConfig.APIKey,
			provider:   providerType,
		}
	}

	return clients, nil
}

func (c *Client) Do(ctx context.Context, body []byte, model string) (*http.Response, error) {
	transformedBody, err := c.transformBody(body, model)
	if err != nil {
		return nil, err
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(transformedBody))
	if err != nil {
		return nil, fmt.Errorf("create provider request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	if c.apiKey != "" {
		request.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.httpClient.Do(request)
}

func (c *Client) transformBody(body []byte, model string) ([]byte, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		return nil, fmt.Errorf("decode provider request: %w", err)
	}
	if fields == nil {
		return nil, fmt.Errorf("decode provider request: expected JSON object")
	}

	encodedModel, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("encode provider model: %w", err)
	}
	fields["model"] = encodedModel
	if c.provider == "ollama" {
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
