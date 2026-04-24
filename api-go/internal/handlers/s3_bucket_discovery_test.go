package handlers

import (
	"context"
	"encoding/xml"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeAccessKeyBucketReader struct {
	bucket *repository.Bucket
	err    error
}

func (r fakeAccessKeyBucketReader) GetByAccessKey(_ context.Context, _ string) (*repository.Bucket, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.bucket, nil
}

func TestListBucketsS3ReturnsAuthenticatedBucket(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	createdAt := time.Date(2026, 4, 24, 12, 30, 45, 0, time.UTC)
	c, w := newHandlerTestContext(t, http.MethodGet, "/", nil)
	c.Set("accessKey", "bucket-access")

	ListBucketsS3(fakeAccessKeyBucketReader{
		bucket: &repository.Bucket{
			UserID:    userID,
			Key:       "bucket-alpha",
			AccessKey: "bucket-access",
			CreatedAt: createdAt,
		},
	})(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("x-amz-bucket-region"); got != s3BucketRegion {
		t.Fatalf("expected region header %q, got %q", s3BucketRegion, got)
	}

	var result listAllMyBucketsResult
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Owner.ID != userID.Hex() || result.Owner.DisplayName != userID.Hex() {
		t.Fatalf("unexpected owner: %+v", result.Owner)
	}
	if len(result.Buckets.Buckets) != 1 {
		t.Fatalf("expected 1 bucket, got %d", len(result.Buckets.Buckets))
	}
	if got := result.Buckets.Buckets[0]; got.Name != "bucket-alpha" || got.CreationDate != createdAt.Format(time.RFC3339) {
		t.Fatalf("unexpected bucket entry: %+v", got)
	}
}

func TestListBucketsS3MissingAccessKeyReturnsAccessDenied(t *testing.T) {
	t.Parallel()

	c, w := newHandlerTestContext(t, http.MethodGet, "/", nil)
	ListBucketsS3(fakeAccessKeyBucketReader{})(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>AccessDenied</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestListBucketsS3UnknownAccessKeyReturnsInvalidAccessKeyID(t *testing.T) {
	t.Parallel()

	c, w := newHandlerTestContext(t, http.MethodGet, "/", nil)
	c.Set("accessKey", "missing-access")

	ListBucketsS3(fakeAccessKeyBucketReader{err: mongo.ErrNoDocuments})(c)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>InvalidAccessKeyId</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestHeadBucketReturnsBucketRegionHeader(t *testing.T) {
	t.Parallel()

	c, w := newHandlerTestContext(t, http.MethodHead, "/bucket-alpha", nil, testRouteParam{Key: "bucket", Value: "bucket-alpha"})
	c.Set("accessKey", "bucket-access")

	HeadBucket(fakeObjectBucketReader{
		bucket: &repository.Bucket{
			ID:        primitive.NewObjectID(),
			Key:       "bucket-alpha",
			AccessKey: "bucket-access",
		},
	})(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("x-amz-bucket-region"); got != s3BucketRegion {
		t.Fatalf("expected region header %q, got %q", s3BucketRegion, got)
	}
}

func TestGetBucketLocationReturnsEmptyLocationConstraint(t *testing.T) {
	t.Parallel()

	c, w := newHandlerTestContext(t, http.MethodGet, "/bucket-alpha?location", nil, testRouteParam{Key: "bucket", Value: "bucket-alpha"})
	c.Set("accessKey", "bucket-access")

	GetBucketLocation(fakeObjectBucketReader{
		bucket: &repository.Bucket{
			ID:        primitive.NewObjectID(),
			Key:       "bucket-alpha",
			AccessKey: "bucket-access",
		},
	})(c)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("x-amz-bucket-region"); got != s3BucketRegion {
		t.Fatalf("expected region header %q, got %q", s3BucketRegion, got)
	}

	var result bucketLocationConstraint
	if err := xml.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if result.Xmlns != "http://s3.amazonaws.com/doc/2006-03-01/" {
		t.Fatalf("unexpected xmlns: %q", result.Xmlns)
	}
	if result.Value != "" {
		t.Fatalf("expected empty location constraint, got %q", result.Value)
	}
}
