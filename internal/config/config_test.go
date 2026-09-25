package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadValidConfigAppliesDefaultsAndExpandsEnvironment(t *testing.T) {
	t.Setenv("TEST_UPSTREAM_KEY", "expanded-secret")

	path := writeConfig(t, `
providers:
  openai:
    type: openai
    base_url: https://api.openai.com/v1/
    api_key: ${TEST_UPSTREAM_KEY}
models:
  chat:
    targets:
      - provider: openai
        model: gpt-4o-mini
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Address != ":8080" {
		t.Errorf("Server.Address = %q, want %q", cfg.Server.Address, ":8080")
	}
	if cfg.Server.MaxBodyBytes != 1<<20 {
		t.Errorf("Server.MaxBodyBytes = %d, want %d", cfg.Server.MaxBodyBytes, 1<<20)
	}
	if got := time.Duration(cfg.Server.ReadHeaderTimeout); got != 5*time.Second {
		t.Errorf("Server.ReadHeaderTimeout = %s, want %s", got, 5*time.Second)
	}
	if got := time.Duration(cfg.Server.ShutdownTimeout); got != 10*time.Second {
		t.Errorf("Server.ShutdownTimeout = %s, want %s", got, 10*time.Second)
	}
	if cfg.Routing.Retries != 1 {
		t.Errorf("Routing.Retries = %d, want 1", cfg.Routing.Retries)
	}
	if cfg.Catalog.UnknownModels != "allow" {
		t.Errorf("Catalog.UnknownModels = %q, want allow", cfg.Catalog.UnknownModels)
	}
	if got := time.Duration(cfg.Routing.ResponseHeaderTimeout); got != 30*time.Second {
		t.Errorf("Routing.ResponseHeaderTimeout = %s, want %s", got, 30*time.Second)
	}
	if got := time.Duration(cfg.Routing.Retry.BaseDelay); got != 200*time.Millisecond {
		t.Errorf("Routing.Retry.BaseDelay = %s, want %s", got, 200*time.Millisecond)
	}
	if got := time.Duration(cfg.Routing.Retry.MaxDelay); got != 5*time.Second {
		t.Errorf("Routing.Retry.MaxDelay = %s, want %s", got, 5*time.Second)
	}
	if got := time.Duration(cfg.Routing.Retry.Budget); got != 15*time.Second {
		t.Errorf("Routing.Retry.Budget = %s, want %s", got, 15*time.Second)
	}
	if got := cfg.Providers["openai"].APIKey; got != "expanded-secret" {
		t.Errorf("Providers[openai].APIKey = %q, want expanded value", got)
	}
	if got := cfg.Providers["openai"].BaseURL; got != "https://api.openai.com/v1" {
		t.Errorf("Providers[openai].BaseURL = %q, want normalized URL", got)
	}
}

func TestLoadPlaygroundConfiguration(t *testing.T) {
	path := writeConfig(t, `
server:
  playground:
    enabled: true
providers:
  local:
    type: ollama
    base_url: http://localhost:11434
models:
  chat:
    targets:
      - provider: local
        model: llama3
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Server.Playground.Enabled {
		t.Fatal("Server.Playground.Enabled = false, want true")
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	path := writeConfig(t, `
server:
  unexpected: true
providers:
  local:
    base_url: http://localhost:11434/v1
models:
  chat:
    targets:
      - provider: local
        model: llama3
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("Load() error = %v, want unknown-field error", err)
	}
}

func TestLoadRejectsBadProviderURL(t *testing.T) {
	path := writeConfig(t, `
providers:
  broken:
    base_url: not-a-url
models:
  chat:
    targets:
      - provider: broken
        model: model-a
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), `provider "broken" base_url`) {
		t.Fatalf("Load() error = %v, want provider URL error", err)
	}
}

func TestLoadRejectsModelWithoutTargets(t *testing.T) {
	path := writeConfig(t, `
providers:
  local:
    base_url: http://localhost:11434/v1
models:
  chat:
    targets: []
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), `model "chat" must have at least one target`) {
		t.Fatalf("Load() error = %v, want missing-target error", err)
	}
}

func TestLoadRejectsMissingProviderReference(t *testing.T) {
	path := writeConfig(t, `
providers:
  local:
    base_url: http://localhost:11434/v1
models:
  chat:
    targets:
      - provider: absent
        model: model-a
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), `references unknown provider "absent"`) {
		t.Fatalf("Load() error = %v, want missing-provider error", err)
	}
}

