package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
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

func TestRequireBucketAccessOwnerSkipsGrantLookupError(t *testing.T) {
	t.Parallel()
	bucketID := primitive.NewObjectID()
	ownerID := primitive.NewObjectID()
	bucketRepo := &fakeBucketLookup{bucket: &repository.Bucket{ID: bucketID, UserID: ownerID}}
	grantRepo := &fakeBucketGrantLookup{err: errors.New("grant lookup unavailable")}

	var acc *bucketAccess
	w := performRequireBucketAccess(t, bucketID, ownerID, bucketRepo, grantRepo, repository.RoleViewer, &acc)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if acc == nil || acc.Role != repository.RoleOwner {
		t.Fatalf("expected owner access, got %#v", acc)
	}
	if grantRepo.calls != 0 {
		t.Fatalf("expected grant lookup to be skipped, got %d calls", grantRepo.calls)
	}
}

func TestRequireBucketAccessMissingGrantReturnsNotFound(t *testing.T) {
	t.Parallel()
	bucketID := primitive.NewObjectID()
	ownerID := primitive.NewObjectID()
	userID := primitive.NewObjectID()
	bucketRepo := &fakeBucketLookup{bucket: &repository.Bucket{ID: bucketID, UserID: ownerID}}
	grantRepo := &fakeBucketGrantLookup{err: mongo.ErrNoDocuments}

	var acc *bucketAccess
	w := performRequireBucketAccess(t, bucketID, userID, bucketRepo, grantRepo, repository.RoleViewer, &acc)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if acc != nil {
		t.Fatalf("expected nil access, got %#v", acc)
	}
}

func TestRequireBucketAccessNilGrantRepositoryReturnsNotFound(t *testing.T) {
	t.Parallel()
	bucketID := primitive.NewObjectID()
	ownerID := primitive.NewObjectID()
	userID := primitive.NewObjectID()
	bucketRepo := &fakeBucketLookup{bucket: &repository.Bucket{ID: bucketID, UserID: ownerID}}
	var grantRepo *repository.BucketGrantRepository

	var acc *bucketAccess
	w := performRequireBucketAccess(t, bucketID, userID, bucketRepo, grantRepo, repository.RoleViewer, &acc)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if acc != nil {
		t.Fatalf("expected nil access, got %#v", acc)
	}
}

func TestRequireBucketAccessValidGrantReturnsRole(t *testing.T) {
	t.Parallel()
	bucketID := primitive.NewObjectID()
	ownerID := primitive.NewObjectID()
	userID := primitive.NewObjectID()
	bucketRepo := &fakeBucketLookup{bucket: &repository.Bucket{ID: bucketID, UserID: ownerID}}
	grantRepo := &fakeBucketGrantLookup{grant: &repository.BucketGrant{
		BucketID: bucketID,
		UserID:   userID,
		Role:     repository.RoleEditor,
	}}

	var acc *bucketAccess
	w := performRequireBucketAccess(t, bucketID, userID, bucketRepo, grantRepo, repository.RoleViewer, &acc)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if acc == nil || acc.Role != repository.RoleEditor {
		t.Fatalf("expected editor grant access, got %#v", acc)
	}
}

func TestRequireBucketAccessGrantLookupErrorReturnsInternalServerError(t *testing.T) {
	t.Parallel()
	bucketID := primitive.NewObjectID()
	ownerID := primitive.NewObjectID()
	userID := primitive.NewObjectID()
	bucketRepo := &fakeBucketLookup{bucket: &repository.Bucket{ID: bucketID, UserID: ownerID}}
	grantRepo := &fakeBucketGrantLookup{err: errors.New("database unavailable")}

	var acc *bucketAccess
	w := performRequireBucketAccess(t, bucketID, userID, bucketRepo, grantRepo, repository.RoleViewer, &acc)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if acc != nil {
		t.Fatalf("expected nil access, got %#v", acc)
	}
}

func performRequireBucketAccess(
	t *testing.T,
	bucketID primitive.ObjectID,
	userID primitive.ObjectID,
	bucketRepo bucketAccessBucketReader,
	grantRepo bucketAccessGrantReader,
	requiredRole repository.BucketRole,
	acc **bucketAccess,
) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.GET("/test/:id", setUserID(userID.Hex()), func(c *gin.Context) {
		*acc = requireBucketAccess(c, bucketRepo, grantRepo, requiredRole)
	})

	req, _ := http.NewRequest(http.MethodGet, "/test/"+bucketID.Hex(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

type fakeBucketLookup struct {
	bucket *repository.Bucket
	err    error
}

func (f *fakeBucketLookup) GetByID(context.Context, primitive.ObjectID) (*repository.Bucket, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.bucket, nil
}

type fakeBucketGrantLookup struct {
	grant *repository.BucketGrant
	err   error
	calls int
}

func (f *fakeBucketGrantLookup) GetByBucketAndUser(context.Context, primitive.ObjectID, primitive.ObjectID) (*repository.BucketGrant, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.grant, nil
}
