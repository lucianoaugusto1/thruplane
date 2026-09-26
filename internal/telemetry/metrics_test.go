package telemetry

import (
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMetricsExposeCoreFamilies(t *testing.T) {
	metrics := New(BuildInfo{
		Version:  "v0.1.0-beta.1",
		Revision: "abc123",
		Date:     "2026-09-25T00:00:00Z",
	}, func() Readiness {
		return Readiness{Total: 3, Available: 2, Open: 1, HalfOpen: 0}
	})

	metrics.RequestStarted()
	metrics.ObserveHTTPRequest("POST", "/v1/chat/completions", 200, 25*time.Millisecond)
	metrics.ObserveRoute("primary", "model-a", 2, 1)

	recorder := httptest.NewRecorder()
	metrics.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))

	if recorder.Code != 200 {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	if got := recorder.Header().Get("Content-Type"); got != "text/plain; version=0.0.4; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
	assertContainsAll(t, recorder.Body.String(),
		`nexoroute_build_info{build_date="2026-09-25T00:00:00Z",revision="abc123",version="v0.1.0-beta.1"} 1`,
		`nexoroute_http_requests_in_flight 1`,
		`nexoroute_http_requests_total{method="POST",route="/v1/chat/completions",status="200"} 1`,
		`nexoroute_http_request_duration_seconds_count{method="POST",route="/v1/chat/completions"} 1`,
		`nexoroute_http_request_duration_seconds_sum{method="POST",route="/v1/chat/completions"} 0.025`,
		`nexoroute_route_selections_total{model="model-a",provider="primary"} 1`,
		`nexoroute_request_attempts_sum{model="model-a",provider="primary"} 2`,
		`nexoroute_request_fallbacks_sum{model="model-a",provider="primary"} 1`,
		`nexoroute_targets{state="available"} 2`,
		`nexoroute_targets{state="open"} 1`,
		`nexoroute_targets{state="total"} 3`,
	)

	metrics.RequestFinished()
	recorder = httptest.NewRecorder()
	metrics.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), "nexoroute_http_requests_in_flight 0") {
		t.Fatal("in-flight gauge did not return to zero")
	}
}

func TestMetricsEscapeLabelValues(t *testing.T) {
	metrics := New(BuildInfo{Version: "dev"}, nil)
	metrics.ObserveRoute("provider\\name", "line\n\"quoted\"", 1, 0)

	recorder := httptest.NewRecorder()
	metrics.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()
	if !strings.Contains(body, `model="line\n\"quoted\"",provider="provider\\name"`) {
		t.Fatalf("escaped labels missing from:\n%s", body)
	}
	if strings.Contains(body, "line\n\"quoted\"") {
		t.Fatal("metrics contain an unescaped newline or quote")
	}
}

func TestMetricsSupportConcurrentUpdates(t *testing.T) {
	metrics := New(BuildInfo{Version: "dev"}, nil)
	const workers = 16
	const iterations = 100

	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for range iterations {
				metrics.RequestStarted()
				metrics.ObserveHTTPRequest("GET", "/healthz", 200, time.Millisecond)
				metrics.ObserveRoute("local", "model-a", 1, 0)
				metrics.RequestFinished()
			}
		}()
	}
	group.Wait()

	recorder := httptest.NewRecorder()
	metrics.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	wantCount := " 1600\n"
	if !strings.Contains(recorder.Body.String(), `nexoroute_http_requests_total{method="GET",route="/healthz",status="200"}`+wantCount) {
		t.Fatalf("HTTP request count missing from:\n%s", recorder.Body.String())
	}
}

func assertContainsAll(t *testing.T, value string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(value, fragment) {
			t.Errorf("metrics output missing %q:\n%s", fragment, value)
		}
	}
}
