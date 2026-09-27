package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/pprof"
	"time"
)

var errPerformanceRegression = errors.New("performance regression detected")

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "performance benchmark:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 && args[0] == "compare" {
		return runCompare(args[1:], stdout, stderr)
	}
	return runBenchmark(args, stdout, stderr)
}

func runBenchmark(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("thruplane-performance", flag.ContinueOnError)
	flags.SetOutput(stderr)
	scenarioName := flags.String("scenario", "all", "scenario name or all")
	requests := flags.Int("requests", 200, "measured requests per phase")
	concurrency := flags.Int("concurrency", 16, "concurrent workers")
	warmup := flags.Int("warmup", 20, "warmup requests per phase")
	format := flags.String("format", "human", "output format: human or json")
	list := flags.Bool("list", false, "list available scenarios")
	cpuProfile := flags.String("cpuprofile", "", "write a Go CPU profile")
	memProfile := flags.String("memprofile", "", "write a Go heap profile")
	runs := flags.Int("runs", 1, "number of complete repetitions")
	artifactPath := flags.String("artifact", "", "write a versioned repeated-run JSON artifact")
	revision := flags.String("revision", defaultRevision(), "source revision stored in the artifact")
	environment := flags.String("environment", defaultEnvironment(), "runner identifier stored in the artifact")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *list {
		for _, name := range scenarioNames() {
			_, _ = fmt.Fprintln(stdout, name)
		}
		return nil
	}
	if *format != "human" && *format != "json" {
		return fmt.Errorf("unsupported output format %q", *format)
	}
	load := loadConfig{Requests: *requests, Concurrency: *concurrency, Warmup: *warmup}
	if load.Requests <= 0 || load.Concurrency <= 0 || load.Warmup < 0 {
		return errors.New("requests and concurrency must be greater than zero, and warmup must not be negative")
	}
	if *runs <= 0 {
		return errors.New("runs must be greater than zero")
	}
	if *artifactPath != "" && *runs < minimumArtifactRuns {
		return fmt.Errorf("artifact output requires at least %d runs", minimumArtifactRuns)
	}
	if *runs > 1 && *artifactPath == "" {
		return errors.New("multiple runs require -artifact output")
	}

	names := []string{*scenarioName}
	if *scenarioName == "all" {
		names = scenarioNames()
	} else if _, err := scenarioByName(*scenarioName); err != nil {
		return err
	}

	stopCPU, err := startCPUProfile(*cpuProfile)
	if err != nil {
		return err
	}
	defer stopCPU()

	allRuns := make([][]benchmarkReport, 0, *runs)
	for runIndex := 0; runIndex < *runs; runIndex++ {
		reports := make([]benchmarkReport, 0, len(names))
		for _, name := range names {
			scenario, _ := scenarioByName(name)
			report, err := runScenario(context.Background(), scenario, load)
			if err != nil {
				return fmt.Errorf("run %d scenario %s: %w", runIndex+1, name, err)
			}
			reports = append(reports, report)
		}
		allRuns = append(allRuns, reports)
	}

	if err := writeHeapProfile(*memProfile); err != nil {
		return err
	}
	var artifact benchmarkArtifact
	if *artifactPath != "" {
		artifact, err = newBenchmarkArtifact(allRuns, *revision, *environment, time.Now())
		if err != nil {
			return err
		}
		if err := writeBenchmarkArtifact(*artifactPath, artifact); err != nil {
			return err
		}
	}
	if *format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if *artifactPath != "" {
			return encoder.Encode(artifact)
		}
		return encoder.Encode(allRuns[0])
	}
	for index, reports := range allRuns {
		if len(allRuns) > 1 {
			if index > 0 {
				_, _ = fmt.Fprintln(stdout)
			}
			_, _ = fmt.Fprintf(stdout, "run: %d/%d\n", index+1, len(allRuns))
		}
		writeHumanReports(stdout, reports)
	}
	if *artifactPath != "" {
		_, _ = fmt.Fprintf(stdout, "\nartifact: %s\n", *artifactPath)
	}
	return nil
}

func runCompare(args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("thruplane-performance compare", flag.ContinueOnError)
	flags.SetOutput(stderr)
	baselinePath := flags.String("baseline", "", "baseline artifact path")
	candidatePath := flags.String("candidate", "", "candidate artifact path")
	thresholdPath := flags.String("thresholds", "bench/performance/thresholds.json", "threshold policy path")
	outputPath := flags.String("output", "", "write the comparison report as JSON")
	format := flags.String("format", "human", "output format: human or json")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *baselinePath == "" || *candidatePath == "" {
		return errors.New("compare requires -baseline and -candidate")
	}
	if *format != "human" && *format != "json" {
		return fmt.Errorf("unsupported output format %q", *format)
	}
	baseline, err := readBenchmarkArtifact(*baselinePath)
	if err != nil {
		return fmt.Errorf("baseline: %w", err)
	}
	candidate, err := readBenchmarkArtifact(*candidatePath)
	if err != nil {
		return fmt.Errorf("candidate: %w", err)
	}
	policy, err := loadThresholdPolicy(*thresholdPath)
	if err != nil {
		return err
	}
	report, err := compareBenchmarkArtifacts(baseline, candidate, policy)
	if err != nil {
		return err
	}
	if *outputPath != "" {
		if err := writeComparisonReport(*outputPath, report); err != nil {
			return err
		}
	}
	if *format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(report); err != nil {
			return err
		}
	} else {
		writeHumanComparison(stdout, report)
	}
	if !report.Passed {
		return errPerformanceRegression
	}
	return nil
}

