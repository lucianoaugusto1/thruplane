package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lucianoaugusto1/thruplane/internal/config"
)

func TestListModelsReturnsSortedOpenAIList(t *testing.T) {
	t.Parallel()

	gateway := New(config.Config{Models: map[string]config.ModelConfig{
		"zeta":   {},
		"alpha":  {},
		"middle": {},
	}}, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	response := httptest.NewRecorder()

	gateway.ListModels(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	if got := response.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var payload struct {
		Object string      `json:"object"`
		Data   []modelInfo `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Object != "list" {
		t.Errorf("object = %q, want list", payload.Object)
	}
	want := []string{"alpha", "middle", "zeta"}
	if len(payload.Data) != len(want) {
		t.Fatalf("models = %d, want %d", len(payload.Data), len(want))
	}
	for index, model := range payload.Data {
		if model.ID != want[index] {
			t.Errorf("model[%d].id = %q, want %q", index, model.ID, want[index])
		}
		if model.Object != "model" || model.Created != 0 || model.OwnedBy != "thruplane" {
			t.Errorf("model[%d] = %#v, want OpenAI model metadata", index, model)
		}
	}
}

func TestGetModelReturnsConfiguredAlias(t *testing.T) {
	t.Parallel()

	gateway := New(config.Config{Models: map[string]config.ModelConfig{
		"team-model.v1": {},
	}}, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/models/team-model.v1", nil)
	request.SetPathValue("model", "team-model.v1")
	response := httptest.NewRecorder()

	gateway.GetModel(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}
	var model modelInfo
	if err := json.Unmarshal(response.Body.Bytes(), &model); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if model.ID != "team-model.v1" || model.Object != "model" || model.OwnedBy != "thruplane" {
		t.Errorf("model = %#v, want configured alias", model)
	}
}

func TestGetModelReturnsOpenAIErrorForUnknownAlias(t *testing.T) {
	t.Parallel()

	gateway := New(config.Config{Models: map[string]config.ModelConfig{"known": {}}}, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/models/unknown", nil)
	request.SetPathValue("model", "unknown")
	response := httptest.NewRecorder()

	gateway.GetModel(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", response.Code)
	}
	assertOpenAIError(t, response)
}

func TestGetModelReturnsCatalogMetadataWithoutCredentials(t *testing.T) {
	t.Parallel()
	gateway := New(config.Config{
		Providers: map[string]config.ProviderConfig{
			"deployment": {Type: "azure-openai", APIKey: "private-secret"},
		},
		Models: map[string]config.ModelConfig{
			"team-model": {Targets: []config.TargetConfig{{
				Provider: "deployment", Model: "prod-2026", CatalogModel: "gpt-6-astra",
			}}},
		},
	}, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/models/team-model", nil)
	request.SetPathValue("model", "team-model")
	response := httptest.NewRecorder()
	gateway.GetModel(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("response = %d %s", response.Code, response.Body.String())
	}
	var info modelInfo
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(info.Targets) != 1 || !info.Targets[0].Cataloged || info.Targets[0].CatalogModel != "gpt-6-astra" {
		t.Fatalf("targets = %#v, want mapped catalog model", info.Targets)
	}
	if info.Targets[0].Limits == nil || info.Targets[0].Limits.ContextTokens == 0 || info.Targets[0].EffectiveCapabilities == nil {
		t.Fatalf("target lacks limits or effective capabilities: %#v", info.Targets[0])
	}
	if len(info.Targets[0].EffectiveCapabilities.Operations) != 1 || info.Targets[0].EffectiveCapabilities.Operations[0] != "chat" {
		t.Fatalf("effective operations = %v, want chat only", info.Targets[0].EffectiveCapabilities.Operations)
	}
	if strings.Contains(response.Body.String(), "private-secret") {
		t.Fatal("response disclosed provider credential")
	}
}

func TestGetModelDistinguishesGoogleModelAndAdapterModalities(t *testing.T) {
	t.Parallel()
	gateway := New(config.Config{
		Providers: map[string]config.ProviderConfig{"google": {Type: "gemini"}},
		Models:    map[string]config.ModelConfig{"assistant": {Targets: []config.TargetConfig{{Provider: "google", Model: "gemini-3.8-flash"}}}},
	}, nil)
	request := httptest.NewRequest(http.MethodGet, "/v1/models/assistant", nil)
	request.SetPathValue("model", "assistant")
	response := httptest.NewRecorder()
	gateway.GetModel(response, request)
	var info modelInfo
	if err := json.Unmarshal(response.Body.Bytes(), &info); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(info.Targets) != 1 || info.Targets[0].CatalogCapabilities == nil || info.Targets[0].EffectiveCapabilities == nil {
		t.Fatalf("targets = %#v, want catalog and effective capabilities", info.Targets)
	}
	if len(info.Targets[0].CatalogCapabilities.InputModalities) <= len(info.Targets[0].EffectiveCapabilities.InputModalities) {
		t.Fatalf("native adapter should narrow modalities: %#v", info.Targets[0])
	}
	if got := info.Targets[0].EffectiveCapabilities.InputModalities; len(got) != 4 || got[0] != "text" || got[1] != "image" || got[2] != "audio" || got[3] != "document" {
		t.Fatalf("effective input modalities = %v, want text/image/audio/document", got)
	}
}
