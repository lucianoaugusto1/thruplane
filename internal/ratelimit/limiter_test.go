package ratelimit

import (
	"context"
	"net/http"
	"runtime"
	"testing"
	"time"
)

func TestLimiterEnforcesRequestRateAndRefillsDeterministically(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter := newLimiter(Policy{RequestsPerMinute: 60, Burst: 2}, func() time.Time { return now })

	for i := 0; i < 2; i++ {
		permit, denied, err := limiter.Acquire(context.Background(), 0)
		if err != nil || denied != nil || permit == nil {
			t.Fatalf("Acquire(%d) = permit %v, denied %#v, error %v", i, permit, denied, err)
		}
		permit.Release()
	}
	if permit, denied, err := limiter.Acquire(context.Background(), 0); err != nil || permit != nil || denied == nil {
		t.Fatalf("third Acquire() = permit %v, denied %#v, error %v", permit, denied, err)
	} else if denied.Reason != ReasonRequestRate || denied.RetryAfter != time.Second {
		t.Fatalf("third denial = %#v, want request-rate wait of 1s", denied)
	}

	now = now.Add(time.Second)
	permit, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied != nil || permit == nil {
		t.Fatalf("Acquire after refill = permit %v, denied %#v, error %v", permit, denied, err)
	}
	permit.Release()
}

func TestLimiterDoesNotConsumeRateTokenWhileConcurrencyIsFull(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter := newLimiter(Policy{RequestsPerMinute: 60, Burst: 1, MaxConcurrency: 1}, func() time.Time { return now })
	first, _, _ := limiter.Acquire(context.Background(), 0)

	now = now.Add(time.Second)
	if permit, denied, err := limiter.Acquire(context.Background(), 0); err != nil || permit != nil || denied == nil || denied.Reason != ReasonConcurrency {
		t.Fatalf("Acquire while full = permit %v, denied %#v, error %v", permit, denied, err)
	}
	first.Release()

	second, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied != nil || second == nil {
		t.Fatalf("Acquire after release = permit %v, denied %#v, error %v; rate token was consumed while full", second, denied, err)
	}
	second.Release()
}

func TestLimiterSharesConcurrencyAndReleaseIsIdempotent(t *testing.T) {
	limiter := New(Policy{MaxConcurrency: 1})
	first, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied != nil || first == nil {
		t.Fatalf("first Acquire() = permit %v, denied %#v, error %v", first, denied, err)
	}

	if permit, denied, err := limiter.Acquire(context.Background(), 0); err != nil || permit != nil || denied == nil || denied.Reason != ReasonConcurrency {
		t.Fatalf("concurrent Acquire() = permit %v, denied %#v, error %v", permit, denied, err)
	}
	first.Release()
	first.Release()

	second, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied != nil || second == nil {
		t.Fatalf("Acquire after release = permit %v, denied %#v, error %v", second, denied, err)
	}
	second.Release()
	if got := limiter.Snapshot().InFlight; got != 0 {
		t.Fatalf("in flight = %d, want 0", got)
	}
}

func TestLimiterWaitStopsOnCancellation(t *testing.T) {
	limiter := New(Policy{MaxConcurrency: 1})
	first, _, _ := limiter.Acquire(context.Background(), 0)
	defer first.Release()

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, _, err := limiter.Acquire(ctx, time.Minute)
		done <- err
	}()
	cancel()

	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Acquire() error = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Acquire() did not stop after cancellation")
	}
}

func TestLimiterQueuedAcquireWakesOnRelease(t *testing.T) {
	limiter := New(Policy{MaxConcurrency: 1})
	first, _, _ := limiter.Acquire(context.Background(), 0)
	result := make(chan *Permit, 1)
	errors := make(chan error, 1)
	go func() {
		permit, denied, err := limiter.Acquire(context.Background(), time.Second)
		if err != nil {
			errors <- err
			return
		}
		if denied != nil {
			errors <- context.DeadlineExceeded
			return
		}
		result <- permit
	}()

	deadline := time.Now().Add(time.Second)
	for limiter.Snapshot().Waited == 0 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if limiter.Snapshot().Waited == 0 {
		t.Fatal("second acquire did not enter the queue")
	}
	first.Release()

	select {
	case permit := <-result:
		permit.Release()
	case err := <-errors:
		t.Fatalf("queued Acquire() error = %v", err)
	case <-time.After(time.Second):
		t.Fatal("queued Acquire() did not wake after release")
	}
}

