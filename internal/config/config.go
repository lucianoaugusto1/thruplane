package config

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
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
)

type Duration time.Duration

func (d *Duration) UnmarshalYAML(node *yaml.Node) error {
	var value string
	if err := node.Decode(&value); err != nil {
		return fmt.Errorf("duration must be a string: %w", err)
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", value, err)
	}

	*d = Duration(parsed)
	return nil
}

type Config struct {
	Server    ServerConfig              `yaml:"server"`
	Catalog   CatalogConfig             `yaml:"catalog"`
	Providers map[string]ProviderConfig `yaml:"providers"`
	Models    map[string]ModelConfig    `yaml:"models"`
	Routing   RoutingConfig             `yaml:"routing"`
}

type CatalogConfig struct {
	UnknownModels string `yaml:"unknown_models"`
}

type ServerConfig struct {
	Address           string   `yaml:"address"`
	APIKey            string   `yaml:"api_key"`
	MaxBodyBytes      int64    `yaml:"max_body_bytes"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
}

type ProviderConfig struct {
	Type            string `yaml:"type"`
	BaseURL         string `yaml:"base_url"`
	APIKey          string `yaml:"api_key"`
	APIVersion      string `yaml:"api_version"`
	AccessToken     string `yaml:"access_token"`
	Project         string `yaml:"project"`
	Location        string `yaml:"location"`
	Region          string `yaml:"region"`
	AccessKeyID     string `yaml:"access_key_id"`
	SecretAccessKey string `yaml:"secret_access_key"`
	SessionToken    string `yaml:"session_token"`
}

type ModelConfig struct {
	Targets []TargetConfig `yaml:"targets"`
}

type TargetConfig struct {
	Provider        string          `yaml:"provider"`
	Model           string          `yaml:"model"`
	CatalogProvider string          `yaml:"catalog_provider"`
	CatalogModel    string          `yaml:"catalog_model"`
	RateLimit       RateLimitConfig `yaml:"rate_limit"`
}

type RoutingConfig struct {
	Retries               int         `yaml:"retries"`
	ResponseHeaderTimeout Duration    `yaml:"response_header_timeout"`
	Retry                 RetryConfig `yaml:"retry"`
}

type RetryConfig struct {
	BaseDelay Duration `yaml:"base_delay"`
	MaxDelay  Duration `yaml:"max_delay"`
	Budget    Duration `yaml:"budget"`
}

type RateLimitConfig struct {
	RequestsPerMinute int      `yaml:"requests_per_minute"`
	Burst             int      `yaml:"burst"`
	MaxConcurrency    int      `yaml:"max_concurrency"`
	QueueTimeout      Duration `yaml:"queue_timeout"`
}

type targetIdentity struct {
	provider string
	model    string
}

func Load(path string) (Config, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := defaultConfig()
	decoder := yaml.NewDecoder(strings.NewReader(os.ExpandEnv(string(contents))))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return Config{}, fmt.Errorf("decode config %q: expected exactly one YAML document", path)
		}
		return Config{}, fmt.Errorf("decode config %q: %w", path, err)
	}

	cfg.normalize()
	if err := cfg.Validate(); err != nil {
		return Config{}, fmt.Errorf("validate config %q: %w", path, err)
	}

	return cfg, nil
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
		},
	}
}

func (cfg *Config) normalize() {
	cfg.Server.Address = strings.TrimSpace(cfg.Server.Address)
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
