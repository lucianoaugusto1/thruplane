package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
)

const thresholdPolicySchemaVersion = 1

type thresholdPolicy struct {
	SchemaVersion int                           `json:"schema_version"`
	MinimumRuns   int                           `json:"minimum_runs"`
	Defaults      thresholdLimits               `json:"defaults"`
	Scenarios     map[string]thresholdOverrides `json:"scenarios,omitempty"`
}

type thresholdLimits struct {
	MaxAddedLatencyP95IncreasePercent      float64 `json:"max_added_latency_p95_increase_percent"`
	MaxAddedLatencyP95IncreaseMS           float64 `json:"max_added_latency_p95_increase_ms"`
	MaxAddedLatencyP99IncreasePercent      float64 `json:"max_added_latency_p99_increase_percent"`
	MaxAddedLatencyP99IncreaseMS           float64 `json:"max_added_latency_p99_increase_ms"`
	MaxTTFTP95IncreasePercent              float64 `json:"max_ttft_p95_increase_percent"`
	MaxTTFTP95IncreaseMS                   float64 `json:"max_ttft_p95_increase_ms"`
	MaxRPSDecreasePercent                  float64 `json:"max_rps_decrease_percent"`
	MaxAllocBytesPerRequestIncreasePercent float64 `json:"max_alloc_bytes_per_request_increase_percent"`
	MaxAllocsPerRequestIncreasePercent     float64 `json:"max_allocs_per_request_increase_percent"`
	MinClientConnectionReusePercent        float64 `json:"min_client_connection_reuse_percent"`
	MinUpstreamConnectionReusePercent      float64 `json:"min_upstream_connection_reuse_percent"`
	MaxConnectionReuseDecreasePoints       float64 `json:"max_connection_reuse_decrease_points"`
	MaxUnexpectedFailures                  int     `json:"max_unexpected_failures"`
}

type thresholdOverrides struct {
	MaxAddedLatencyP95IncreasePercent      *float64 `json:"max_added_latency_p95_increase_percent,omitempty"`
	MaxAddedLatencyP95IncreaseMS           *float64 `json:"max_added_latency_p95_increase_ms,omitempty"`
	MaxAddedLatencyP99IncreasePercent      *float64 `json:"max_added_latency_p99_increase_percent,omitempty"`
	MaxAddedLatencyP99IncreaseMS           *float64 `json:"max_added_latency_p99_increase_ms,omitempty"`
	MaxTTFTP95IncreasePercent              *float64 `json:"max_ttft_p95_increase_percent,omitempty"`
	MaxTTFTP95IncreaseMS                   *float64 `json:"max_ttft_p95_increase_ms,omitempty"`
	MaxRPSDecreasePercent                  *float64 `json:"max_rps_decrease_percent,omitempty"`
	MaxAllocBytesPerRequestIncreasePercent *float64 `json:"max_alloc_bytes_per_request_increase_percent,omitempty"`
	MaxAllocsPerRequestIncreasePercent     *float64 `json:"max_allocs_per_request_increase_percent,omitempty"`
	MinClientConnectionReusePercent        *float64 `json:"min_client_connection_reuse_percent,omitempty"`
	MinUpstreamConnectionReusePercent      *float64 `json:"min_upstream_connection_reuse_percent,omitempty"`
	MaxConnectionReuseDecreasePoints       *float64 `json:"max_connection_reuse_decrease_points,omitempty"`
	MaxUnexpectedFailures                  *int     `json:"max_unexpected_failures,omitempty"`
}

