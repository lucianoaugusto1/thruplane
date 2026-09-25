package gateway

import (
	"encoding/json"
	"net/http"

	"nexoroute/internal/catalog"
	"nexoroute/internal/config"
	"nexoroute/internal/provider"
)

type chatPlan struct {
	stream  bool
	targets []config.TargetConfig
}

func (g *Gateway) planChat(body []byte) (chatPlan, *gatewayError) {
	alias, stream, requestError := validateChatRequest(body)
	if requestError != nil {
		return chatPlan{}, requestError
	}

	model, ok := g.settings.models[alias]
	if !ok {
		return chatPlan{}, newGatewayError(http.StatusNotFound, "The requested model is not configured.", "invalid_request_error", "model_not_found")
	}

	requirements, err := catalog.InspectChatRequest(body)
	if err != nil {
		return chatPlan{}, newGatewayError(http.StatusBadRequest, "The request body must be valid JSON.", "invalid_request_error", "invalid_json")
	}

	eligible := make([]config.TargetConfig, 0, len(model.Targets))
	var unknown int
	for _, target := range model.Targets {
		catalogModel, known := g.lookupTarget(target)
		if known {
			providerType := g.settings.providerType(target.Provider)
			catalogModel.Capabilities = provider.EffectiveCapabilities(providerType, catalogModel)
			if !catalogModel.Supports(requirements) || !provider.SupportsRequirements(providerType, requirements) {
				continue
			}
		} else if g.settings.unknownModels == "reject" {
			unknown++
			continue
		}
		eligible = append(eligible, target)
	}
	if len(eligible) == 0 {
		if unknown == len(model.Targets) {
			return chatPlan{}, newGatewayError(http.StatusBadRequest, "No target model is present in the model catalog.", "invalid_request_error", "model_not_cataloged")
		}
		return chatPlan{}, newGatewayError(http.StatusBadRequest, "No configured target supports the requested capabilities.", "invalid_request_error", "unsupported_capability")
	}

	targets := make([]config.TargetConfig, 0, len(eligible))
	for _, target := range eligible {
		if g.clients[target.Provider] != nil {
			targets = append(targets, target)
		}
	}
	return chatPlan{stream: stream, targets: targets}, nil
}

func validateChatRequest(body []byte) (string, bool, *gatewayError) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		return "", false, newGatewayError(http.StatusBadRequest, "The request body must be a JSON object.", "invalid_request_error", "invalid_json")
	}

	modelField, ok := fields["model"]
	if !ok {
		return "", false, newGatewayError(http.StatusBadRequest, "The model field is required.", "invalid_request_error", "missing_model")
	}
	var model string
	if err := json.Unmarshal(modelField, &model); err != nil {
		return "", false, newGatewayError(http.StatusBadRequest, "The model field must be a string.", "invalid_request_error", "invalid_model")
	}
	if _, ok := fields["messages"]; !ok {
		return "", false, newGatewayError(http.StatusBadRequest, "The messages field is required.", "invalid_request_error", "missing_messages")
	}

	if nField, ok := fields["n"]; ok {
		var n int
		if err := json.Unmarshal(nField, &n); err != nil || n != 1 {
			return "", false, newGatewayError(http.StatusBadRequest, "The n field must be 1 when provided.", "invalid_request_error", "unsupported_n")
		}
	}

	var stream bool
	if streamField, ok := fields["stream"]; ok {
		if err := json.Unmarshal(streamField, &stream); err != nil {
			return "", false, newGatewayError(http.StatusBadRequest, "The stream field must be a boolean.", "invalid_request_error", "invalid_stream")
		}
	}

	return model, stream, nil
}

func (g *Gateway) lookupTarget(target config.TargetConfig) (catalog.Model, bool) {
	providerID := g.settings.providerType(target.Provider)
	modelID := target.Model
	if target.CatalogProvider != "" {
		providerID = target.CatalogProvider
	}
	if target.CatalogModel != "" {
		modelID = target.CatalogModel
	}
	return g.catalog.Lookup(providerID, modelID)
}
