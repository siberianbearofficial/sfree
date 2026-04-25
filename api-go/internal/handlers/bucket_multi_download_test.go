package handlers

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
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

type fakeFileByIDMapReader struct {
	files map[primitive.ObjectID]*repository.File
	err   error
}

func (r fakeFileByIDMapReader) GetByID(_ context.Context, id primitive.ObjectID) (*repository.File, error) {
	if r.err != nil {
		return nil, r.err
	}
	file, ok := r.files[id]
	if !ok {
		return nil, mongo.ErrNoDocuments
	}
	return file, nil
}

func TestDownloadFilesArchiveRejectsTooManyFileIDs(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucket := &repository.Bucket{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Key:    "batch",
	}

	fileIDs := make([]string, 0, maxMultiFileDownloadCount+1)
	for i := 0; i < maxMultiFileDownloadCount+1; i++ {
		fileIDs = append(fileIDs, primitive.NewObjectID().Hex())
	}

	resp := performDownloadFilesArchiveRequest(
		t,
		userID,
		bucketDetailBucketReader{bucket: bucket},
		fakeFileByIDMapReader{},
		multiFileDownloadRequest{FileIDs: fileIDs},
	)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "between 1 and 50") {
		t.Fatalf("unexpected response body: %s", resp.Body.String())
	}
}

func TestDownloadFilesArchiveRejectsSelectionsOverSizeLimit(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	bucket := &repository.Bucket{
		ID:     bucketID,
		UserID: userID,
		Key:    "batch",
	}
	file := &repository.File{
		ID:        fileID,
		BucketID:  bucketID,
		Name:      "large.bin",
		CreatedAt: time.Now().UTC(),
		Chunks: []repository.FileChunk{
			{SourceID: primitive.NewObjectID(), Name: "chunk", Size: maxMultiFileDownloadTotal + 1},
		},
	}

	resp := performDownloadFilesArchiveRequest(
		t,
		userID,
		bucketDetailBucketReader{bucket: bucket},
		fakeFileByIDMapReader{files: map[primitive.ObjectID]*repository.File{fileID: file}},
		multiFileDownloadRequest{FileIDs: []string{fileID.Hex()}},
	)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), maxMultiFileDownloadTotalStr) {
		t.Fatalf("unexpected response body: %s", resp.Body.String())
	}
}

func TestDownloadFilesArchiveReturnsNotFoundForFilesOutsideBucket(t *testing.T) {
	t.Parallel()

	userID := primitive.NewObjectID()
	bucket := &repository.Bucket{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Key:    "batch",
	}
	fileID := primitive.NewObjectID()
	file := &repository.File{
		ID:        fileID,
		BucketID:  primitive.NewObjectID(),
		Name:      "wrong.txt",
		CreatedAt: time.Now().UTC(),
	}

	resp := performDownloadFilesArchiveRequest(
		t,
		userID,
		bucketDetailBucketReader{bucket: bucket},
		fakeFileByIDMapReader{files: map[primitive.ObjectID]*repository.File{fileID: file}},
		multiFileDownloadRequest{FileIDs: []string{fileID.Hex()}},
	)

	if resp.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.Code)
	}
}

