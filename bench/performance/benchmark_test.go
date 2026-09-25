package main

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
)

func BenchmarkGateway(b *testing.B) {
	benchmarkScenarios(b, false)
}

func BenchmarkDirect(b *testing.B) {
	benchmarkScenarios(b, true)
}

func benchmarkScenarios(b *testing.B, direct bool) {
	for _, name := range scenarioNames() {
		scenario, _ := scenarioByName(name)
		b.Run(name, func(b *testing.B) {
			benchmarkScenario(b, scenario, direct)
		})
	}
}

func benchmarkScenario(b *testing.B, scenario performanceScenario, direct bool) {
	b.StopTimer()
	environment, err := newBenchmarkEnvironment(scenario)
	if err != nil {
		b.Fatal(err)
	}
	defer environment.Close()
	client := newLoadClient(128)
	defer client.CloseIdleConnections()
	if scenario.ExpectCancellation {
		client.Transport.(*http.Transport).DisableKeepAlives = true
	}
	url := environment.gateway.URL
	if direct {
		url = environment.directURL()
	}
	connections := &connectionCounter{}
	warmup := performRequest(context.Background(), client, url, scenario, direct, 1, connections)
	if warmup.unexpectedFailure {
		b.Fatal("benchmark warmup failed")
	}
	before := environment.snapshot()
	var sequence atomic.Uint64
	var failures atomic.Int64
	var ttftTotal atomic.Int64
	b.SetBytes(int64(len(scenario.requestBody(0))))
	b.ReportAllocs()
	b.ResetTimer()
	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			id := sequence.Add(1) + 10_000
			measurement := performRequest(context.Background(), client, url, scenario, direct, id, connections)
			if measurement.unexpectedFailure || (!measurement.completed && !measurement.expectedFailure) {
				failures.Add(1)
			}
			if measurement.ttft > 0 {
				ttftTotal.Add(int64(measurement.ttft))
			}
		}
	})
	b.StopTimer()
	if failures.Load() != 0 {
		b.Fatalf("%d benchmark requests failed", failures.Load())
	}
	if total := ttftTotal.Load(); total > 0 {
		b.ReportMetric(float64(total)/float64(b.N), "ttft-ns/op")
	}
	after := environment.snapshot().subtract(before)
	if b.N > 0 {
		b.ReportMetric(float64(after.requests)/float64(b.N), "upstream-calls/op")
	}
}
