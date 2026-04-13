package observability

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMiddleware_SetsTraceIDHeader(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	traceID := w.Header().Get(TraceIDHeader)
	if traceID == "" {
		t.Error("expected X-Trace-ID header to be set")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestMiddleware_PreservesIncomingTraceID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())
	r.GET("/ping", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(TraceIDHeader, "my-trace-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	traceID := w.Header().Get(TraceIDHeader)
	if traceID != "my-trace-123" {
		t.Errorf("expected trace ID to be preserved, got %s", traceID)
	}
}

func TestMiddleware_TraceIDInContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Middleware())

	var gotTraceID string
	r.GET("/ping", func(c *gin.Context) {
		gotTraceID = TraceIDFromContext(c.Request.Context())
		c.Status(http.StatusOK)
	})

	req, _ := http.NewRequest(http.MethodGet, "/ping", nil)
	req.Header.Set(TraceIDHeader, "ctx-trace-456")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if gotTraceID != "ctx-trace-456" {
		t.Errorf("expected trace ID in context to be ctx-trace-456, got %s", gotTraceID)
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}
