package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestSetupRouter(t *testing.T) {
	r := SetupRouter(nil, nil)
	endpoints := []string{"/readyz", "/healthz", "/publication/ready"}
	for _, e := range endpoints {
		req, _ := http.NewRequest(http.MethodGet, e, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", e, w.Code)
		}
	}
}

func TestSetupRouterNilMongoRouteSet(t *testing.T) {
	r := SetupRouter(nil, nil)

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/readyz"},
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/publication/ready"},
		{http.MethodGet, "/dbz"},
		{http.MethodGet, "/swagger/*any"},
		{http.MethodGet, "/metrics"},
	}
	for _, expected := range expectedRoutes {
		if !hasRoute(r, expected.method, expected.path) {
			t.Fatalf("expected %s %s to be registered", expected.method, expected.path)
		}
	}

	unexpectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/buckets"},
		{http.MethodGet, "/share/:token"},
		{http.MethodHead, "/api/s3/:bucket/*object"},
	}
	for _, unexpected := range unexpectedRoutes {
		if hasRoute(r, unexpected.method, unexpected.path) {
			t.Fatalf("did not expect %s %s to be registered", unexpected.method, unexpected.path)
		}
	}
}

func hasRoute(r *gin.Engine, method, path string) bool {
	for _, route := range r.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}
