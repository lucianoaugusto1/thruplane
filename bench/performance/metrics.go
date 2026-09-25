package main

import (
	"math"
	"sort"
	"time"
)

type durationSummary struct {
	Samples int     `json:"samples"`
	MeanMS  float64 `json:"mean_ms"`
	P50MS   float64 `json:"p50_ms"`
	P95MS   float64 `json:"p95_ms"`
	P99MS   float64 `json:"p99_ms"`
	MaxMS   float64 `json:"max_ms"`
}

func summarizeDurations(samples []time.Duration) durationSummary {
	if len(samples) == 0 {
		return durationSummary{}
	}
	ordered := append([]time.Duration(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })
	var total time.Duration
	for _, sample := range ordered {
		total += sample
	}
	return durationSummary{
		Samples: len(ordered),
		MeanMS:  milliseconds(total) / float64(len(ordered)),
		P50MS:   milliseconds(nearestRank(ordered, 0.50)),
		P95MS:   milliseconds(nearestRank(ordered, 0.95)),
		P99MS:   milliseconds(nearestRank(ordered, 0.99)),
		MaxMS:   milliseconds(ordered[len(ordered)-1]),
	}
}

func nearestRank(ordered []time.Duration, percentile float64) time.Duration {
	index := int(math.Ceil(percentile*float64(len(ordered)))) - 1
	if index < 0 {
		index = 0
	}
	if index >= len(ordered) {
		index = len(ordered) - 1
	}
	return ordered[index]
}

func milliseconds(duration time.Duration) float64 {
	return float64(duration) / float64(time.Millisecond)
}

func subtractDurations(gateway, direct durationSummary) durationSummary {
	return durationSummary{
		Samples: gateway.Samples,
		MeanMS:  gateway.MeanMS - direct.MeanMS,
		P50MS:   gateway.P50MS - direct.P50MS,
		P95MS:   gateway.P95MS - direct.P95MS,
		P99MS:   gateway.P99MS - direct.P99MS,
		MaxMS:   gateway.MaxMS - direct.MaxMS,
	}
}

type connectionSummary struct {
	Requests     int64   `json:"requests"`
	New          int64   `json:"new"`
	Reused       int64   `json:"reused"`
	ReusePercent float64 `json:"reuse_percent"`
}

func summarizeConnections(requests, newConnections, reused int64) connectionSummary {
	result := connectionSummary{Requests: requests, New: newConnections, Reused: reused}
	if requests > 0 {
		result.ReusePercent = float64(reused) * 100 / float64(requests)
	}
	return result
}

func inferUpstreamConnections(requests, newConnections int64) connectionSummary {
	reused := requests - newConnections
	if reused < 0 {
		reused = 0
	}
	return summarizeConnections(requests, newConnections, reused)
}

type memorySample struct {
	TotalAlloc  uint64
	Mallocs     uint64
	HeapAlloc   uint64
	HeapObjects uint64
	NumGC       uint32
}

type memorySummary struct {
	TotalAllocBytes uint64 `json:"total_alloc_bytes"`
	Mallocs         uint64 `json:"mallocs"`
	HeapAllocBytes  uint64 `json:"heap_alloc_bytes_after"`
	HeapObjects     uint64 `json:"heap_objects_after"`
	GCCycles        uint32 `json:"gc_cycles"`
}

func subtractMemory(after, before memorySample) memorySummary {
	return memorySummary{
		TotalAllocBytes: difference(after.TotalAlloc, before.TotalAlloc),
		Mallocs:         difference(after.Mallocs, before.Mallocs),
		HeapAllocBytes:  after.HeapAlloc,
		HeapObjects:     after.HeapObjects,
		GCCycles:        uint32(difference(uint64(after.NumGC), uint64(before.NumGC))),
	}
}

func difference(after, before uint64) uint64 {
	if after < before {
		return 0
	}
	return after - before
}
