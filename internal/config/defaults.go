package config

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const (
	defaultAddress               = ":8080"
	defaultMaxBodyBytes          = 1 << 20
	defaultReadHeaderTimeout     = 5 * time.Second
	defaultShutdownTimeout       = 10 * time.Second
	defaultRetries               = 1
	defaultResponseHeaderTimeout = 30 * time.Second
	defaultRetryBaseDelay        = 200 * time.Millisecond
	defaultRetryMaxDelay         = 5 * time.Second
	defaultRetryBudget           = 15 * time.Second
	defaultCircuitOpenDuration   = 30 * time.Second
)

func defaultConfig() Config {
	return Config{
		Catalog: CatalogConfig{UnknownModels: "allow"},
		Server: ServerConfig{
			Address:           defaultAddress,
			MaxBodyBytes:      defaultMaxBodyBytes,
			ReadHeaderTimeout: Duration(defaultReadHeaderTimeout),
			ShutdownTimeout:   Duration(defaultShutdownTimeout),
		},
		Routing: RoutingConfig{
			Retries:               defaultRetries,
			ResponseHeaderTimeout: Duration(defaultResponseHeaderTimeout),
			Retry: RetryConfig{
				BaseDelay: Duration(defaultRetryBaseDelay),
				MaxDelay:  Duration(defaultRetryMaxDelay),
				Budget:    Duration(defaultRetryBudget),
			},
			CircuitBreaker: CircuitBreakerConfig{
				OpenDuration: Duration(defaultCircuitOpenDuration),
			},
		},
	}
}

func (cfg *Config) normalize() {
	cfg.Server.Address = strings.TrimSpace(cfg.Server.Address)
	cfg.Server.APIKey = strings.TrimSpace(cfg.Server.APIKey)
	cfg.Catalog.UnknownModels = strings.ToLower(strings.TrimSpace(cfg.Catalog.UnknownModels))
	if cfg.Catalog.UnknownModels == "" {
		cfg.Catalog.UnknownModels = "allow"
	}

	for name, provider := range cfg.Providers {
		provider.Type = strings.ToLower(strings.TrimSpace(provider.Type))
		if provider.Type == "" {
			provider.Type = "openai"
		}
		if provider.Type == "grok" {
			provider.Type = "xai"
		}
		provider.Project = strings.TrimSpace(provider.Project)
		provider.Location = strings.TrimSpace(provider.Location)
		provider.Region = strings.TrimSpace(provider.Region)
		provider.BaseURL = strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
		if provider.BaseURL == "" {
			provider.BaseURL = defaultProviderBaseURL(provider)
		}
		cfg.Providers[name] = provider
	}

	allowedBaseURLs := make([]string, 0, len(cfg.Server.Playground.CredentialTesting.AllowedBaseURLs))
	seenBaseURLs := make(map[string]struct{}, len(cfg.Server.Playground.CredentialTesting.AllowedBaseURLs))
	for _, raw := range cfg.Server.Playground.CredentialTesting.AllowedBaseURLs {
		normalized, err := NormalizeBaseURL(raw)
		if err != nil {
			normalized = strings.TrimRight(strings.TrimSpace(raw), "/")
		}
		if _, exists := seenBaseURLs[normalized]; exists {
			continue
		}
		seenBaseURLs[normalized] = struct{}{}
		allowedBaseURLs = append(allowedBaseURLs, normalized)
	}
	cfg.Server.Playground.CredentialTesting.AllowedBaseURLs = allowedBaseURLs

	for alias, model := range cfg.Models {
		for index := range model.Targets {
			model.Targets[index].Provider = strings.TrimSpace(model.Targets[index].Provider)
			model.Targets[index].Model = strings.TrimSpace(model.Targets[index].Model)
			model.Targets[index].CatalogProvider = strings.ToLower(strings.TrimSpace(model.Targets[index].CatalogProvider))
			model.Targets[index].CatalogModel = strings.TrimSpace(model.Targets[index].CatalogModel)
			if model.Targets[index].RateLimit.RequestsPerMinute > 0 && model.Targets[index].RateLimit.Burst == 0 {
				model.Targets[index].RateLimit.Burst = 1
			}
		}
		cfg.Models[alias] = model
	}
}

// NormalizeBaseURL returns the canonical representation used for provider
// destinations and credential-testing allowlist comparisons.
func NormalizeBaseURL(raw string) (string, error) {
	trimmed := strings.TrimRight(strings.TrimSpace(raw), "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return "", fmt.Errorf("must be an absolute HTTP URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", fmt.Errorf("must not contain credentials, a query, or a fragment")
	}
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	return strings.TrimRight(parsed.String(), "/"), nil
}

func defaultProviderBaseURL(provider ProviderConfig) string {
	switch provider.Type {
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "gemini":
		return "https://generativelanguage.googleapis.com"
	case "vertex":
		if provider.Location != "" {
			return "https://" + provider.Location + "-aiplatform.googleapis.com"
		}
	case "bedrock":
		if provider.Region != "" {
			return "https://bedrock-runtime." + provider.Region + ".amazonaws.com"
		}
	case "xai":
		return "https://api.x.ai"
	case "ollama":
		return "http://localhost:11434"
	}
	return ""
}
