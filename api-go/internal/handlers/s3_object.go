package handlers

import (
	"context"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const (
	defaultObjectContentType = "application/octet-stream"
	userMetadataHeaderPrefix = "x-amz-meta-"
)

type copyObjectResult struct {
	XMLName      xml.Name `xml:"CopyObjectResult"`
	Xmlns        string   `xml:"xmlns,attr"`
	ETag         string   `xml:"ETag"`
	LastModified string   `xml:"LastModified"`
}

type objectRange struct {
	start int64
	end   int64
}

var (
	streamS3Object      = manager.StreamFile
	streamS3ObjectRange = manager.StreamFileRange
)

type objectFileReader interface {
	GetByName(ctx context.Context, bucketID primitive.ObjectID, name string) (*repository.File, error)
}

func s3ObjectKey(c *gin.Context) string {
	return strings.TrimPrefix(c.Param("object"), "/")
}

func parseCopySource(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "/")
	raw, _, _ = strings.Cut(raw, "?")
	parts := strings.SplitN(raw, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	bucket, err := url.PathUnescape(parts[0])
	if err != nil || bucket == "" {
		return "", "", false
	}
	key, err := url.PathUnescape(parts[1])
	if err != nil || key == "" {
		return "", "", false
	}
	return bucket, key, true
}

func parseObjectRange(raw string, total int64) (objectRange, bool) {
	if raw == "" {
		return objectRange{}, false
	}
	if total <= 0 || !strings.HasPrefix(raw, "bytes=") || strings.Contains(raw, ",") {
		return objectRange{}, false
	}
	spec := strings.TrimPrefix(raw, "bytes=")
	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return objectRange{}, false
	}
	if parts[0] == "" {
		suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLen <= 0 {
			return objectRange{}, false
		}
		if suffixLen >= total {
			return objectRange{start: 0, end: total - 1}, true
		}
		return objectRange{start: total - suffixLen, end: total - 1}, true
	}

	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= total {
		return objectRange{}, false
	}
	end := total - 1
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return objectRange{}, false
		}
		if end >= total {
			end = total - 1
		}
	}
	return objectRange{start: start, end: end}, true
}

func lookupObject(c *gin.Context, bucketRepo objectBucketReader, fileRepo objectFileReader) (*repository.File, int64, bool) {
	ctx := c.Request.Context()
	name := s3ObjectKey(c)
	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return nil, 0, false
	}
	fileDoc, err := fileRepo.GetByName(ctx, bucketDoc.ID, name)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchKey", "")
			return nil, 0, false
		}
		slog.ErrorContext(ctx, "lookup object: get file", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return nil, 0, false
	}
	return fileDoc, manager.FileSize(*fileDoc), true
}

func setObjectHeaders(c *gin.Context, fileDoc *repository.File, total int64) {
	c.Header("Accept-Ranges", "bytes")
	c.Header("ETag", manager.ObjectETag(*fileDoc))
	c.Header("Last-Modified", fileDoc.CreatedAt.UTC().Format(http.TimeFormat))
	c.Header("Content-Length", strconv.FormatInt(total, 10))
	c.Header("Content-Type", objectContentType(fileDoc.ContentType))
	for key, value := range fileDoc.UserMetadata {
		c.Header(userMetadataHeaderPrefix+key, value)
	}
}

func objectContentType(contentType string) string {
	contentType = strings.TrimSpace(contentType)
	if contentType == "" {
		return defaultObjectContentType
	}
	return contentType
}

func requestObjectContentType(r *http.Request) string {
	return objectContentType(r.Header.Get("Content-Type"))
}

