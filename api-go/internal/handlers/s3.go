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
	maxDeleteObjects                = 1000
	maxDeleteObjectsRequestBodySize = 8 * 1024 * 1024
)

var (
	errDeleteObjectsMalformedXML = errors.New("malformed delete objects XML")
	errDeleteObjectsTooMany      = errors.New("too many delete objects")
)

type s3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

type deleteObjectsRequest struct {
	XMLName xml.Name              `xml:"Delete"`
	Quiet   bool                  `xml:"Quiet"`
	Objects []deleteObjectRequest `xml:"Object"`
}

type deleteObjectRequest struct {
	Key       string `xml:"Key"`
	VersionID string `xml:"VersionId,omitempty"`
}

type deleteObjectsResult struct {
	XMLName xml.Name              `xml:"DeleteResult"`
	Xmlns   string                `xml:"xmlns,attr"`
	Deleted []deletedObjectResult `xml:"Deleted,omitempty"`
	Errors  []deleteObjectError   `xml:"Error,omitempty"`
}

type copyObjectResult struct {
	XMLName      xml.Name `xml:"CopyObjectResult"`
	Xmlns        string   `xml:"xmlns,attr"`
	ETag         string   `xml:"ETag"`
	LastModified string   `xml:"LastModified"`
}

type deletedObjectResult struct {
	Key string `xml:"Key"`
}

type deleteObjectError struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

func decodeDeleteObjectsRequest(r io.Reader) (deleteObjectsRequest, error) {
	decoder := xml.NewDecoder(r)
	var req deleteObjectsRequest
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return req, errDeleteObjectsMalformedXML
			}
			return req, err
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "Delete" {
			return req, errDeleteObjectsMalformedXML
		}
		if err := decodeDeleteObjectsElement(decoder, &req); err != nil {
			return req, err
		}
		return req, nil
	}
}

func decodeDeleteObjectsElement(decoder *xml.Decoder, req *deleteObjectsRequest) error {
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return errDeleteObjectsMalformedXML
			}
			return err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "Quiet":
				if err := decoder.DecodeElement(&req.Quiet, &tok); err != nil {
					return err
				}
			case "Object":
				if len(req.Objects) >= maxDeleteObjects {
					return errDeleteObjectsTooMany
				}
				obj, err := decodeDeleteObjectElement(decoder)
				if err != nil {
					return err
				}
				req.Objects = append(req.Objects, obj)
			default:
				if err := decoder.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if tok.Name.Local == "Delete" {
				return nil
			}
		}
	}
}

func decodeDeleteObjectElement(decoder *xml.Decoder) (deleteObjectRequest, error) {
	var obj deleteObjectRequest
	for {
		token, err := decoder.Token()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return obj, errDeleteObjectsMalformedXML
			}
			return obj, err
		}
		switch tok := token.(type) {
		case xml.StartElement:
			switch tok.Name.Local {
			case "Key":
				if err := decoder.DecodeElement(&obj.Key, &tok); err != nil {
					return obj, err
				}
			case "VersionId":
				if err := decoder.DecodeElement(&obj.VersionID, &tok); err != nil {
					return obj, err
				}
			default:
				if err := decoder.Skip(); err != nil {
					return obj, err
				}
			}
		case xml.EndElement:
			if tok.Name.Local == "Object" {
				return obj, nil
			}
		}
	}
}

type objectRange struct {
	start int64
	end   int64
}

const (
	s3GetPreflightBytes = int64(1)
)

var (
	streamS3Object      = manager.StreamFile
	streamS3ObjectRange = manager.StreamFileRange
)

type objectBucketReader interface {
	GetByKey(ctx context.Context, key string) (*repository.Bucket, error)
}

type objectFileReader interface {
	GetByName(ctx context.Context, bucketID primitive.ObjectID, name string) (*repository.File, error)
}

func writeS3Error(c *gin.Context, status int, code, message string) {
	c.XML(status, s3Error{Code: code, Message: message})
}

func lookupBucket(c *gin.Context, bucketRepo objectBucketReader) (*repository.Bucket, bool) {
	ctx := c.Request.Context()
	bucketDoc, err := bucketRepo.GetByKey(ctx, c.Param("bucket"))
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return nil, false
		}
		slog.ErrorContext(ctx, "lookup bucket", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return nil, false
	}
	accessKey := c.GetString("accessKey")
	if accessKey == "" || bucketDoc.AccessKey != accessKey {
		writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
		return nil, false
	}
	return bucketDoc, true
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
	name := strings.TrimPrefix(c.Param("object"), "/")
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
	var total int64
	for _, ch := range fileDoc.Chunks {
		total += ch.Size
	}
	return fileDoc, total, true
}

func setObjectHeaders(c *gin.Context, fileDoc *repository.File, total int64) {
	c.Header("Accept-Ranges", "bytes")
	c.Header("ETag", manager.ObjectETag(*fileDoc))
	c.Header("Last-Modified", fileDoc.CreatedAt.UTC().Format(http.TimeFormat))
	c.Header("Content-Length", strconv.FormatInt(total, 10))
	c.Header("Content-Type", "application/octet-stream")
}