func (p thresholdPolicy) limitsFor(scenario string) thresholdLimits {
	limits := p.Defaults
	override := p.Scenarios[scenario]
	applyFloat := func(value *float64, target *float64) {
		if value != nil {
			*target = *value
		}
	}
	applyFloat(override.MaxAddedLatencyP95IncreasePercent, &limits.MaxAddedLatencyP95IncreasePercent)
	applyFloat(override.MaxAddedLatencyP95IncreaseMS, &limits.MaxAddedLatencyP95IncreaseMS)
	applyFloat(override.MaxAddedLatencyP99IncreasePercent, &limits.MaxAddedLatencyP99IncreasePercent)
	applyFloat(override.MaxAddedLatencyP99IncreaseMS, &limits.MaxAddedLatencyP99IncreaseMS)
	applyFloat(override.MaxTTFTP95IncreasePercent, &limits.MaxTTFTP95IncreasePercent)
	applyFloat(override.MaxTTFTP95IncreaseMS, &limits.MaxTTFTP95IncreaseMS)
	applyFloat(override.MaxRPSDecreasePercent, &limits.MaxRPSDecreasePercent)
	applyFloat(override.MaxAllocBytesPerRequestIncreasePercent, &limits.MaxAllocBytesPerRequestIncreasePercent)
	applyFloat(override.MaxAllocsPerRequestIncreasePercent, &limits.MaxAllocsPerRequestIncreasePercent)
	applyFloat(override.MinClientConnectionReusePercent, &limits.MinClientConnectionReusePercent)
	applyFloat(override.MinUpstreamConnectionReusePercent, &limits.MinUpstreamConnectionReusePercent)
	applyFloat(override.MaxConnectionReuseDecreasePoints, &limits.MaxConnectionReuseDecreasePoints)
	if override.MaxUnexpectedFailures != nil {
		limits.MaxUnexpectedFailures = *override.MaxUnexpectedFailures
	}
	return limits
}

func loadThresholdPolicy(path string) (thresholdPolicy, error) {
	file, err := os.Open(path)
	if err != nil {
		return thresholdPolicy{}, fmt.Errorf("open performance thresholds: %w", err)
	}
	defer file.Close()

	var policy thresholdPolicy
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&policy); err != nil {
		return thresholdPolicy{}, fmt.Errorf("decode performance thresholds: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return thresholdPolicy{}, errors.New("performance thresholds contain multiple JSON values")
		}
		return thresholdPolicy{}, fmt.Errorf("decode performance thresholds trailer: %w", err)
	}
	if err := validateThresholdPolicy(policy); err != nil {
		return thresholdPolicy{}, err
	}
	return policy, nil
}

func validateThresholdPolicy(policy thresholdPolicy) error {
	if policy.SchemaVersion != thresholdPolicySchemaVersion {
		return fmt.Errorf("unsupported threshold schema %d", policy.SchemaVersion)
	}
	if policy.MinimumRuns < minimumArtifactRuns {
		return fmt.Errorf("minimum_runs must be at least %d", minimumArtifactRuns)
	}
	if err := validateThresholdLimits(policy.Defaults); err != nil {
		return fmt.Errorf("default thresholds: %w", err)
	}
	for scenario := range policy.Scenarios {
		if scenario == "" {
			return errors.New("scenario threshold name is required")
		}
		if err := validateThresholdLimits(policy.limitsFor(scenario)); err != nil {
			return fmt.Errorf("scenario %q thresholds: %w", scenario, err)
		}
	}
	return nil
}

func validateThresholdLimits(limits thresholdLimits) error {
	values := map[string]float64{
		"max_added_latency_p95_increase_percent":       limits.MaxAddedLatencyP95IncreasePercent,
		"max_added_latency_p95_increase_ms":            limits.MaxAddedLatencyP95IncreaseMS,
		"max_added_latency_p99_increase_percent":       limits.MaxAddedLatencyP99IncreasePercent,
		"max_added_latency_p99_increase_ms":            limits.MaxAddedLatencyP99IncreaseMS,
		"max_ttft_p95_increase_percent":                limits.MaxTTFTP95IncreasePercent,
		"max_ttft_p95_increase_ms":                     limits.MaxTTFTP95IncreaseMS,
		"max_rps_decrease_percent":                     limits.MaxRPSDecreasePercent,
		"max_alloc_bytes_per_request_increase_percent": limits.MaxAllocBytesPerRequestIncreasePercent,
		"max_allocs_per_request_increase_percent":      limits.MaxAllocsPerRequestIncreasePercent,
		"max_connection_reuse_decrease_points":         limits.MaxConnectionReuseDecreasePoints,
	}
	for name, value := range values {
		if value < 0 || math.IsNaN(value) || math.IsInf(value, 0) {
			return fmt.Errorf("%s must be a finite non-negative number", name)
		}
	}
	for name, value := range map[string]float64{
		"min_client_connection_reuse_percent":   limits.MinClientConnectionReusePercent,
		"min_upstream_connection_reuse_percent": limits.MinUpstreamConnectionReusePercent,
	} {
		if value < 0 || value > 100 {
			return fmt.Errorf("%s must be between 0 and 100", name)
		}
	}
	if limits.MaxUnexpectedFailures < 0 {
		return errors.New("max_unexpected_failures must not be negative")
	}
	return nil
}

