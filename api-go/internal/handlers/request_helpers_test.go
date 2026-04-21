package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestAuthenticatedUserIDRejectsMissingAndInvalidUserID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		userID string
	}{
		{name: "missing"},
		{name: "invalid", userID: "not-a-valid-oid"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			r := gin.New()
			handlers := []gin.HandlerFunc{}
			if tt.userID != "" {
				handlers = append(handlers, setUserID(tt.userID))
			}
			handlers = append(handlers, func(c *gin.Context) {
				if _, ok := authenticatedUserID(c); ok {
					t.Fatal("expected user id parse to fail")
				}
			})
			r.GET("/me", handlers...)

			req, _ := http.NewRequest(http.MethodGet, "/me", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", w.Code)
			}
		})
	}
}

func TestRouteObjectIDRejectsShareLinkFileID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets/:id/files/:file_id/share", func(c *gin.Context) {
		if _, ok := routeObjectID(c, "file_id"); ok {
			t.Fatal("expected file id parse to fail")
		}
	})

	req, _ := http.NewRequest(http.MethodPost, "/buckets/"+primitive.NewObjectID().Hex()+"/files/not-a-valid-oid/share", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateShareLinkRejectsInvalidAuthenticatedUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST(
		"/buckets/:id/files/:file_id/share",
		setUserID("not-a-valid-oid"),
		CreateShareLink(&repository.BucketRepository{}, &repository.FileRepository{}, &repository.ShareLinkRepository{}, nil),
	)

	req, _ := http.NewRequest(
		http.MethodPost,
		"/buckets/"+primitive.NewObjectID().Hex()+"/files/"+primitive.NewObjectID().Hex()+"/share",
		nil,
	)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
