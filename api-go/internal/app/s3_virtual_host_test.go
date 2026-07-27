package app

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/example/sfree/api-go/internal/handlers"
	"github.com/gin-gonic/gin"
)

func TestS3VirtualHostedBucket(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		target string
		host   string
		want   string
	}{
		{name: "bucket subdomain", target: "/", host: "bucket.localhost:9000", want: "bucket"},
		{name: "legacy path ignored", target: "/api/s3/bucket/object.txt", host: "bucket.localhost:9000"},
		{name: "localhost ignored", target: "/", host: "localhost:9000"},
		{name: "ipv4 ignored", target: "/", host: "127.0.0.1:9000"},
		{name: "ipv6 ignored", target: "/", host: "[::1]:9000"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			req.Host = tt.host

			if got := s3VirtualHostedBucket(req); got != tt.want {
				t.Fatalf("expected bucket %q, got %q", tt.want, got)
			}
		})
	}
}

func TestS3GetDispatchHandlerListsBucketsAtBareRoot(t *testing.T) {
	t.Parallel()

	c, _ := newS3DispatchTestContext(t, http.MethodGet, "/", "", nil)
	var listBucketsCalled bool
	handler := s3GetDispatchHandler(
		func(c *gin.Context) {
			listBucketsCalled = true
			c.Status(http.StatusOK)
		},
		func(c *gin.Context) {
			t.Fatal("list objects handler should not run")
		},
		func(c *gin.Context) {
			t.Fatal("get object handler should not run")
		},
	)

	handler(c)

	if !listBucketsCalled {
		t.Fatal("expected list buckets handler to run")
	}
	if got := c.Writer.Status(); got != http.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
}

func TestS3GetDispatchHandlerListsObjectsAtVirtualRoot(t *testing.T) {
	t.Parallel()

	c, _ := newS3DispatchTestContext(t, http.MethodGet, "/", "bucket.localhost:9000", nil)
	var gotBucket string
	handler := s3GetDispatchHandler(
		func(c *gin.Context) {
			t.Fatal("list buckets handler should not run")
		},
		func(c *gin.Context) {
			gotBucket = handlers.S3BucketKey(c)
			c.Status(http.StatusOK)
		},
		func(c *gin.Context) {
			t.Fatal("get object handler should not run")
		},
	)

	handler(c)

	if gotBucket != "bucket" {
		t.Fatalf("expected virtual bucket %q, got %q", "bucket", gotBucket)
	}
	if got := c.Param("object"); got != "" {
		t.Fatalf("expected empty object param, got %q", got)
	}
	if got := c.Writer.Status(); got != http.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
}

func TestS3GetDispatchHandlerGetsObjectForVirtualSingleSegmentPath(t *testing.T) {
	t.Parallel()

	c, _ := newS3DispatchTestContext(t, http.MethodGet, "/readme.txt", "bucket.localhost:9000", gin.Params{
		{Key: "bucket", Value: "readme.txt"},
	})
	var gotBucket string
	var gotObject string
	handler := s3GetDispatchHandler(
		func(c *gin.Context) {
			t.Fatal("list buckets handler should not run")
		},
		func(c *gin.Context) {
			t.Fatal("list objects handler should not run")
		},
		func(c *gin.Context) {
			gotBucket = handlers.S3BucketKey(c)
			gotObject = c.Param("object")
			c.Status(http.StatusOK)
		},
	)

	handler(c)

	if gotBucket != "bucket" {
		t.Fatalf("expected virtual bucket %q, got %q", "bucket", gotBucket)
	}
	if gotObject != "/readme.txt" {
		t.Fatalf("expected object param %q, got %q", "/readme.txt", gotObject)
	}
	if got := c.Writer.Status(); got != http.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
}

func TestS3HeadDispatchHandlerRejectsBareRoot(t *testing.T) {
	t.Parallel()

	c, _ := newS3DispatchTestContext(t, http.MethodHead, "/", "", nil)
	handler := s3HeadDispatchHandler(
		func(c *gin.Context) {
			t.Fatal("head bucket handler should not run")
		},
		func(c *gin.Context) {
			t.Fatal("head object handler should not run")
		},
	)

	handler(c)

	if got := c.Writer.Status(); got != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", got)
	}
}

func TestS3PostDispatchHandlerUsesBucketOperationAtVirtualRoot(t *testing.T) {
	t.Parallel()

	c, _ := newS3DispatchTestContext(t, http.MethodPost, "/?delete", "bucket.localhost:9000", nil)
	var gotBucket string
	handler := s3PostDispatchHandler(
		func(c *gin.Context) {
			gotBucket = handlers.S3BucketKey(c)
			c.Status(http.StatusNoContent)
		},
		func(c *gin.Context) {
			t.Fatal("post object handler should not run")
		},
	)

	handler(c)

	if gotBucket != "bucket" {
		t.Fatalf("expected virtual bucket %q, got %q", "bucket", gotBucket)
	}
	if got := c.Writer.Status(); got != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", got)
	}
}

func TestS3PutDispatchHandlerUsesVirtualSingleSegmentObject(t *testing.T) {
	t.Parallel()

	c, _ := newS3DispatchTestContext(t, http.MethodPut, "/upload.bin", "bucket.localhost:9000", gin.Params{
		{Key: "bucket", Value: "upload.bin"},
	})
	var gotBucket string
	var gotObject string
	handler := s3PutDispatchHandler(func(c *gin.Context) {
		gotBucket = handlers.S3BucketKey(c)
		gotObject = c.Param("object")
		c.Status(http.StatusOK)
	})

	handler(c)

	if gotBucket != "bucket" {
		t.Fatalf("expected virtual bucket %q, got %q", "bucket", gotBucket)
	}
	if gotObject != "/upload.bin" {
		t.Fatalf("expected object param %q, got %q", "/upload.bin", gotObject)
	}
	if got := c.Writer.Status(); got != http.StatusOK {
		t.Fatalf("expected 200, got %d", got)
	}
}

func newS3DispatchTestContext(t *testing.T, method, target, host string, params gin.Params) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()

	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, target, nil)
	req.Host = host
	c.Request = req
	c.Params = params
	return c, w
}