type comparisonReport struct {
	Passed            bool                 `json:"passed"`
	BaselineRevision  string               `json:"baseline_revision"`
	CandidateRevision string               `json:"candidate_revision"`
	Environment       string               `json:"environment"`
	BaselineRuns      int                  `json:"baseline_runs"`
	CandidateRuns     int                  `json:"candidate_runs"`
	Scenarios         []scenarioComparison `json:"scenarios"`
}

type scenarioComparison struct {
	Scenario string             `json:"scenario"`
	Passed   bool               `json:"passed"`
	Metrics  []metricComparison `json:"metrics"`
}

type metricComparison struct {
	Metric        string   `json:"metric"`
	Unit          string   `json:"unit"`
	Baseline      float64  `json:"baseline"`
	Candidate     float64  `json:"candidate"`
	ChangePercent *float64 `json:"change_percent,omitempty"`
	Limit         string   `json:"limit"`
	Passed        bool     `json:"passed"`
}

func compareBenchmarkArtifacts(baseline, candidate benchmarkArtifact, policy thresholdPolicy) (comparisonReport, error) {
	if err := validateThresholdPolicy(policy); err != nil {
		return comparisonReport{}, err
	}
	if err := validateBenchmarkArtifact(baseline); err != nil {
		return comparisonReport{}, fmt.Errorf("baseline artifact: %w", err)
	}
	if err := validateBenchmarkArtifact(candidate); err != nil {
		return comparisonReport{}, fmt.Errorf("candidate artifact: %w", err)
	}
	if baseline.Environment != candidate.Environment {
		return comparisonReport{}, fmt.Errorf("environment differs: baseline %q, candidate %q", baseline.Environment, candidate.Environment)
	}
	if len(baseline.Runs) < policy.MinimumRuns || len(candidate.Runs) < policy.MinimumRuns {
		return comparisonReport{}, fmt.Errorf("comparison requires at least %d runs in both artifacts", policy.MinimumRuns)
	}
	baselineShapes, _ := artifactRunShapes(baseline.Runs[0])
	candidateShapes, _ := artifactRunShapes(candidate.Runs[0])
	if err := compareArtifactShapes(baselineShapes, candidateShapes); err != nil {
		return comparisonReport{}, fmt.Errorf("artifacts are incompatible: %w", err)
	}
	for scenario := range policy.Scenarios {
		if _, exists := baselineShapes[scenario]; !exists {
			return comparisonReport{}, fmt.Errorf("threshold override references unknown scenario %q", scenario)
		}
	}

	baselineReports := reportsByScenario(baseline.Runs)
	candidateReports := reportsByScenario(candidate.Runs)
	names := make([]string, 0, len(baselineReports))
	for name := range baselineReports {
		names = append(names, name)
	}
	sort.Strings(names)

	report := comparisonReport{
		Passed:            true,
		BaselineRevision:  baseline.Revision,
		CandidateRevision: candidate.Revision,
		Environment:       baseline.Environment,
		BaselineRuns:      len(baseline.Runs),
		CandidateRuns:     len(candidate.Runs),
	}
	for _, name := range names {
		result, err := compareScenario(name, baselineReports[name], candidateReports[name], policy.limitsFor(name))
		if err != nil {
			return comparisonReport{}, err
		}
		report.Scenarios = append(report.Scenarios, result)
		if !result.Passed {
			report.Passed = false
		}
	}
	return report, nil
}

