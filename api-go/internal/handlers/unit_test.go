package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// setUserID is a helper middleware that injects a userID into the gin context.
func setUserID(id string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("userID", id)
		c.Next()
	}
}

func validUserID() string { return primitive.NewObjectID().Hex() }

// --- Source handler unit tests ---

func TestCreateGDriveSourceMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/gdrive", CreateGDriveSource(nil))

	body, _ := json.Marshal(map[string]string{"name": "only-name"}) // missing key
	req, _ := http.NewRequest(http.MethodPost, "/sources/gdrive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateGDriveSourceMalformedKeyJSON(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/gdrive", CreateGDriveSource(nil))

	body, _ := json.Marshal(map[string]string{"name": "gdrive", "key": "{"})
	req, _ := http.NewRequest(http.MethodPost, "/sources/gdrive", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateTelegramSourceMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/telegram", CreateTelegramSource(nil))

	body, _ := json.Marshal(map[string]string{"name": "tg"}) // missing token and chat_id
	req, _ := http.NewRequest(http.MethodPost, "/sources/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateTelegramSourceRejectsBlankConfig(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/telegram", CreateTelegramSource(nil))

	body, _ := json.Marshal(map[string]string{"name": "tg", "token": "   ", "chat_id": "123"})
	req, _ := http.NewRequest(http.MethodPost, "/sources/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateTelegramSourceValidConfigReachesPersistence(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/telegram", CreateTelegramSource(nil))

	body, _ := json.Marshal(map[string]string{"name": "tg", "token": "token", "chat_id": "123"})
	req, _ := http.NewRequest(http.MethodPost, "/sources/telegram", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCreateS3SourceMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/s3", CreateS3Source(nil))

	body, _ := json.Marshal(map[string]string{"name": "s3"}) // missing required fields
	req, _ := http.NewRequest(http.MethodPost, "/sources/s3", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateS3SourceRejectsMalformedEndpoint(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/s3", CreateS3Source(nil))

	body, _ := json.Marshal(map[string]any{
		"name":              "s3",
		"endpoint":          "not a url",
		"bucket":            "bucket",
		"access_key_id":     "access",
		"secret_access_key": "secret",
	})
	req, _ := http.NewRequest(http.MethodPost, "/sources/s3", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateS3SourceRejectsBlankConfig(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/s3", CreateS3Source(nil))

	body, _ := json.Marshal(map[string]any{
		"name":              "s3",
		"endpoint":          "https://s3.example.com",
		"bucket":            "   ",
		"access_key_id":     "access",
		"secret_access_key": "secret",
	})
	req, _ := http.NewRequest(http.MethodPost, "/sources/s3", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateS3SourceValidConfigReachesPersistence(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/sources/s3", CreateS3Source(nil))

	body, _ := json.Marshal(map[string]any{
		"name":              "s3",
		"endpoint":          "https://s3.example.com",
		"bucket":            "bucket",
		"access_key_id":     "access",
		"secret_access_key": "secret",
	})
	req, _ := http.NewRequest(http.MethodPost, "/sources/s3", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListSourcesNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources", ListSources(nil))

	req, _ := http.NewRequest(http.MethodGet, "/sources", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListSourcesNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources", ListSources(&repository.SourceRepository{}))

	req, _ := http.NewRequest(http.MethodGet, "/sources", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListSourcesInvalidUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources", setUserID("not-a-valid-oid"), ListSources(&repository.SourceRepository{}))

	req, _ := http.NewRequest(http.MethodGet, "/sources", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteSourceNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/sources/:id", DeleteSource(nil, nil))

	req, _ := http.NewRequest(http.MethodDelete, "/sources/"+primitive.NewObjectID().Hex(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDeleteSourceNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/sources/:id", DeleteSource(&repository.SourceRepository{}, &repository.BucketRepository{}))

	req, _ := http.NewRequest(http.MethodDelete, "/sources/"+primitive.NewObjectID().Hex(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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

	req, _ := http.NewRequest(http.MethodDelete, "/sources/not-a-valid-oid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestGetSourceInfoNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/info", GetSourceInfo(nil))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+primitive.NewObjectID().Hex()+"/info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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

	req, _ := http.NewRequest(http.MethodGet, "/sources/not-a-valid-oid/info", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestDownloadSourceFileNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download", DownloadSourceFile(nil))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+primitive.NewObjectID().Hex()+"/files/somefile/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestDownloadSourceFileNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/sources/:id/files/:file_id/download", DownloadSourceFile(&repository.SourceRepository{}))

	req, _ := http.NewRequest(http.MethodGet, "/sources/"+primitive.NewObjectID().Hex()+"/files/somefile/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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

	req, _ := http.NewRequest(http.MethodGet, "/sources/not-a-valid-oid/files/somefile/download", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- Bucket handler unit tests ---

func TestCreateBucketMissingFields(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets", CreateBucket(nil, nil, "secret"))

	body, _ := json.Marshal(map[string]string{"key": "k"}) // missing source_ids
	req, _ := http.NewRequest(http.MethodPost, "/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateBucketNilRepos(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets", CreateBucket(nil, nil, "secret"))

	body, _ := json.Marshal(map[string]any{"key": "k", "source_ids": []string{primitive.NewObjectID().Hex()}})
	req, _ := http.NewRequest(http.MethodPost, "/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestCreateBucketNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/buckets", CreateBucket(&repository.BucketRepository{}, &repository.SourceRepository{}, "secret"))

	body, _ := json.Marshal(map[string]any{"key": "k", "source_ids": []string{primitive.NewObjectID().Hex()}})
	req, _ := http.NewRequest(http.MethodPost, "/buckets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestListBucketsNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/buckets", ListBuckets(nil, nil))

	req, _ := http.NewRequest(http.MethodGet, "/buckets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestListBucketsNoUserID(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.GET("/buckets", ListBuckets(&repository.BucketRepository{}, nil))

	req, _ := http.NewRequest(http.MethodGet, "/buckets", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestDeleteBucketNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.DELETE("/buckets/:id", DeleteBucket(nil, nil, nil, nil, nil))

	req, _ := http.NewRequest(http.MethodDelete, "/buckets/"+primitive.NewObjectID().Hex(), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

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

	req, _ := http.NewRequest(http.MethodDelete, "/buckets/not-a-valid-oid", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// --- User handler unit tests ---

func TestCreateUserMissingUsername(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/users", CreateUser(nil))

	body, _ := json.Marshal(map[string]string{}) // missing username
	req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateUserNilRepo(t *testing.T) {
	t.Parallel()
	r := gin.New()
	r.POST("/users", CreateUser(nil))

	body, _ := json.Marshal(map[string]string{"username": "alice"})
	req, _ := http.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
