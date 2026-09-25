package circuitbreaker

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *testClock) Time() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *testClock) Advance(duration time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(duration)
	c.mu.Unlock()
}

func TestDisabledBreakerAlwaysAllows(t *testing.T) {
	breaker := NewWithClock(Policy{}, time.Now)
	for attempt := 0; attempt < 10; attempt++ {
		permit, denial := breaker.Acquire()
		if permit == nil || denial != nil {
			t.Fatalf("Acquire() = %#v, %#v; want permit", permit, denial)
		}
		permit.Failure()
	}
	if snapshot := breaker.Snapshot(); snapshot.State != StateDisabled || snapshot.ConsecutiveFailures != 0 {
		t.Fatalf("Snapshot() = %#v, want disabled", snapshot)
	}
}

func TestBreakerOpensAtThresholdAndReportsRetry(t *testing.T) {
	clock := &testClock{now: time.Unix(100, 0)}
	breaker := NewWithClock(Policy{FailureThreshold: 2, OpenDuration: 10 * time.Second}, clock.Time)

	first, _ := breaker.Acquire()
	first.Failure()
	if snapshot := breaker.Snapshot(); snapshot.State != StateClosed || snapshot.ConsecutiveFailures != 1 {
		t.Fatalf("after first failure = %#v, want closed with one failure", snapshot)
	}
	second, _ := breaker.Acquire()
	second.Failure()

	permit, denial := breaker.Acquire()
	if permit != nil || denial == nil || denial.RetryAfter != 10*time.Second {
		t.Fatalf("open Acquire() = %#v, %#v; want 10s denial", permit, denial)
	}
	if snapshot := breaker.Snapshot(); snapshot.State != StateOpen || snapshot.Opened != 1 {
		t.Fatalf("Snapshot() = %#v, want one open transition", snapshot)
	}
}

func TestSuccessResetsConsecutiveFailures(t *testing.T) {
	breaker := New(Policy{FailureThreshold: 2, OpenDuration: time.Minute})
	first, _ := breaker.Acquire()
	first.Failure()
	success, _ := breaker.Acquire()
	success.Success()
	third, _ := breaker.Acquire()
	third.Failure()
	if snapshot := breaker.Snapshot(); snapshot.State != StateClosed || snapshot.ConsecutiveFailures != 1 {
		t.Fatalf("Snapshot() = %#v, want reset followed by one failure", snapshot)
	}
}

func TestHalfOpenAllowsOneProbeAndClosesOnSuccess(t *testing.T) {
	clock := &testClock{now: time.Unix(100, 0)}
	breaker := NewWithClock(Policy{FailureThreshold: 1, OpenDuration: 5 * time.Second}, clock.Time)
	failed, _ := breaker.Acquire()
	failed.Failure()
	clock.Advance(5 * time.Second)

	probe, denial := breaker.Acquire()
	if probe == nil || denial != nil || !probe.Probe() {
		t.Fatalf("half-open Acquire() = %#v, %#v; want probe", probe, denial)
	}
	if second, denied := breaker.Acquire(); second != nil || denied == nil {
		t.Fatalf("concurrent half-open Acquire() = %#v, %#v; want denial", second, denied)
	}
	probe.Success()

	permit, denial := breaker.Acquire()
	if permit == nil || denial != nil || permit.Probe() {
		t.Fatalf("closed Acquire() = %#v, %#v; want regular permit", permit, denial)
	}
	permit.Success()
	if snapshot := breaker.Snapshot(); snapshot.State != StateClosed || snapshot.ConsecutiveFailures != 0 {
		t.Fatalf("Snapshot() = %#v, want recovered circuit", snapshot)
	}
}

func TestHalfOpenFailureReopensForFullDuration(t *testing.T) {
	clock := &testClock{now: time.Unix(100, 0)}
	breaker := NewWithClock(Policy{FailureThreshold: 1, OpenDuration: 7 * time.Second}, clock.Time)
	failed, _ := breaker.Acquire()
	failed.Failure()
	clock.Advance(7 * time.Second)
	probe, _ := breaker.Acquire()
	probe.Failure()

	_, denial := breaker.Acquire()
	if denial == nil || denial.RetryAfter != 7*time.Second {
		t.Fatalf("reopened denial = %#v, want full 7s", denial)
	}
}

func TestCanceledProbeAllowsReplacement(t *testing.T) {
	clock := &testClock{now: time.Unix(100, 0)}
	breaker := NewWithClock(Policy{FailureThreshold: 1, OpenDuration: time.Second}, clock.Time)
	failed, _ := breaker.Acquire()
	failed.Failure()
	clock.Advance(time.Second)
	probe, _ := breaker.Acquire()
	probe.Cancel()

	replacement, denial := breaker.Acquire()
	if replacement == nil || denial != nil || !replacement.Probe() {
		t.Fatalf("replacement Acquire() = %#v, %#v; want probe", replacement, denial)
	}
	replacement.Success()
}

func TestLateResultFromOlderGenerationDoesNotCloseCircuit(t *testing.T) {
	breaker := New(Policy{FailureThreshold: 2, OpenDuration: time.Minute})
	first, _ := breaker.Acquire()
	second, _ := breaker.Acquire()
	late, _ := breaker.Acquire()
	first.Failure()
	second.Failure()
	late.Success()

	if snapshot := breaker.Snapshot(); snapshot.State != StateOpen {
		t.Fatalf("Snapshot() = %#v, want stale success ignored", snapshot)
	}
}

func TestPermitCompletionIsIdempotent(t *testing.T) {
	breaker := New(Policy{FailureThreshold: 2, OpenDuration: time.Minute})
	permit, _ := breaker.Acquire()
	permit.Failure()
	permit.Failure()
	permit.Success()
	if snapshot := breaker.Snapshot(); snapshot.ConsecutiveFailures != 1 {
		t.Fatalf("Snapshot() = %#v, want one recorded completion", snapshot)
	}
}

func TestHalfOpenProbeIsConcurrencySafe(t *testing.T) {
	clock := &testClock{now: time.Unix(100, 0)}
	breaker := NewWithClock(Policy{FailureThreshold: 1, OpenDuration: time.Second}, clock.Time)
	failed, _ := breaker.Acquire()
	failed.Failure()
	clock.Advance(time.Second)

	var probes atomic.Int32
	start := make(chan struct{})
	results := make(chan *Permit, 32)
	var group sync.WaitGroup
	for index := 0; index < 32; index++ {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			permit, _ := breaker.Acquire()
			if permit != nil {
				probes.Add(1)
			}
			results <- permit
		}()
	}
	close(start)
	group.Wait()
	close(results)
	if got := probes.Load(); got != 1 {
		t.Fatalf("half-open probes = %d, want exactly one", got)
	}
	for permit := range results {
		if permit != nil {
			permit.Cancel()
		}
	}
}
