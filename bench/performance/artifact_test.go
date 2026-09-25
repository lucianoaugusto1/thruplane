package main

import (
	"path/filepath"
	"testing"
	"time"
)

func TestNewBenchmarkArtifactCapturesRawRuns(t *testing.T) {
	runs := sampleArtifactRuns(3)
	generatedAt := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)

	artifact, err := newBenchmarkArtifact(runs, "abc123", "ci-linux-x64", generatedAt)
	if err != nil {
		t.Fatal(err)
	}
	if artifact.SchemaVersion != performanceArtifactSchemaVersion {
		t.Fatalf("schema version = %d", artifact.SchemaVersion)
	}
	if artifact.Revision != "abc123" || artifact.Environment != "ci-linux-x64" {
		t.Fatalf("metadata = %#v", artifact)
	}
	if !artifact.GeneratedAt.Equal(generatedAt) || len(artifact.Runs) != 3 {
		t.Fatalf("artifact = %#v", artifact)
	}
}

func TestNewBenchmarkArtifactRejectsInvalidEvidence(t *testing.T) {
	tests := []struct {
		name        string
		runs        [][]benchmarkReport
		revision    string
		environment string
	}{
		{name: "too few runs", runs: sampleArtifactRuns(2), revision: "abc", environment: "ci"},
		{name: "missing revision", runs: sampleArtifactRuns(3), environment: "ci"},
		{name: "missing environment", runs: sampleArtifactRuns(3), revision: "abc"},
		{name: "different load", runs: incompatibleArtifactRuns(), revision: "abc", environment: "ci"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := newBenchmarkArtifact(test.runs, test.revision, test.environment, time.Now()); err == nil {
				t.Fatal("newBenchmarkArtifact() error = nil")
			}
		})
	}
}

func TestBenchmarkArtifactRoundTrip(t *testing.T) {
	want, err := newBenchmarkArtifact(
		sampleArtifactRuns(3),
		"abc123",
		"ci-linux-x64",
		time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "performance.json")
	if err := writeBenchmarkArtifact(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := readBenchmarkArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != want.SchemaVersion || got.Revision != want.Revision ||
		got.Environment != want.Environment || !got.GeneratedAt.Equal(want.GeneratedAt) ||
		len(got.Runs) != len(want.Runs) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func sampleArtifactRuns(count int) [][]benchmarkReport {
	runs := make([][]benchmarkReport, count)
	for index := range runs {
		runs[index] = []benchmarkReport{sampleBenchmarkReport("text", float64(index))}
	}
	return runs
}

func incompatibleArtifactRuns() [][]benchmarkReport {
	runs := sampleArtifactRuns(3)
	runs[2][0].Load.Concurrency++
	return runs
}

func sampleBenchmarkReport(scenario string, offset float64) benchmarkReport {
	return benchmarkReport{
		Scenario: scenario,
		System: systemMetadata{
			GoVersion: "go1.26.0", GOOS: "linux", GOARCH: "amd64", LogicalCPUs: 4,
		},
		Load:         loadConfig{Requests: 100, Concurrency: 4, Warmup: 10},
		PayloadBytes: 128,
		Gateway: phaseReport{
			Completed:         100,
			RequestsPerSecond: 1000 - offset,
			Latency:           durationSummary{Samples: 100, P95MS: 2 + offset, P99MS: 3 + offset},
			ClientConnections: connectionSummary{Requests: 100, Reused: 99, ReusePercent: 99},
			Memory:            memorySummary{TotalAllocBytes: 100_000, Mallocs: 5_000},
		},
		AddedLatency: durationSummary{Samples: 100, P95MS: 1 + offset, P99MS: 2 + offset},
		Upstream: upstreamReport{
			Requests:    100,
			Connections: connectionSummary{Requests: 100, Reused: 99, ReusePercent: 99},
		},
	}
}
