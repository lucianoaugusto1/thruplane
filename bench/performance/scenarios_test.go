package main

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestScenarioCatalogCoversRequestedMatrix(t *testing.T) {
	t.Parallel()
	want := []string{
		"audio",
		"cancellation",
		"fallback",
		"image",
		"pdf",
		"rate-limit-429",
		"retry",
		"slow-client",
		"sse",
		"text",
		"tools",
	}
	got := scenarioNames()
	if !slices.Equal(got, want) {
		t.Fatalf("scenario names = %v, want %v", got, want)
	}
}

func TestModalityScenariosUseNativeAdaptersAndLargeMedia(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name         string
		providerType string
		mediaBytes   int
	}{
		{name: "text", providerType: "openai-compatible"},
		{name: "tools", providerType: "anthropic"},
		{name: "image", providerType: "anthropic", mediaBytes: minimumMediaBytes},
		{name: "pdf", providerType: "anthropic", mediaBytes: minimumMediaBytes},
		{name: "audio", providerType: "gemini", mediaBytes: minimumMediaBytes},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			scenario, err := scenarioByName(test.name)
			if err != nil {
				t.Fatal(err)
			}
			if len(scenario.Targets) != 1 || scenario.Targets[0].ProviderType != test.providerType {
				t.Fatalf("targets = %#v", scenario.Targets)
			}
			if scenario.MediaBytes != test.mediaBytes {
				t.Fatalf("media bytes = %d, want %d", scenario.MediaBytes, test.mediaBytes)
			}
			var body map[string]any
			if err := json.Unmarshal(scenario.requestBody(7), &body); err != nil {
				t.Fatalf("request body: %v", err)
			}
			if body["model"] != benchmarkModelAlias {
				t.Fatalf("model = %v, want %q", body["model"], benchmarkModelAlias)
			}
		})
	}
}

func TestFailureScenariosDefineExpectedTopology(t *testing.T) {
	t.Parallel()
	retry := mustScenario(t, "retry")
	if retry.Retries != 1 || len(retry.Targets) != 1 || retry.Targets[0].Mode != upstreamRetryOnce {
		t.Fatalf("retry scenario = %#v", retry)
	}

	fallback := mustScenario(t, "fallback")
	if fallback.Retries != 0 || len(fallback.Targets) != 2 || fallback.Targets[0].Mode != upstreamUnavailable || fallback.Targets[1].Mode != upstreamSuccess {
		t.Fatalf("fallback scenario = %#v", fallback)
	}

	rateLimit := mustScenario(t, "rate-limit-429")
	if len(rateLimit.Targets) != 2 || rateLimit.Targets[0].Mode != upstreamPermanentRateLimit {
		t.Fatalf("rate-limit scenario = %#v", rateLimit)
	}

	cancellation := mustScenario(t, "cancellation")
	if !cancellation.ExpectCancellation || cancellation.CancelAfter <= 0 || cancellation.Targets[0].Mode != upstreamDelayed {
		t.Fatalf("cancellation scenario = %#v", cancellation)
	}

	slow := mustScenario(t, "slow-client")
	if !slow.Stream || slow.SlowReadDelay <= 0 {
		t.Fatalf("slow-client scenario = %#v", slow)
	}
}

func TestScenarioByNameRejectsUnknownName(t *testing.T) {
	t.Parallel()
	if _, err := scenarioByName("unknown"); err == nil {
		t.Fatal("scenarioByName() error = nil")
	}
}

func mustScenario(t *testing.T, name string) performanceScenario {
	t.Helper()
	scenario, err := scenarioByName(name)
	if err != nil {
		t.Fatal(err)
	}
	return scenario
}
