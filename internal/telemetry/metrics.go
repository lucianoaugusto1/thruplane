package telemetry

import (
	"bytes"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const contentType = "text/plain; version=0.0.4; charset=utf-8"

var (
	durationBuckets = []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}
	attemptBuckets  = []float64{1, 2, 3, 4, 5, 8}
	fallbackBuckets = []float64{0, 1, 2, 3, 4}
)

type BuildInfo struct {
	Version  string
	Revision string
	Date     string
}

type Readiness struct {
	Total     int
	Available int
	Open      int
	HalfOpen  int
}

type Metrics struct {
	mu        sync.Mutex
	build     BuildInfo
	readiness func() Readiness
	inFlight  int64

	httpRequests map[httpKey]uint64
	httpDuration map[httpRoute]*histogram
	routes       map[routeKey]uint64
	attempts     map[routeKey]*histogram
	fallbacks    map[routeKey]*histogram
}

type httpKey struct {
	method string
	route  string
	status int
}

type httpRoute struct {
	method string
	route  string
}

type routeKey struct {
	provider string
	model    string
}

type histogram struct {
	buckets []uint64
	count   uint64
	sum     float64
}

func New(build BuildInfo, readiness func() Readiness) *Metrics {
	if build.Version == "" {
		build.Version = "dev"
	}
	if build.Revision == "" {
		build.Revision = "unknown"
	}
	if build.Date == "" {
		build.Date = "unknown"
	}
	return &Metrics{
		build:        build,
		readiness:    readiness,
		httpRequests: make(map[httpKey]uint64),
		httpDuration: make(map[httpRoute]*histogram),
		routes:       make(map[routeKey]uint64),
		attempts:     make(map[routeKey]*histogram),
		fallbacks:    make(map[routeKey]*histogram),
	}
}

func (m *Metrics) RequestStarted() {
	m.mu.Lock()
	m.inFlight++
	m.mu.Unlock()
}

func (m *Metrics) RequestFinished() {
	m.mu.Lock()
	if m.inFlight > 0 {
		m.inFlight--
	}
	m.mu.Unlock()
}

func (m *Metrics) ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	if duration < 0 {
		duration = 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	m.httpRequests[httpKey{method: method, route: route, status: status}]++
	key := httpRoute{method: method, route: route}
	hist := m.httpDuration[key]
	if hist == nil {
		hist = newHistogram(durationBuckets)
		m.httpDuration[key] = hist
	}
	hist.observe(duration.Seconds(), durationBuckets)
}