func requestObjectUserMetadata(r *http.Request) map[string]string {
	metadata := make(map[string]string)
	for name, values := range r.Header {
		name = strings.ToLower(name)
		if !strings.HasPrefix(name, userMetadataHeaderPrefix) {
			continue
		}
		key := strings.TrimPrefix(name, userMetadataHeaderPrefix)
		if key == "" || len(values) == 0 {
			continue
		}
		metadata[key] = values[0]
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

// HeadObject godoc
// @Summary Head object
// @Tags s3
// @Param bucket path string true "Bucket key"
// @Param object path string true "Object key"
// @Success 200 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/s3/{bucket}/{object} [head]

func HeadObject(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		fileDoc, total, ok := lookupObject(c, bucketRepo, fileRepo)
		if !ok {
			return
		}
		setObjectHeaders(c, fileDoc, total)
		c.Status(http.StatusOK)
	}
}

// GetObject godoc
// @Summary Get object
// @Tags s3
// @Produce octet-stream
// @Param bucket path string true "Bucket key"
// @Param object path string true "Object key"
// @Success 200 {file} file
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/s3/{bucket}/{object} [get]
func GetObject(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return GetObjectWithFactory(bucketRepo, sourceRepo, fileRepo, nil)
}

func GetObjectWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
		return func(c *gin.Context) {
			c.Status(http.StatusServiceUnavailable)
		}
	}
	return getObject(bucketRepo, sourceRepo, fileRepo, factory)
}

func getObject(bucketRepo objectBucketReader, sourceRepo *repository.SourceRepository, fileRepo objectFileReader, factory manager.SourceClientFactory) gin.HandlerFunc {
	streamFile := streamS3Object
	streamRange := streamS3ObjectRange
	if factory != nil {
		streamFile = func(ctx context.Context, sourceRepo *repository.SourceRepository, fileDoc *repository.File, w io.Writer) error {
			return manager.StreamFileWithFactory(ctx, sourceRepo, fileDoc, w, factory)
		}
		streamRange = func(ctx context.Context, sourceRepo *repository.SourceRepository, fileDoc *repository.File, w io.Writer, start, end int64) error {
			return manager.StreamFileRangeWithFactory(ctx, sourceRepo, fileDoc, w, start, end, factory)
		}
	}
	return func(c *gin.Context) {
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		fileDoc, total, ok := lookupObject(c, bucketRepo, fileRepo)
		if !ok {
			return
		}
		rangeHeader := c.GetHeader("Range")
		if rangeHeader == "" {
			w := newDeferredResponseWriter(c, func() {
				setObjectHeaders(c, fileDoc, total)
				c.Status(http.StatusOK)
			})
			if err := streamFile(c.Request.Context(), sourceRepo, fileDoc, w); err != nil {
				if !w.isCommitted() {
					slog.ErrorContext(c.Request.Context(), "get object: stream failed", slog.String("error", err.Error()))
					writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
					return
				}
				slog.ErrorContext(c.Request.Context(), "get object: stream failed after response commit", slog.String("error", err.Error()))
				return
			}
			w.commitNow()
			return
		}

		objRange, ok := parseObjectRange(rangeHeader, total)
		if !ok {
			c.Header("Accept-Ranges", "bytes")
			c.Header("Content-Range", "bytes */"+strconv.FormatInt(total, 10))
			writeS3Error(c, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "The requested range is not satisfiable")
			return
		}
		w := newDeferredResponseWriter(c, func() {
			setObjectHeaders(c, fileDoc, total)
			c.Header("Content-Length", strconv.FormatInt(objRange.end-objRange.start+1, 10))
			c.Header("Content-Range", "bytes "+strconv.FormatInt(objRange.start, 10)+"-"+strconv.FormatInt(objRange.end, 10)+"/"+strconv.FormatInt(total, 10))
			c.Status(http.StatusPartialContent)
		})
		if err := streamRange(c.Request.Context(), sourceRepo, fileDoc, w, objRange.start, objRange.end); err != nil {
			if !w.isCommitted() {
				slog.ErrorContext(c.Request.Context(), "get object: stream failed", slog.String("error", err.Error()))
				writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
				return
			}
			slog.ErrorContext(c.Request.Context(), "get object: stream failed after response commit", slog.String("error", err.Error()))
			return
		}
		w.commitNow()
	}
}

// PutObject godoc
// @Summary Put object
// @Tags s3
// @Accept octet-stream
// @Param bucket path string true "Bucket key"
// @Param object path string true "Object key"
// @Param body body string true "Object content"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/s3/{bucket}/{object} [put]
func PutObject(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, chunkSize int) gin.HandlerFunc {
	return PutObjectWithFactory(bucketRepo, sourceRepo, fileRepo, chunkSize, nil)
}

func PutObjectWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, chunkSize int, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectServiceWithSourceClientFactory(sourceRepo, fileRepo, nil, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		name := s3ObjectKey(c)
		if name == "" {
			writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "")
			return
		}
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		result, err := objectSvc.PutObject(ctx, bucketDoc, name, c.Request.Body, chunkSize, requestObjectContentType(c.Request), requestObjectUserMetadata(c.Request))
		if err != nil {
			if isBucketSourceResolutionError(err) {
				writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "")
				return
			}
			slog.ErrorContext(ctx, "put object: mutate object", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if result.CleanupErr != nil {
			slog.WarnContext(ctx, "put object: delete old chunks", slog.String("error", result.CleanupErr.Error()))
		}
		c.Status(http.StatusOK)
	}
}

// CopyObject godoc
// @Summary Copy object
// @Tags s3
// @Param bucket path string true "Destination bucket key"
// @Param object path string true "Destination object key"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/s3/{bucket}/{object} [put]
func CopyObject(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return CopyObjectWithFactory(bucketRepo, sourceRepo, fileRepo, nil)
}

func CopyObjectWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectServiceWithSourceClientFactory(sourceRepo, fileRepo, nil, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		destBucket, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		destKey := s3ObjectKey(c)
		if destKey == "" {
			writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "empty destination object key")
			return
		}
		directive := strings.ToUpper(strings.TrimSpace(c.GetHeader("x-amz-metadata-directive")))
		if directive == "" {
			directive = "COPY"
		}
		if directive != "COPY" {
			writeS3Error(c, http.StatusNotImplemented, "NotImplemented", "x-amz-metadata-directive REPLACE is not supported")
			return
		}
		sourceBucketKey, sourceKey, ok := parseCopySource(c.GetHeader("x-amz-copy-source"))
		if !ok {
			writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "x-amz-copy-source must be /bucket/key")
			return
		}
		sourceBucket, err := bucketRepo.GetByKey(ctx, sourceBucketKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
				return
			}
			slog.ErrorContext(ctx, "copy object: get source bucket", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		result, err := objectSvc.CopyObject(ctx, sourceBucket, destBucket, sourceKey, destKey)
		if err != nil {
			switch {
			case errors.Is(err, manager.ErrAccessDenied):
				writeS3Error(c, http.StatusForbidden, "AccessDenied", "")
			case errors.Is(err, manager.ErrObjectNotFound):
				writeS3Error(c, http.StatusNotFound, "NoSuchKey", "")
			default:
				slog.ErrorContext(ctx, "copy object: mutate object", slog.String("error", err.Error()))
				writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			}
			return
		}
		if result.CleanupErr != nil {
			slog.WarnContext(ctx, "copy object: delete old chunks", slog.String("error", result.CleanupErr.Error()))
		}
		c.XML(http.StatusOK, copyObjectResult{
			Xmlns:        "http://s3.amazonaws.com/doc/2006-03-01/",
			ETag:         manager.ObjectETag(result.File),
			LastModified: result.File.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
}

// DeleteObject godoc
// @Summary Delete object
// @Tags s3
// @Param bucket path string true "Bucket key"
// @Param object path string true "Object key"
// @Success 204 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/s3/{bucket}/{object} [delete]
func DeleteObject(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return DeleteObjectWithFactory(bucketRepo, sourceRepo, fileRepo, nil)
}

func DeleteObjectWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectServiceWithSourceClientFactory(sourceRepo, fileRepo, nil, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		name := s3ObjectKey(c)
		if name == "" {
			c.Status(http.StatusNoContent)
			return
		}
		result, err := objectSvc.DeleteObject(ctx, bucketDoc.ID, name)
		if err != nil {
			slog.ErrorContext(ctx, "delete object: delete file", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if result.CleanupErr != nil {
			slog.WarnContext(ctx, "delete object: delete chunks", slog.String("error", result.CleanupErr.Error()))
		}
		c.Status(http.StatusNoContent)
	}
}
