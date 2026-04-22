package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/db"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestSetupRouter(t *testing.T) {
	r, err := SetupRouter(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	endpoints := []string{"/readyz", "/healthz", "/publication/ready"}
	for _, e := range endpoints {
		req, _ := http.NewRequest(http.MethodGet, e, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d", e, w.Code)
		}
	}
}

func TestSetupRouterNilMongoRouteSet(t *testing.T) {
	r, err := SetupRouter(nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	expectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/readyz"},
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/publication/ready"},
		{http.MethodGet, "/dbz"},
		{http.MethodGet, "/api/openapi.json"},
		{http.MethodGet, "/api/docs"},
		{http.MethodGet, "/api/docs/*any"},
		{http.MethodGet, "/swagger/*any"},
		{http.MethodGet, "/metrics"},
	}
	for _, expected := range expectedRoutes {
		if !hasRoute(r, expected.method, expected.path) {
			t.Fatalf("expected %s %s to be registered", expected.method, expected.path)
		}
	}

	unexpectedRoutes := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/buckets"},
		{http.MethodGet, "/share/:token"},
		{http.MethodHead, "/api/s3/:bucket/*object"},
	}
	for _, unexpected := range unexpectedRoutes {
		if hasRoute(r, unexpected.method, unexpected.path) {
			t.Fatalf("did not expect %s %s to be registered", unexpected.method, unexpected.path)
		}
	}
}

func TestOpenAPIJSONRoute(t *testing.T) {
	r, err := SetupRouter(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/api/openapi.json", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if contentType := w.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Fatalf("expected API docs JSON content type, got %q", contentType)
	}
	var doc struct {
		Swagger string                         `json:"swagger"`
		Paths   map[string]map[string]struct{} `json:"paths"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &doc); err != nil {
		t.Fatalf("expected valid API docs JSON: %v", err)
	}
	if doc.Swagger != "2.0" {
		t.Fatalf("expected Swagger 2.0, got %q", doc.Swagger)
	}
	for _, path := range []string{"/api/v1/buckets", "/api/v1/sources/s3", "/api/s3/{bucket}", "/api/s3/{bucket}/{object}"} {
		if _, ok := doc.Paths[path]; !ok {
			t.Fatalf("expected OpenAPI path %s", path)
		}
	}
}

func TestOpenAPIDocsRouteRedirectsToIndex(t *testing.T) {
	r, err := SetupRouter(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "/api/docs", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusMovedPermanently {
		t.Fatalf("expected 301, got %d", w.Code)
	}
	if got := w.Header().Get("Location"); got != "/api/docs/index.html" {
		t.Fatalf("expected redirect to /api/docs/index.html, got %q", got)
	}
}

func TestNewRouterDependenciesReturnsRepositoryErrors(t *testing.T) {
	tests := []struct {
		name string
		fail func(error)
	}{
		{
			name: "user repository",
			fail: func(failure error) {
				newUserRepository = func(*mongo.Database) (*repository.UserRepository, error) {
					return nil, failure
				}
			},
		},
		{
			name: "bucket repository",
			fail: func(failure error) {
				newBucketRepository = func(*mongo.Database) (*repository.BucketRepository, error) {
					return nil, failure
				}
			},
		},
		{
			name: "source repository",
			fail: func(failure error) {
				newSourceRepository = func(*mongo.Database, ...string) (*repository.SourceRepository, error) {
					return nil, failure
				}
			},
		},
		{
			name: "file repository",
			fail: func(failure error) {
				newFileRepository = func(*mongo.Database) (*repository.FileRepository, error) {
					return nil, failure
				}
			},
		},
		{
			name: "multipart upload repository",
			fail: func(failure error) {
				newMultipartUploadRepository = func(*mongo.Database) (*repository.MultipartUploadRepository, error) {
					return nil, failure
				}
			},
		},
		{
			name: "share link repository",
			fail: func(failure error) {
				newShareLinkRepository = func(*mongo.Database) (*repository.ShareLinkRepository, error) {
					return nil, failure
				}
			},
		},
		{
			name: "bucket grant repository",
			fail: func(failure error) {
				newBucketGrantRepository = func(*mongo.Database) (*repository.BucketGrantRepository, error) {
					return nil, failure
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubRouterRepositoryConstructors(t)
			failure := errors.New("create index")
			tt.fail(failure)

			deps, err := newRouterDependencies(&db.Mongo{}, nil)
			if err == nil {
				t.Fatal("expected repository initialization error")
			}
			if deps != nil {
				t.Fatal("expected nil dependencies on initialization error")
			}
			if !errors.Is(err, failure) {
				t.Fatalf("expected error to wrap failure, got %v", err)
			}
			if !strings.Contains(err.Error(), "initialize "+tt.name) {
				t.Fatalf("expected repository name in error, got %v", err)
			}
		})
	}
}

func hasRoute(r *gin.Engine, method, path string) bool {
	for _, route := range r.Routes() {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

func stubRouterRepositoryConstructors(t *testing.T) {
	originalUser := newUserRepository
	originalBucket := newBucketRepository
	originalSource := newSourceRepository
	originalFile := newFileRepository
	originalShareLink := newShareLinkRepository
	originalMultipartUpload := newMultipartUploadRepository
	originalBucketGrant := newBucketGrantRepository

	t.Cleanup(func() {
		newUserRepository = originalUser
		newBucketRepository = originalBucket
		newSourceRepository = originalSource
		newFileRepository = originalFile
		newShareLinkRepository = originalShareLink
		newMultipartUploadRepository = originalMultipartUpload
		newBucketGrantRepository = originalBucketGrant
	})

	newUserRepository = func(*mongo.Database) (*repository.UserRepository, error) {
		return &repository.UserRepository{}, nil
	}
	newBucketRepository = func(*mongo.Database) (*repository.BucketRepository, error) {
		return &repository.BucketRepository{}, nil
	}
	newSourceRepository = func(*mongo.Database, ...string) (*repository.SourceRepository, error) {
		return &repository.SourceRepository{}, nil
	}
	newFileRepository = func(*mongo.Database) (*repository.FileRepository, error) {
		return &repository.FileRepository{}, nil
	}
	newMultipartUploadRepository = func(*mongo.Database) (*repository.MultipartUploadRepository, error) {
		return &repository.MultipartUploadRepository{}, nil
	}
	newShareLinkRepository = func(*mongo.Database) (*repository.ShareLinkRepository, error) {
		return &repository.ShareLinkRepository{}, nil
	}
	newBucketGrantRepository = func(*mongo.Database) (*repository.BucketGrantRepository, error) {
		return &repository.BucketGrantRepository{}, nil
	}
}
