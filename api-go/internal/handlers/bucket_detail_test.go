package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type bucketDetailBucketReader struct {
	bucket *repository.Bucket
	err    error
}

func (r bucketDetailBucketReader) GetByID(context.Context, primitive.ObjectID) (*repository.Bucket, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.bucket, nil
}

type bucketDetailGrantReader struct {
	grant *repository.BucketGrant
	err   error
}

func (r bucketDetailGrantReader) GetByBucketAndUser(context.Context, primitive.ObjectID, primitive.ObjectID) (*repository.BucketGrant, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.grant, nil
}

func TestGetBucketOwnerAccess(t *testing.T) {
	t.Parallel()

	ownerID := primitive.NewObjectID()
	bucket := bucketDetailTestBucket(ownerID)
	resp := performGetBucketRequest(t, ownerID, bucketDetailBucketReader{bucket: bucket}, nil, bucket.ID.Hex())

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var body bucketResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.ID != bucket.ID.Hex() || body.Key != bucket.Key || body.AccessKey != bucket.AccessKey {
		t.Fatalf("unexpected bucket response: %+v", body)
	}
	if body.Role != string(repository.RoleOwner) || body.Shared {
		t.Fatalf("expected owner non-shared bucket, got role=%q shared=%v", body.Role, body.Shared)
	}
}

func TestGetBucketSharedAccess(t *testing.T) {
	t.Parallel()

	ownerID := primitive.NewObjectID()
	viewerID := primitive.NewObjectID()
	bucket := bucketDetailTestBucket(ownerID)
	grant := &repository.BucketGrant{
		BucketID: bucket.ID,
		UserID:   viewerID,
		Role:     repository.RoleViewer,
	}
	resp := performGetBucketRequest(
		t,
		viewerID,
		bucketDetailBucketReader{bucket: bucket},
		bucketDetailGrantReader{grant: grant},
		bucket.ID.Hex(),
	)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var body bucketResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if body.Role != string(repository.RoleViewer) || !body.Shared {
		t.Fatalf("expected shared viewer bucket, got role=%q shared=%v", body.Role, body.Shared)
	}
}

func TestGetBucketMissingOrInaccessible(t *testing.T) {
	t.Parallel()

	ownerID := primitive.NewObjectID()
	otherUserID := primitive.NewObjectID()
	bucket := bucketDetailTestBucket(ownerID)

	tests := []struct {
		name       string
		bucketRepo bucketAccessBucketReader
		grantRepo  bucketAccessGrantReader
	}{
		{
			name:       "missing",
			bucketRepo: bucketDetailBucketReader{err: mongo.ErrNoDocuments},
		},
		{
			name:       "inaccessible",
			bucketRepo: bucketDetailBucketReader{bucket: bucket},
			grantRepo:  bucketDetailGrantReader{err: mongo.ErrNoDocuments},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			resp := performGetBucketRequest(t, otherUserID, tt.bucketRepo, tt.grantRepo, bucket.ID.Hex())
			if resp.Code != http.StatusNotFound {
				t.Fatalf("expected 404, got %d", resp.Code)
			}
		})
	}
}

func TestGetBucketBadID(t *testing.T) {
	t.Parallel()

	resp := performGetBucketRequest(t, primitive.NewObjectID(), bucketDetailBucketReader{}, nil, "not-a-valid-oid")
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
}

func TestGetBucketRepositoryFailure(t *testing.T) {
	t.Parallel()

	resp := performGetBucketRequest(
		t,
		primitive.NewObjectID(),
		bucketDetailBucketReader{err: errors.New("repo down")},
		nil,
		primitive.NewObjectID().Hex(),
	)
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
}

func performGetBucketRequest(
	t *testing.T,
	userID primitive.ObjectID,
	bucketRepo bucketAccessBucketReader,
	grantRepo bucketAccessGrantReader,
	bucketID string,
) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.GET("/buckets/:id", setUserID(userID.Hex()), getBucket(bucketRepo, grantRepo))

	req, _ := http.NewRequest(http.MethodGet, "/buckets/"+bucketID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func bucketDetailTestBucket(ownerID primitive.ObjectID) *repository.Bucket {
	return &repository.Bucket{
		ID:        primitive.NewObjectID(),
		UserID:    ownerID,
		Key:       "bucket-detail-test",
		AccessKey: "bucket-detail-access",
		CreatedAt: time.Now().UTC(),
	}
}