func reportsByScenario(runs [][]benchmarkReport) map[string][]benchmarkReport {
	reports := make(map[string][]benchmarkReport)
	for _, run := range runs {
		for _, report := range run {
			reports[report.Scenario] = append(reports[report.Scenario], report)
		}
	}
	return reports
}

func compareScenario(name string, baseline, candidate []benchmarkReport, limits thresholdLimits) (scenarioComparison, error) {
	result := scenarioComparison{Scenario: name, Passed: true}
	add := func(metric metricComparison) {
		result.Metrics = append(result.Metrics, metric)
		if !metric.Passed {
			result.Passed = false
		}
	}

	add(increaseWithAbsoluteGuard(
		"added_latency_p95_ms", "ms",
		medianReports(baseline, func(report benchmarkReport) float64 { return report.AddedLatency.P95MS }),
		medianReports(candidate, func(report benchmarkReport) float64 { return report.AddedLatency.P95MS }),
		limits.MaxAddedLatencyP95IncreasePercent, limits.MaxAddedLatencyP95IncreaseMS,
	))
	add(increaseWithAbsoluteGuard(
		"added_latency_p99_ms", "ms",
		medianReports(baseline, func(report benchmarkReport) float64 { return report.AddedLatency.P99MS }),
		medianReports(candidate, func(report benchmarkReport) float64 { return report.AddedLatency.P99MS }),
		limits.MaxAddedLatencyP99IncreasePercent, limits.MaxAddedLatencyP99IncreaseMS,
	))

	baselineHasTTFT := baseline[0].AddedTTFT.Samples > 0
	candidateHasTTFT := candidate[0].AddedTTFT.Samples > 0
	if baselineHasTTFT != candidateHasTTFT {
		return scenarioComparison{}, fmt.Errorf("scenario %q TTFT shape differs", name)
	}
	if baselineHasTTFT {
		add(increaseWithAbsoluteGuard(
			"added_ttft_p95_ms", "ms",
			medianReports(baseline, func(report benchmarkReport) float64 { return report.AddedTTFT.P95MS }),
			medianReports(candidate, func(report benchmarkReport) float64 { return report.AddedTTFT.P95MS }),
			limits.MaxTTFTP95IncreasePercent, limits.MaxTTFTP95IncreaseMS,
		))
	}

	add(maxDecrease(
		"gateway_requests_per_second", "requests/s",
		medianReports(baseline, func(report benchmarkReport) float64 { return report.Gateway.RequestsPerSecond }),
		medianReports(candidate, func(report benchmarkReport) float64 { return report.Gateway.RequestsPerSecond }),
		limits.MaxRPSDecreasePercent,
	))
	add(maxIncrease(
		"gateway_alloc_bytes_per_request", "bytes/request",
		medianReports(baseline, allocBytesPerRequest), medianReports(candidate, allocBytesPerRequest),
		limits.MaxAllocBytesPerRequestIncreasePercent,
	))
	add(maxIncrease(
		"gateway_allocs_per_request", "allocs/request",
		medianReports(baseline, allocsPerRequest), medianReports(candidate, allocsPerRequest),
		limits.MaxAllocsPerRequestIncreasePercent,
	))
	add(reuseMetric(
		"client_connection_reuse_percent",
		medianReports(baseline, func(report benchmarkReport) float64 { return report.Gateway.ClientConnections.ReusePercent }),
		medianReports(candidate, func(report benchmarkReport) float64 { return report.Gateway.ClientConnections.ReusePercent }),
		limits.MinClientConnectionReusePercent, limits.MaxConnectionReuseDecreasePoints,
	))
	add(reuseMetric(
		"upstream_connection_reuse_percent",
		medianReports(baseline, func(report benchmarkReport) float64 { return report.Upstream.Connections.ReusePercent }),
		medianReports(candidate, func(report benchmarkReport) float64 { return report.Upstream.Connections.ReusePercent }),
		limits.MinUpstreamConnectionReusePercent, limits.MaxConnectionReuseDecreasePoints,
	))
	add(maximumMetric(
		"unexpected_failures", "requests",
		medianReports(baseline, func(report benchmarkReport) float64 { return float64(report.Gateway.UnexpectedFailures) }),
		medianReports(candidate, func(report benchmarkReport) float64 { return float64(report.Gateway.UnexpectedFailures) }),
		float64(limits.MaxUnexpectedFailures),
	))
	return result, nil
}

