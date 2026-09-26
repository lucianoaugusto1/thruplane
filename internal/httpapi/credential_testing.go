package httpapi

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"nexoroute/internal/config"
	"nexoroute/internal/gateway"
	"nexoroute/internal/provider"
)

const credentialEnvelopeAllowance = 64 << 10

const (
	credentialAlias    = "playground-credential"
	credentialChatPath = "/playground/api/credentials/chat/completions"
)

type credentialChatRequest struct {
	Provider credentialProvider `json:"provider"`
	Model    string             `json:"model"`
	Request  json.RawMessage    `json:"request"`
}

type credentialProvider struct {
	Type            string `json:"type"`
	BaseURL         string `json:"base_url"`
	APIKey          string `json:"api_key"`
	APIVersion      string `json:"api_version"`
	AccessToken     string `json:"access_token"`
	Project         string `json:"project"`
	Location        string `json:"location"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
}

type credentialTester struct {
	baseConfig config.Config
	allowed    map[string]struct{}
}

func newCredentialTester(cfg config.Config) *credentialTester {
	allowed := make(map[string]struct{}, len(cfg.Server.Playground.CredentialTesting.AllowedBaseURLs))
	for _, baseURL := range cfg.Server.Playground.CredentialTesting.AllowedBaseURLs {
		allowed[baseURL] = struct{}{}
	}
	return &credentialTester{baseConfig: cfg, allowed: allowed}
}

func (t *credentialTester) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	request, err := decodeCredentialChatRequest(r, t.baseConfig.Server.MaxBodyBytes)
	if err != nil {
		writeError(w, err.status, err.message, err.errorType, err.code)
		return
	}

	ephemeral, body, requestError := t.buildConfig(request)
	if requestError != nil {
		writeError(w, requestError.status, requestError.message, requestError.errorType, requestError.code)
		return
	}
	clients, clientError := provider.NewClients(ephemeral)
	if clientError != nil {
		writeError(w, http.StatusBadRequest, "The provider credential configuration is invalid.", "invalid_request_error", "invalid_provider_credentials")
		return
	}
	defer provider.CloseIdleConnections(clients)

	inner := r.Clone(r.Context())
	inner.Body = io.NopCloser(bytes.NewReader(body))
	inner.ContentLength = int64(len(body))
	inner.Header = r.Header.Clone()
	inner.Header.Set("Content-Type", "application/json")
	gateway.New(ephemeral, clients).ChatCompletions(w, inner)
}

func (t *credentialTester) buildConfig(request credentialChatRequest) (config.Config, []byte, *apiRequestError) {
	providerConfig, err := t.providerConfig(request.Provider)
	if err != nil {
		return config.Config{}, nil, err
	}
	model := strings.TrimSpace(request.Model)
	if model == "" {
		return config.Config{}, nil, newAPIRequestError(http.StatusBadRequest, "A physical provider model is required.", "invalid_request_error", "model_required")
	}

	body, bodyError := credentialRequestBody(request.Request)
	if bodyError != nil {
		return config.Config{}, nil, bodyError
	}

	ephemeral := t.baseConfig
	ephemeral.Catalog.UnknownModels = "allow"
	ephemeral.Providers = map[string]config.ProviderConfig{providerConfig.Type: providerConfig}
	ephemeral.Models = map[string]config.ModelConfig{
		credentialAlias: {Targets: []config.TargetConfig{{Provider: providerConfig.Type, Model: model}}},
	}
	ephemeral.Routing.Retries = 0
	ephemeral.Routing.CircuitBreaker.FailureThreshold = 0

	ephemeral, validationError := config.NormalizeAndValidate(ephemeral)
	if validationError != nil {
		return config.Config{}, nil, newAPIRequestError(http.StatusBadRequest, "The provider credential configuration is invalid.", "invalid_request_error", "invalid_provider_credentials")
	}
	return ephemeral, body, nil
}

func (t *credentialTester) providerConfig(input credentialProvider) (config.ProviderConfig, *apiRequestError) {
	providerType := strings.ToLower(strings.TrimSpace(input.Type))
	if providerType == "grok" {
		providerType = "xai"
	}
	switch providerType {
	case "openai", "anthropic", "gemini", "vertex", "bedrock", "azure-openai", "ollama", "openai-compatible", "nexoroute-inference", "xai":
	default:
		return config.ProviderConfig{}, newAPIRequestError(http.StatusBadRequest, "The provider type is not supported.", "invalid_request_error", "unsupported_provider")
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL != "" {
		normalized, err := config.NormalizeBaseURL(baseURL)
		if err != nil {
			return config.ProviderConfig{}, newAPIRequestError(http.StatusBadRequest, "The provider base URL is invalid.", "invalid_request_error", "invalid_base_url")
		}
		if _, ok := t.allowed[normalized]; !ok {
			return config.ProviderConfig{}, newAPIRequestError(http.StatusBadRequest, "The provider base URL is not allowed by the server.", "invalid_request_error", "base_url_not_allowed")
		}
		baseURL = normalized
	} else if providerNeedsBaseURL(providerType) {
		return config.ProviderConfig{}, newAPIRequestError(http.StatusBadRequest, "This provider requires an allowed base URL.", "invalid_request_error", "base_url_required")
	}

	if providerNeedsAPIKey(providerType) && strings.TrimSpace(input.APIKey) == "" {
		return config.ProviderConfig{}, newAPIRequestError(http.StatusBadRequest, "An API key is required for this provider.", "invalid_request_error", "provider_api_key_required")
	}

	return config.ProviderConfig{
		Type:            providerType,
		BaseURL:         baseURL,
		APIKey:          input.APIKey,
		APIVersion:      strings.TrimSpace(input.APIVersion),
		AccessToken:     input.AccessToken,
		Project:         strings.TrimSpace(input.Project),
		Location:        strings.TrimSpace(input.Location),
		Region:          strings.TrimSpace(input.Region),
		AccessKeyID:     input.AccessKeyID,
		SecretAccessKey: input.SecretAccessKey,
		SessionToken:    input.SessionToken,
	}, nil
}

func providerNeedsBaseURL(providerType string) bool {
	switch providerType {
	case "azure-openai", "openai-compatible", "nexoroute-inference":
		return true
	default:
		return false
	}
}

func providerNeedsAPIKey(providerType string) bool {
	switch providerType {
	case "openai", "anthropic", "gemini", "azure-openai", "openai-compatible", "nexoroute-inference", "xai":
		return true
	default:
		return false
	}
}

func credentialRequestBody(raw json.RawMessage) ([]byte, *apiRequestError) {
	if len(raw) == 0 {
		return nil, newAPIRequestError(http.StatusBadRequest, "A chat completion request is required.", "invalid_request_error", "request_required")
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(raw, &body); err != nil || body == nil {
		return nil, newAPIRequestError(http.StatusBadRequest, "The chat completion request must be a JSON object.", "invalid_request_error", "invalid_request")
	}
	model, _ := json.Marshal(credentialAlias)
	body["model"] = model
	normalized, err := json.Marshal(body)
	if err != nil {
		return nil, newAPIRequestError(http.StatusBadRequest, "The chat completion request is invalid.", "invalid_request_error", "invalid_request")
	}
	return normalized, nil
}

type apiRequestError struct {
	status    int
	message   string
	errorType string
	code      string
}

func newAPIRequestError(status int, message, errorType, code string) *apiRequestError {
	return &apiRequestError{status: status, message: message, errorType: errorType, code: code}
}

func decodeCredentialChatRequest(r *http.Request, maxRequestBytes int64) (credentialChatRequest, *apiRequestError) {
	if maxRequestBytes <= 0 {
		maxRequestBytes = 1 << 20
	}
	limit := maxRequestBytes + credentialEnvelopeAllowance
	reader := io.LimitReader(r.Body, limit+1)
	body, err := io.ReadAll(reader)
	if err != nil {
		return credentialChatRequest{}, newAPIRequestError(http.StatusBadRequest, "The request body could not be read.", "invalid_request_error", "invalid_body")
	}
	if int64(len(body)) > limit {
		return credentialChatRequest{}, newAPIRequestError(http.StatusRequestEntityTooLarge, "The request body exceeds the configured limit.", "invalid_request_error", "request_too_large")
	}

	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var request credentialChatRequest
	if err := decoder.Decode(&request); err != nil {
		return credentialChatRequest{}, newAPIRequestError(http.StatusBadRequest, "The credential request must be a valid JSON object.", "invalid_request_error", "invalid_json")
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return credentialChatRequest{}, newAPIRequestError(http.StatusBadRequest, "The credential request must contain exactly one JSON object.", "invalid_request_error", "invalid_json")
	}
	return request, nil
}

func (e *apiRequestError) Error() string {
	return fmt.Sprintf("%s (%s)", e.message, e.code)
}
