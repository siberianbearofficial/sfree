//go:build integration
// +build integration

package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCreateShareLinkRequestBodyValidation(t *testing.T) {
	tests := []struct {
		name           string
		body           *string
		wantStatus     int
		wantLinks      int
		wantExpiration bool
	}{
		{name: "empty body", wantStatus: http.StatusOK, wantLinks: 1},
		{name: "valid expires_in", body: stringPtr(`{"expires_in":3600}`), wantStatus: http.StatusOK, wantLinks: 1, wantExpiration: true},
		{name: "malformed json", body: stringPtr(`{"expires_in":`), wantStatus: http.StatusBadRequest},
		{name: "wrongly typed expires_in", body: stringPtr(`{"expires_in":"3600"}`), wantStatus: http.StatusBadRequest},
		{name: "zero expires_in", body: stringPtr(`{"expires_in":0}`), wantStatus: http.StatusOK, wantLinks: 1},
		{name: "negative expires_in", body: stringPtr(`{"expires_in":-1}`), wantStatus: http.StatusOK, wantLinks: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucketRepo, fileRepo, shareLinkRepo := newShareLinkHandlerTestRepos(t)
			ctx := context.Background()
			userID := primitive.NewObjectID()
			bucket := createBucketFileTestBucket(t, ctx, bucketRepo, userID)
			fileDoc, err := fileRepo.Create(ctx, repository.File{
				BucketID:  bucket.ID,
				Name:      "shared.txt",
				CreatedAt: time.Now().UTC(),
			})
			if err != nil {
				t.Fatal(err)
			}

			router := gin.New()
			router.POST("/buckets/:id/files/:file_id/share", setUserID(userID.Hex()), CreateShareLink(bucketRepo, fileRepo, shareLinkRepo, nil))

			var bodyReader *bytes.Reader
			if tt.body == nil {
				bodyReader = bytes.NewReader(nil)
			} else {
				bodyReader = bytes.NewReader([]byte(*tt.body))
			}
			req, _ := http.NewRequest(
				http.MethodPost,
				"/buckets/"+bucket.ID.Hex()+"/files/"+fileDoc.ID.Hex()+"/share",
				bodyReader,
			)
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Fatalf("expected %d, got %d", tt.wantStatus, w.Code)
			}

			links, err := shareLinkRepo.ListByFile(ctx, fileDoc.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(links) != tt.wantLinks {
				t.Fatalf("expected %d share links, got %d", tt.wantLinks, len(links))
			}
			if tt.wantLinks == 0 {
				return
			}

			var resp shareLinkResponse
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("invalid json: %v", err)
			}
			if (resp.ExpiresAt != nil) != tt.wantExpiration {
				t.Fatalf("expected expiration presence %v, got %v", tt.wantExpiration, resp.ExpiresAt != nil)
			}
			if (links[0].ExpiresAt != nil) != tt.wantExpiration {
				t.Fatalf("expected stored expiration presence %v, got %v", tt.wantExpiration, links[0].ExpiresAt != nil)
			}
		})
	}
}

func newShareLinkHandlerTestRepos(t *testing.T) (*repository.BucketRepository, *repository.FileRepository, *repository.ShareLinkRepository) {
	t.Helper()
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mongoConn, err := db.Connect(context.Background(), cfg.Mongo)
	if err != nil {
		t.Fatal(err)
	}
	testDB := mongoConn.Client.Database("sfree_share_link_handlers_" + primitive.NewObjectID().Hex())
	t.Cleanup(func() {
		_ = testDB.Drop(context.Background())
		_ = mongoConn.Close(context.Background())
	})
	bucketRepo, err := repository.NewBucketRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	fileRepo, err := repository.NewFileRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	shareLinkRepo, err := repository.NewShareLinkRepository(testDB)
	if err != nil {
		t.Fatal(err)
	}
	return bucketRepo, fileRepo, shareLinkRepo
}

func stringPtr(v string) *string {
	return &v
}