func defaultRevision() string {
	if revision := os.Getenv("GITHUB_SHA"); revision != "" {
		return revision
	}
	return "working-tree"
}

func defaultEnvironment() string {
	if environment := os.Getenv("THRUPLANE_PERF_ENVIRONMENT"); environment != "" {
		return environment
	}
	return fmt.Sprintf("local-%s-%s-%dcpu", runtime.GOOS, runtime.GOARCH, runtime.NumCPU())
}

func writeComparisonReport(path string, report comparisonReport) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create comparison report: %w", err)
	}
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	encodeErr := encoder.Encode(report)
	closeErr := file.Close()
	if encodeErr != nil {
		return fmt.Errorf("encode comparison report: %w", encodeErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close comparison report: %w", closeErr)
	}
	return nil
}

func writeHumanComparison(w io.Writer, report comparisonReport) {
	status := "PASS"
	if !report.Passed {
		status = "REGRESSION"
	}
	_, _ = fmt.Fprintf(w, "performance comparison: %s\n", status)
	_, _ = fmt.Fprintf(w, "  revisions: %s -> %s\n", report.BaselineRevision, report.CandidateRevision)
	_, _ = fmt.Fprintf(w, "  environment: %s\n", report.Environment)
	_, _ = fmt.Fprintf(w, "  runs: baseline %d, candidate %d\n", report.BaselineRuns, report.CandidateRuns)
	for _, scenario := range report.Scenarios {
		scenarioStatus := "PASS"
		if !scenario.Passed {
			scenarioStatus = "REGRESSION"
		}
		_, _ = fmt.Fprintf(w, "  scenario %s: %s\n", scenario.Scenario, scenarioStatus)
		for _, metric := range scenario.Metrics {
			if metric.Passed {
				continue
			}
			_, _ = fmt.Fprintf(w, "    %s: %.3f -> %.3f %s; limit %s\n", metric.Metric, metric.Baseline, metric.Candidate, metric.Unit, metric.Limit)
		}
	}
}

func startCPUProfile(path string) (func(), error) {
	if path == "" {
		return func() {}, nil
	}
	file, err := os.Create(path)
	if err != nil {
		return nil, fmt.Errorf("create CPU profile: %w", err)
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("start CPU profile: %w", err)
	}
	return func() {
		pprof.StopCPUProfile()
		_ = file.Close()
	}, nil
}

func writeHeapProfile(path string) error {
	if path == "" {
		return nil
	}
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("create heap profile: %w", err)
	}
	defer file.Close()
	runtime.GC()
	if err := pprof.WriteHeapProfile(file); err != nil {
		return fmt.Errorf("write heap profile: %w", err)
	}
	return nil
}

func writeHumanReports(w io.Writer, reports []benchmarkReport) {
	for index, report := range reports {
		if index > 0 {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintf(w, "scenario: %s\n", report.Scenario)
		_, _ = fmt.Fprintf(w, "  system: %s %s/%s, %d logical CPUs\n", report.System.GoVersion, report.System.GOOS, report.System.GOARCH, report.System.LogicalCPUs)
		_, _ = fmt.Fprintf(w, "  load: %d requests, concurrency %d, warmup %d, payload %d bytes\n", report.Load.Requests, report.Load.Concurrency, report.Load.Warmup, report.PayloadBytes)
		writeLatencyLine(w, "direct latency p50/p95/p99", report.Direct.Latency)
		writeLatencyLine(w, "gateway latency p50/p95/p99", report.Gateway.Latency)
		writeLatencyLine(w, "added latency p50/p95/p99", report.AddedLatency)
		if report.Gateway.TTFT.Samples > 0 {
			writeLatencyLine(w, "direct TTFT p50/p95/p99", report.Direct.TTFT)
			writeLatencyLine(w, "gateway TTFT p50/p95/p99", report.Gateway.TTFT)
			writeLatencyLine(w, "added TTFT p50/p95/p99", report.AddedTTFT)
		}
		_, _ = fmt.Fprintf(w, "  throughput: direct %.2f requests/s, gateway %.2f requests/s\n", report.Direct.RequestsPerSecond, report.Gateway.RequestsPerSecond)
		_, _ = fmt.Fprintf(w, "  connection reuse: client %.2f%%, upstream %.2f%%\n", report.Gateway.ClientConnections.ReusePercent, report.Upstream.Connections.ReusePercent)
		_, _ = fmt.Fprintf(w, "  gateway memory: %d allocated bytes, %d mallocs, %d GC cycles\n", report.Gateway.Memory.TotalAllocBytes, report.Gateway.Memory.Mallocs, report.Gateway.Memory.GCCycles)
		_, _ = fmt.Fprintf(w, "  outcomes: %d completed, %d expected failures, %d unexpected failures\n", report.Gateway.Completed, report.Gateway.ExpectedFailures, report.Gateway.UnexpectedFailures)
		_, _ = fmt.Fprintf(w, "  upstream: %d requests, %d error responses, %d cancellations\n", report.Upstream.Requests, report.Upstream.ErrorResponses, report.Upstream.Canceled)
	}
}

func writeLatencyLine(w io.Writer, label string, summary durationSummary) {
	_, _ = fmt.Fprintf(w, "  %s: %.3f / %.3f / %.3f ms\n", label, summary.P50MS, summary.P95MS, summary.P99MS)
}