func TestLoadRejectsMultipleYAMLDocuments(t *testing.T) {
	path := writeConfig(t, `
providers:
  local:
    base_url: http://localhost:11434/v1
models:
  chat:
    targets:
      - provider: local
        model: model-a
---
providers: {}
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "exactly one YAML document") {
		t.Fatalf("Load() error = %v, want multiple-document error", err)
	}
}

func TestExampleConfigurationLoads(t *testing.T) {
	t.Setenv("NEXOROUTE_API_KEY", "gateway-key")
	t.Setenv("OPENAI_API_KEY", "provider-key")
	if _, err := Load(filepath.Join("..", "..", "config.example.yaml")); err != nil {
		t.Fatalf("Load(config.example.yaml) error = %v", err)
	}
}

func TestValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "invalid unknown model policy",
			mutate: func(cfg *Config) {
				cfg.Catalog.UnknownModels = "guess"
			},
			wantErr: "catalog unknown_models",
		},
		{
			name: "negative retries",
			mutate: func(cfg *Config) {
				cfg.Routing.Retries = -1
			},
			wantErr: "routing retries",
		},
		{
			name: "retry base delay is zero",
			mutate: func(cfg *Config) {
				cfg.Routing.Retry.BaseDelay = 0
			},
			wantErr: "routing retry base_delay",
		},
		{
			name: "retry maximum below base",
			mutate: func(cfg *Config) {
				cfg.Routing.Retry.MaxDelay = Duration(100 * time.Millisecond)
			},
			wantErr: "routing retry max_delay",
		},
		{
			name: "retry budget below base",
			mutate: func(cfg *Config) {
				cfg.Routing.Retry.Budget = Duration(100 * time.Millisecond)
			},
			wantErr: "routing retry budget",
		},
		{
			name: "empty target model",
			mutate: func(cfg *Config) {
				cfg.Models["chat"] = ModelConfig{Targets: []TargetConfig{{Provider: "local"}}}
			},
			wantErr: "target model must not be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			if err := cfg.Validate(); err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadNormalizesAndValidatesTargetRateLimits(t *testing.T) {
	path := writeConfig(t, `
providers:
  openai: {type: openai}
models:
  chat:
    targets:
      - provider: openai
        model: gpt-4o-mini
        rate_limit:
          requests_per_minute: 120
          max_concurrency: 8
          queue_timeout: 250ms
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	got := cfg.Models["chat"].Targets[0].RateLimit
	if got.RequestsPerMinute != 120 || got.Burst != 1 || got.MaxConcurrency != 8 || time.Duration(got.QueueTimeout) != 250*time.Millisecond {
		t.Fatalf("rate limit = %#v, want normalized target policy", got)
	}
}

func TestLoadRejectsInvalidTargetRateLimits(t *testing.T) {
	tests := []struct {
		name    string
		fields  string
		wantErr string
	}{
		{name: "negative requests", fields: "requests_per_minute: -1", wantErr: "requests_per_minute"},
		{name: "burst without rate", fields: "burst: 2", wantErr: "burst requires"},
		{name: "negative concurrency", fields: "max_concurrency: -1", wantErr: "max_concurrency"},
		{name: "negative queue", fields: "queue_timeout: -1s", wantErr: "queue_timeout"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, "providers:\n  openai: {type: openai}\nmodels:\n  chat:\n    targets:\n      - provider: openai\n        model: gpt-4o-mini\n        rate_limit:\n          "+tt.fields+"\n")
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestLoadRejectsConflictingPoliciesForSharedTarget(t *testing.T) {
	path := writeConfig(t, `
providers:
  openai: {type: openai}
models:
  fast:
    targets:
      - provider: openai
        model: gpt-4o-mini
        rate_limit: {max_concurrency: 8}
  smart:
    targets:
      - provider: openai
        model: gpt-4o-mini
        rate_limit: {max_concurrency: 4}
`)

	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "conflicting rate_limit") {
		t.Fatalf("Load() error = %v, want conflicting shared-target policy", err)
	}
}

func TestLoadAcceptsIdenticalPoliciesForSharedTarget(t *testing.T) {
	path := writeConfig(t, `
providers:
  openai: {type: openai}
models:
  fast:
    targets:
      - provider: openai
        model: gpt-4o-mini
        rate_limit: {requests_per_minute: 60, burst: 5, max_concurrency: 8}
  smart:
    targets:
      - provider: openai
        model: gpt-4o-mini
        rate_limit: {requests_per_minute: 60, burst: 5, max_concurrency: 8}
`)

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() error = %v, want identical shared-target policies accepted", err)
	}
}

func TestLoadNormalizesCatalogTargetMapping(t *testing.T) {
	path := writeConfig(t, `
catalog:
  unknown_models: REJECT
providers:
  azure:
    type: azure-openai
    base_url: https://customer.openai.azure.com
models:
  chat:
    targets:
      - provider: azure
        model: customer-deployment
        catalog_provider: OpenAI
        catalog_model: GPT-5.6-Terra
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	target := cfg.Models["chat"].Targets[0]
	if cfg.Catalog.UnknownModels != "reject" || target.CatalogProvider != "openai" || target.CatalogModel != "GPT-5.6-Terra" {
		t.Fatalf("normalized catalog config = %#v, target = %#v", cfg.Catalog, target)
	}
}

func TestLoadNormalizesProviderTypesAndDefaults(t *testing.T) {
	path := writeConfig(t, `
providers:
  openai:
    type: openai
  claude:
    type: anthropic
  gemini:
    type: gemini
  grok:
    type: grok
  local:
    type: ollama
models:
  chat:
    targets:
      - provider: grok
        model: grok-model
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	wants := map[string]struct {
		typeName string
		baseURL  string
	}{
		"openai": {"openai", "https://api.openai.com"},
		"claude": {"anthropic", "https://api.anthropic.com"},
		"gemini": {"gemini", "https://generativelanguage.googleapis.com"},
		"grok":   {"xai", "https://api.x.ai"},
		"local":  {"ollama", "http://localhost:11434"},
	}
	for name, want := range wants {
		got := cfg.Providers[name]
		if got.Type != want.typeName || got.BaseURL != want.baseURL {
			t.Errorf("provider %s = type %q URL %q, want type %q URL %q", name, got.Type, got.BaseURL, want.typeName, want.baseURL)
		}
	}
}

func TestLoadAcceptsEveryProviderType(t *testing.T) {
	path := writeConfig(t, `
providers:
  openai: {type: openai}
  anthropic: {type: anthropic}
  gemini: {type: gemini}
  vertex:
    type: vertex
    project: customer-project
    location: us-central1
    access_token: test-token
  bedrock:
    type: bedrock
    region: us-east-1
    access_key_id: test-access-key
    secret_access_key: test-secret
  azure:
    type: azure-openai
    base_url: https://customer.openai.azure.com
  ollama: {type: ollama}
  compatible:
    type: openai-compatible
    base_url: https://models.example.com
  inference:
    type: nexoroute-inference
    base_url: https://inference.example.com
  xai: {type: xai}
models:
  chat:
    targets:
      - provider: openai
        model: upstream-model
`)

	if _, err := Load(path); err != nil {
		t.Fatalf("Load() error = %v", err)
	}
}

func TestLoadRejectsMissingTypeSpecificProviderFields(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		wantErr  string
	}{
		{"vertex project", "type: vertex\n    location: us-central1\n    access_token: token", "project"},
		{"vertex location", "type: vertex\n    project: project\n    access_token: token", "location"},
		{"vertex token", "type: vertex\n    project: project\n    location: us-central1", "access_token"},
		{"bedrock region", "type: bedrock\n    access_key_id: key\n    secret_access_key: secret", "region"},
		{"bedrock access key", "type: bedrock\n    region: us-east-1\n    secret_access_key: secret", "access_key_id"},
		{"bedrock secret", "type: bedrock\n    region: us-east-1\n    access_key_id: key", "secret_access_key"},
		{"azure URL", "type: azure-openai", "base_url"},
		{"compatible URL", "type: openai-compatible", "base_url"},
		{"inference URL", "type: nexoroute-inference", "base_url"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeConfig(t, "providers:\n  tested:\n    "+tt.provider+"\nmodels:\n  chat:\n    targets:\n      - provider: tested\n        model: model\n")
			_, err := Load(path)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("Load() error = %v, want error containing %q", err, tt.wantErr)
			}
		})
	}
}

func validConfig() Config {
	return Config{
		Server: ServerConfig{
			Address:           ":8080",
			MaxBodyBytes:      1 << 20,
			ReadHeaderTimeout: Duration(5 * time.Second),
			ShutdownTimeout:   Duration(10 * time.Second),
		},
		Providers: map[string]ProviderConfig{
			"local": {BaseURL: "http://localhost:11434/v1"},
		},
		Models: map[string]ModelConfig{
			"chat": {Targets: []TargetConfig{{Provider: "local", Model: "model-a"}}},
		},
		Routing: RoutingConfig{
			Retries:               1,
			ResponseHeaderTimeout: Duration(30 * time.Second),
			Retry: RetryConfig{
				BaseDelay: Duration(200 * time.Millisecond),
				MaxDelay:  Duration(5 * time.Second),
				Budget:    Duration(15 * time.Second),
			},
		},
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
