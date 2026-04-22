package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type fakeSourceGetter struct {
	source *repository.Source
	err    error
}

func (f fakeSourceGetter) GetByID(context.Context, primitive.ObjectID) (*repository.Source, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.source, nil
}

type sourceHealthHandlerClient struct {
	headErr error
}

func (c sourceHealthHandlerClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c sourceHealthHandlerClient) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (c sourceHealthHandlerClient) Delete(context.Context, string) error {
	return nil
}

func (c sourceHealthHandlerClient) HeadBucket(context.Context) error {
	return c.headErr
}

func TestGetSourceHealthBadID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/health", setUserID(validUserID()), getSourceHealth(fakeSourceGetter{}, nil))

	req, _ := http.NewRequest(http.MethodGet, "/sources/not-an-id/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetSourceHealthMissingSource(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/health", setUserID(validUserID()), getSourceHealth(fakeSourceGetter{err: mongo.ErrNoDocuments}, nil))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+primitive.NewObjectID().Hex()+"/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetSourceHealthNonOwnedSource(t *testing.T) {
	t.Parallel()
	source := &repository.Source{
		ID:     primitive.NewObjectID(),
		UserID: primitive.NewObjectID(),
		Type:   repository.SourceTypeS3,
	}
	r := gin.New()
	r.GET("/sources/:id/health", setUserID(validUserID()), getSourceHealth(fakeSourceGetter{source: source}, nil))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+source.ID.Hex()+"/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetSourceHealthSuccessfulProbe(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	source := &repository.Source{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Type:   repository.SourceTypeS3,
	}
	r := gin.New()
	r.GET("/sources/:id/health", setUserID(userID.Hex()), getSourceHealth(fakeSourceGetter{source: source}, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return sourceHealthHandlerClient{}, nil
	}))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+source.ID.Hex()+"/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp sourceHealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != string(manager.SourceHealthHealthy) || resp.ReasonCode != "ok" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.QuotaTotalBytes != nil || resp.QuotaUsedBytes != nil || resp.QuotaFreeBytes != nil {
		t.Fatalf("expected null quota for s3, got %+v", resp)
	}
}

func TestGetSourceHealthProviderFailureMapping(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	source := &repository.Source{
		ID:     primitive.NewObjectID(),
		UserID: userID,
		Type:   repository.SourceTypeS3,
	}
	r := gin.New()
	r.GET("/sources/:id/health", setUserID(userID.Hex()), getSourceHealth(fakeSourceGetter{source: source}, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return sourceHealthHandlerClient{headErr: errors.New("access denied")}, nil
	}))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+source.ID.Hex()+"/health", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp sourceHealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != string(manager.SourceHealthUnhealthy) || resp.ReasonCode != "probe_failed" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
