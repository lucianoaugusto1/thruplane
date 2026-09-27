package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httptrace"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lucianoaugusto1/thruplane/internal/config"
	"github.com/lucianoaugusto1/thruplane/internal/gateway"
	"github.com/lucianoaugusto1/thruplane/internal/httpapi"
	"github.com/lucianoaugusto1/thruplane/internal/provider"
)

const directRequestHeader = "X-Thruplane-Benchmark-Direct"

type loadConfig struct {
	Requests    int `json:"requests"`
	Concurrency int `json:"concurrency"`
	Warmup      int `json:"warmup"`
}

type systemMetadata struct {
	GoVersion   string `json:"go_version"`
	GOOS        string `json:"goos"`
	GOARCH      string `json:"goarch"`
	LogicalCPUs int    `json:"logical_cpus"`
}

type phaseReport struct {
	Completed          int               `json:"completed"`
	ExpectedFailures   int               `json:"expected_failures"`
	UnexpectedFailures int               `json:"unexpected_failures"`
	WallTimeMS         float64           `json:"wall_time_ms"`
	RequestsPerSecond  float64           `json:"requests_per_second"`
	Latency            durationSummary   `json:"latency"`
	TTFT               durationSummary   `json:"ttft,omitempty"`
	ClientConnections  connectionSummary `json:"client_connections"`
	Memory             memorySummary     `json:"memory"`
}

type upstreamReport struct {
	Requests       int64             `json:"requests"`
	ErrorResponses int64             `json:"error_responses"`
	Canceled       int64             `json:"canceled"`
	Connections    connectionSummary `json:"connections"`
}

type benchmarkReport struct {
	Scenario     string          `json:"scenario"`
	System       systemMetadata  `json:"system"`
	Load         loadConfig      `json:"load"`
	PayloadBytes int             `json:"payload_bytes"`
	MediaBytes   int             `json:"media_bytes,omitempty"`
	Direct       phaseReport     `json:"direct"`
	Gateway      phaseReport     `json:"gateway"`
	AddedLatency durationSummary `json:"added_latency"`
	AddedTTFT    durationSummary `json:"added_ttft,omitempty"`
	Upstream     upstreamReport  `json:"upstream"`
}

func runScenario(ctx context.Context, scenario performanceScenario, cfg loadConfig) (benchmarkReport, error) {
	if cfg.Requests <= 0 {
		return benchmarkReport{}, errors.New("requests must be greater than zero")
	}
	if cfg.Concurrency <= 0 {
		return benchmarkReport{}, errors.New("concurrency must be greater than zero")
	}
	if cfg.Warmup < 0 {
		return benchmarkReport{}, errors.New("warmup must not be negative")
	}

	environment, err := newBenchmarkEnvironment(scenario)
	if err != nil {
		return benchmarkReport{}, err
	}
	defer environment.Close()

	directClient := newLoadClient(cfg.Concurrency)
	defer directClient.CloseIdleConnections()
	gatewayClient := newLoadClient(cfg.Concurrency)
	defer gatewayClient.CloseIdleConnections()
	if scenario.ExpectCancellation {
		gatewayClient.Transport.(*http.Transport).DisableKeepAlives = true
	}

	if cfg.Warmup > 0 {
		if _, err := runPhase(ctx, directClient, environment.directURL(), scenario, loadConfig{Requests: cfg.Warmup, Concurrency: cfg.Concurrency}, true, 1); err != nil {
			return benchmarkReport{}, fmt.Errorf("direct warmup: %w", err)
		}
	}
	direct, err := runPhase(ctx, directClient, environment.directURL(), scenario, cfg, true, 1_000_000)
	if err != nil {
		return benchmarkReport{}, fmt.Errorf("direct phase: %w", err)
	}

	if cfg.Warmup > 0 {
		if _, err := runPhase(ctx, gatewayClient, environment.gateway.URL, scenario, loadConfig{Requests: cfg.Warmup, Concurrency: cfg.Concurrency}, false, 2_000_000); err != nil {
			return benchmarkReport{}, fmt.Errorf("gateway warmup: %w", err)
		}
		if scenario.ExpectCancellation {
			environment.waitForCancellations(int64(cfg.Warmup), 250*time.Millisecond)
		}
	}
	beforeUpstream := environment.snapshot()
	gatewayPhase, err := runPhase(ctx, gatewayClient, environment.gateway.URL, scenario, cfg, false, 3_000_000)
	if err != nil {
		return benchmarkReport{}, fmt.Errorf("gateway phase: %w", err)
	}
	if scenario.ExpectCancellation {
		environment.waitForCancellations(beforeUpstream.canceled+int64(cfg.Requests), 250*time.Millisecond)
	}
	afterUpstream := environment.snapshot()
	upstream := afterUpstream.subtract(beforeUpstream)

	report := benchmarkReport{
		Scenario:     scenario.Name,
		System:       currentSystemMetadata(),
		Load:         cfg,
		PayloadBytes: len(scenario.requestBody(0)),
		MediaBytes:   scenario.MediaBytes,
		Direct:       direct,
		Gateway:      gatewayPhase,
		AddedLatency: subtractDurations(gatewayPhase.Latency, direct.Latency),
		AddedTTFT:    subtractDurations(gatewayPhase.TTFT, direct.TTFT),
		Upstream: upstreamReport{
			Requests:       upstream.requests,
			ErrorResponses: upstream.errorResponses,
			Canceled:       upstream.canceled,
			Connections:    inferUpstreamConnections(upstream.requests, upstream.newConnections),
		},
	}
	return report, nil
}

