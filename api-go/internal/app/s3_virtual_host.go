package app

import (
	"net"
	"net/http"
	"strings"

	"github.com/example/sfree/api-go/internal/handlers"
	"github.com/gin-gonic/gin"
)

func s3GetDispatchHandler(listBuckets, listObjectsOrUploads, getObjectOrParts gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, objectKey, hasBucket := applyS3DispatchTarget(c)
		if !hasBucket {
			listBuckets(c)
			return
		}
		if objectKey == "" {
			listObjectsOrUploads(c)
			return
		}
		getObjectOrParts(c)
	}
}

func s3HeadDispatchHandler(headBucket, headObject gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, objectKey, hasBucket := applyS3DispatchTarget(c)
		if !hasBucket {
			c.Status(http.StatusNotFound)
			return
		}
		if objectKey == "" {
			headBucket(c)
			return
		}
		headObject(c)
	}
}

func s3PutDispatchHandler(putObjectOrPart gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, objectKey, hasBucket := applyS3DispatchTarget(c)
		if !hasBucket || objectKey == "" {
			c.Status(http.StatusNotFound)
			return
		}
		putObjectOrPart(c)
	}
}

func s3PostDispatchHandler(postBucket, postObject gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, objectKey, hasBucket := applyS3DispatchTarget(c)
		if !hasBucket {
			c.Status(http.StatusNotFound)
			return
		}
		if objectKey == "" {
			postBucket(c)
			return
		}
		postObject(c)
	}
}

func s3DeleteDispatchHandler(deleteObjectOrAbort gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		_, objectKey, hasBucket := applyS3DispatchTarget(c)
		if !hasBucket || objectKey == "" {
			c.Status(http.StatusNotFound)
			return
		}
		deleteObjectOrAbort(c)
	}
}

func applyS3DispatchTarget(c *gin.Context) (string, string, bool) {
	if bucketKey, objectKey, ok := s3VirtualHostedTarget(c.Request); ok {
		handlers.SetS3BucketKey(c, bucketKey)
		handlers.SetS3ObjectKey(c, objectKey)
		return bucketKey, objectKey, true
	}

	bucketKey := handlers.S3BucketKey(c)
	if bucketKey == "" {
		return "", "", false
	}

	objectKey := strings.TrimPrefix(c.Param("object"), "/")
	handlers.SetS3BucketKey(c, bucketKey)
	handlers.SetS3ObjectKey(c, objectKey)
	return bucketKey, objectKey, true
}

func s3VirtualHostedTarget(r *http.Request) (string, string, bool) {
	bucketKey := s3VirtualHostedBucket(r)
	if bucketKey == "" {
		return "", "", false
	}
	objectKey := ""
	if r != nil && r.URL != nil {
		objectKey = strings.TrimPrefix(r.URL.Path, "/")
	}
	return bucketKey, objectKey, true
}

func s3VirtualHostedBucket(r *http.Request) string {
	if r == nil || r.URL == nil || strings.HasPrefix(r.URL.Path, "/api/s3") {
		return ""
	}

	host := s3RequestHost(r)
	if host == "" || net.ParseIP(host) != nil {
		return ""
	}

	labels := strings.Split(strings.ToLower(host), ".")
	if len(labels) < 2 || labels[0] == "" {
		return ""
	}
	return labels[0]
}

func s3RequestHost(r *http.Request) string {
	if r == nil {
		return ""
	}

	host := r.Host
	if host == "" && r.URL != nil {
		host = r.URL.Host
	}
	if host == "" {
		host = r.Header.Get("Host")
	}
	if host == "" {
		return ""
	}

	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		return parsedHost
	}
	return strings.Trim(host, "[]")
}
