package ratelimit

import (
	"fmt"
	"math"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Config holds rate-limiting parameters.
type Config struct {
	PerIPReqsPerMin  int // rate limit for unauthenticated requests per IP (default 60)
	PerKeyReqsPerMin int // rate limit for authenticated requests per user (default 600)
	CleanupInterval  time.Duration
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		PerIPReqsPerMin:  60,
		PerKeyReqsPerMin: 600,
		CleanupInterval:  10 * time.Minute,
	}
}

type Limiters struct {
	ipLimiter  *Limiter
	keyLimiter *Limiter
}

func NewLimiters(cfg Config) *Limiters {
	if cfg.PerIPReqsPerMin <= 0 {
		cfg.PerIPReqsPerMin = 60
	}
	if cfg.PerKeyReqsPerMin <= 0 {
		cfg.PerKeyReqsPerMin = 600
	}
	if cfg.CleanupInterval <= 0 {
		cfg.CleanupInterval = 10 * time.Minute
	}

	ipLimiter := NewLimiter(cfg.PerIPReqsPerMin)
	keyLimiter := NewLimiter(cfg.PerKeyReqsPerMin)

	limiters := &Limiters{
		ipLimiter:  ipLimiter,
		keyLimiter: keyLimiter,
	}

	go func() {
		ticker := time.NewTicker(cfg.CleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			ipLimiter.Cleanup(cfg.CleanupInterval * 3)
			keyLimiter.Cleanup(cfg.CleanupInterval * 3)
		}
	}()

	return limiters
}

// Middleware preserves the single-middleware behavior for callers that already
// run authentication before rate limiting.
func Middleware(cfg Config) gin.HandlerFunc {
	return NewLimiters(cfg).Middleware()
}

func (l *Limiters) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c, l.identityOrIPKey(c)) {
			return
		}
		c.Next()
	}
}

func (l *Limiters) IPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.allow(c, limitKey{limiter: l.ipLimiter, value: c.ClientIP()}) {
			return
		}
		c.Next()
	}
}

func (l *Limiters) PreAuthIPMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		reservation := l.ipLimiter.reserve(c.ClientIP())
		if !reservation.allowed {
			abortTooManyRequests(c, reservation.retryAfter)
			return
		}

		c.Next()

		if authenticatedIdentityKey(c) != "" {
			reservation.cancel()
		}
	}
}

func (l *Limiters) IdentityMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		key := authenticatedIdentityKey(c)
		if key == "" {
			key = "ip:" + c.ClientIP()
			if !l.allow(c, limitKey{limiter: l.ipLimiter, value: key}) {
				return
			}
			c.Next()
			return
		}
		if !l.allow(c, limitKey{limiter: l.keyLimiter, value: key}) {
			return
		}
		c.Next()
	}
}

type limitKey struct {
	limiter *Limiter
	value   string
}

func (l *Limiters) identityOrIPKey(c *gin.Context) limitKey {
	if key := authenticatedIdentityKey(c); key != "" {
		return limitKey{limiter: l.keyLimiter, value: key}
	}
	return limitKey{limiter: l.ipLimiter, value: c.ClientIP()}
}

func (l *Limiters) allow(c *gin.Context, key limitKey) bool {
	ok, retryAfter := key.limiter.Allow(key.value)
	if !ok {
		abortTooManyRequests(c, retryAfter)
		return false
	}
	return true
}

func authenticatedIdentityKey(c *gin.Context) string {
	if userID := c.GetString("userID"); userID != "" {
		return "user:" + userID
	}
	if accessKey := c.GetString("accessKey"); accessKey != "" {
		return "s3:" + accessKey
	}
	return ""
}

func abortTooManyRequests(c *gin.Context, retryAfter float64) {
	retrySeconds := int(math.Ceil(retryAfter))
	if retrySeconds < 1 {
		retrySeconds = 1
	}
	c.Header("Retry-After", fmt.Sprintf("%d", retrySeconds))
	c.JSON(http.StatusTooManyRequests, gin.H{
		"error": "rate limit exceeded",
	})
	c.Abort()
}
