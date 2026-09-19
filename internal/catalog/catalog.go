package catalog

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"sort"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const schemaVersion = 1

//go:embed data/*.yaml
var builtInFiles embed.FS

type Source struct {
	URL   string `yaml:"url" json:"url"`
	Title string `yaml:"title" json:"title"`
}

type Provider struct {
	ID        string   `yaml:"id" json:"id"`
	Name      string   `yaml:"name" json:"name"`
	UpdatedAt string   `yaml:"updated_at" json:"updated_at"`
	Sources   []Source `yaml:"sources" json:"sources"`
}

type ToolCapabilities struct {
	FunctionCalling bool `yaml:"function_calling" json:"function_calling"`
	ParallelCalls   bool `yaml:"parallel_calls" json:"parallel_calls"`
	StrictSchema    bool `yaml:"strict_schema" json:"strict_schema"`
}

type Capabilities struct {
	Operations        []string         `yaml:"operations" json:"operations"`
	InputModalities   []string         `yaml:"input_modalities" json:"input_modalities"`
	OutputModalities  []string         `yaml:"output_modalities" json:"output_modalities"`
	Streaming         bool             `yaml:"streaming" json:"streaming"`
	StructuredOutputs bool             `yaml:"structured_outputs" json:"structured_outputs"`
	PromptCaching     bool             `yaml:"prompt_caching" json:"prompt_caching"`
	Reasoning         bool             `yaml:"reasoning" json:"reasoning"`
	Tools             ToolCapabilities `yaml:"tools" json:"tools"`
}

type Limits struct {
	ContextTokens    int64 `yaml:"context_tokens" json:"context_tokens,omitempty"`
	MaxInputTokens   int64 `yaml:"max_input_tokens" json:"max_input_tokens,omitempty"`
	MaxOutputTokens  int64 `yaml:"max_output_tokens" json:"max_output_tokens,omitempty"`
	MaxImages        int64 `yaml:"max_images" json:"max_images,omitempty"`
	MaxImageBytes    int64 `yaml:"max_image_bytes" json:"max_image_bytes,omitempty"`
	MaxDocumentBytes int64 `yaml:"max_document_bytes" json:"max_document_bytes,omitempty"`
}

type PriceRate struct {
	Kind      string  `yaml:"kind" json:"kind"`
	Modality  string  `yaml:"modality" json:"modality,omitempty"`
	Unit      string  `yaml:"unit" json:"unit"`
	Price     float64 `yaml:"price" json:"price"`
	Condition string  `yaml:"condition" json:"condition,omitempty"`
}

type Pricing struct {
	Currency string      `yaml:"currency" json:"currency,omitempty"`
	Rates    []PriceRate `yaml:"rates" json:"rates,omitempty"`
	Notes    string      `yaml:"notes" json:"notes,omitempty"`
}

type Performance struct {
	LatencyClass          string   `yaml:"latency_class" json:"latency_class,omitempty"`
	TimeToFirstTokenMS    *float64 `yaml:"time_to_first_token_ms" json:"time_to_first_token_ms,omitempty"`
	OutputTokensPerSecond *float64 `yaml:"output_tokens_per_second" json:"output_tokens_per_second,omitempty"`
	Notes                 string   `yaml:"notes" json:"notes,omitempty"`
}

type Model struct {
	ID              string       `yaml:"id" json:"id"`
	Aliases         []string     `yaml:"aliases" json:"aliases,omitempty"`
	Name            string       `yaml:"name" json:"name,omitempty"`
	Family          string       `yaml:"family" json:"family,omitempty"`
	Status          string       `yaml:"status" json:"status,omitempty"`
	KnowledgeCutoff string       `yaml:"knowledge_cutoff" json:"knowledge_cutoff,omitempty"`
	Capabilities    Capabilities `yaml:"capabilities" json:"capabilities"`
	Limits          Limits       `yaml:"limits" json:"limits"`
	Pricing         Pricing      `yaml:"pricing" json:"pricing"`
	Performance     Performance  `yaml:"performance" json:"performance"`
	Notes           string       `yaml:"notes" json:"notes,omitempty"`
	Provider        Provider     `yaml:"-" json:"provider"`
}

type Requirements struct {
	Operation         string
	InputModalities   []string
	OutputModalities  []string
	Tools             bool
	StrictTools       bool
	ParallelTools     bool
	Streaming         bool
	StructuredOutputs bool
	UnknownContent    bool
}

func (m Model) Supports(requirements Requirements) bool {
	if requirements.UnknownContent || !contains(m.Capabilities.Operations, requirements.Operation) {
		return false
	}
	for _, modality := range requirements.InputModalities {
		if !contains(m.Capabilities.InputModalities, modality) {
			return false
		}
	}
	for _, modality := range requirements.OutputModalities {
		if !contains(m.Capabilities.OutputModalities, modality) {
			return false
		}
	}
	if requirements.Tools && !m.Capabilities.Tools.FunctionCalling {
		return false
	}
	if requirements.StrictTools && !m.Capabilities.Tools.StrictSchema {
		return false
	}
	if requirements.ParallelTools && !m.Capabilities.Tools.ParallelCalls {
		return false
	}
	if requirements.Streaming && !m.Capabilities.Streaming {
		return false
	}
	return !requirements.StructuredOutputs || m.Capabilities.StructuredOutputs
}