func currentSystemMetadata() systemMetadata {
	return systemMetadata{
		GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH,
		LogicalCPUs: runtime.NumCPU(),
	}
}

type requestMeasurement struct {
	latency           time.Duration
	ttft              time.Duration
	completed         bool
	expectedFailure   bool
	unexpectedFailure bool
}

type connectionCounter struct {
	new    atomic.Int64
	reused atomic.Int64
}

func (c *connectionCounter) trace() *httptrace.ClientTrace {
	return &httptrace.ClientTrace{GotConn: func(info httptrace.GotConnInfo) {
		if info.Reused {
			c.reused.Add(1)
			return
		}
		c.new.Add(1)
	}}
}

func runPhase(ctx context.Context, client *http.Client, gatewayURL string, scenario performanceScenario, cfg loadConfig, direct bool, startID uint64) (phaseReport, error) {
	measurements := make([]requestMeasurement, cfg.Requests)
	connections := &connectionCounter{}
	var next atomic.Int64
	beforeMemory := readMemorySample()
	started := time.Now()

	workers := cfg.Concurrency
	if workers > cfg.Requests {
		workers = cfg.Requests
	}
	var group sync.WaitGroup
	group.Add(workers)
	for worker := 0; worker < workers; worker++ {
		go func() {
			defer group.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= cfg.Requests {
					return
				}
				measurements[index] = performRequest(ctx, client, gatewayURL, scenario, direct, startID+uint64(index), connections)
			}
		}()
	}
	group.Wait()
	wallTime := time.Since(started)
	afterMemory := readMemorySample()
	if err := ctx.Err(); err != nil {
		return phaseReport{}, err
	}

	latencies := make([]time.Duration, 0, cfg.Requests)
	ttfts := make([]time.Duration, 0, cfg.Requests)
	report := phaseReport{WallTimeMS: milliseconds(wallTime), Memory: subtractMemory(afterMemory, beforeMemory)}
	for _, measurement := range measurements {
		latencies = append(latencies, measurement.latency)
		if measurement.ttft > 0 {
			ttfts = append(ttfts, measurement.ttft)
		}
		if measurement.completed {
			report.Completed++
		}
		if measurement.expectedFailure {
			report.ExpectedFailures++
		}
		if measurement.unexpectedFailure {
			report.UnexpectedFailures++
		}
	}
	report.Latency = summarizeDurations(latencies)
	report.TTFT = summarizeDurations(ttfts)
	if wallTime > 0 {
		report.RequestsPerSecond = float64(cfg.Requests) / wallTime.Seconds()
	}
	newConnections := connections.new.Load()
	reusedConnections := connections.reused.Load()
	report.ClientConnections = summarizeConnections(newConnections+reusedConnections, newConnections, reusedConnections)
	return report, nil
}

