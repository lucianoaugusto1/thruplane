package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

type targetIdentity struct {
	provider string
	model    string
}

func (cfg Config) Validate() error {
	if cfg.Catalog.UnknownModels != "" && cfg.Catalog.UnknownModels != "allow" && cfg.Catalog.UnknownModels != "reject" {
		return errors.New("catalog unknown_models must be allow or reject")
	}
	if strings.TrimSpace(cfg.Server.Address) == "" {
		return errors.New("server address must not be empty")
	}
	if cfg.Server.MaxBodyBytes <= 0 {
		return errors.New("server max_body_bytes must be greater than zero")
	}
	if time.Duration(cfg.Server.ReadHeaderTimeout) <= 0 {
		return errors.New("server read_header_timeout must be greater than zero")
	}
	if time.Duration(cfg.Server.ShutdownTimeout) <= 0 {
		return errors.New("server shutdown_timeout must be greater than zero")
	}
	credentialTesting := cfg.Server.Playground.CredentialTesting
	if credentialTesting.Enabled && !cfg.Server.Playground.Enabled {
		return errors.New("server playground credential_testing requires playground enabled")
	}
	if credentialTesting.Enabled && strings.TrimSpace(cfg.Server.APIKey) == "" {
		return errors.New("server playground credential_testing requires server api_key")
	}
	for index, baseURL := range credentialTesting.AllowedBaseURLs {
		normalized, err := NormalizeBaseURL(baseURL)
		if err != nil {
			return fmt.Errorf("server playground credential_testing allowed_base_urls entry %d %s", index, err)
		}
		if normalized != baseURL {
			return fmt.Errorf("server playground credential_testing allowed_base_urls entry %d must be normalized", index)
		}
	}
	if cfg.Routing.Retries < 0 {
		return errors.New("routing retries must not be negative")
	}
	if time.Duration(cfg.Routing.ResponseHeaderTimeout) <= 0 {
		return errors.New("routing response_header_timeout must be greater than zero")
	}
	baseDelay := time.Duration(cfg.Routing.Retry.BaseDelay)
	maxDelay := time.Duration(cfg.Routing.Retry.MaxDelay)
	retryBudget := time.Duration(cfg.Routing.Retry.Budget)
	if baseDelay <= 0 {
		return errors.New("routing retry base_delay must be greater than zero")
	}
	if maxDelay < baseDelay {
		return errors.New("routing retry max_delay must be greater than or equal to base_delay")
	}
	if retryBudget < baseDelay {
		return errors.New("routing retry budget must be greater than or equal to base_delay")
	}
	if cfg.Routing.CircuitBreaker.FailureThreshold < 0 {
		return errors.New("routing circuit_breaker failure_threshold must not be negative")
	}
	circuitOpenDuration := time.Duration(cfg.Routing.CircuitBreaker.OpenDuration)
	if circuitOpenDuration < 0 {
		return errors.New("routing circuit_breaker open_duration must not be negative")
	}
	if cfg.Routing.CircuitBreaker.FailureThreshold > 0 && circuitOpenDuration <= 0 {
		return errors.New("routing circuit_breaker open_duration must be greater than zero when enabled")
	}
	if len(cfg.Providers) == 0 {
		return errors.New("at least one provider is required")
	}

	for name, provider := range cfg.Providers {
		if strings.TrimSpace(name) == "" {
			return errors.New("provider name must not be empty")
		}
		if err := validateProvider(name, provider); err != nil {
			return err
		}
	}

	if len(cfg.Models) == 0 {
		return errors.New("at least one model is required")
	}
	sharedLimits := make(map[targetIdentity]RateLimitConfig)
	for alias, model := range cfg.Models {
		if strings.TrimSpace(alias) == "" {
			return errors.New("model alias must not be empty")
		}
		if len(model.Targets) == 0 {
			return fmt.Errorf("model %q must have at least one target", alias)
		}
		for index, target := range model.Targets {
			if _, ok := cfg.Providers[target.Provider]; !ok {
				return fmt.Errorf("model %q target %d references unknown provider %q", alias, index, target.Provider)
			}
			if strings.TrimSpace(target.Model) == "" {
				return fmt.Errorf("model %q target %d target model must not be empty", alias, index)
			}
			if target.CatalogProvider != "" && target.CatalogModel == "" {
				return fmt.Errorf("model %q target %d catalog_provider requires catalog_model", alias, index)
			}
			if err := validateRateLimit(alias, index, target.RateLimit); err != nil {
				return err
			}
			key := targetIdentity{provider: target.Provider, model: target.Model}
			if previous, exists := sharedLimits[key]; exists && previous != target.RateLimit {
				return fmt.Errorf("model %q target %d has conflicting rate_limit for shared provider/model target %q/%q", alias, index, target.Provider, target.Model)
			}
			sharedLimits[key] = target.RateLimit
		}
	}

	return nil
}

func validateRateLimit(alias string, index int, limit RateLimitConfig) error {
	prefix := fmt.Sprintf("model %q target %d rate_limit", alias, index)
	if limit.RequestsPerMinute < 0 {
		return fmt.Errorf("%s requests_per_minute must not be negative", prefix)
	}
	if limit.Burst < 0 {
		return fmt.Errorf("%s burst must not be negative", prefix)
	}
	if limit.RequestsPerMinute == 0 && limit.Burst != 0 {
		return fmt.Errorf("%s burst requires requests_per_minute", prefix)
	}
	if limit.RequestsPerMinute > 0 && limit.Burst == 0 {
		return fmt.Errorf("%s burst must be greater than zero when requests_per_minute is configured", prefix)
	}
	if limit.MaxConcurrency < 0 {
		return fmt.Errorf("%s max_concurrency must not be negative", prefix)
	}
	if time.Duration(limit.QueueTimeout) < 0 {
		return fmt.Errorf("%s queue_timeout must not be negative", prefix)
	}
	return nil
}

func validateProvider(name string, provider ProviderConfig) error {
	switch provider.Type {
	case "", "openai", "anthropic", "gemini", "vertex", "bedrock",
		"azure-openai", "ollama", "openai-compatible",
		"nexoroute-inference", "xai":
	default:
		return fmt.Errorf("provider %q has unsupported type %q", name, provider.Type)
	}

	if provider.Type == "vertex" {
		if provider.Project == "" {
			return fmt.Errorf("provider %q project must not be empty for vertex", name)
		}
		if provider.Location == "" {
			return fmt.Errorf("provider %q location must not be empty for vertex", name)
		}
		if strings.TrimSpace(provider.AccessToken) == "" {
			return fmt.Errorf("provider %q access_token must not be empty for vertex", name)
		}
	}
	if provider.Type == "bedrock" {
		if provider.Region == "" {
			return fmt.Errorf("provider %q region must not be empty for bedrock", name)
		}
		if strings.TrimSpace(provider.AccessKeyID) == "" {
			return fmt.Errorf("provider %q access_key_id must not be empty for bedrock", name)
		}
		if strings.TrimSpace(provider.SecretAccessKey) == "" {
			return fmt.Errorf("provider %q secret_access_key must not be empty for bedrock", name)
		}
	}

	parsed, err := url.Parse(provider.BaseURL)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("provider %q base_url must be an absolute HTTP URL", name)
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("provider %q base_url must not contain credentials, a query, or a fragment", name)
	}

	return nil
}
