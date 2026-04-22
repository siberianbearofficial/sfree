package handlers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeObjectBucketReader struct {
	bucket *repository.Bucket
	err    error
}

func (r fakeObjectBucketReader) GetByKey(_ context.Context, _ string) (*repository.Bucket, error) {
	return r.bucket, r.err
}

type fakeObjectFileReader struct {
	file *repository.File
	err  error
}

func (r fakeObjectFileReader) GetByName(_ context.Context, _ primitive.ObjectID, _ string) (*repository.File, error) {
	return r.file, r.err
}

func newLookupObjectTestContext(accessKey string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/s3/bucket/object.txt", nil)
	c.Params = gin.Params{
		{Key: "bucket", Value: "bucket"},
		{Key: "object", Value: "/object.txt"},
	}
	if accessKey != "" {
		c.Set("accessKey", accessKey)
	}
	return c, w
}

func TestLookupObjectMissingBucketReturnsNoSuchBucket(t *testing.T) {
	c, w := newLookupObjectTestContext("access-key")

	fileDoc, total, ok := lookupObject(c, fakeObjectBucketReader{err: mongo.ErrNoDocuments}, fakeObjectFileReader{})

	if ok || fileDoc != nil || total != 0 {
		t.Fatalf("expected lookup failure, got ok=%v file=%v total=%d", ok, fileDoc, total)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>NoSuchBucket</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestLookupObjectWrongAccessKeyReturnsNoSuchBucket(t *testing.T) {
	c, w := newLookupObjectTestContext("wrong-key")
	bucketID := primitive.NewObjectID()

	fileDoc, total, ok := lookupObject(c, fakeObjectBucketReader{
		bucket: &repository.Bucket{
			ID:        bucketID,
			Key:       "bucket",
			AccessKey: "access-key",
		},
	}, fakeObjectFileReader{})

	if ok || fileDoc != nil || total != 0 {
		t.Fatalf("expected lookup failure, got ok=%v file=%v total=%d", ok, fileDoc, total)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>NoSuchBucket</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestLookupObjectMissingObjectReturnsNoSuchKey(t *testing.T) {
	c, w := newLookupObjectTestContext("access-key")
	bucketID := primitive.NewObjectID()

	fileDoc, total, ok := lookupObject(c, fakeObjectBucketReader{
		bucket: &repository.Bucket{
			ID:        bucketID,
			Key:       "bucket",
			AccessKey: "access-key",
		},
	}, fakeObjectFileReader{err: mongo.ErrNoDocuments})

	if ok || fileDoc != nil || total != 0 {
		t.Fatalf("expected lookup failure, got ok=%v file=%v total=%d", ok, fileDoc, total)
	}
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>NoSuchKey</Code>") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func getObjectFailureTestHandler(t *testing.T) gin.HandlerFunc {
	t.Helper()

	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	bucketRepo := fakeObjectBucketReader{
		bucket: &repository.Bucket{
			ID:        bucketID,
			Key:       "bucket",
			AccessKey: "access-key",
		},
	}
	fileRepo := fakeObjectFileReader{
		file: &repository.File{
			ID:           primitive.NewObjectID(),
			BucketID:     bucketID,
			Name:         "object.txt",
			CreatedAt:    time.Unix(1700000000, 0).UTC(),
			ContentType:  "text/plain",
			UserMetadata: map[string]string{"owner": "alice", "trace-id": "abc-123"},
			Chunks: []repository.FileChunk{
				{SourceID: sourceID, Name: "chunk-1", Order: 0, Size: 7, Checksum: "bad"},
			},
		},
	}
	sourceRepo := &repository.SourceRepository{}

	return getObject(bucketRepo, sourceRepo, fileRepo, nil)
}

func TestGetObjectStreamFailureReturnsS3ErrorBeforeSuccess(t *testing.T) {
	origStreamRange := streamS3ObjectRange
	t.Cleanup(func() { streamS3ObjectRange = origStreamRange })
	var gotStart, gotEnd int64
	streamS3ObjectRange = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer, start, end int64) error {
		gotStart, gotEnd = start, end
		_, _ = io.WriteString(w, "partial")
		return manager.ErrChecksumMismatch
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accessKey", "access-key")
		c.Next()
	})
	r.GET("/api/s3/:bucket/*object", getObjectFailureTestHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/api/s3/bucket/object.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if gotStart != 0 || gotEnd != 0 {
		t.Fatalf("expected bounded preflight range 0-0, got %d-%d", gotStart, gotEnd)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>InternalError</Code>") || strings.Contains(body, "partial") {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := w.Header().Get("ETag"); got != "" {
		t.Fatalf("expected no success ETag header, got %q", got)
	}
	if got := w.Header().Get("Accept-Ranges"); got != "" {
		t.Fatalf("expected no success Accept-Ranges header, got %q", got)
	}
}

func TestGetObjectRangeStreamFailureReturnsS3ErrorBeforePartialContent(t *testing.T) {
	origStreamRange := streamS3ObjectRange
	t.Cleanup(func() { streamS3ObjectRange = origStreamRange })
	var gotStart, gotEnd int64
	streamS3ObjectRange = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer, start, end int64) error {
		gotStart, gotEnd = start, end
		_, _ = io.WriteString(w, "par")
		return manager.ErrChecksumMismatch
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accessKey", "access-key")
		c.Next()
	})
	r.GET("/api/s3/:bucket/*object", getObjectFailureTestHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/api/s3/bucket/object.txt", nil)
	req.Header.Set("Range", "bytes=2-4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if gotStart != 2 || gotEnd != 2 {
		t.Fatalf("expected bounded preflight range 2-2, got %d-%d", gotStart, gotEnd)
	}
	if body := w.Body.String(); !strings.Contains(body, "<Code>InternalError</Code>") || strings.Contains(body, "par") {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := w.Header().Get("Content-Range"); got != "" {
		t.Fatalf("expected no success Content-Range header, got %q", got)
	}
	if got := w.Header().Get("ETag"); got != "" {
		t.Fatalf("expected no success ETag header, got %q", got)
	}
}

func TestGetObjectStreamsBodyAfterBoundedPreflight(t *testing.T) {
	origStream := streamS3Object
	origStreamRange := streamS3ObjectRange
	t.Cleanup(func() {
		streamS3Object = origStream
		streamS3ObjectRange = origStreamRange
	})
	var gotStart, gotEnd int64
	streamS3ObjectRange = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, _ io.Writer, start, end int64) error {
		gotStart, gotEnd = start, end
		return nil
	}
	streamS3Object = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer) error {
		_, err := io.WriteString(w, "complete")
		return err
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accessKey", "access-key")
		c.Next()
	})
	r.GET("/api/s3/:bucket/*object", getObjectFailureTestHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/api/s3/bucket/object.txt", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if gotStart != 0 || gotEnd != 0 {
		t.Fatalf("expected bounded preflight range 0-0, got %d-%d", gotStart, gotEnd)
	}
	if body := w.Body.String(); body != "complete" {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("expected stored Content-Type, got %q", got)
	}
	if got := w.Header().Get("x-amz-meta-owner"); got != "alice" {
		t.Fatalf("expected stored owner metadata, got %q", got)
	}
	if got := w.Header().Get("x-amz-meta-trace-id"); got != "abc-123" {
		t.Fatalf("expected stored trace metadata, got %q", got)
	}
}

