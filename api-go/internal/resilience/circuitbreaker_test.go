package resilience

import (
	"testing"
	"time"
)

type fakeClock struct {
	t time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 4, 22, 0, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.t = c.t.Add(d)
}

func TestCircuitBreakerStartsClosed(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected closed circuit to allow, got %v", err)
	}
}

func TestCircuitBreakerOpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	for i := 0; i < 3; i++ {
		cb.RecordFailure()
	}

	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}
}

func TestCircuitBreakerDoesNotOpenBelowThreshold(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()

	if err := cb.Allow(); err != nil {
		t.Fatalf("expected circuit to still be closed, got %v", err)
	}
}

func TestCircuitBreakerResetOnSuccess(t *testing.T) {
	cb := NewCircuitBreaker(3, 100*time.Millisecond)

	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()

	// Only 2 consecutive failures since last success, should be open=false.
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected circuit to be closed after reset, got %v", err)
	}
}

func TestCircuitBreakerHalfOpen(t *testing.T) {
	clock := newFakeClock()
	cb := newCircuitBreakerWithClock(2, 50*time.Millisecond, clock.Now)

	cb.RecordFailure()
	cb.RecordFailure()

	// Circuit is open.
	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Fatalf("expected ErrCircuitOpen, got %v", err)
	}

	clock.Advance(50 * time.Millisecond)

	// First call should be allowed (half-open probe).
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected half-open to allow probe, got %v", err)
	}

	// Second call during half-open should be rejected.
	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Fatalf("expected half-open to reject additional requests, got %v", err)
	}
}

func TestCircuitBreakerClosesAfterHalfOpenSuccess(t *testing.T) {
	clock := newFakeClock()
	cb := newCircuitBreakerWithClock(2, 50*time.Millisecond, clock.Now)

	cb.RecordFailure()
	cb.RecordFailure()

	clock.Advance(50 * time.Millisecond)

	// Half-open probe.
	_ = cb.Allow()
	cb.RecordSuccess()

	// Circuit should be closed again.
	if err := cb.Allow(); err != nil {
		t.Fatalf("expected circuit to be closed after successful probe, got %v", err)
	}
}

func TestCircuitBreakerReopensAfterHalfOpenFailure(t *testing.T) {
	clock := newFakeClock()
	cb := newCircuitBreakerWithClock(2, 50*time.Millisecond, clock.Now)

	cb.RecordFailure()
	cb.RecordFailure()

	clock.Advance(50 * time.Millisecond)

	// Half-open probe.
	_ = cb.Allow()
	cb.RecordFailure()

	// Circuit should be open again.
	if err := cb.Allow(); err != ErrCircuitOpen {
		t.Fatalf("expected circuit to reopen after failed probe, got %v", err)
	}
}
