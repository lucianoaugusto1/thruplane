package catalog

import (
	"testing"
	"testing/fstest"
)

func TestLoadValidatesAndIndexesCatalogFiles(t *testing.T) {
	registry, err := LoadFS(fstest.MapFS{
		"data/example.yaml": {Data: []byte(`
schema_version: 1
provider:
  id: example
  name: Example
  updated_at: "2026-09-19"
  sources:
    - url: https://example.com/models
      title: Model documentation
models:
  - id: model-v1
    aliases: [model-latest]
    name: Model V1
    status: active
    capabilities:
      operations: [chat]
      input_modalities: [text, image]
      output_modalities: [text]
      streaming: true
      structured_outputs: true
      prompt_caching: true
      tools:
        function_calling: true
        parallel_calls: true
        strict_schema: false
    limits:
      context_tokens: 128000
      max_output_tokens: 16000
    pricing:
      currency: USD
      rates:
        - kind: input
          modality: text
          unit: million_tokens
          price: 1.25
        - kind: cache_read
          modality: text
          unit: million_tokens
          price: 0.25
    performance:
      latency_class: fast
      notes: Provider does not publish comparable TTFT or output TPS.
`)}}, "data/*.yaml")
	if err != nil {
		t.Fatalf("LoadFS() error = %v", err)
	}

	model, ok := registry.Lookup("example", "model-latest")
	if !ok {
		t.Fatal("Lookup() did not resolve model alias")
	}
	if model.ID != "model-v1" || model.Limits.ContextTokens != 128000 {
		t.Fatalf("Lookup() model = %#v", model)
	}
	if !model.Supports(Requirements{Operation: "chat", InputModalities: []string{"image"}, Tools: true, Streaming: true}) {
		t.Fatal("Supports() = false for supported requirements")
	}
	if model.Supports(Requirements{Operation: "chat", InputModalities: []string{"audio"}}) {
		t.Fatal("Supports() = true for unsupported audio")
	}
	if len(model.Pricing.Rates) != 2 || model.Pricing.Rates[1].Kind != "cache_read" {
		t.Fatalf("pricing rates = %#v", model.Pricing.Rates)
	}
}

func TestLoadRejectsUnknownFields(t *testing.T) {
	_, err := LoadFS(fstest.MapFS{
		"data/bad.yaml": {Data: []byte(`
schema_version: 1
provider:
  id: example
  name: Example
  updated_at: "2026-09-19"
  sources:
    - url: https://example.com
      title: Source
models:
  - id: model-v1
    surprise: true
    capabilities:
      operations: [chat]
      input_modalities: [text]
      output_modalities: [text]
`)}}, "data/*.yaml")
	if err == nil {
		t.Fatal("LoadFS() error = nil, want strict YAML error")
	}
}

func TestLoadRejectsDuplicateProviderModel(t *testing.T) {
	contents := []byte(`
schema_version: 1
provider:
  id: example
  name: Example
  updated_at: "2026-09-19"
  sources:
    - url: https://example.com
      title: Source
models:
  - id: duplicate
    capabilities:
      operations: [chat]
      input_modalities: [text]
      output_modalities: [text]
`)
	_, err := LoadFS(fstest.MapFS{
		"data/one.yaml": {Data: contents},
		"data/two.yaml": {Data: contents},
	}, "data/*.yaml")
	if err == nil {
		t.Fatal("LoadFS() error = nil, want duplicate model error")
	}
}

func TestBuiltInCatalogContainsInitialProviderModels(t *testing.T) {
	registry, err := BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn() error = %v", err)
	}

	tests := []struct {
		provider string
		model    string
	}{
		{provider: "openai", model: "gpt-6-astra"},
		{provider: "anthropic", model: "claude-sonnet-5"},
		{provider: "gemini", model: "gemini-3.8-flash"},
		{provider: "vertex", model: "gemini-3.8-flash"},
		{provider: "bedrock", model: "global.amazon.nova-2-lite-v1:0"},
		{provider: "azure-openai", model: "gpt-5.6-terra"},
		{provider: "ollama", model: "llama3.2:latest"},
		{provider: "xai", model: "grok-4.6"},
	}
	for _, test := range tests {
		t.Run(test.provider+"/"+test.model, func(t *testing.T) {
			model, ok := registry.Lookup(test.provider, test.model)
			if !ok {
				t.Fatal("Lookup() did not find embedded model")
			}
			if len(model.Provider.Sources) == 0 || model.Provider.UpdatedAt == "" {
				t.Fatalf("model provenance = %#v", model.Provider)
			}
			if model.Performance.OutputTokensPerSecond != nil || model.Performance.TimeToFirstTokenMS != nil {
				t.Fatal("provider-independent measured performance must remain unknown")
			}
		})
	}
}

func TestBuiltInOpenAIFileAndAudioStatus(t *testing.T) {
	registry, err := BuiltIn()
	if err != nil {
		t.Fatalf("BuiltIn() error = %v", err)
	}
	model, ok := registry.Lookup("openai", "gpt-6-astra")
	if !ok || !model.Supports(Requirements{Operation: "chat", InputModalities: []string{"document"}}) {
		t.Fatalf("GPT-6 Astra PDF capability = %#v", model.Capabilities.InputModalities)
	}
	audio, ok := registry.Lookup("openai", "gpt-4o-mini-audio-preview")
	if !ok || audio.Status != "deprecated" {
		t.Fatalf("audio model status = %q, want deprecated", audio.Status)
	}
}
