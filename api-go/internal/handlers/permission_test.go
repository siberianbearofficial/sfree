package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestRequireBucketAccessNoIDParam(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/test/:id", setUserID(validUserID()), func(c *gin.Context) {
		acc := requireBucketAccess(c, nil, nil, "viewer")
		if acc != nil {
			t.Fatal("expected nil access")
		}
	})

	req, _ := http.NewRequest(http.MethodGet, "/test/not-a-valid-oid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestRequireBucketAccessNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/test/:id", func(c *gin.Context) {
		acc := requireBucketAccess(c, nil, nil, "viewer")
		if acc != nil {
			t.Fatal("expected nil access")
		}
	})

	req, _ := http.NewRequest(http.MethodGet, "/test/"+primitive.NewObjectID().Hex(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestRequireBucketAccessInvalidUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/test/:id", setUserID("not-valid"), func(c *gin.Context) {
		acc := requireBucketAccess(c, nil, nil, "viewer")
		if acc != nil {
			t.Fatal("expected nil access")
		}
	})

	req, _ := http.NewRequest(http.MethodGet, "/test/"+primitive.NewObjectID().Hex(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