func TestDownloadFilesArchiveStreamsZipContents(t *testing.T) {
	origStream := streamDownloadFile
	t.Cleanup(func() {
		streamDownloadFile = origStream
	})

	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, fileDoc *repository.File, w io.Writer) error {
		_, err := io.WriteString(w, "payload:"+fileDoc.Name)
		return err
	}

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	bucket := &repository.Bucket{
		ID:     bucketID,
		UserID: userID,
		Key:    "batch",
	}
	first := &repository.File{
		ID:        primitive.NewObjectID(),
		BucketID:  bucketID,
		Name:      "alpha.txt",
		CreatedAt: time.Unix(1700000000, 0).UTC(),
		Chunks: []repository.FileChunk{
			{SourceID: primitive.NewObjectID(), Name: "a", Size: 17},
		},
	}
	second := &repository.File{
		ID:        primitive.NewObjectID(),
		BucketID:  bucketID,
		Name:      "beta.txt",
		CreatedAt: time.Unix(1700000100, 0).UTC(),
		Chunks: []repository.FileChunk{
			{SourceID: primitive.NewObjectID(), Name: "b", Size: 16},
		},
	}

	resp := performDownloadFilesArchiveRequest(
		t,
		userID,
		bucketDetailBucketReader{bucket: bucket},
		fakeFileByIDMapReader{files: map[primitive.ObjectID]*repository.File{
			first.ID:  first,
			second.ID: second,
		}},
		multiFileDownloadRequest{FileIDs: []string{first.ID.Hex(), second.ID.Hex()}},
	)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	if got := resp.Header().Get("Content-Type"); got != "application/zip" {
		t.Fatalf("expected zip content type, got %q", got)
	}
	if got := resp.Header().Get("Content-Disposition"); !strings.Contains(got, "batch-files.zip") {
		t.Fatalf("unexpected content disposition: %q", got)
	}

	reader, err := zip.NewReader(bytes.NewReader(resp.Body.Bytes()), int64(resp.Body.Len()))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	if len(reader.File) != 2 {
		t.Fatalf("expected 2 files, got %d", len(reader.File))
	}
	got := make(map[string]string, len(reader.File))
	for _, entry := range reader.File {
		rc, err := entry.Open()
		if err != nil {
			t.Fatalf("open zip entry: %v", err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read zip entry: %v", err)
		}
		got[entry.Name] = string(data)
	}
	if got["alpha.txt"] != "payload:alpha.txt" || got["beta.txt"] != "payload:beta.txt" {
		t.Fatalf("unexpected zip contents: %+v", got)
	}
}

func TestDownloadFilesArchiveStreamFailureReturnsErrorBeforeSuccessHeaders(t *testing.T) {
	origStream := streamDownloadFile
	t.Cleanup(func() {
		streamDownloadFile = origStream
	})

	streamDownloadFile = func(_ context.Context, _ *repository.SourceRepository, _ *repository.File, _ io.Writer) error {
		return manager.ErrChecksumMismatch
	}

	userID := primitive.NewObjectID()
	bucketID := primitive.NewObjectID()
	fileID := primitive.NewObjectID()
	bucket := &repository.Bucket{
		ID:     bucketID,
		UserID: userID,
		Key:    "batch",
	}
	file := &repository.File{
		ID:        fileID,
		BucketID:  bucketID,
		Name:      "alpha.txt",
		CreatedAt: time.Now().UTC(),
		Chunks: []repository.FileChunk{
			{SourceID: primitive.NewObjectID(), Name: "chunk", Size: 1},
		},
	}

	resp := performDownloadFilesArchiveRequest(
		t,
		userID,
		bucketDetailBucketReader{bucket: bucket},
		fakeFileByIDMapReader{files: map[primitive.ObjectID]*repository.File{fileID: file}},
		multiFileDownloadRequest{FileIDs: []string{fileID.Hex()}},
	)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.Code)
	}
	if got := resp.Header().Get("Content-Disposition"); got != "" {
		t.Fatalf("expected no success Content-Disposition header, got %q", got)
	}
	if got := resp.Header().Get("Content-Type"); got != "" {
		t.Fatalf("expected no success Content-Type header, got %q", got)
	}
	if resp.Body.Len() != 0 {
		t.Fatalf("expected no response body, got %q", resp.Body.String())
	}
}

func performDownloadFilesArchiveRequest(
	t *testing.T,
	userID primitive.ObjectID,
	bucketRepo bucketAccessBucketReader,
	fileRepo fileByIDReader,
	body multiFileDownloadRequest,
) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	r.POST("/buckets/:id/files/download", setUserID(userID.Hex()), func(c *gin.Context) {
		downloadFilesArchive(bucketRepo, &repository.SourceRepository{}, fileRepo, nil, nil)(c)
	})

	bucketID := primitive.NewObjectID().Hex()
	if stub, ok := bucketRepo.(bucketDetailBucketReader); ok && stub.bucket != nil {
		bucketID = stub.bucket.ID.Hex()
	}
	if stub, ok := bucketRepo.(fakeBucketByIDReader); ok && stub.bucket != nil {
		bucketID = stub.bucket.ID.Hex()
	}

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/buckets/"+bucketID+"/files/download", bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
