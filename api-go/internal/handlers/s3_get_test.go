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
			ID:        primitive.NewObjectID(),
			BucketID:  bucketID,
			Name:      "object.txt",
			CreatedAt: time.Unix(1700000000, 0).UTC(),
			Chunks: []repository.FileChunk{
				{SourceID: sourceID, Name: "chunk-1", Order: 0, Size: 7, Checksum: "bad"},
			},
		},
	}
	sourceRepo := &repository.SourceRepository{}

	return getObject(bucketRepo, sourceRepo, fileRepo)
}

func TestGetObjectStreamFailureReturnsS3ErrorBeforeSuccess(t *testing.T) {
	origStream := streamS3Object
	t.Cleanup(func() { streamS3Object = origStream })
	streamS3Object = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer) error {
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
	if gotStart != 2 || gotEnd != 4 {
		t.Fatalf("expected requested range 2-4, got %d-%d", gotStart, gotEnd)
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
