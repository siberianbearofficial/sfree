package handlers

import (
	"net/http"
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

			w := serveHandlerTestRequest(t, r, http.MethodGet, "/me", nil)

			if w.Code != http.StatusUnauthorized {
				t.Fatalf("expected 401, got %d", w.Code)
			}
		})
	}
}

func TestRouteObjectIDRejectsShareLinkFileID(t *testing.T) {
	t.Parallel()
	c, w := newHandlerTestContext(t, http.MethodPost, "/buckets/"+primitive.NewObjectID().Hex()+"/files/not-a-valid-oid/share", nil,
		testRouteParam{Key: "file_id", Value: "not-a-valid-oid"},
	)
	if _, ok := routeObjectID(c, "file_id"); ok {
		t.Fatal("expected file id parse to fail")
	}

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

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/buckets/"+primitive.NewObjectID().Hex()+"/files/"+primitive.NewObjectID().Hex()+"/share", nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}
