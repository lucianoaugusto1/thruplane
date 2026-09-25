package main

import (
	"bytes"
	"encoding/json"
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
