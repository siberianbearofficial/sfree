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
	t.Cleanup(func() {
		streamDownloadFile = origStream
	})

	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, _ io.Writer) error {
		return manager.ErrChecksumMismatch
	}

	bucket, file, userID := testDownloadFile()
	r := gin.New()
	r.GET(
		"/buckets/:id/files/:file_id/download",
		setUserID(userID.Hex()),
		downloadFile(fakeBucketByIDReader{bucket: bucket}, &repository.SourceRepository{}, fakeFileByIDReader{file: file}, nil),
	)

	req := httptest.NewRequest(http.MethodGet, "/buckets/"+bucket.ID.Hex()+"/files/"+file.ID.Hex()+"/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if body := w.Body.String(); strings.Contains(body, "complete") {
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
	t.Cleanup(func() {
		streamDownloadFile = origStream
	})

	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, _ io.Writer) error {
		return manager.ErrChecksumMismatch
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
	r.GET("/share/:token", getSharedFile(fakeShareLinkByTokenReader{link: link}, &repository.SourceRepository{}, fakeFileByIDReader{file: file}))

	req := httptest.NewRequest(http.MethodGet, "/share/token", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if body := w.Body.String(); strings.Contains(body, "complete") {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := w.Header().Get("Content-Disposition"); got != "" {
		t.Fatalf("expected no success Content-Disposition header, got %q", got)
	}
	if got := w.Header().Get("Content-Length"); got != "" {
		t.Fatalf("expected no success Content-Length header, got %q", got)
	}
}

func TestDownloadFileStreamsBodyWithoutPreflight(t *testing.T) {
	origStream := streamDownloadFile
	t.Cleanup(func() {
		streamDownloadFile = origStream
	})

	streamCalls := 0
	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, w io.Writer) error {
		streamCalls++
		_, err := io.WriteString(w, "complete")
		return err
	}

	bucket, file, userID := testDownloadFile()
	r := gin.New()
	r.GET(
		"/buckets/:id/files/:file_id/download",
		setUserID(userID.Hex()),
		downloadFile(fakeBucketByIDReader{bucket: bucket}, &repository.SourceRepository{}, fakeFileByIDReader{file: file}, nil),
	)

	req := httptest.NewRequest(http.MethodGet, "/buckets/"+bucket.ID.Hex()+"/files/"+file.ID.Hex()+"/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if streamCalls != 1 {
		t.Fatalf("expected one real stream call, got %d", streamCalls)
	}
	if body := w.Body.String(); body != "complete" {
		t.Fatalf("unexpected body: %s", body)
	}
	if got := w.Header().Get("Content-Length"); got != "7" {
		t.Fatalf("expected Content-Length 7, got %q", got)
	}
}
