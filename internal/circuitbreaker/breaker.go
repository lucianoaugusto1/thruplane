package circuitbreaker

import (
	"sync"
	"time"
)

type State string

const (
	StateDisabled State = "disabled"
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

type Policy struct {
	FailureThreshold int
	OpenDuration     time.Duration
}

type Denial struct {
	State      State
	RetryAfter time.Duration
}

type Snapshot struct {
	State               State
	ConsecutiveFailures int
	OpenUntil           time.Time
	Opened              uint64
	Rejected            uint64
}

type Breaker struct {
	mu sync.Mutex

	policy Policy
	now    func() time.Time

	consecutiveFailures int
	openUntil           time.Time
	probeInFlight       bool
	generation          uint64
	opened              uint64
	rejected            uint64
}

type Permit struct {
	once       sync.Once
	breaker    *Breaker
	generation uint64
	probe      bool
}

type outcome uint8

const (
	outcomeCanceled outcome = iota
	outcomeSuccess
	outcomeFailure
)

func New(policy Policy) *Breaker {
	return NewWithClock(policy, time.Now)
}

func NewWithClock(policy Policy, now func() time.Time) *Breaker {
	return &Breaker{policy: policy, now: now}
}

func (b *Breaker) Acquire() (*Permit, *Denial) {
	if b.policy.FailureThreshold <= 0 {
		return &Permit{}, nil
	}

	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.openUntil.IsZero() {
		return b.permitLocked(false), nil
	}
	if now.Before(b.openUntil) {
		b.rejected++
		return nil, &Denial{State: StateOpen, RetryAfter: b.openUntil.Sub(now)}
	}
	if b.probeInFlight {
		b.rejected++
		return nil, &Denial{State: StateHalfOpen}
	}
	b.probeInFlight = true
	return b.permitLocked(true), nil
}

func (b *Breaker) Snapshot() Snapshot {
	if b.policy.FailureThreshold <= 0 {
		return Snapshot{State: StateDisabled}
	}

	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()
	return Snapshot{
		State:               b.stateLocked(now),
		ConsecutiveFailures: b.consecutiveFailures,
		OpenUntil:           b.openUntil,
		Opened:              b.opened,
		Rejected:            b.rejected,
	}
}

func (p *Permit) Probe() bool {
	return p != nil && p.probe
}

func (p *Permit) Success() {
	p.complete(outcomeSuccess)
}

func (p *Permit) Failure() {
	p.complete(outcomeFailure)
}

func (p *Permit) Cancel() {
	p.complete(outcomeCanceled)
}

func (p *Permit) complete(result outcome) {
	if p == nil {
		return
	}
	p.once.Do(func() {
		if p.breaker != nil {
			p.breaker.complete(p.generation, p.probe, result)
		}
	})
}

func (b *Breaker) permitLocked(probe bool) *Permit {
	return &Permit{breaker: b, generation: b.generation, probe: probe}
}

func (b *Breaker) complete(generation uint64, probe bool, result outcome) {
	now := b.now()
	b.mu.Lock()
	defer b.mu.Unlock()
	if generation != b.generation {
		return
	}

	switch result {
	case outcomeCanceled:
		if probe {
			b.probeInFlight = false
		}
	case outcomeSuccess:
		b.consecutiveFailures = 0
		if probe {
			b.openUntil = time.Time{}
			b.probeInFlight = false
			b.generation++
		}
	case outcomeFailure:
		if probe {
			b.openLocked(now)
			return
		}
		b.consecutiveFailures++
		if b.consecutiveFailures >= b.policy.FailureThreshold {
			b.openLocked(now)
		}
	}
}

func (b *Breaker) openLocked(now time.Time) {
	b.consecutiveFailures = b.policy.FailureThreshold
	b.openUntil = now.Add(b.policy.OpenDuration)
	b.probeInFlight = false
	b.opened++
	b.generation++
}

func (b *Breaker) stateLocked(now time.Time) State {
	if b.openUntil.IsZero() {
		return StateClosed
	}
	if now.Before(b.openUntil) {
		return StateOpen
	}
	return StateHalfOpen
}
