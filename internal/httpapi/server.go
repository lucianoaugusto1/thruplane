package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"nexoroute/internal/config"
	"nexoroute/internal/gateway"
)

type requestIDKey struct{}

func New(cfg config.Config, gateway *gateway.Gateway, logger *slog.Logger) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("POST /v1/chat/completions", gateway.ChatCompletions)
	mux.HandleFunc("GET /v1/models", gateway.ListModels)
	mux.HandleFunc("GET /v1/models/{model}", gateway.GetModel)
	if cfg.Server.Playground.Enabled {
		mux.HandleFunc("GET /playground", redirectPlayground)
		mux.HandleFunc("GET /playground/", servePlayground)
	}

	var handler http.Handler = mux
	handler = authenticate(cfg.Server.APIKey, handler)
	handler = recoverPanics(logger, handler)
	handler = accessLog(logger, handler)
	handler = assignRequestID(handler)
	return handler
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func assignRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := newRequestID()
		w.Header().Set("X-Request-Id", requestID)
		ctx := context.WithValue(r.Context(), requestIDKey{}, requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func authenticate(apiKey string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if apiKey == "" || !strings.HasPrefix(r.URL.Path, "/v1/") {
			next.ServeHTTP(w, r)
			return
		}

		scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
		valid := ok && strings.EqualFold(scheme, "Bearer") && token != "" &&
			subtle.ConstantTimeCompare([]byte(token), []byte(apiKey)) == 1
		if !valid {
			w.Header().Set("WWW-Authenticate", "Bearer")
			writeError(w, http.StatusUnauthorized, "Invalid or missing API key.", "authentication_error", "invalid_api_key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func recoverPanics(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recover() == nil {
				return
			}
			logger.Error("request panic recovered",
				"request_id", requestID(r.Context()),
				"method", r.Method,
				"path", r.URL.Path,
			)
			if recorder, ok := w.(*responseRecorder); ok && recorder.wroteHeader {
				return
			}
			writeError(w, http.StatusInternalServerError, "The gateway encountered an internal error.", "api_error", "internal_error")
		}()
		next.ServeHTTP(w, r)
	})
}

func accessLog(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		next.ServeHTTP(recorder, r)
		logger.Info("request completed",
			"request_id", requestID(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"status", recorder.statusCode(),
			"duration_ms", float64(time.Since(started).Microseconds())/1000,
		)
	})
}

type responseRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *responseRecorder) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	w.status = status
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseRecorder) Write(body []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(body)
}

func (w *responseRecorder) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

func (w *responseRecorder) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}

func requestID(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey{}).(string)
	return value
}

func newRequestID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "req-unknown"
	}
	return "req-" + hex.EncodeToString(bytes)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message, errorType, code string) {
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"message": message,
			"type":    errorType,
			"param":   nil,
			"code":    code,
		},
	})
}
