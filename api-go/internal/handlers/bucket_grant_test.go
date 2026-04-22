package handlers

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCreateGrantNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets/:id/grants", CreateGrant(nil, nil, nil))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/buckets/"+primitive.NewObjectID().Hex()+"/grants", map[string]string{"username": "alice", "role": "viewer"})

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListGrantsNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/buckets/:id/grants", ListGrants(nil, nil, nil))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/buckets/"+primitive.NewObjectID().Hex()+"/grants", nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestUpdateGrantNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.PATCH("/buckets/:id/grants/:grant_id", UpdateGrant(nil, nil))

	w := serveHandlerTestRequest(t, r, http.MethodPatch,
		"/buckets/"+primitive.NewObjectID().Hex()+"/grants/"+primitive.NewObjectID().Hex(),
		map[string]string{"role": "editor"},
	)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteGrantNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/buckets/:id/grants/:grant_id", DeleteGrant(nil, nil))

	w := serveHandlerTestRequest(t, r, http.MethodDelete,
		"/buckets/"+primitive.NewObjectID().Hex()+"/grants/"+primitive.NewObjectID().Hex(),
		nil,
	)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteGrantInvalidGrantID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/buckets/:id/grants/:grant_id",
		setUserID(validUserID()),
		DeleteGrant(nil, nil),
	)

	w := serveHandlerTestRequest(t, r, http.MethodDelete,
		"/buckets/"+primitive.NewObjectID().Hex()+"/grants/not-valid",
		nil,
	)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
