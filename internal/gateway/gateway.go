package gateway

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"nexoroute/internal/catalog"
	"nexoroute/internal/config"
	"nexoroute/internal/provider"
)

const streamBufferSize = 32 * 1024

type Gateway struct {
	config  config.Config
	clients map[string]*provider.Client
	catalog *catalog.Registry
}

func New(cfg config.Config, clients map[string]*provider.Client) *Gateway {
	registry, err := catalog.BuiltIn()
	if err != nil {
		panic("invalid embedded model catalog: " + err.Error())
	}
	return NewWithCatalog(cfg, clients, registry)
}

func NewWithCatalog(cfg config.Config, clients map[string]*provider.Client, registry *catalog.Registry) *Gateway {
	return &Gateway{config: cfg, clients: clients, catalog: registry}
}

func (g *Gateway) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	body, ok := readBody(w, r, g.config.Server.MaxBodyBytes)
	if !ok {
		return
	}

	alias, stream, ok := validateChatRequest(w, body)
	if !ok {
		return
	}
	model, ok := g.config.Models[alias]
	if !ok {
		writeError(w, http.StatusNotFound, "The requested model is not configured.", "invalid_request_error", "model_not_found")
		return
	}
	requirements, err := catalog.InspectChatRequest(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "The request body must be valid JSON.", "invalid_request_error", "invalid_json")
		return
	}
	eligible := make([]config.TargetConfig, 0, len(model.Targets))
	var unknown int
	for _, target := range model.Targets {
		catalogModel, known := g.lookupTarget(target)
		if known {
			catalogModel.Capabilities = provider.EffectiveCapabilities(g.config.Providers[target.Provider], catalogModel)
			if !catalogModel.Supports(requirements) || !provider.SupportsRequirements(g.config.Providers[target.Provider], requirements) {
				continue
			}
		} else if g.config.Catalog.UnknownModels == "reject" {
			unknown++
			continue
		}
		eligible = append(eligible, target)
	}
	if len(eligible) == 0 {
		if unknown == len(model.Targets) {
			writeError(w, http.StatusBadRequest, "No target model is present in the model catalog.", "invalid_request_error", "model_not_cataloged")
			return
		}
		writeError(w, http.StatusBadRequest, "No configured target supports the requested capabilities.", "invalid_request_error", "unsupported_capability")
		return
	}

	for targetIndex, target := range eligible {
		client := g.clients[target.Provider]
		if client == nil {
			continue
		}

		for attempt := 0; attempt <= g.config.Routing.Retries; attempt++ {
			if r.Context().Err() != nil {
				return
			}

			response, err := client.Do(r.Context(), body, target.Model)
			if err != nil {
				if r.Context().Err() != nil {
					return
				}
				var requestError *provider.RequestError
				if errors.As(err, &requestError) {
					writeError(w, http.StatusBadRequest, requestError.Message, "invalid_request_error", requestError.Code)
					return
				}
				continue
			}

			if !isRetryable(response.StatusCode) {
				relayResponse(w, response, stream)
				return
			}

			lastAttempt := attempt == g.config.Routing.Retries
			lastTarget := targetIndex == len(eligible)-1
			if lastAttempt && lastTarget {
				relayResponse(w, response, stream)
				return
			}

			_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 64<<10))
			response.Body.Close()
		}
	}

	if r.Context().Err() != nil {
		return
	}
	writeError(w, http.StatusBadGateway, "The gateway could not reach an upstream provider.", "api_error", "upstream_unavailable")
}

func (g *Gateway) lookupTarget(target config.TargetConfig) (catalog.Model, bool) {
	providerID := g.config.Providers[target.Provider].Type
	modelID := target.Model
	if target.CatalogProvider != "" {
		providerID = target.CatalogProvider
	}
	if target.CatalogModel != "" {
		modelID = target.CatalogModel
	}
	return g.catalog.Lookup(providerID, modelID)
}

func readBody(w http.ResponseWriter, r *http.Request, maxBytes int64) ([]byte, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "The request body could not be read.", "invalid_request_error", "invalid_body")
		return nil, false
	}
	if int64(len(body)) > maxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "The request body exceeds the configured limit.", "invalid_request_error", "request_too_large")
		return nil, false
	}
	return body, true
}

func validateChatRequest(w http.ResponseWriter, body []byte) (string, bool, bool) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil || fields == nil {
		writeError(w, http.StatusBadRequest, "The request body must be a JSON object.", "invalid_request_error", "invalid_json")
		return "", false, false
	}

	modelField, ok := fields["model"]
	if !ok {
		writeError(w, http.StatusBadRequest, "The model field is required.", "invalid_request_error", "missing_model")
		return "", false, false
	}
	var model string
	if err := json.Unmarshal(modelField, &model); err != nil {
		writeError(w, http.StatusBadRequest, "The model field must be a string.", "invalid_request_error", "invalid_model")
		return "", false, false
	}
	if _, ok := fields["messages"]; !ok {
		writeError(w, http.StatusBadRequest, "The messages field is required.", "invalid_request_error", "missing_messages")
		return "", false, false
	}

	if nField, ok := fields["n"]; ok {
		var n int
		if err := json.Unmarshal(nField, &n); err != nil || n != 1 {
			writeError(w, http.StatusBadRequest, "The n field must be 1 when provided.", "invalid_request_error", "unsupported_n")
			return "", false, false
		}
	}

	var stream bool
	if streamField, ok := fields["stream"]; ok {
		if err := json.Unmarshal(streamField, &stream); err != nil {
			writeError(w, http.StatusBadRequest, "The stream field must be a boolean.", "invalid_request_error", "invalid_stream")
			return "", false, false
		}
	}

	return model, stream, true
}

func isRetryable(status int) bool {
	switch status {
	case http.StatusRequestTimeout,
		http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func relayResponse(w http.ResponseWriter, response *http.Response, stream bool) {
	defer response.Body.Close()
	relayHeaders(w.Header(), response.Header)
	w.WriteHeader(response.StatusCode)
	if !stream {
		_, _ = io.Copy(w, response.Body)
		return
	}

	controller := http.NewResponseController(w)
	buffer := make([]byte, streamBufferSize)
	for {
		read, err := response.Body.Read(buffer)
		if read > 0 {
			if _, writeErr := w.Write(buffer[:read]); writeErr != nil {
				return
			}
			_ = controller.Flush()
		}
		if err != nil {
			return
		}
	}
}

func relayHeaders(destination, source http.Header) {
	for _, name := range []string{"Content-Type", "Cache-Control", "Retry-After"} {
		copyHeader(destination, source, name, name)
	}
	for name := range source {
		if strings.HasPrefix(strings.ToLower(name), "x-ratelimit-") {
			copyHeader(destination, source, name, name)
		}
	}
	copyHeader(destination, source, "X-Request-Id", "X-Upstream-Request-Id")
}

func copyHeader(destination, source http.Header, sourceName, destinationName string) {
	values := source.Values(sourceName)
	if len(values) == 0 {
		return
	}
	destination.Del(destinationName)
	for _, value := range values {
		destination.Add(destinationName, value)
	}
}

type openAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   any    `json:"param"`
		Code    string `json:"code"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, message, errorType, code string) {
	payload := openAIError{}
	payload.Error.Message = message
	payload.Error.Type = errorType
	payload.Error.Code = code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