func performRequest(parent context.Context, client *http.Client, gatewayURL string, scenario performanceScenario, direct bool, id uint64, connections *connectionCounter) requestMeasurement {
	ctx := parent
	var cancel context.CancelFunc
	if !direct && scenario.ExpectCancellation {
		ctx, cancel = context.WithTimeout(parent, scenario.CancelAfter)
		defer cancel()
	}
	ctx = httptrace.WithClientTrace(ctx, connections.trace())

	url := gatewayURL + "/v1/chat/completions"
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(scenario.requestBody(id)))
	if err != nil {
		return requestMeasurement{unexpectedFailure: true}
	}
	request.Header.Set("Content-Type", "application/json")
	if direct {
		request.URL.Path = "/direct"
		request.Header.Set(directRequestHeader, "1")
	}

	started := time.Now()
	response, err := client.Do(request)
	if err != nil {
		measurement := requestMeasurement{latency: time.Since(started)}
		if !direct && scenario.ExpectCancellation && ctx.Err() != nil {
			measurement.expectedFailure = true
		} else {
			measurement.unexpectedFailure = true
		}
		return measurement
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		_, _ = io.Copy(io.Discard, response.Body)
		return requestMeasurement{latency: time.Since(started), unexpectedFailure: true}
	}

	measurement := requestMeasurement{}
	if scenario.Stream {
		first := make([]byte, 1)
		read, readErr := response.Body.Read(first)
		if read > 0 {
			measurement.ttft = time.Since(started)
		}
		if readErr != nil && readErr != io.EOF {
			measurement.latency = time.Since(started)
			measurement.unexpectedFailure = true
			return measurement
		}
	}
	if err := drainResponse(response.Body, scenario.SlowReadDelay); err != nil {
		measurement.latency = time.Since(started)
		measurement.unexpectedFailure = true
		return measurement
	}
	measurement.latency = time.Since(started)
	measurement.completed = true
	return measurement
}

func drainResponse(body io.Reader, delay time.Duration) error {
	bufferSize := 32 * 1024
	if delay > 0 {
		bufferSize = 32
	}
	buffer := make([]byte, bufferSize)
	for {
		read, err := body.Read(buffer)
		if read > 0 && delay > 0 {
			time.Sleep(delay)
		}
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

func readMemorySample() memorySample {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return memorySample{
		TotalAlloc: stats.TotalAlloc, Mallocs: stats.Mallocs,
		HeapAlloc: stats.HeapAlloc, HeapObjects: stats.HeapObjects, NumGC: stats.NumGC,
	}
}

func newLoadClient(concurrency int) *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxIdleConns = concurrency * 2
	transport.MaxIdleConnsPerHost = concurrency
	transport.MaxConnsPerHost = concurrency
	transport.ForceAttemptHTTP2 = true
	return &http.Client{Transport: transport}
}

type upstreamSnapshot struct {
	requests       int64
	newConnections int64
	errorResponses int64
	canceled       int64
}

func (s upstreamSnapshot) subtract(before upstreamSnapshot) upstreamSnapshot {
	return upstreamSnapshot{
		requests:       s.requests - before.requests,
		newConnections: s.newConnections - before.newConnections,
		errorResponses: s.errorResponses - before.errorResponses,
		canceled:       s.canceled - before.canceled,
	}
}

type trackedUpstream struct {
	scenario       performanceScenario
	target         targetScenario
	server         *httptest.Server
	requests       atomic.Int64
	newConnections atomic.Int64
	errorResponses atomic.Int64
	canceled       atomic.Int64
	mu             sync.Mutex
	attempts       map[string]int
}

func newTrackedUpstream(scenario performanceScenario, target targetScenario) *trackedUpstream {
	upstream := &trackedUpstream{scenario: scenario, target: target, attempts: make(map[string]int)}
	server := httptest.NewUnstartedServer(http.HandlerFunc(upstream.serveHTTP))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			upstream.newConnections.Add(1)
		}
	}
	server.Start()
	upstream.server = server
	return upstream
}

