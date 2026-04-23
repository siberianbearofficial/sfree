package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupRouter(cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(Middleware(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func setupRouterWithAuth(cfg Config) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if c.GetHeader("Authorization") != "" {
			c.Set("userID", "test-user-123")
		}
		c.Next()
	})
	r.Use(Middleware(cfg))
	r.GET("/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func setupProtectedRouter(cfg Config, auth gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	limits := NewLimiters(cfg)
	r.GET("/test", limits.PreAuthIPMiddleware(), auth, limits.IdentityMiddleware(), func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})
	return r
}

func TestMiddlewareAllowsNormalTraffic(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 10, PerKeyReqsPerMin: 100})
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestMiddlewareReturns429WhenExceeded(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 2, PerKeyReqsPerMin: 100})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestMiddlewareAuthenticatedUsesKeyLimiter(t *testing.T) {
	r := setupRouterWithAuth(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 5})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.2:1234"
		req.Header.Set("Authorization", "Bearer test-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("authenticated request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	req.Header.Set("Authorization", "Bearer test-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for exceeded auth limit, got %d", w.Code)
	}
}

func TestProtectedRouteAuthenticatedUsesIdentityAfterAuth(t *testing.T) {
	r := setupProtectedRouter(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 3}, func(c *gin.Context) {
		c.Set("userID", "route-user-123")
		c.Next()
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.9:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("authenticated request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.9:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for exceeded identity limit, got %d", w.Code)
	}
}

func TestProtectedRouteInvalidAuthUsesIPLimit(t *testing.T) {
	r := setupProtectedRouter(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 100}, func(c *gin.Context) {
		c.AbortWithStatus(http.StatusUnauthorized)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected first invalid auth request to return 401, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.10:1234"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated invalid auth to return 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestProtectedRouteFakeAuthHeadersDoNotBypassIPLimit(t *testing.T) {
	r := setupProtectedRouter(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 100}, func(c *gin.Context) {
		if c.GetHeader("Authorization") == "Bearer valid-token" {
			c.Set("userID", "route-user-123")
			c.Next()
			return
		}
		c.AbortWithStatus(http.StatusUnauthorized)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.12:1234"
	req.Header.Set("Authorization", "Bearer fake-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected first fake auth request to return 401, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.12:1234"
	req.Header.Set("Authorization", "Bearer different-fake-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated fake auth to return 429, got %d", w.Code)
	}
}

func TestProtectedRouteS3AccessKeyUsesIdentityLimit(t *testing.T) {
	r := setupProtectedRouter(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 2}, func(c *gin.Context) {
		c.Set("accessKey", "bucket-key-123")
		c.Next()
	})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.11:1234"
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("s3 request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.11:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 for exceeded s3 identity limit, got %d", w.Code)
	}
}

func TestMiddlewareFakeAuthHeaderDoesNotBypassIPLimit(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 2, PerKeyReqsPerMin: 100})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.5:1234"
		req.Header.Set("Authorization", "Bearer fake-token")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}

	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.5:1234"
	req.Header.Set("Authorization", "Bearer different-fake-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, fake auth header should not bypass IP limit, got %d", w.Code)
	}
}

func TestNoRouteRequestsUseIPLimit(t *testing.T) {
	r := gin.New()
	limits := NewLimiters(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 100})
	r.NoRoute(limits.IPMiddleware(), func(c *gin.Context) {
		c.Status(http.StatusNotFound)
	})

	req := httptest.NewRequest("GET", "/missing-one", nil)
	req.RemoteAddr = "10.0.0.13:1234"
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected first missing route to return 404, got %d", w.Code)
	}

	req = httptest.NewRequest("GET", "/missing-two", nil)
	req.RemoteAddr = "10.0.0.13:1234"
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("expected repeated missing route to return 429, got %d", w.Code)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header")
	}
}

func TestMiddlewareDifferentIPsIndependent(t *testing.T) {
	r := setupRouter(Config{PerIPReqsPerMin: 1, PerKeyReqsPerMin: 100})

	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.RemoteAddr = "10.0.0.3:1234"
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("IP1 first request: expected 200, got %d", w1.Code)
	}

	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.RemoteAddr = "10.0.0.4:1234"
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("IP2 first request: expected 200, got %d", w2.Code)
	}
}
