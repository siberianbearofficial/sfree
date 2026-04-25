package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/sourcecap"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type bucketPreflightClient struct {
	health sourcecap.Health
}

func (c bucketPreflightClient) Upload(context.Context, string, io.Reader) (string, error) {
	return "", nil
}

func (c bucketPreflightClient) Download(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("")), nil
}

func (c bucketPreflightClient) Delete(context.Context, string) error {
	return nil
}

func (c bucketPreflightClient) ProbeSourceHealth(context.Context) (sourcecap.Health, error) {
	return c.health, nil
}

func TestBucketPreflightReturnsConfirmRequiredForDegradedSources(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	sourceRepo := fakeBucketSourceGetter{sources: map[primitive.ObjectID]repository.Source{
		sourceID: {
			ID:     sourceID,
			UserID: userID,
			Type:   repository.SourceTypeGDrive,
			Name:   "Crowded Drive",
			Key:    "{}",
		},
	}}
	r := gin.New()
	r.POST("/buckets/preflight", setUserID(userID.Hex()), BucketPreflightWithFactory(sourceRepo, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return bucketPreflightClient{health: sourcecap.Health{
			Status:     sourcecap.HealthDegraded,
			ReasonCode: "quota_low",
			Message:    "Google Drive quota is nearly exhausted.",
			Quota: sourcecap.Quota{
				TotalBytes: ptrInt64(1024),
				UsedBytes:  ptrInt64(1000),
				FreeBytes:  ptrInt64(24),
			},
		}}, nil
	}))

	body, _ := json.Marshal(map[string]any{"source_ids": []string{sourceID.Hex()}})
	req, _ := http.NewRequest(http.MethodPost, "/buckets/preflight", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp bucketPreflightResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Decision != string(bucketPreflightDecisionConfirmRequired) {
		t.Fatalf("expected confirm_required, got %+v", resp)
	}
	if len(resp.Sources) != 1 || !resp.Sources[0].RequiresConfirmation || resp.Sources[0].BlocksCreation {
		t.Fatalf("unexpected sources: %+v", resp.Sources)
	}
	if resp.NearCapacitySourceCount != 1 || resp.DegradedSourceCount != 1 {
		t.Fatalf("unexpected counts: %+v", resp)
	}
}

func TestBucketPreflightReturnsBlockedForUnhealthySources(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	sourceRepo := fakeBucketSourceGetter{sources: map[primitive.ObjectID]repository.Source{
		sourceID: {
			ID:     sourceID,
			UserID: userID,
			Type:   repository.SourceTypeS3,
			Name:   "Offline Bucket",
			Key:    "{}",
		},
	}}
	r := gin.New()
	r.POST("/buckets/preflight", setUserID(userID.Hex()), BucketPreflightWithFactory(sourceRepo, func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return bucketPreflightClient{health: sourcecap.Health{
			Status:     sourcecap.HealthUnhealthy,
			ReasonCode: "probe_failed",
			Message:    "Source health probe failed.",
		}}, nil
	}))

	body, _ := json.Marshal(map[string]any{"source_ids": []string{sourceID.Hex()}})
	req, _ := http.NewRequest(http.MethodPost, "/buckets/preflight", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp bucketPreflightResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Decision != string(bucketPreflightDecisionBlocked) {
		t.Fatalf("expected blocked, got %+v", resp)
	}
	if len(resp.Sources) != 1 || !resp.Sources[0].BlocksCreation {
		t.Fatalf("unexpected sources: %+v", resp.Sources)
	}
}

func TestCreateBucketRejectsConfirmRequiredSourcesWithoutAcknowledgement(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	bucketRepo := &fakeBucketCreator{}
	sourceRepo := fakeBucketSourceGetter{sources: map[primitive.ObjectID]repository.Source{
		sourceID: {
			ID:     sourceID,
			UserID: userID,
			Type:   repository.SourceTypeGDrive,
			Name:   "Crowded Drive",
			Key:    "{}",
		},
	}}
	r := gin.New()
	r.POST("/buckets", setUserID(userID.Hex()), CreateBucketWithFactory(bucketRepo, sourceRepo, "secret", func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return bucketPreflightClient{health: sourcecap.Health{
			Status:     sourcecap.HealthDegraded,
			ReasonCode: "quota_low",
			Message:    "Google Drive quota is nearly exhausted.",
		}}, nil
	}))

	body, _ := json.Marshal(map[string]any{"key": "risk-bucket", "source_ids": []string{sourceID.Hex()}})
	req, _ := http.NewRequest(http.MethodPost, "/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
	if bucketRepo.createCalls != 0 {
		t.Fatalf("expected bucket not to be created, got %d create calls", bucketRepo.createCalls)
	}
}

func TestCreateBucketAllowsConfirmRequiredSourcesWithAcknowledgement(t *testing.T) {
	t.Parallel()
	userID := primitive.NewObjectID()
	sourceID := primitive.NewObjectID()
	bucketRepo := &fakeBucketCreator{}
	sourceRepo := fakeBucketSourceGetter{sources: map[primitive.ObjectID]repository.Source{
		sourceID: {
			ID:     sourceID,
			UserID: userID,
			Type:   repository.SourceTypeGDrive,
			Name:   "Crowded Drive",
			Key:    "{}",
		},
	}}
	r := gin.New()
	r.POST("/buckets", setUserID(userID.Hex()), CreateBucketWithFactory(bucketRepo, sourceRepo, "secret", func(context.Context, *repository.Source) (manager.SourceClient, error) {
		return bucketPreflightClient{health: sourcecap.Health{
			Status:     sourcecap.HealthDegraded,
			ReasonCode: "quota_low",
			Message:    "Google Drive quota is nearly exhausted.",
		}}, nil
	}))

	body, _ := json.Marshal(map[string]any{
		"key":               "risk-bucket",
		"source_ids":        []string{sourceID.Hex()},
		"risk_acknowledged": true,
	})
	req, _ := http.NewRequest(http.MethodPost, "/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if bucketRepo.createCalls != 1 {
		t.Fatalf("expected bucket to be created once, got %d", bucketRepo.createCalls)
	}
}

func ptrInt64(v int64) *int64 {
	return &v
}