type catalogFile struct {
	SchemaVersion int      `yaml:"schema_version"`
	Provider      Provider `yaml:"provider"`
	Models        []Model  `yaml:"models"`
}

type Registry struct {
	models    map[string]Model
	providers map[string]Provider
}

func BuiltIn() (*Registry, error) {
	return LoadFS(builtInFiles, "data/*.yaml")
}

func LoadFS(files fs.FS, pattern string) (*Registry, error) {
	paths, err := fs.Glob(files, pattern)
	if err != nil {
		return nil, fmt.Errorf("glob model catalogs: %w", err)
	}
	if len(paths) == 0 {
		return nil, errors.New("no model catalog files found")
	}
	sort.Strings(paths)
	registry := &Registry{models: make(map[string]Model), providers: make(map[string]Provider)}
	for _, path := range paths {
		contents, err := fs.ReadFile(files, path)
		if err != nil {
			return nil, fmt.Errorf("read model catalog %q: %w", path, err)
		}
		var catalog catalogFile
		decoder := yaml.NewDecoder(strings.NewReader(string(contents)))
		decoder.KnownFields(true)
		if err := decoder.Decode(&catalog); err != nil {
			return nil, fmt.Errorf("decode model catalog %q: %w", path, err)
		}
		var extra any
		if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
			if err == nil {
				return nil, fmt.Errorf("decode model catalog %q: expected one YAML document", path)
			}
			return nil, fmt.Errorf("decode model catalog %q: %w", path, err)
		}
		if err := registry.add(catalog); err != nil {
			return nil, fmt.Errorf("validate model catalog %q: %w", path, err)
		}
	}
	return registry, nil
}

func (r *Registry) add(file catalogFile) error {
	if file.SchemaVersion != schemaVersion {
		return fmt.Errorf("schema_version must be %d", schemaVersion)
	}
	provider := file.Provider
	provider.ID = strings.ToLower(strings.TrimSpace(provider.ID))
	if provider.ID == "" || strings.TrimSpace(provider.Name) == "" {
		return errors.New("provider id and name are required")
	}
	if _, exists := r.providers[provider.ID]; exists {
		return fmt.Errorf("provider %q is duplicated", provider.ID)
	}
	if _, err := time.Parse("2006-01-02", provider.UpdatedAt); err != nil {
		return fmt.Errorf("provider %q has invalid updated_at: %w", provider.ID, err)
	}
	if len(provider.Sources) == 0 {
		return fmt.Errorf("provider %q requires at least one source", provider.ID)
	}
	for index, source := range provider.Sources {
		parsed, err := url.ParseRequestURI(source.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || strings.TrimSpace(source.Title) == "" {
			return fmt.Errorf("provider %q source %d must have an HTTPS URL and title", provider.ID, index)
		}
	}
	if len(file.Models) == 0 {
		return fmt.Errorf("provider %q requires at least one model", provider.ID)
	}

	r.providers[provider.ID] = provider
	for _, model := range file.Models {
		model.ID = strings.TrimSpace(model.ID)
		if err := validateModel(provider.ID, model); err != nil {
			return err
		}
		model.Provider = provider
		identifiers := append([]string{model.ID}, model.Aliases...)
		for _, identifier := range identifiers {
			key := catalogKey(provider.ID, identifier)
			if _, exists := r.models[key]; exists {
				return fmt.Errorf("provider %q model or alias %q is duplicated", provider.ID, identifier)
			}
			r.models[key] = model
		}
	}
	return nil
}

func validateModel(providerID string, model Model) error {
	if model.ID == "" {
		return fmt.Errorf("provider %q has a model without an id", providerID)
	}
	if len(model.Capabilities.Operations) == 0 || len(model.Capabilities.InputModalities) == 0 || len(model.Capabilities.OutputModalities) == 0 {
		return fmt.Errorf("provider %q model %q requires operations and input/output modalities", providerID, model.ID)
	}
	if model.Limits.ContextTokens < 0 || model.Limits.MaxInputTokens < 0 || model.Limits.MaxOutputTokens < 0 {
		return fmt.Errorf("provider %q model %q has a negative token limit", providerID, model.ID)
	}
	if len(model.Pricing.Rates) > 0 && model.Pricing.Currency == "" {
		return fmt.Errorf("provider %q model %q pricing requires currency", providerID, model.ID)
	}
	for index, rate := range model.Pricing.Rates {
		if rate.Kind == "" || rate.Unit == "" || rate.Price < 0 {
			return fmt.Errorf("provider %q model %q rate %d requires kind, unit, and non-negative price", providerID, model.ID, index)
		}
	}
	return nil
}

func (r *Registry) Lookup(providerID, modelID string) (Model, bool) {
	if r == nil {
		return Model{}, false
	}
	model, ok := r.models[catalogKey(providerID, modelID)]
	return model, ok
}

func (r *Registry) Models(providerID string) []Model {
	if r == nil {
		return nil
	}
	seen := make(map[string]bool)
	models := make([]Model, 0)
	for _, model := range r.models {
		if model.Provider.ID == providerID && !seen[model.ID] {
			seen[model.ID] = true
			models = append(models, model)
		}
	}
	sort.Slice(models, func(i, j int) bool { return models[i].ID < models[j].ID })
	return models
}

func catalogKey(providerID, modelID string) string {
	return strings.ToLower(strings.TrimSpace(providerID)) + "\x00" + strings.ToLower(strings.TrimSpace(modelID))
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
