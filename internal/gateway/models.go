package gateway

import (
	"encoding/json"
	"net/http"
	"sort"

	"nexoroute/internal/catalog"
	"nexoroute/internal/provider"
)

type modelInfo struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	OwnedBy string       `json:"owned_by"`
	Targets []targetInfo `json:"targets,omitempty"`
}

type targetInfo struct {
	Provider              string                `json:"provider"`
	Model                 string                `json:"model"`
	Cataloged             bool                  `json:"cataloged"`
	CatalogProvider       string                `json:"catalog_provider,omitempty"`
	CatalogModel          string                `json:"catalog_model,omitempty"`
	Name                  string                `json:"name,omitempty"`
	Status                string                `json:"status,omitempty"`
	CatalogCapabilities   *catalog.Capabilities `json:"catalog_capabilities,omitempty"`
	EffectiveCapabilities *catalog.Capabilities `json:"effective_capabilities,omitempty"`
	Limits                *catalog.Limits       `json:"limits,omitempty"`
	Pricing               *catalog.Pricing      `json:"pricing,omitempty"`
	Performance           *catalog.Performance  `json:"performance,omitempty"`
	Sources               []catalog.Source      `json:"sources,omitempty"`
	UpdatedAt             string                `json:"updated_at,omitempty"`
}

func (g *Gateway) ListModels(w http.ResponseWriter, _ *http.Request) {
	aliases := make([]string, 0, len(g.config.Models))
	for alias := range g.config.Models {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)

	models := make([]modelInfo, 0, len(aliases))
	for _, alias := range aliases {
		models = append(models, newModelInfo(alias))
	}

	writeJSON(w, http.StatusOK, struct {
		Object string      `json:"object"`
		Data   []modelInfo `json:"data"`
	}{Object: "list", Data: models})
}

func (g *Gateway) GetModel(w http.ResponseWriter, r *http.Request) {
	alias := r.PathValue("model")
	configured, ok := g.config.Models[alias]
	if !ok {
		writeError(w, http.StatusNotFound, "The requested model is not configured.", "invalid_request_error", "model_not_found")
		return
	}
	info := newModelInfo(alias)
	for _, target := range configured.Targets {
		entry := targetInfo{Provider: target.Provider, Model: target.Model}
		if model, known := g.lookupTarget(target); known {
			effective := provider.EffectiveCapabilities(g.config.Providers[target.Provider], model)
			entry.Cataloged = true
			entry.CatalogProvider = model.Provider.ID
			entry.CatalogModel = model.ID
			entry.Name = model.Name
			entry.Status = model.Status
			entry.CatalogCapabilities = &model.Capabilities
			entry.EffectiveCapabilities = &effective
			entry.Limits = &model.Limits
			entry.Pricing = &model.Pricing
			entry.Performance = &model.Performance
			entry.Sources = model.Provider.Sources
			entry.UpdatedAt = model.Provider.UpdatedAt
		}
		info.Targets = append(info.Targets, entry)
	}
	writeJSON(w, http.StatusOK, info)
}

func newModelInfo(alias string) modelInfo {
	return modelInfo{
		ID:      alias,
		Object:  "model",
		Created: 0,
		OwnedBy: "nexoroute",
	}
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