func TestLimiterQueueTimeoutIsExplicit(t *testing.T) {
	limiter := New(Policy{MaxConcurrency: 1})
	first, _, _ := limiter.Acquire(context.Background(), 0)
	defer first.Release()

	permit, denied, err := limiter.Acquire(context.Background(), 10*time.Millisecond)
	if err != nil || permit != nil || denied == nil || denied.Reason != ReasonQueueTimeout {
		t.Fatalf("Acquire() = permit %v, denied %#v, error %v; want queue timeout", permit, denied, err)
	}
}

func TestLimiterObservesProviderResetHeaders(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter := newLimiter(Policy{}, func() time.Time { return now })

	openAI := http.Header{
		"X-Ratelimit-Remaining-Tokens": []string{"0"},
		"X-Ratelimit-Reset-Tokens":     []string{"2s"},
	}
	limiter.Observe("openai", http.StatusOK, openAI)
	_, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied == nil || denied.Reason != ReasonProviderCooldown || denied.RetryAfter != 2*time.Second {
		t.Fatalf("OpenAI cooldown denial = %#v, error %v", denied, err)
	}

	now = now.Add(2 * time.Second)
	permit, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied != nil || permit == nil {
		t.Fatalf("Acquire after OpenAI reset = permit %v, denied %#v, error %v", permit, denied, err)
	}
	permit.Release()

	anthropic := http.Header{
		"Anthropic-Ratelimit-Requests-Remaining": []string{"0"},
		"Anthropic-Ratelimit-Requests-Reset":     []string{now.Add(3 * time.Second).Format(time.RFC3339)},
	}
	limiter.Observe("anthropic", http.StatusOK, anthropic)
	_, denied, err = limiter.Acquire(context.Background(), 0)
	if err != nil || denied == nil || denied.RetryAfter != 3*time.Second {
		t.Fatalf("Anthropic cooldown denial = %#v, error %v", denied, err)
	}
}

func TestLimiterIgnoresAvailableOrMalformedResetHeaders(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter := newLimiter(Policy{}, func() time.Time { return now })
	limiter.Observe("openai", http.StatusOK, http.Header{
		"X-Ratelimit-Remaining-Requests": []string{"1"},
		"X-Ratelimit-Reset-Requests":     []string{"10s"},
		"X-Ratelimit-Remaining-Tokens":   []string{"0"},
		"X-Ratelimit-Reset-Tokens":       []string{"not-a-duration"},
	})

	permit, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied != nil || permit == nil {
		t.Fatalf("Acquire() = permit %v, denied %#v, error %v; malformed/available headers must not block", permit, denied, err)
	}
	permit.Release()
}

func TestLimiterObservesRetryAfterAndNeverShortensCooldown(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	limiter := newLimiter(Policy{}, func() time.Time { return now })
	limiter.Observe("gemini", http.StatusTooManyRequests, http.Header{"Retry-After": []string{"4"}})
	limiter.Observe("gemini", http.StatusTooManyRequests, http.Header{"Retry-After": []string{"1"}})

	_, denied, err := limiter.Acquire(context.Background(), 0)
	if err != nil || denied == nil || denied.RetryAfter != 4*time.Second {
		t.Fatalf("cooldown denial = %#v, error %v, want 4s", denied, err)
	}
}

func TestParseRetryAfterSupportsSecondsAndHTTPDate(t *testing.T) {
	now := time.Date(2026, 9, 24, 12, 0, 0, 0, time.UTC)
	if got, ok := ParseRetryAfter("1.5", now); !ok || got != 1500*time.Millisecond {
		t.Fatalf("seconds Retry-After = %s, %v; want 1.5s", got, ok)
	}
	date := now.Add(7 * time.Second).Format(http.TimeFormat)
	if got, ok := ParseRetryAfter(date, now); !ok || got != 7*time.Second {
		t.Fatalf("date Retry-After = %s, %v; want 7s", got, ok)
	}
	for _, value := range []string{"", "-1", "tomorrow"} {
		if got, ok := ParseRetryAfter(value, now); ok || got != 0 {
			t.Errorf("ParseRetryAfter(%q) = %s, %v, want invalid", value, got, ok)
		}
	}
}
