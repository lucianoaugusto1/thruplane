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
	if got := time.Duration(cfg.Routing.ResponseHeaderTimeout); got != 30*time.Second {
		t.Errorf("Routing.ResponseHeaderTimeout = %s, want %s", got, 30*time.Second)
	}
	if got := cfg.Providers["openai"].APIKey; got != "expanded-secret" {
		t.Errorf("Providers[openai].APIKey = %q, want expanded value", got)
	}
	if got := cfg.Providers["openai"].BaseURL; got != "https://api.openai.com/v1" {
		t.Errorf("Providers[openai].BaseURL = %q, want normalized URL", got)
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

func TestValidateRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{
			name: "negative retries",
			mutate: func(cfg *Config) {
				cfg.Routing.Retries = -1
			},
			wantErr: "routing retries",
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
