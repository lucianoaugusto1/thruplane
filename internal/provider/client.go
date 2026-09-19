package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"nexoroute/internal/config"
)

type RequestError struct {
	Code    string
	Message string
}

func (e *RequestError) Error() string { return e.Message }

type adapter interface {
	buildRequest(context.Context, []byte, string) (*http.Request, error)
	normalizeResponse(*http.Response, string, bool) (*http.Response, error)
}

type Client struct {
	httpClient *http.Client
	adapter    adapter
}

func NewClients(cfg config.Config) (map[string]*Client, error) {
	defaultTransport, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return nil, fmt.Errorf("default HTTP transport has type %T, want *http.Transport", http.DefaultTransport)
	}

	transport := defaultTransport.Clone()
	transport.ResponseHeaderTimeout = time.Duration(cfg.Routing.ResponseHeaderTimeout)
	transport.MaxIdleConns = 512
	transport.MaxIdleConnsPerHost = 64
	transport.MaxConnsPerHost = 0
	transport.IdleConnTimeout = 90 * time.Second
	transport.ForceAttemptHTTP2 = true
	httpClient := &http.Client{Transport: transport}
	clients := make(map[string]*Client, len(cfg.Providers))

	for name, providerConfig := range cfg.Providers {
		providerAdapter, err := newAdapter(providerConfig)
		if err != nil {
			return nil, fmt.Errorf("provider %q: %w", name, err)
		}
		clients[name] = &Client{httpClient: httpClient, adapter: providerAdapter}
	}

	return clients, nil
}

func (c *Client) Do(ctx context.Context, body []byte, model string) (*http.Response, error) {
	stream, err := requestStreams(body)
	if err != nil {
		return nil, err
	}
	request, err := c.adapter.buildRequest(ctx, body, model)
	if err != nil {
		return nil, err
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return response, nil
	}

	normalized, err := c.adapter.normalizeResponse(response, model, stream)
	if err != nil {
		response.Body.Close()
		return nil, err
	}
	return normalized, nil
}

func newAdapter(cfg config.ProviderConfig) (adapter, error) {
	switch cfg.Type {
	case "", "openai", "openai-compatible", "nexoroute-inference", "xai":
		return newCompatibleAdapter(cfg, "/v1/chat/completions", "bearer", false)
	case "ollama":
		return newCompatibleAdapter(cfg, "/v1/chat/completions", "bearer", true)
	case "azure-openai":
		return newCompatibleAdapter(cfg, "/openai/v1/chat/completions", "api-key", false)
	case "anthropic":
		return newAnthropicAdapter(cfg)
	case "gemini":
		return newGoogleAdapter(cfg, false)
	case "vertex":
		return newGoogleAdapter(cfg, true)
	case "bedrock":
		return newBedrockAdapter(cfg)
	default:
		return nil, fmt.Errorf("unsupported provider type %q", cfg.Type)
	}
}

func requestStreams(body []byte) (bool, error) {
	var request struct {
		Stream bool `json:"stream"`
	}
	if err := json.Unmarshal(body, &request); err != nil {
		return false, &RequestError{Code: "invalid_json", Message: "The request body must be a JSON object."}
	}
	return request.Stream, nil
}
