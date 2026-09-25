package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCompareArtifactsUsesMedianAndDualLatencyGuard(t *testing.T) {
	baseline := comparisonArtifact(t, []float64{1, 1, 40})
	candidate := comparisonArtifact(t, []float64{1.4, 1.4, 1.4})
	policy := testThresholdPolicy()
	policy.Defaults.MaxAddedLatencyP95IncreasePercent = 20
	policy.Defaults.MaxAddedLatencyP95IncreaseMS = 0.5

	report, err := compareBenchmarkArtifacts(baseline, candidate, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !report.Passed {
		t.Fatalf("comparison unexpectedly failed: %#v", report)
	}
	metric := findMetric(t, report, "text", "added_latency_p95_ms")
	if metric.Baseline != 1 || metric.Candidate != 1.4 {
		t.Fatalf("median metric = %#v", metric)
	}
}

func TestCompareArtifactsFailsWhenLatencyExceedsBothGuards(t *testing.T) {
	baseline := comparisonArtifact(t, []float64{1, 1, 1})
	candidate := comparisonArtifact(t, []float64{1.8, 1.8, 1.8})
	policy := testThresholdPolicy()
	policy.Defaults.MaxAddedLatencyP95IncreasePercent = 20
	policy.Defaults.MaxAddedLatencyP95IncreaseMS = 0.5

	report, err := compareBenchmarkArtifacts(baseline, candidate, policy)
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed || findMetric(t, report, "text", "added_latency_p95_ms").Passed {
		t.Fatalf("comparison = %#v, want latency regression", report)
	}
}

func TestCompareArtifactsChecksDirectionalAndAbsoluteLimits(t *testing.T) {
	baseline := comparisonArtifact(t, []float64{1, 1, 1})
	candidate := comparisonArtifact(t, []float64{1, 1, 1})
	for run := range candidate.Runs {
		report := &candidate.Runs[run][0]
		report.Gateway.RequestsPerSecond = 600
		report.Gateway.Memory.TotalAllocBytes = 200_000
		report.Gateway.Memory.Mallocs = 10_000
		report.Gateway.ClientConnections.ReusePercent = 70
		report.Upstream.Connections.ReusePercent = 60
		report.Gateway.UnexpectedFailures = 1
	}

	report, err := compareBenchmarkArtifacts(baseline, candidate, testThresholdPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if report.Passed {
		t.Fatal("comparison passed, want regressions")
	}
	for _, metric := range []string{
		"gateway_requests_per_second",
		"gateway_alloc_bytes_per_request",
		"gateway_allocs_per_request",
		"client_connection_reuse_percent",
		"upstream_connection_reuse_percent",
		"unexpected_failures",
	} {
		if findMetric(t, report, "text", metric).Passed {
			t.Errorf("metric %s passed, want failure", metric)
		}
	}
}

func TestCompareArtifactsRejectsIncompatibleEvidence(t *testing.T) {
	baseline := comparisonArtifact(t, []float64{1, 1, 1})
	candidate := comparisonArtifact(t, []float64{1, 1, 1})
	candidate.Environment = "different-runner"
	if _, err := compareBenchmarkArtifacts(baseline, candidate, testThresholdPolicy()); err == nil ||
		!strings.Contains(err.Error(), "environment") {
		t.Fatalf("compare error = %v", err)
	}

	candidate = comparisonArtifact(t, []float64{1, 1, 1})
	candidate.Runs[0][0].Load.Concurrency++
	if _, err := compareBenchmarkArtifacts(baseline, candidate, testThresholdPolicy()); err == nil {
		t.Fatal("compare error = nil for incompatible load")
	}
}

func TestLoadThresholdPolicyMergesScenarioOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thresholds.json")
	contents := `{
  "schema_version": 1,
  "minimum_runs": 3,
  "defaults": {
    "max_added_latency_p95_increase_percent": 20,
    "max_added_latency_p95_increase_ms": 0.5,
    "max_added_latency_p99_increase_percent": 25,
    "max_added_latency_p99_increase_ms": 1,
    "max_ttft_p95_increase_percent": 20,
    "max_ttft_p95_increase_ms": 0.5,
    "max_rps_decrease_percent": 20,
    "max_alloc_bytes_per_request_increase_percent": 30,
    "max_allocs_per_request_increase_percent": 30,
    "min_client_connection_reuse_percent": 90,
    "min_upstream_connection_reuse_percent": 80,
    "max_connection_reuse_decrease_points": 5,
    "max_unexpected_failures": 0
  },
  "scenarios": {
    "cancellation": {
      "min_client_connection_reuse_percent": 0,
      "min_upstream_connection_reuse_percent": 0
    }
  }
}`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	policy, err := loadThresholdPolicy(path)
	if err != nil {
		t.Fatal(err)
	}
	limits := policy.limitsFor("cancellation")
	if limits.MinClientConnectionReusePercent != 0 || limits.MaxRPSDecreasePercent != 20 {
		t.Fatalf("merged limits = %#v", limits)
	}
}

func comparisonArtifact(t *testing.T, p95Values []float64) benchmarkArtifact {
	t.Helper()
	runs := sampleArtifactRuns(len(p95Values))
	for index, value := range p95Values {
		report := &runs[index][0]
		report.AddedLatency.P95MS = value
		report.AddedLatency.P99MS = value + 0.25
		report.Gateway.RequestsPerSecond = 1000
		report.Gateway.Memory.TotalAllocBytes = 100_000
		report.Gateway.Memory.Mallocs = 5_000
		report.Gateway.ClientConnections.ReusePercent = 99
		report.Upstream.Connections.ReusePercent = 99
	}
	artifact, err := newBenchmarkArtifact(
		runs, "revision", "ci-linux", time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	return artifact
}

func testThresholdPolicy() thresholdPolicy {
	return thresholdPolicy{
		SchemaVersion: thresholdPolicySchemaVersion,
		MinimumRuns:   3,
		Defaults: thresholdLimits{
			MaxAddedLatencyP95IncreasePercent:      20,
			MaxAddedLatencyP95IncreaseMS:           0.5,
			MaxAddedLatencyP99IncreasePercent:      25,
			MaxAddedLatencyP99IncreaseMS:           1,
			MaxTTFTP95IncreasePercent:              20,
			MaxTTFTP95IncreaseMS:                   0.5,
			MaxRPSDecreasePercent:                  20,
			MaxAllocBytesPerRequestIncreasePercent: 30,
			MaxAllocsPerRequestIncreasePercent:     30,
			MinClientConnectionReusePercent:        90,
			MinUpstreamConnectionReusePercent:      80,
			MaxConnectionReuseDecreasePoints:       5,
			MaxUnexpectedFailures:                  0,
		},
		Scenarios: map[string]thresholdOverrides{},
	}
}

func findMetric(t *testing.T, report comparisonReport, scenario, metric string) metricComparison {
	t.Helper()
	for _, result := range report.Scenarios {
		if result.Scenario != scenario {
			continue
		}
		for _, candidate := range result.Metrics {
			if candidate.Metric == metric {
				return candidate
			}
		}
	}
	t.Fatalf("metric %s/%s not found in %#v", scenario, metric, report)
	return metricComparison{}
}
