package httpapi

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"nexoroute/internal/config"
	"nexoroute/internal/gateway"
	"nexoroute/internal/telemetry"
)

type requestIDKey struct{}

type readinessResponse struct {
	Status  string           `json:"status"`
	Targets readinessTargets `json:"targets"`
}

type readinessTargets struct {
	Total     int `json:"total"`
	Available int `json:"available"`
	Open      int `json:"open"`
	HalfOpen  int `json:"half_open"`
}

func New(cfg config.Config, gateway *gateway.Gateway, logger *slog.Logger) http.Handler {
	return NewWithBuildInfo(cfg, gateway, logger, telemetry.BuildInfo{})
}

func NewWithBuildInfo(cfg config.Config, gatewayService *gateway.Gateway, logger *slog.Logger, build telemetry.BuildInfo) http.Handler {
	if logger == nil {
		logger = slog.Default()
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", health)
	mux.HandleFunc("GET /readyz", readiness(gatewayService))
	mux.HandleFunc("POST /v1/chat/completions", gatewayService.ChatCompletions)
	mux.HandleFunc("GET /v1/models", gatewayService.ListModels)
	mux.HandleFunc("GET /v1/models/{model}", gatewayService.GetModel)
	if cfg.Server.Playground.Enabled {
		mux.HandleFunc("GET /playground", redirectPlayground)
		mux.HandleFunc("GET /playground/", servePlayground)
	}

	var metrics *telemetry.Metrics
	if cfg.Server.Metrics.Enabled {
		metrics = telemetry.New(build, func() telemetry.Readiness {
			status := gateway.ReadinessStatus{}
			if gatewayService != nil {
				status = gatewayService.Readiness()
			}
			return telemetry.Readiness{
				Total: status.Total, Available: status.Available,
				Open: status.Open, HalfOpen: status.HalfOpen,
			}
		})
		mux.Handle("GET /metrics", metrics)
	}

	var handler http.Handler = mux
	handler = authenticate(cfg.Server.APIKey, handler)
	handler = recoverPanics(logger, handler)
	if metrics != nil {
		handler = observeMetrics(metrics, handler)
	}
	handler = accessLog(logger, handler)
	handler = assignRequestID(handler)
	return handler
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readiness(gatewayService *gateway.Gateway) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		status := gateway.ReadinessStatus{}
		if gatewayService != nil {
			status = gatewayService.Readiness()
		}
		httpStatus := http.StatusOK
		state := "ready"
		if !status.Ready {
			httpStatus = http.StatusServiceUnavailable
			state = "not_ready"
		}
		writeJSON(w, httpStatus, readinessResponse{
			Status: state,
			Targets: readinessTargets{
				Total: status.Total, Available: status.Available,
				Open: status.Open, HalfOpen: status.HalfOpen,
			},
		})
	}
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

func observeMetrics(metrics *telemetry.Metrics, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := metricRoute(r.URL.Path)
		if route == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		started := time.Now()
		recorder := &responseRecorder{ResponseWriter: w}
		metrics.RequestStarted()
		defer metrics.RequestFinished()
		next.ServeHTTP(recorder, r)
		metrics.ObserveHTTPRequest(r.Method, route, recorder.statusCode(), time.Since(started))

		attempts, _ := strconv.Atoi(recorder.Header().Get("X-NexoRoute-Attempts"))
		fallbacks, _ := strconv.Atoi(recorder.Header().Get("X-NexoRoute-Fallbacks"))
		metrics.ObserveRoute(
			recorder.Header().Get("X-NexoRoute-Provider"),
			recorder.Header().Get("X-NexoRoute-Model"),
			attempts,
			fallbacks,
		)
	})
}

func metricRoute(path string) string {
	switch path {
	case "/healthz", "/readyz", "/metrics", "/playground", "/v1/chat/completions", "/v1/models":
		return path
	}
	if strings.HasPrefix(path, "/v1/models/") {
		return "/v1/models/{model}"
	}
	if strings.HasPrefix(path, "/playground/") {
		return "/playground/{asset}"
	}
	return "unmatched"
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
