package ratelimit

import (
	"context"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	ReasonRequestRate      = "request_rate"
	ReasonConcurrency      = "concurrency"
	ReasonProviderCooldown = "provider_cooldown"
	ReasonQueueTimeout     = "queue_timeout"
)

type Policy struct {
	RequestsPerMinute int
	Burst             int
	MaxConcurrency    int
}

type Denial struct {
	Reason     string
	RetryAfter time.Duration
}

type Snapshot struct {
	InFlight     int
	BlockedUntil time.Time
	Acquired     uint64
	Rejected     uint64
	Waited       uint64
}

type Limiter struct {
	mu sync.Mutex

	policy Policy
	now    func() time.Time
	notify chan struct{}

	initialized  bool
	tokens       float64
	lastRefill   time.Time
	inFlight     int
	blockedUntil time.Time
	acquired     uint64
	rejected     uint64
	waited       uint64
}

type Permit struct {
	once    sync.Once
	release func()
}

func New(policy Policy) *Limiter {
	return newLimiter(policy, time.Now)
}

func NewWithClock(policy Policy, now func() time.Time) *Limiter {
	return newLimiter(policy, now)
}

func newLimiter(policy Policy, now func() time.Time) *Limiter {
	if policy.RequestsPerMinute > 0 && policy.Burst <= 0 {
		policy.Burst = 1
	}
	return &Limiter{
		policy: policy,
		now:    now,
		notify: make(chan struct{}),
	}
}

func (l *Limiter) Acquire(ctx context.Context, queueTimeout time.Duration) (*Permit, *Denial, error) {
	start := l.now()
	var deadline time.Time
	if queueTimeout > 0 {
		deadline = start.Add(queueTimeout)
	}
	waited := false

	for {
		now := l.now()
		permit, denial, changed := l.tryAcquire(now)
		if permit != nil {
			return permit, nil, nil
		}
		if queueTimeout <= 0 {
			l.recordRejection()
			return nil, denial, nil
		}
		if !waited {
			l.recordWait()
			waited = true
		}

		remaining := deadline.Sub(now)
		if remaining <= 0 {
			l.recordRejection()
			return nil, &Denial{Reason: ReasonQueueTimeout, RetryAfter: denial.RetryAfter}, nil
		}
		waitFor := denial.RetryAfter
		if waitFor <= 0 || waitFor > remaining {
			waitFor = remaining
		}

		timer := time.NewTimer(waitFor)
		select {
		case <-ctx.Done():
			stopTimer(timer)
			return nil, nil, ctx.Err()
		case <-changed:
			stopTimer(timer)
		case <-timer.C:
		}
	}
}

func (l *Limiter) tryAcquire(now time.Time) (*Permit, *Denial, <-chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill(now)
	denial := l.denial(now)
	if denial != nil {
		return nil, denial, l.notify
	}

	if l.policy.RequestsPerMinute > 0 {
		l.tokens--
	}
	l.inFlight++
	l.acquired++
	permit := &Permit{release: l.release}
	return permit, nil, nil
}

func (l *Limiter) denial(now time.Time) *Denial {
	if now.Before(l.blockedUntil) {
		return &Denial{Reason: ReasonProviderCooldown, RetryAfter: l.blockedUntil.Sub(now)}
	}
	if l.policy.MaxConcurrency > 0 && l.inFlight >= l.policy.MaxConcurrency {
		return &Denial{Reason: ReasonConcurrency}
	}
	if l.policy.RequestsPerMinute > 0 && l.tokens < 1 {
		ratePerSecond := float64(l.policy.RequestsPerMinute) / 60
		seconds := (1 - l.tokens) / ratePerSecond
		return &Denial{Reason: ReasonRequestRate, RetryAfter: time.Duration(math.Ceil(seconds * float64(time.Second)))}
	}
	return nil
}

