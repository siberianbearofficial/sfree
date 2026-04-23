package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type sourceDownloadHandlerClient struct {
	body         io.ReadCloser
	downloadName *string
}

func (c sourceDownloadHandlerClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c sourceDownloadHandlerClient) Download(_ context.Context, name string) (io.ReadCloser, error) {
	if c.downloadName != nil {
		*c.downloadName = name
	}
	return c.body, nil
}

func (c sourceDownloadHandlerClient) Delete(context.Context, string) error {
	return nil
}

type failingReadCloser struct{}

func (f failingReadCloser) Read([]byte) (int, error) {
	return 0, errors.New("source stream failed")
}

func (f failingReadCloser) Close() error {
	return nil
}

func TestDownloadSourceFileAcceptsEscapedQueryFileID(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	source := &repository.Source{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Type:   repository.SourceTypeS3,
	}
	fileID := "folder/a+b #?.txt"
	var downloadedName string
	r := gin.New()
	r.GET("/sources/:id/download", setUserID(userID.Hex()), downloadSourceFile(fakeSourceGetter{source: source}, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return sourceDownloadHandlerClient{
			body:         io.NopCloser(strings.NewReader("payload")),
			downloadName: &downloadedName,
		}, nil
	}))

	params := url.Values{}
	params.Set("file_id", fileID)
	req, _ := http.NewRequest(http.MethodGet, "/sources/"+source.ID.Hex()+"/download?"+params.Encode(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if downloadedName != fileID {
		t.Fatalf("expected source download key %q, got %q", fileID, downloadedName)
	}
}

func TestDownloadSourceFilePreflightFailureBeforeHeaders(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	source := &repository.Source{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Type:   repository.SourceTypeS3,
	}
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download", setUserID(userID.Hex()), downloadSourceFile(fakeSourceGetter{source: source}, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return sourceDownloadHandlerClient{body: failingReadCloser{}}, nil
	}))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+source.ID.Hex()+"/files/object-key/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Disposition"); got != "" {
		t.Fatalf("expected no success download headers, got Content-Disposition %q", got)
	}
}

func TestDownloadSourceFileStreamsAfterPreflight(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	source := &repository.Source{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Type:   repository.SourceTypeS3,
	}
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download", setUserID(userID.Hex()), downloadSourceFile(fakeSourceGetter{source: source}, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return sourceDownloadHandlerClient{body: io.NopCloser(strings.NewReader("streamed payload"))}, nil
	}))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+source.ID.Hex()+"/files/object-key/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if got := w.Header().Get("Content-Type"); got != "application/octet-stream" {
		t.Fatalf("expected octet stream content type, got %q", got)
	}
	if got := w.Body.String(); got != "streamed payload" {
		t.Fatalf("expected full streamed payload, got %q", got)
	}
}

func TestDownloadSourceFileNilBodyRemainsBadRequest(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	source := &repository.Source{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Type:   repository.SourceTypeS3,
	}
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download", setUserID(userID.Hex()), downloadSourceFile(fakeSourceGetter{source: source}, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return sourceDownloadHandlerClient{}, nil
	}))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+source.ID.Hex()+"/files/object-key/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}