func (m *Metrics) ObserveRoute(provider, model string, attempts, fallbacks int) {
	if provider == "" || model == "" {
		return
	}
	if attempts < 0 {
		attempts = 0
	}
	if fallbacks < 0 {
		fallbacks = 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	key := routeKey{provider: provider, model: model}
	m.routes[key]++
	observeHistogram(m.attempts, key, float64(attempts), attemptBuckets)
	observeHistogram(m.fallbacks, key, float64(fallbacks), fallbackBuckets)
}

func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	ready := Readiness{}
	if m.readiness != nil {
		ready = m.readiness()
	}
	body := m.render(ready)
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (m *Metrics) render(ready Readiness) []byte {
	m.mu.Lock()
	defer m.mu.Unlock()

	var output bytes.Buffer
	writeMetadata(&output, "nexoroute_build_info", "Build identity for this NexoRoute process.", "gauge")
	fmt.Fprintf(&output, "nexoroute_build_info%s 1\n", labels(
		"build_date", m.build.Date,
		"revision", m.build.Revision,
		"version", m.build.Version,
	))

	writeMetadata(&output, "nexoroute_http_requests_in_flight", "Current HTTP requests being handled.", "gauge")
	fmt.Fprintf(&output, "nexoroute_http_requests_in_flight %d\n", m.inFlight)

	writeMetadata(&output, "nexoroute_http_requests_total", "Completed HTTP requests by bounded route and status.", "counter")
	for _, key := range sortedHTTPKeys(m.httpRequests) {
		fmt.Fprintf(&output, "nexoroute_http_requests_total%s %d\n", labels(
			"method", key.method,
			"route", key.route,
			"status", strconv.Itoa(key.status),
		), m.httpRequests[key])
	}

	writeMetadata(&output, "nexoroute_http_request_duration_seconds", "End-to-end HTTP handler duration in seconds.", "histogram")
	for _, key := range sortedHTTPRoutes(m.httpDuration) {
		writeHistogram(&output, "nexoroute_http_request_duration_seconds", []string{
			"method", key.method,
			"route", key.route,
		}, m.httpDuration[key], durationBuckets)
	}

	writeMetadata(&output, "nexoroute_route_selections_total", "Completed requests by final selected provider and model.", "counter")
	for _, key := range sortedRouteKeys(m.routes) {
		fmt.Fprintf(&output, "nexoroute_route_selections_total%s %d\n", routeLabels(key), m.routes[key])
	}

	writeMetadata(&output, "nexoroute_request_attempts", "Upstream attempts per completed routed request, labeled by the final selected route.", "histogram")
	for _, key := range sortedRouteHistograms(m.attempts) {
		writeHistogram(&output, "nexoroute_request_attempts", []string{
			"model", key.model,
			"provider", key.provider,
		}, m.attempts[key], attemptBuckets)
	}

	writeMetadata(&output, "nexoroute_request_fallbacks", "Fallback count per completed routed request, labeled by the final selected route.", "histogram")
	for _, key := range sortedRouteHistograms(m.fallbacks) {
		writeHistogram(&output, "nexoroute_request_fallbacks", []string{
			"model", key.model,
			"provider", key.provider,
		}, m.fallbacks[key], fallbackBuckets)
	}

	writeMetadata(&output, "nexoroute_targets", "Configured upstream targets by aggregate readiness state.", "gauge")
	states := []struct {
		name  string
		value int
	}{
		{name: "available", value: ready.Available},
		{name: "half_open", value: ready.HalfOpen},
		{name: "open", value: ready.Open},
		{name: "total", value: ready.Total},
	}
	for _, state := range states {
		fmt.Fprintf(&output, "nexoroute_targets%s %d\n", labels("state", state.name), state.value)
	}
	return output.Bytes()
}

func newHistogram(bounds []float64) *histogram {
	return &histogram{buckets: make([]uint64, len(bounds))}
}

func observeHistogram(collection map[routeKey]*histogram, key routeKey, value float64, bounds []float64) {
	hist := collection[key]
	if hist == nil {
		hist = newHistogram(bounds)
		collection[key] = hist
	}
	hist.observe(value, bounds)
}

func (h *histogram) observe(value float64, bounds []float64) {
	h.count++
	h.sum += value
	for index, bound := range bounds {
		if value <= bound {
			h.buckets[index]++
		}
	}
}

func writeHistogram(output *bytes.Buffer, name string, baseLabels []string, hist *histogram, bounds []float64) {
	for index, bound := range bounds {
		withBucket := append(append([]string(nil), baseLabels...), "le", formatFloat(bound))
		fmt.Fprintf(output, "%s_bucket%s %d\n", name, labels(withBucket...), hist.buckets[index])
	}
	withInfinity := append(append([]string(nil), baseLabels...), "le", "+Inf")
	fmt.Fprintf(output, "%s_bucket%s %d\n", name, labels(withInfinity...), hist.count)
	fmt.Fprintf(output, "%s_sum%s %s\n", name, labels(baseLabels...), formatFloat(hist.sum))
	fmt.Fprintf(output, "%s_count%s %d\n", name, labels(baseLabels...), hist.count)
}

func writeMetadata(output *bytes.Buffer, name, help, metricType string) {
	fmt.Fprintf(output, "# HELP %s %s\n", name, help)
	fmt.Fprintf(output, "# TYPE %s %s\n", name, metricType)
}

func routeLabels(key routeKey) string {
	return labels("model", key.model, "provider", key.provider)
}

func labels(pairs ...string) string {
	if len(pairs) == 0 {
		return ""
	}
	var value strings.Builder
	value.WriteByte('{')
	for index := 0; index < len(pairs); index += 2 {
		if index > 0 {
			value.WriteByte(',')
		}
		value.WriteString(pairs[index])
		value.WriteString("=\"")
		value.WriteString(escapeLabelValue(pairs[index+1]))
		value.WriteByte('"')
	}
	value.WriteByte('}')
	return value.String()
}

func escapeLabelValue(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, "\n", `\n`)
	value = strings.ReplaceAll(value, "\r", `\r`)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'g', -1, 64)
}

func sortedHTTPKeys(values map[httpKey]uint64) []httpKey {
	keys := make([]httpKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		if keys[i].route != keys[j].route {
			return keys[i].route < keys[j].route
		}
		return keys[i].status < keys[j].status
	})
	return keys
}

func sortedHTTPRoutes(values map[httpRoute]*histogram) []httpRoute {
	keys := make([]httpRoute, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].method != keys[j].method {
			return keys[i].method < keys[j].method
		}
		return keys[i].route < keys[j].route
	})
	return keys
}

func sortedRouteKeys(values map[routeKey]uint64) []routeKey {
	keys := make([]routeKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortRouteKeys(keys)
	return keys
}

func sortedRouteHistograms(values map[routeKey]*histogram) []routeKey {
	keys := make([]routeKey, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sortRouteKeys(keys)
	return keys
}

func sortRouteKeys(keys []routeKey) {
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].model != keys[j].model {
			return keys[i].model < keys[j].model
		}
		return keys[i].provider < keys[j].provider
	})
}