func (l *Limiter) refill(now time.Time) {
	if l.policy.RequestsPerMinute <= 0 {
		return
	}
	if !l.initialized {
		l.initialized = true
		l.tokens = float64(l.policy.Burst)
		l.lastRefill = now
		return
	}
	if !now.After(l.lastRefill) {
		return
	}
	perSecond := float64(l.policy.RequestsPerMinute) / 60
	l.tokens = math.Min(float64(l.policy.Burst), l.tokens+now.Sub(l.lastRefill).Seconds()*perSecond)
	l.lastRefill = now
}

func (p *Permit) Release() {
	if p == nil {
		return
	}
	p.once.Do(p.release)
}

func (l *Limiter) release() {
	l.mu.Lock()
	if l.inFlight > 0 {
		l.inFlight--
	}
	l.signalLocked()
	l.mu.Unlock()
}

func (l *Limiter) BlockUntil(until time.Time) {
	l.mu.Lock()
	if until.After(l.blockedUntil) {
		l.blockedUntil = until
		l.signalLocked()
	}
	l.mu.Unlock()
}

func (l *Limiter) Observe(providerType string, status int, headers http.Header) {
	now := l.now()
	if status == http.StatusTooManyRequests || status == http.StatusServiceUnavailable {
		if delay, ok := ParseRetryAfter(headers.Get("Retry-After"), now); ok {
			l.BlockUntil(now.Add(delay))
		}
	}

	providerType = strings.ToLower(strings.TrimSpace(providerType))
	switch providerType {
	case "openai", "azure-openai", "openai-compatible", "thruplane-inference", "xai", "ollama":
		l.observeDurationReset(headers, now, "X-RateLimit-Remaining-Requests", "X-RateLimit-Reset-Requests")
		l.observeDurationReset(headers, now, "X-RateLimit-Remaining-Tokens", "X-RateLimit-Reset-Tokens")
		l.observeDurationReset(headers, now, "X-RateLimit-Remaining-Project-Tokens", "X-RateLimit-Reset-Project-Tokens")
	case "anthropic":
		for _, resource := range []string{"Requests", "Tokens", "Input-Tokens", "Output-Tokens"} {
			l.observeRFC3339Reset(headers, now,
				"Anthropic-RateLimit-"+resource+"-Remaining",
				"Anthropic-RateLimit-"+resource+"-Reset",
			)
		}
	}
}

func (l *Limiter) observeDurationReset(headers http.Header, now time.Time, remainingName, resetName string) {
	if !exhausted(headers.Get(remainingName)) {
		return
	}
	delay, err := time.ParseDuration(strings.TrimSpace(headers.Get(resetName)))
	if err == nil && delay > 0 {
		l.BlockUntil(now.Add(delay))
	}
}

func (l *Limiter) observeRFC3339Reset(headers http.Header, now time.Time, remainingName, resetName string) {
	if !exhausted(headers.Get(remainingName)) {
		return
	}
	reset, err := time.Parse(time.RFC3339, strings.TrimSpace(headers.Get(resetName)))
	if err == nil && reset.After(now) {
		l.BlockUntil(reset)
	}
}

func exhausted(value string) bool {
	remaining, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	return err == nil && remaining <= 0
}

func ParseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseFloat(value, 64); err == nil {
		if seconds < 0 || math.IsInf(seconds, 0) || math.IsNaN(seconds) {
			return 0, false
		}
		return time.Duration(math.Ceil(seconds * float64(time.Second))), true
	}
	date, err := http.ParseTime(value)
	if err != nil || !date.After(now) {
		return 0, false
	}
	return date.Sub(now), true
}

func (l *Limiter) Snapshot() Snapshot {
	l.mu.Lock()
	defer l.mu.Unlock()
	return Snapshot{
		InFlight:     l.inFlight,
		BlockedUntil: l.blockedUntil,
		Acquired:     l.acquired,
		Rejected:     l.rejected,
		Waited:       l.waited,
	}
}

func (l *Limiter) recordRejection() {
	l.mu.Lock()
	l.rejected++
	l.mu.Unlock()
}

func (l *Limiter) recordWait() {
	l.mu.Lock()
	l.waited++
	l.mu.Unlock()
}

func (l *Limiter) signalLocked() {
	close(l.notify)
	l.notify = make(chan struct{})
}

func stopTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}
