package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"nexoroute/internal/ratelimit"
)

const streamBufferSize = 32 * 1024

type gatewayError struct {
	status    int
	message   string
	errorType string
	code      string
}

func newGatewayError(status int, message, errorType, code string) *gatewayError {
	return &gatewayError{status: status, message: message, errorType: errorType, code: code}
}

func readBody(r *http.Request, maxBytes int64) ([]byte, *gatewayError) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes+1))
	if err != nil {
		return nil, newGatewayError(http.StatusBadRequest, "The request body could not be read.", "invalid_request_error", "invalid_body")
	}
	if int64(len(body)) > maxBytes {
		return nil, newGatewayError(http.StatusRequestEntityTooLarge, "The request body exceeds the configured limit.", "invalid_request_error", "request_too_large")
	}
	return body, nil
}

func writeGatewayError(w http.ResponseWriter, err *gatewayError) {
	writeError(w, err.status, err.message, err.errorType, err.code)
}

func writeRateLimitError(w http.ResponseWriter, denied *ratelimit.Denial) {
	if retryAfter := retryAfterSeconds(denied.RetryAfter); retryAfter != "" {
		w.Header().Set("Retry-After", retryAfter)
	}
	w.Header().Set("X-NexoRoute-RateLimit-Reason", denied.Reason)
	writeError(w, http.StatusTooManyRequests, "All eligible upstream targets are currently rate limited.", "rate_limit_error", "gateway_rate_limited")
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
		lower := strings.ToLower(name)
		if strings.HasPrefix(lower, "x-ratelimit-") || strings.HasPrefix(lower, "anthropic-ratelimit-") {
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
