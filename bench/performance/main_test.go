package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunEmitsJSONReport(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-scenario", "text",
		"-requests", "3",
		"-concurrency", "1",
		"-warmup", "1",
		"-format", "json",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}
	var reports []benchmarkReport
	if err := json.Unmarshal(stdout.Bytes(), &reports); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout.String())
	}
	if len(reports) != 1 || reports[0].Scenario != "text" || reports[0].Gateway.Completed != 3 {
		t.Fatalf("reports = %#v", reports)
	}
	if reports[0].System.GoVersion == "" || reports[0].System.LogicalCPUs == 0 {
		t.Fatalf("system metadata = %#v", reports[0].System)
	}
}

func TestRunEmitsHumanSummary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-scenario", "sse",
		"-requests", "2",
		"-concurrency", "1",
		"-warmup", "1",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}
	for _, want := range []string{"scenario: sse", "p50/p95/p99", "TTFT", "requests/s", "connection reuse"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("output = %q, want %q", stdout.String(), want)
		}
	}
}

func TestRunListsScenarios(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"-list"}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, want := range scenarioNames() {
		if !strings.Contains(stdout.String(), want+"\n") {
			t.Fatalf("list output missing %q: %s", want, stdout.String())
		}
	}
}

func TestRunRejectsInvalidOptions(t *testing.T) {
	for _, args := range [][]string{
		{"-scenario", "unknown"},
		{"-format", "xml"},
		{"-requests", "0"},
	} {
		var stdout, stderr bytes.Buffer
		if err := run(args, &stdout, &stderr); err == nil {
			t.Fatalf("run(%v) error = nil", args)
		}
	}
}

func TestRunWritesRepeatedArtifact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "candidate.json")
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-scenario", "text",
		"-requests", "1",
		"-concurrency", "1",
		"-warmup", "0",
		"-runs", "3",
		"-artifact", path,
		"-revision", "abc123",
		"-environment", "test-runner",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run() error = %v, stderr = %s", err, stderr.String())
	}
	artifact, err := readBenchmarkArtifact(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(artifact.Runs) != 3 || artifact.Revision != "abc123" || artifact.Environment != "test-runner" {
		t.Fatalf("artifact = %#v", artifact)
	}
}

func TestRunRequiresThreeRunsForArtifact(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-scenario", "text",
		"-runs", "2",
		"-artifact", filepath.Join(t.TempDir(), "invalid.json"),
	}, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "at least 3") {
		t.Fatalf("run() error = %v", err)
	}
}

func TestRunCompareWritesReportAndReturnsRegression(t *testing.T) {
	directory := t.TempDir()
	baselinePath := filepath.Join(directory, "baseline.json")
	candidatePath := filepath.Join(directory, "candidate.json")
	outputPath := filepath.Join(directory, "comparison.json")
	baseline := comparisonArtifact(t, []float64{1, 1, 1, 1, 1})
	candidate := comparisonArtifact(t, []float64{2, 2, 2, 2, 2})
	if err := writeBenchmarkArtifact(baselinePath, baseline); err != nil {
		t.Fatal(err)
	}
	if err := writeBenchmarkArtifact(candidatePath, candidate); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := run([]string{
		"compare",
		"-baseline", baselinePath,
		"-candidate", candidatePath,
		"-thresholds", "thresholds.json",
		"-output", outputPath,
	}, &stdout, &stderr)
	if !errors.Is(err, errPerformanceRegression) {
		t.Fatalf("run(compare) error = %v", err)
	}
	if !strings.Contains(stdout.String(), "REGRESSION") {
		t.Fatalf("compare output = %q", stdout.String())
	}
	var report comparisonReport
	readJSONFile(t, outputPath, &report)
	if report.Passed {
		t.Fatalf("comparison report = %#v", report)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatal(err)
	}
}
