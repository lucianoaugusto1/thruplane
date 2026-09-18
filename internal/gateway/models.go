package gateway

import (
	"encoding/json"
	"net/http"
	"sort"
)

type modelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
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
	if _, ok := g.config.Models[alias]; !ok {
		writeError(w, http.StatusNotFound, "The requested model is not configured.", "invalid_request_error", "model_not_found")
		return
	}
	writeJSON(w, http.StatusOK, newModelInfo(alias))
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
