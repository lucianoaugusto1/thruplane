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
	Providers map[string]ProviderConfig `yaml:"providers"`
	Models    map[string]ModelConfig    `yaml:"models"`
	Routing   RoutingConfig             `yaml:"routing"`
}

type ServerConfig struct {
	Address           string   `yaml:"address"`
	APIKey            string   `yaml:"api_key"`
	MaxBodyBytes      int64    `yaml:"max_body_bytes"`
	ReadHeaderTimeout Duration `yaml:"read_header_timeout"`
	ShutdownTimeout   Duration `yaml:"shutdown_timeout"`
}

type ProviderConfig struct {
	Type    string `yaml:"type"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
}

type ModelConfig struct {
	Targets []TargetConfig `yaml:"targets"`
}

type TargetConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type RoutingConfig struct {
	Retries               int      `yaml:"retries"`
	ResponseHeaderTimeout Duration `yaml:"response_header_timeout"`
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
		}
	}

	return nil
}

func defaultConfig() Config {
	return Config{
		Server: ServerConfig{
			Address:           defaultAddress,
			MaxBodyBytes:      defaultMaxBodyBytes,
			ReadHeaderTimeout: Duration(defaultReadHeaderTimeout),
			ShutdownTimeout:   Duration(defaultShutdownTimeout),
		},
		Routing: RoutingConfig{
			Retries:               defaultRetries,
			ResponseHeaderTimeout: Duration(defaultResponseHeaderTimeout),
		},
	}
}

func (cfg *Config) normalize() {
	cfg.Server.Address = strings.TrimSpace(cfg.Server.Address)

	for name, provider := range cfg.Providers {
		provider.Type = strings.TrimSpace(provider.Type)
		if provider.Type == "" {
			provider.Type = "openai"
		}
		provider.BaseURL = strings.TrimRight(strings.TrimSpace(provider.BaseURL), "/")
		cfg.Providers[name] = provider
	}

	for alias, model := range cfg.Models {
		for index := range model.Targets {
			model.Targets[index].Provider = strings.TrimSpace(model.Targets[index].Provider)
			model.Targets[index].Model = strings.TrimSpace(model.Targets[index].Model)
		}
		cfg.Models[alias] = model
	}
}

func validateProvider(name string, provider ProviderConfig) error {
	switch provider.Type {
	case "", "openai", "ollama":
	default:
		return fmt.Errorf("provider %q type must be openai or ollama", name)
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
