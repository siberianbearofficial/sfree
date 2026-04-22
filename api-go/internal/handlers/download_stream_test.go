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

type fakeBucketByIDReader struct {
	bucket *repository.Bucket
	err    error
}

func (r fakeBucketByIDReader) GetByID(_ context.Context, _ primitive.ObjectID) (*repository.Bucket, error) {
	return r.bucket, r.err
}

type fakeFileByIDReader struct {
	file *repository.File
	err  error
}

func (r fakeFileByIDReader) GetByID(_ context.Context, _ primitive.ObjectID) (*repository.File, error) {
	return r.file, r.err
}

type fakeShareLinkByTokenReader struct {
	link *repository.ShareLink
	err  error
}

func (r fakeShareLinkByTokenReader) GetByToken(_ context.Context, _ string) (*repository.ShareLink, error) {
	return r.link, r.err
}

func testDownloadFile() (*repository.Bucket, *repository.File, primitive.ObjectID) {
	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	bucket := &repository.Bucket{
		ID:     bucketID,
		UserID: userID,
		Key:    "bucket",
	}
	file := &repository.File{
		ID:        primitive.NewObjectID(),
		BucketID:  bucketID,
		Name:      "object.txt",
		CreatedAt: time.Unix(1700000000, 0).UTC(),
		Chunks: []repository.FileChunk{
			{SourceID: sourceID, Name: "chunk-1", Order: 0, Size: 7, Checksum: "bad"},
		},
	}
	return bucket, file, userID
}

func TestDownloadFileStreamFailureReturnsErrorBeforeSuccessHeaders(t *testing.T) {
	origStream := streamDownloadFile
	origStreamRange := streamDownloadFileRange
	t.Cleanup(func() {
		streamDownloadFile = origStream
		streamDownloadFileRange = origStreamRange
	})

	var gotStart, gotEnd int64
	streamDownloadFileRange = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer, start, end int64) error {
		gotStart, gotEnd = start, end
		_, _ = io.WriteString(w, "partial")
		return manager.ErrChecksumMismatch
	}
	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer) error {
		_, err := io.WriteString(w, "complete")
		return err
	}

	bucket, file, userID := testDownloadFile()
	r := gin.New()
	r.GET(
		"/buckets/:id/files/:file_id/download",
		setUserID(userID.Hex()),
		downloadFile(fakeBucketByIDReader{bucket: bucket}, &repository.SourceRepository{}, fakeFileByIDReader{file: file}, nil, nil),
	)

	req := httptest.NewRequest(http.MethodGet, "/buckets/"+bucket.ID.Hex()+"/files/"+file.ID.Hex()+"/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if gotStart != 0 || gotEnd != 0 {
		t.Fatalf("expected bounded preflight range 0-0, got %d-%d", gotStart, gotEnd)
	}
	if body := w.Body.String(); strings.Contains(body, "partial") || strings.Contains(body, "complete") {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := w.Header().Get("Content-Disposition"); got != "" {
		t.Fatalf("expected no success Content-Disposition header, got %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != "" {
		t.Fatalf("expected no success Content-Length header, got %q", got)
	}
}

func TestGetSharedFileStreamFailureReturnsErrorBeforeSuccessHeaders(t *testing.T) {
	origStream := streamDownloadFile
	origStreamRange := streamDownloadFileRange
	t.Cleanup(func() {
		streamDownloadFile = origStream
		streamDownloadFileRange = origStreamRange
	})

	streamDownloadFileRange = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer, start, end int64) error {
		if start != 0 || end != 0 {
			t.Fatalf("expected bounded preflight range 0-0, got %d-%d", start, end)
		}
		_, _ = io.WriteString(w, "partial")
		return manager.ErrChecksumMismatch
	}
	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer) error {
		_, err := io.WriteString(w, "complete")
		return err
	}

	bucket, file, userID := testDownloadFile()
	link := &repository.ShareLink{
		ID:        primitive.NewObjectID(),
		FileID:    file.ID,
		BucketID:  bucket.ID,
		UserID:    userID,
		Token:     "token",
		CreatedAt: time.Now().UTC(),
	}
	r := gin.New()
	r.GET("/share/:token", getSharedFile(fakeShareLinkByTokenReader{link: link}, &repository.SourceRepository{}, fakeFileByIDReader{file: file}, nil))

	req := httptest.NewRequest(http.MethodGet, "/share/token", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if body := w.Body.String(); strings.Contains(body, "partial") || strings.Contains(body, "complete") {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := w.Header().Get("Content-Disposition"); got != "" {
		t.Fatalf("expected no success Content-Disposition header, got %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != "" {
		t.Fatalf("expected no success Content-Length header, got %q", got)
	}
}

func TestDownloadFileStreamsBodyAfterBoundedPreflight(t *testing.T) {
	origStream := streamDownloadFile
	origStreamRange := streamDownloadFileRange
	t.Cleanup(func() {
		streamDownloadFile = origStream
		streamDownloadFileRange = origStreamRange
	})

	var gotStart, gotEnd int64
	streamDownloadFileRange = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, _ io.Writer, start, end int64) error {
		gotStart, gotEnd = start, end
		return nil
	}
	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer) error {
		_, err := io.WriteString(w, "complete")
		return err
	}

	bucket, file, userID := testDownloadFile()
	r := gin.New()
	r.GET(
		"/buckets/:id/files/:file_id/download",
		setUserID(userID.Hex()),
		downloadFile(fakeBucketByIDReader{bucket: bucket}, &repository.SourceRepository{}, fakeFileByIDReader{file: file}, nil, nil),
	)

	req := httptest.NewRequest(http.MethodGet, "/buckets/"+bucket.ID.Hex()+"/files/"+file.ID.Hex()+"/download", nil)
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
	if got := w.Header().Get("Content-Length"); got != "7" {
		t.Fatalf("expected Content-Length 7, got %q", got)
	}
}