func medianReports(reports []benchmarkReport, value func(benchmarkReport) float64) float64 {
	values := make([]float64, len(reports))
	for index, report := range reports {
		values[index] = value(report)
	}
	sort.Float64s(values)
	middle := len(values) / 2
	if len(values)%2 == 1 {
		return values[middle]
	}
	return (values[middle-1] + values[middle]) / 2
}

func allocBytesPerRequest(report benchmarkReport) float64 {
	return float64(report.Gateway.Memory.TotalAllocBytes) / float64(report.Load.Requests)
}

func allocsPerRequest(report benchmarkReport) float64 {
	return float64(report.Gateway.Memory.Mallocs) / float64(report.Load.Requests)
}

func increaseWithAbsoluteGuard(name, unit string, baseline, candidate, maxPercent, maxAbsolute float64) metricComparison {
	change := relativeChange(baseline, candidate)
	relativeExceeded := baseline <= 0 || (change != nil && *change > maxPercent)
	passed := !(candidate-baseline > maxAbsolute && relativeExceeded)
	return metricComparison{
		Metric: name, Unit: unit, Baseline: baseline, Candidate: candidate,
		ChangePercent: change,
		Limit:         fmt.Sprintf("increase <= %.2f%% or <= %.3f %s", maxPercent, maxAbsolute, unit),
		Passed:        passed,
	}
}

func maxIncrease(name, unit string, baseline, candidate, maxPercent float64) metricComparison {
	change := relativeChange(baseline, candidate)
	passed := candidate <= baseline
	if baseline > 0 && change != nil {
		passed = *change <= maxPercent
	}
	return metricComparison{
		Metric: name, Unit: unit, Baseline: baseline, Candidate: candidate,
		ChangePercent: change, Limit: fmt.Sprintf("increase <= %.2f%%", maxPercent), Passed: passed,
	}
}

func maxDecrease(name, unit string, baseline, candidate, maxPercent float64) metricComparison {
	change := relativeChange(baseline, candidate)
	passed := candidate >= baseline
	if baseline > 0 {
		decrease := (baseline - candidate) * 100 / baseline
		passed = decrease <= maxPercent
	}
	return metricComparison{
		Metric: name, Unit: unit, Baseline: baseline, Candidate: candidate,
		ChangePercent: change, Limit: fmt.Sprintf("decrease <= %.2f%%", maxPercent), Passed: passed,
	}
}

func reuseMetric(name string, baseline, candidate, minimum, maxDecreasePoints float64) metricComparison {
	return metricComparison{
		Metric: name, Unit: "percent", Baseline: baseline, Candidate: candidate,
		ChangePercent: relativeChange(baseline, candidate),
		Limit:         fmt.Sprintf(">= %.2f%% and decrease <= %.2f points", minimum, maxDecreasePoints),
		Passed:        candidate >= minimum && baseline-candidate <= maxDecreasePoints,
	}
}

func maximumMetric(name, unit string, baseline, candidate, maximum float64) metricComparison {
	return metricComparison{
		Metric: name, Unit: unit, Baseline: baseline, Candidate: candidate,
		ChangePercent: relativeChange(baseline, candidate),
		Limit:         fmt.Sprintf("<= %.0f %s", maximum, unit),
		Passed:        candidate <= maximum,
	}
}

func relativeChange(baseline, candidate float64) *float64 {
	if baseline == 0 {
		return nil
	}
	change := (candidate - baseline) * 100 / math.Abs(baseline)
	return &change
}
