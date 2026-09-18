package gateway

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"gollm-gateway/internal/config"
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
		if model.Object != "model" || model.Created != 0 || model.OwnedBy != "gateway" {
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
	if model.ID != "team-model.v1" || model.Object != "model" || model.OwnedBy != "gateway" {
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
