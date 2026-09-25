package main

import (
	"context"
	"testing"
)

func TestHarnessRunsModalityScenarios(t *testing.T) {
	for _, name := range []string{"text", "tools", "image", "pdf", "audio"} {
		t.Run(name, func(t *testing.T) {
			scenario := mustScenario(t, name)
			report, err := runScenario(context.Background(), scenario, loadConfig{
				Requests: 4, Concurrency: 2, Warmup: 2,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertHealthyPhase(t, report.Direct, 4)
			assertHealthyPhase(t, report.Gateway, 4)
			if report.PayloadBytes != len(scenario.requestBody(0)) || report.MediaBytes != scenario.MediaBytes {
				t.Fatalf("payload metadata = %d/%d", report.PayloadBytes, report.MediaBytes)
			}
			if report.Upstream.Requests != 4 {
				t.Fatalf("upstream requests = %d, want 4", report.Upstream.Requests)
			}
		})
	}
}

func TestHarnessMeasuresSSETTFTAndConnectionReuse(t *testing.T) {
	scenario := mustScenario(t, "sse")
	report, err := runScenario(context.Background(), scenario, loadConfig{
		Requests: 6, Concurrency: 1, Warmup: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	assertHealthyPhase(t, report.Direct, 6)
	assertHealthyPhase(t, report.Gateway, 6)
	if report.Direct.TTFT.Samples != 6 || report.Gateway.TTFT.Samples != 6 || report.AddedTTFT.Samples != 6 {
		t.Fatalf("TTFT samples = direct:%d gateway:%d added:%d", report.Direct.TTFT.Samples, report.Gateway.TTFT.Samples, report.AddedTTFT.Samples)
	}
	if report.Gateway.TTFT.P50MS <= 0 {
		t.Fatalf("gateway TTFT = %#v", report.Gateway.TTFT)
	}
	if report.Gateway.ClientConnections.Reused == 0 || report.Upstream.Connections.Reused == 0 {
		t.Fatalf("connections = client:%#v upstream:%#v", report.Gateway.ClientConnections, report.Upstream.Connections)
	}
}

func TestHarnessExercisesFailureAndBackpressureScenarios(t *testing.T) {
	for _, name := range []string{"retry", "fallback", "rate-limit-429", "slow-client"} {
		t.Run(name, func(t *testing.T) {
			report, err := runScenario(context.Background(), mustScenario(t, name), loadConfig{
				Requests: 4, Concurrency: 1, Warmup: 1,
			})
			if err != nil {
				t.Fatal(err)
			}
			assertHealthyPhase(t, report.Gateway, 4)
			if name != "slow-client" && report.Upstream.ErrorResponses == 0 {
				t.Fatalf("upstream errors = %d, want at least one", report.Upstream.ErrorResponses)
			}
			if name == "slow-client" && report.Gateway.TTFT.Samples != 4 {
				t.Fatalf("slow-client TTFT = %#v", report.Gateway.TTFT)
			}
		})
	}
}

func TestHarnessCancellationDoesNotRetry(t *testing.T) {
	report, err := runScenario(context.Background(), mustScenario(t, "cancellation"), loadConfig{
		Requests: 5, Concurrency: 1, Warmup: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if report.Gateway.UnexpectedFailures != 0 || report.Gateway.ExpectedFailures != 5 {
		t.Fatalf("gateway cancellation phase = %#v", report.Gateway)
	}
	if report.Upstream.Requests != 5 {
		t.Fatalf("upstream requests = %d, want 5 without retries", report.Upstream.Requests)
	}
	if report.Upstream.Canceled != 5 {
		t.Fatalf("upstream cancellations = %d, want 5", report.Upstream.Canceled)
	}
}

func assertHealthyPhase(t *testing.T, phase phaseReport, requests int) {
	t.Helper()
	if phase.Completed != requests || phase.UnexpectedFailures != 0 {
		t.Fatalf("phase = %#v", phase)
	}
	if phase.Latency.Samples != requests || phase.RequestsPerSecond <= 0 {
		t.Fatalf("phase metrics = %#v", phase)
	}
}