// preflightObjectRange checks the first response byte before success headers are
// committed. Later source failures are logged and surface to clients as an
// incomplete body under the declared Content-Length, which preserves large
// object streaming without staging the full response in memory or on disk.
func preflightObjectRange(ctx context.Context, sourceRepo *repository.SourceRepository, fileDoc *repository.File, start, end int64) error {
	if end < start {
		return nil
	}
	preflightEnd := start + s3GetPreflightBytes - 1
	if preflightEnd > end {
		preflightEnd = end
	}
	return streamS3ObjectRange(ctx, sourceRepo, fileDoc, io.Discard, start, preflightEnd)
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
	if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
		return func(c *gin.Context) {
			c.Status(http.StatusServiceUnavailable)
		}
	}
	return getObject(bucketRepo, sourceRepo, fileRepo)
}

func getObject(bucketRepo objectBucketReader, sourceRepo *repository.SourceRepository, fileRepo objectFileReader) gin.HandlerFunc {
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
			if err := preflightObjectRange(c.Request.Context(), sourceRepo, fileDoc, 0, total-1); err != nil {
				slog.ErrorContext(c.Request.Context(), "get object: stream failed", slog.String("error", err.Error()))
				writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
				return
			}
			setObjectHeaders(c, fileDoc, total)
			c.Status(http.StatusOK)
			if err := streamS3Object(c.Request.Context(), sourceRepo, fileDoc, c.Writer); err != nil {
				slog.ErrorContext(c.Request.Context(), "get object: stream failed after response commit", slog.String("error", err.Error()))
			}
			return
		}

		objRange, ok := parseObjectRange(rangeHeader, total)
		if !ok {
			c.Header("Accept-Ranges", "bytes")
			c.Header("Content-Range", "bytes */"+strconv.FormatInt(total, 10))
			writeS3Error(c, http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "The requested range is not satisfiable")
			return
		}
		if err := preflightObjectRange(c.Request.Context(), sourceRepo, fileDoc, objRange.start, objRange.end); err != nil {
			slog.ErrorContext(c.Request.Context(), "get object: stream failed", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		setObjectHeaders(c, fileDoc, total)
		c.Header("Content-Length", strconv.FormatInt(objRange.end-objRange.start+1, 10))
		c.Header("Content-Range", "bytes "+strconv.FormatInt(objRange.start, 10)+"-"+strconv.FormatInt(objRange.end, 10)+"/"+strconv.FormatInt(total, 10))
		c.Status(http.StatusPartialContent)
		if err := streamS3ObjectRange(c.Request.Context(), sourceRepo, fileDoc, c.Writer, objRange.start, objRange.end); err != nil {
			slog.ErrorContext(c.Request.Context(), "get object: stream failed after response commit", slog.String("error", err.Error()))
		}
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
	objectSvc := manager.NewObjectService(sourceRepo, fileRepo, nil)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		name := strings.TrimPrefix(c.Param("object"), "/")
		if name == "" {
			writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "")
			return
		}
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		result, err := objectSvc.PutObject(ctx, bucketDoc, name, c.Request.Body, chunkSize)
		if err != nil {
			if errors.Is(err, manager.ErrNoSources) {
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
	objectSvc := manager.NewObjectService(sourceRepo, fileRepo, nil)
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
		destKey := strings.TrimPrefix(c.Param("object"), "/")
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
	objectSvc := manager.NewObjectService(sourceRepo, fileRepo, nil)
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
		name := strings.TrimPrefix(c.Param("object"), "/")
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

func DeleteObjects(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	objectSvc := manager.NewObjectService(sourceRepo, fileRepo, nil)
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

		req, err := decodeDeleteObjectsRequest(http.MaxBytesReader(c.Writer, c.Request.Body, maxDeleteObjectsRequestBodySize))
		if err != nil {
			if errors.Is(err, errDeleteObjectsTooMany) {
				writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "DeleteObjects supports at most 1000 objects per request")
				return
			}
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "DeleteObjects request body is too large")
				return
			}
			writeS3Error(c, http.StatusBadRequest, "MalformedXML", "The XML you provided was not well-formed or did not validate against our published schema")
			return
		}

		result := deleteObjectsResult{
			Xmlns:   "http://s3.amazonaws.com/doc/2006-03-01/",
			Deleted: make([]deletedObjectResult, 0, len(req.Objects)),
			Errors:  make([]deleteObjectError, 0),
		}
		for _, obj := range req.Objects {
			key := obj.Key
			if key == "" {
				result.Errors = append(result.Errors, deleteObjectError{
					Key:     key,
					Code:    "InvalidArgument",
					Message: "Object key must not be empty",
				})
				continue
			}
			deleteResult, err := objectSvc.DeleteObject(ctx, bucketDoc.ID, key)
			if err != nil {
				slog.ErrorContext(ctx, "delete objects: delete file", slog.String("key", key), slog.String("error", err.Error()))
				result.Errors = append(result.Errors, deleteObjectError{
					Key:     key,
					Code:    "InternalError",
					Message: "Internal error deleting object",
				})
				continue
			}
			if deleteResult.CleanupErr != nil {
				slog.WarnContext(ctx, "delete objects: delete chunks", slog.String("key", key), slog.String("error", deleteResult.CleanupErr.Error()))
			}
			if !req.Quiet {
				result.Deleted = append(result.Deleted, deletedObjectResult{Key: key})
			}
		}
		c.XML(http.StatusOK, result)
	}
}
