package resilience

import (
	"errors"
	"sync"
	"time"
)

// ErrCircuitOpen is returned when the circuit breaker is open.
var ErrCircuitOpen = errors.New("circuit breaker is open")

type state int

const (
	stateClosed state = iota
	stateOpen
	stateHalfOpen
)

// CircuitBreaker implements a simple circuit breaker pattern.
// After FailureThreshold consecutive failures, the circuit opens and
// rejects requests for RecoveryTimeout. After that period, one probe
// request is allowed (half-open). If it succeeds, the circuit closes;
// if it fails, the circuit re-opens.
type CircuitBreaker struct {
	mu               sync.Mutex
	state            state
	failures         int
	failureThreshold int
	recoveryTimeout  time.Duration
	openedAt         time.Time
}

// NewCircuitBreaker creates a circuit breaker with the given parameters.
// failureThreshold is the number of consecutive failures before opening.
// recoveryTimeout is how long the circuit stays open before allowing a probe.
func NewCircuitBreaker(failureThreshold int, recoveryTimeout time.Duration) *CircuitBreaker {
	if failureThreshold <= 0 {
		failureThreshold = 5
	}
	if recoveryTimeout <= 0 {
		recoveryTimeout = 30 * time.Second
	}
	return &CircuitBreaker{
		state:            stateClosed,
		failureThreshold: failureThreshold,
		recoveryTimeout:  recoveryTimeout,
	}
}

// Allow checks whether a request is allowed. Returns an error if the
// circuit is open.
func (cb *CircuitBreaker) Allow() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case stateClosed:
		return nil
	case stateOpen:
		if time.Since(cb.openedAt) >= cb.recoveryTimeout {
			cb.state = stateHalfOpen
			return nil
		}
		return ErrCircuitOpen
	case stateHalfOpen:
		// Only one probe at a time in half-open; reject additional requests.
		return ErrCircuitOpen
	}
	return nil
}

// RecordSuccess records a successful request. Resets the circuit to closed.
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = stateClosed
}

// RecordFailure records a failed request. If the failure threshold is
// reached, the circuit opens.
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	if cb.failures >= cb.failureThreshold {
		cb.state = stateOpen
		cb.openedAt = time.Now()
	}
}
