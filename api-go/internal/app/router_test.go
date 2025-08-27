package app

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
