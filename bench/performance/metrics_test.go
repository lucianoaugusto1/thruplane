package main

import (
	"math"
	"testing"
	"time"
)

func TestSummarizeDurationsUsesNearestRank(t *testing.T) {
	t.Parallel()
	got := summarizeDurations([]time.Duration{
		100 * time.Millisecond,
		2 * time.Millisecond,
		4 * time.Millisecond,
		1 * time.Millisecond,
		3 * time.Millisecond,
	})

	if got.Samples != 5 || got.P50MS != 3 || got.P95MS != 100 || got.P99MS != 100 || got.MaxMS != 100 {
		t.Fatalf("summary = %#v", got)
	}
	if got.MeanMS != 22 {
		t.Fatalf("mean = %v ms, want 22", got.MeanMS)
	}
}

func TestDurationDifferencePreservesMeasurementNoise(t *testing.T) {
	t.Parallel()
	gateway := durationSummary{MeanMS: 5, P50MS: 4, P95MS: 8, P99MS: 10, MaxMS: 12}
	direct := durationSummary{MeanMS: 2, P50MS: 5, P95MS: 3, P99MS: 4, MaxMS: 6}

	got := subtractDurations(gateway, direct)
	if got.MeanMS != 3 || got.P50MS != -1 || got.P95MS != 5 || got.P99MS != 6 || got.MaxMS != 6 {
		t.Fatalf("difference = %#v", got)
	}
}

func TestSummarizeConnections(t *testing.T) {
	t.Parallel()
	got := summarizeConnections(100, 5, 95)
	if got.Requests != 100 || got.New != 5 || got.Reused != 95 || got.ReusePercent != 95 {
		t.Fatalf("connections = %#v", got)
	}

	inferred := inferUpstreamConnections(40, 4)
	if inferred.Reused != 36 || inferred.ReusePercent != 90 {
		t.Fatalf("inferred connections = %#v", inferred)
	}
}

func TestMemoryDifference(t *testing.T) {
	t.Parallel()
	before := memorySample{TotalAlloc: 100, Mallocs: 20, HeapAlloc: 80, HeapObjects: 10, NumGC: 2}
	after := memorySample{TotalAlloc: 900, Mallocs: 70, HeapAlloc: 60, HeapObjects: 8, NumGC: 5}

	got := subtractMemory(after, before)
	if got.TotalAllocBytes != 800 || got.Mallocs != 50 || got.HeapAllocBytes != 60 || got.HeapObjects != 8 || got.GCCycles != 3 {
		t.Fatalf("memory delta = %#v", got)
	}
}

func TestSummariesHandleEmptySamples(t *testing.T) {
	t.Parallel()
	if got := summarizeDurations(nil); got != (durationSummary{}) {
		t.Fatalf("empty duration summary = %#v", got)
	}
	connections := summarizeConnections(0, 0, 0)
	if connections.ReusePercent != 0 || math.IsNaN(connections.ReusePercent) {
		t.Fatalf("empty connection summary = %#v", connections)
	}
}
