package handlers

import (
	"net/http"
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// --- Source handler unit tests ---

func TestCreateGDriveSourceMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/gdrive", CreateGDriveSource(nil))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/sources/gdrive", map[string]string{"name": "only-name"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateTelegramSourceMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/telegram", CreateTelegramSource(nil))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/sources/telegram", map[string]string{"name": "tg"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateS3SourceMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/s3", CreateS3Source(nil))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/sources/s3", map[string]string{"name": "s3"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestListSourcesNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources", ListSources(nil))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources", nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListSourcesNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources", ListSources(&repository.SourceRepository{}))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources", nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListSourcesInvalidUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources", setUserID("not-a-valid-oid"), ListSources(&repository.SourceRepository{}))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources", nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteSourceNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/sources/:id", DeleteSource(nil, nil))

	w := serveHandlerTestRequest(t, r, http.MethodDelete, "/sources/"+primitive.NewObjectID().Hex(), nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteSourceNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/sources/:id", DeleteSource(&repository.SourceRepository{}, &repository.BucketRepository{}))

	w := serveHandlerTestRequest(t, r, http.MethodDelete, "/sources/"+primitive.NewObjectID().Hex(), nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteSourceInvalidIDParam(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/sources/:id",
		setUserID(validUserID()),
		DeleteSource(&repository.SourceRepository{}, &repository.BucketRepository{}),
	)

	w := serveHandlerTestRequest(t, r, http.MethodDelete, "/sources/not-a-valid-oid", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetSourceInfoNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/info", GetSourceInfo(nil))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources/"+primitive.NewObjectID().Hex()+"/info", nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestGetSourceInfoInvalidIDParam(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/info",
		setUserID(validUserID()),
		GetSourceInfo(&repository.SourceRepository{}),
	)

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources/not-a-valid-oid/info", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDownloadSourceFileNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download", DownloadSourceFile(nil))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources/"+primitive.NewObjectID().Hex()+"/files/somefile/download", nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDownloadSourceFileNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download", DownloadSourceFile(&repository.SourceRepository{}))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources/"+primitive.NewObjectID().Hex()+"/files/somefile/download", nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDownloadSourceFileInvalidSourceID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download",
		setUserID(validUserID()),
		DownloadSourceFile(&repository.SourceRepository{}),
	)

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/sources/not-a-valid-oid/files/somefile/download", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Bucket handler unit tests ---

func TestCreateBucketMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets", CreateBucket(nil, nil, "secret"))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/buckets", map[string]string{"key": "k"})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateBucketNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets", CreateBucket(nil, nil, "secret"))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/buckets", map[string]any{"key": "k", "source_ids": []string{primitive.NewObjectID().Hex()}})

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCreateBucketNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets", CreateBucket(&repository.BucketRepository{}, &repository.SourceRepository{}, "secret"))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/buckets", map[string]any{"key": "k", "source_ids": []string{primitive.NewObjectID().Hex()}})

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListBucketsNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/buckets", ListBuckets(nil, nil))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/buckets", nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListBucketsNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/buckets", ListBuckets(&repository.BucketRepository{}, nil))

	w := serveHandlerTestRequest(t, r, http.MethodGet, "/buckets", nil)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteBucketNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/buckets/:id", DeleteBucket(nil, nil, nil, nil, nil))

	w := serveHandlerTestRequest(t, r, http.MethodDelete, "/buckets/"+primitive.NewObjectID().Hex(), nil)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteBucketInvalidIDParam(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/buckets/:id",
		setUserID(validUserID()),
		DeleteBucket(&repository.BucketRepository{}, nil, nil, nil, nil),
	)

	w := serveHandlerTestRequest(t, r, http.MethodDelete, "/buckets/not-a-valid-oid", nil)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- User handler unit tests ---

func TestCreateUserMissingUsername(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/users", CreateUser(nil))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/users", map[string]string{})

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateUserNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/users", CreateUser(nil))

	w := serveHandlerTestRequest(t, r, http.MethodPost, "/users", map[string]string{"username": "alice"})

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