func (u *trackedUpstream) serveHTTP(w http.ResponseWriter, r *http.Request) {
	u.requests.Add(1)
	if r.Header.Get(directRequestHeader) == "1" {
		writeSuccessfulResponse(w, r, u.scenario, u.target)
		return
	}

	switch u.target.Mode {
	case upstreamRetryOnce:
		var payload struct {
			BenchmarkID string `json:"benchmark_id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		u.mu.Lock()
		attempt := u.attempts[payload.BenchmarkID]
		u.attempts[payload.BenchmarkID] = attempt + 1
		u.mu.Unlock()
		if attempt == 0 {
			u.writeError(w, http.StatusServiceUnavailable, `{"error":{"code":"temporary_unavailable"}}`)
			return
		}
	case upstreamUnavailable:
		u.writeError(w, http.StatusServiceUnavailable, `{"error":{"code":"temporary_unavailable"}}`)
		return
	case upstreamPermanentRateLimit:
		u.writeError(w, http.StatusTooManyRequests, `{"error":{"code":"insufficient_quota"}}`)
		return
	case upstreamDelayed:
		if !waitForRequest(r, 25*time.Millisecond) {
			u.canceled.Add(1)
			return
		}
		if u.writeDelayedResponse(w) {
			u.canceled.Add(1)
		}
		return
	}
	writeSuccessfulResponse(w, r, u.scenario, u.target)
}

func (u *trackedUpstream) writeDelayedResponse(w http.ResponseWriter) bool {
	w.Header().Set("Content-Type", "application/json")
	flusher, _ := w.(http.Flusher)
	chunk := bytes.Repeat([]byte("x"), 32<<10)
	for index := 0; index < 128; index++ {
		if _, err := w.Write(chunk); err != nil {
			return true
		}
		if flusher != nil {
			flusher.Flush()
		}
	}
	return false
}

func (u *trackedUpstream) writeError(w http.ResponseWriter, status int, body string) {
	u.errorResponses.Add(1)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func (u *trackedUpstream) snapshot() upstreamSnapshot {
	return upstreamSnapshot{
		requests: u.requests.Load(), newConnections: u.newConnections.Load(),
		errorResponses: u.errorResponses.Load(), canceled: u.canceled.Load(),
	}
}

type benchmarkEnvironment struct {
	scenario  performanceScenario
	upstreams []*trackedUpstream
	gateway   *httptest.Server
}

func newBenchmarkEnvironment(scenario performanceScenario) (*benchmarkEnvironment, error) {
	environment := &benchmarkEnvironment{scenario: scenario}
	providers := make(map[string]config.ProviderConfig, len(scenario.Targets))
	targets := make([]config.TargetConfig, 0, len(scenario.Targets))
	for _, target := range scenario.Targets {
		upstream := newTrackedUpstream(scenario, target)
		environment.upstreams = append(environment.upstreams, upstream)
		providers[target.Name] = config.ProviderConfig{
			Type: target.ProviderType, BaseURL: upstream.server.URL, APIKey: "benchmark-key",
		}
		targets = append(targets, config.TargetConfig{Provider: target.Name, Model: target.Model})
	}

	cfg := config.Config{
		Server:    config.ServerConfig{MaxBodyBytes: 4 << 20},
		Catalog:   config.CatalogConfig{UnknownModels: "allow"},
		Providers: providers,
		Models:    map[string]config.ModelConfig{benchmarkModelAlias: {Targets: targets}},
		Routing: config.RoutingConfig{
			Retries: scenario.Retries, ResponseHeaderTimeout: config.Duration(time.Second),
			Retry: config.RetryConfig{
				BaseDelay: config.Duration(250 * time.Microsecond),
				MaxDelay:  config.Duration(time.Millisecond),
				Budget:    config.Duration(250 * time.Millisecond),
			},
		},
	}
	clients, err := provider.NewClients(cfg)
	if err != nil {
		environment.Close()
		return nil, fmt.Errorf("create provider clients: %w", err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	environment.gateway = httptest.NewServer(httpapi.New(cfg, gateway.New(cfg, clients), logger))
	return environment, nil
}

func (e *benchmarkEnvironment) directURL() string {
	return e.upstreams[len(e.upstreams)-1].server.URL
}

func (e *benchmarkEnvironment) snapshot() upstreamSnapshot {
	var result upstreamSnapshot
	for _, upstream := range e.upstreams {
		snapshot := upstream.snapshot()
		result.requests += snapshot.requests
		result.newConnections += snapshot.newConnections
		result.errorResponses += snapshot.errorResponses
		result.canceled += snapshot.canceled
	}
	return result
}

func (e *benchmarkEnvironment) waitForCancellations(want int64, timeout time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if e.snapshot().canceled >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
}

func (e *benchmarkEnvironment) Close() {
	if e.gateway != nil {
		e.gateway.Close()
	}
	for _, upstream := range e.upstreams {
		upstream.server.Close()
	}
}
