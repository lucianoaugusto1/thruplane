package config

import (
	"fmt"
	"time"

	"go.yaml.in/yaml/v3"
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
	Address           string           `yaml:"address"`
	APIKey            string           `yaml:"api_key"`
	MaxBodyBytes      int64            `yaml:"max_body_bytes"`
	ReadHeaderTimeout Duration         `yaml:"read_header_timeout"`
	ShutdownTimeout   Duration         `yaml:"shutdown_timeout"`
	Playground        PlaygroundConfig `yaml:"playground"`
	Metrics           MetricsConfig    `yaml:"metrics"`
}

type PlaygroundConfig struct {
	Enabled           bool                    `yaml:"enabled"`
	CredentialTesting CredentialTestingConfig `yaml:"credential_testing"`
}

type CredentialTestingConfig struct {
	Enabled         bool     `yaml:"enabled"`
	AllowedBaseURLs []string `yaml:"allowed_base_urls"`
}

type MetricsConfig struct {
	Enabled bool `yaml:"enabled"`
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
	Retries               int                  `yaml:"retries"`
	ResponseHeaderTimeout Duration             `yaml:"response_header_timeout"`
	Retry                 RetryConfig          `yaml:"retry"`
	CircuitBreaker        CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type RetryConfig struct {
	BaseDelay Duration `yaml:"base_delay"`
	MaxDelay  Duration `yaml:"max_delay"`
	Budget    Duration `yaml:"budget"`
}

type CircuitBreakerConfig struct {
	FailureThreshold int      `yaml:"failure_threshold"`
	OpenDuration     Duration `yaml:"open_duration"`
}

type RateLimitConfig struct {
	RequestsPerMinute int      `yaml:"requests_per_minute"`
	Burst             int      `yaml:"burst"`
	MaxConcurrency    int      `yaml:"max_concurrency"`
	QueueTimeout      Duration `yaml:"queue_timeout"`
}