func TestGetObjectRangeReturnsStoredMetadataHeaders(t *testing.T) {
	origStreamRange := streamS3ObjectRange
	t.Cleanup(func() { streamS3ObjectRange = origStreamRange })
	streamS3ObjectRange = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer, start, end int64) error {
		if end >= start {
			_, _ = io.WriteString(w, "cde")
		}
		return nil
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("accessKey", "access-key")
		c.Next()
	})
	r.GET("/api/s3/:bucket/*object", getObjectFailureTestHandler(t))

	req := httptest.NewRequest(http.MethodGet, "/api/s3/bucket/object.txt", nil)
	req.Header.Set("Range", "bytes=2-4")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusPartialContent {
		t.Fatalf("expected 206, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Length"); got != "3" {
		t.Fatalf("expected ranged Content-Length, got %q", got)
	}
	if got := w.Header().Get("Content-Range"); got != "bytes 2-4/7" {
		t.Fatalf("expected Content-Range, got %q", got)
	}
	if got := w.Header().Get("Content-Type"); got != "text/plain" {
		t.Fatalf("expected stored Content-Type, got %q", got)
	}
	if got := w.Header().Get("x-amz-meta-owner"); got != "alice" {
		t.Fatalf("expected stored metadata, got %q", got)
	}
}
