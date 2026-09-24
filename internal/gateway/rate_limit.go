package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"nexoroute/internal/config"
	"nexoroute/internal/ratelimit"
)

const errorBodyInspectionLimit = 64 << 10

type targetKey struct {
	provider string
	model    string
}

type retryDecision struct {
	retryTarget bool
	fallback    bool
}

func buildLimiters(cfg config.Config, now func() time.Time) map[targetKey]*ratelimit.Limiter {
	limiters := make(map[targetKey]*ratelimit.Limiter)
	for _, model := range cfg.Models {
		for _, target := range model.Targets {
			key := targetKey{provider: target.Provider, model: target.Model}
			if _, exists := limiters[key]; exists {
				continue
			}
			limiters[key] = ratelimit.NewWithClock(ratelimit.Policy{
				RequestsPerMinute: target.RateLimit.RequestsPerMinute,
				Burst:             target.RateLimit.Burst,
				MaxConcurrency:    target.RateLimit.MaxConcurrency,
			}, now)
		}
	}
	return limiters
}

func classifyResponse(providerType string, response *http.Response) retryDecision {
	switch response.StatusCode {
	case http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return retryDecision{retryTarget: true, fallback: true}
	case http.StatusTooManyRequests:
		body, err := peekResponseBody(response, errorBodyInspectionLimit)
		if err == nil && permanentRateLimitError(providerType, body) {
			return retryDecision{fallback: true}
		}
		return retryDecision{retryTarget: true, fallback: true}
	default:
		return retryDecision{}
	}
}

func permanentRateLimitError(providerType string, body []byte) bool {
	var payload any
	if json.Unmarshal(body, &payload) != nil {
		return false
	}
	values := make(map[string]struct{})
	collectErrorValues(payload, values)

	var permanent []string
	switch strings.ToLower(providerType) {
	case "openai", "azure-openai", "openai-compatible", "nexoroute-inference", "xai", "ollama":
		permanent = []string{
			"insufficient_quota",
			"billing_hard_limit_reached",
			"organization_usage_limit_exceeded",
			"project_usage_limit_exceeded",
			"billing_not_active",
			"spend_limit_reached",
		}
	case "anthropic":
		permanent = []string{"enforced_spend_limit_reached", "billing_error"}
	case "gemini", "vertex":
		permanent = []string{"quota_exceeded", "daily_quota_exceeded"}
	}
	for _, code := range permanent {
		if _, exists := values[code]; exists {
			return true
		}
	}
	return false
}

func collectErrorValues(value any, values map[string]struct{}) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			switch strings.ToLower(key) {
			case "code", "type", "reason", "status", "error_code":
				if text, ok := child.(string); ok {
					values[strings.ToLower(text)] = struct{}{}
				}
			}
			collectErrorValues(child, values)
		}
	case []any:
		for _, child := range typed {
			collectErrorValues(child, values)
		}
	}
}

func peekResponseBody(response *http.Response, limit int64) ([]byte, error) {
	original := response.Body
	prefix, err := io.ReadAll(io.LimitReader(original, limit))
	response.Body = &replayReadCloser{
		Reader: io.MultiReader(bytes.NewReader(prefix), original),
		closer: original,
	}
	return prefix, err
}

type replayReadCloser struct {
	io.Reader
	closer io.Closer
}

func (r *replayReadCloser) Close() error {
	return r.closer.Close()
}

func retryDelay(cfg config.RetryConfig, attempt int, response *http.Response, now time.Time, jitter func(time.Duration) time.Duration) time.Duration {
	if response != nil {
		if delay, ok := ratelimit.ParseRetryAfter(response.Header.Get("Retry-After"), now); ok {
			return delay
		}
	}
	base := time.Duration(cfg.BaseDelay)
	if base <= 0 {
		return 0
	}
	delay := base
	for i := 0; i < attempt; i++ {
		if delay > time.Duration(cfg.MaxDelay)/2 {
			delay = time.Duration(cfg.MaxDelay)
			break
		}
		delay *= 2
	}
	if maxDelay := time.Duration(cfg.MaxDelay); maxDelay > 0 && delay > maxDelay {
		delay = maxDelay
	}
	return jitter(delay)
}

func equalJitter(delay time.Duration) time.Duration {
	if delay <= 1 {
		return delay
	}
	half := delay / 2
	return half + time.Duration(rand.Int64N(int64(delay-half)+1))
}

func waitContext(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func retryFitsBudget(start, now time.Time, budget, delay time.Duration) bool {
	remaining, limited := remainingRetryBudget(start, now, budget)
	if !limited {
		return true
	}
	return remaining > 0 && delay < remaining
}

func remainingRetryBudget(start, now time.Time, budget time.Duration) (time.Duration, bool) {
	if budget <= 0 || start.IsZero() {
		return 0, false
	}
	return budget - now.Sub(start), true
}

func wrapPermit(response *http.Response, permit *ratelimit.Permit) {
	response.Body = &permitReadCloser{ReadCloser: response.Body, permit: permit}
}

type permitReadCloser struct {
	io.ReadCloser
	permit *ratelimit.Permit
	once   sync.Once
}

func (r *permitReadCloser) Read(buffer []byte) (int, error) {
	read, err := r.ReadCloser.Read(buffer)
	if err == io.EOF {
		r.release()
	}
	return read, err
}

func (r *permitReadCloser) Close() error {
	err := r.ReadCloser.Close()
	r.release()
	return err
}

func (r *permitReadCloser) release() {
	r.once.Do(r.permit.Release)
}

func drainAndClose(response *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, errorBodyInspectionLimit))
	_ = response.Body.Close()
}

func retryAfterSeconds(delay time.Duration) string {
	if delay <= 0 {
		return ""
	}
	seconds := int64(math.Ceil(delay.Seconds()))
	if seconds < 1 {
		seconds = 1
	}
	return strconv.FormatInt(seconds, 10)
}
