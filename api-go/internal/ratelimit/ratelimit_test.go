package ratelimit

import (
	"testing"
	"time"
)

func TestLimiterAllow(t *testing.T) {
	l := NewLimiter(60) // 60 req/min = 1 req/sec

	// First request should always pass.
	ok, _ := l.Allow("test")
	if !ok {
		t.Fatal("first request should be allowed")
	}
}

func TestLimiterBurst(t *testing.T) {
	l := NewLimiter(5) // 5 req/min burst of 5

	for i := 0; i < 5; i++ {
		ok, _ := l.Allow("burst")
		if !ok {
			t.Fatalf("request %d within burst should be allowed", i+1)
		}
	}

	ok, retryAfter := l.Allow("burst")
	if ok {
		t.Fatal("request beyond burst should be denied")
	}
	if retryAfter <= 0 {
		t.Fatal("retryAfter should be positive when denied")
	}
}

func TestLimiterSeparateKeys(t *testing.T) {
	l := NewLimiter(1) // 1 req/min, burst of 1

	ok, _ := l.Allow("a")
	if !ok {
		t.Fatal("key 'a' first request should be allowed")
	}

	ok, _ = l.Allow("b")
	if !ok {
		t.Fatal("key 'b' first request should be allowed (separate bucket)")
	}
}

func TestLimiterReservationCancelRestoresToken(t *testing.T) {
	l := NewLimiter(1)

	reservation := l.reserve("reservation")
	if !reservation.allowed {
		t.Fatal("first reservation should be allowed")
	}

	ok, _ := l.Allow("reservation")
	if ok {
		t.Fatal("second request should be denied before reservation is cancelled")
	}

	reservation.cancel()
	ok, _ = l.Allow("reservation")
	if !ok {
		t.Fatal("request should be allowed after reservation is cancelled")
	}
}

func TestLimiterRefill(t *testing.T) {
	l := NewLimiter(600) // 600 req/min = 10 req/sec

	// Exhaust all tokens.
	for i := 0; i < 600; i++ {
		l.Allow("refill")
	}

	ok, _ := l.Allow("refill")
	if ok {
		t.Fatal("should be denied after exhausting tokens")
	}

	// Wait enough time for at least 1 token to refill (need 0.1s for 1 token at 10/sec).
	time.Sleep(150 * time.Millisecond)

	ok, _ = l.Allow("refill")
	if !ok {
		t.Fatal("should be allowed after token refill")
	}
}

func TestCleanup(t *testing.T) {
	l := NewLimiter(60)
	l.Allow("stale")
	l.Allow("fresh")

	// Manually backdate the "stale" entry.
	l.mu.Lock()
	l.buckets["stale"].lastTime = time.Now().Add(-1 * time.Hour)
	l.mu.Unlock()

	l.Cleanup(30 * time.Minute)

	l.mu.Lock()
	_, staleExists := l.buckets["stale"]
	_, freshExists := l.buckets["fresh"]
	l.mu.Unlock()

	if staleExists {
		t.Fatal("stale entry should have been cleaned up")
	}
	if !freshExists {
		t.Fatal("fresh entry should still exist")
	}
}
