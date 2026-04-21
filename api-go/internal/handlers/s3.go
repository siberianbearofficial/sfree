package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"log/slog"
	"net/http"
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

type listBucketResult struct {
	XMLName               xml.Name           `xml:"ListBucketResult"`
	Xmlns                 string             `xml:"xmlns,attr"`
	Name                  string             `xml:"Name"`
	Prefix                string             `xml:"Prefix"`
	Marker                string             `xml:"Marker,omitempty"`
	NextMarker            string             `xml:"NextMarker,omitempty"`
	MaxKeys               int                `xml:"MaxKeys"`
	Delimiter             string             `xml:"Delimiter,omitempty"`
	IsTruncated           bool               `xml:"IsTruncated"`
	Contents              []listBucketEntry  `xml:"Contents"`
	CommonPrefixes        []listCommonPrefix `xml:"CommonPrefixes"`
	KeyCount              int                `xml:"KeyCount"`
	ContinuationToken     string             `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string             `xml:"NextContinuationToken,omitempty"`
	StartAfter            string             `xml:"StartAfter,omitempty"`
}

type listBucketEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type listCommonPrefix struct {
	Prefix string `xml:"Prefix"`
}

type listBucketPageItem struct {
	sortKey      string
	entry        listBucketEntry
	hasEntry     bool
	commonPrefix string
}

type objectRange struct {
	start int64
	end   int64
}

const (
	listCommonPrefixSkipSuffix = "\U0010FFFF"
	s3GetPreflightBytes        = int64(1)
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

func deleteObjectByName(ctx context.Context, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, bucketID primitive.ObjectID, name string) (bool, error) {
	fileDoc, err := fileRepo.GetByName(ctx, bucketID, name)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	if err := fileRepo.Delete(ctx, fileDoc.ID); err != nil {
		if err == mongo.ErrNoDocuments {
			return false, nil
		}
		return false, err
	}
	if err := manager.DeleteFileChunks(ctx, sourceRepo, fileDoc.Chunks); err != nil {
		slog.WarnContext(ctx, "delete object: delete chunks", slog.String("error", err.Error()))
	}
	return true, nil
}

func objectETag(file repository.File) string {
	h := sha256.New()
	_, _ = h.Write([]byte(file.Name))
	_, _ = h.Write([]byte(file.CreatedAt.UTC().Format(time.RFC3339Nano)))
	for _, chunk := range file.Chunks {
		_, _ = h.Write([]byte(chunk.SourceID.Hex()))
		_, _ = h.Write([]byte(chunk.Name))
		_, _ = h.Write([]byte(strconv.Itoa(chunk.Order)))
		_, _ = h.Write([]byte(":"))
		_, _ = h.Write([]byte(strconv.FormatInt(chunk.Size, 10)))
	}
	return "\"" + hex.EncodeToString(h.Sum(nil)) + "\""
}

func parseListMaxKeys(c *gin.Context) (int, bool) {
	maxKeys := 1000
	raw := c.Query("max-keys")
	if raw == "" {
		return maxKeys, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 0 {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "max-keys must be a non-negative integer")
		return 0, false
	}
	if parsed > maxKeys {
		parsed = maxKeys
	}
	return parsed, true
}

func fileListBucketEntry(file repository.File) listBucketEntry {
	var size int64
	for _, chunk := range file.Chunks {
		size += chunk.Size
	}
	return listBucketEntry{
		Key:          file.Name,
		LastModified: file.CreatedAt.UTC().Format(time.RFC3339),
		ETag:         objectETag(file),
		Size:         size,
		StorageClass: "STANDARD",
	}
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

func buildListBucketPage(ctx context.Context, fileRepo *repository.FileRepository, bucketID primitive.ObjectID, prefix, delimiter, after string, maxKeys int) ([]listBucketEntry, []listCommonPrefix, bool, string, error) {
	batchLimit := maxKeys + 1
	if batchLimit < 1 {
		batchLimit = 1
	}
	if batchLimit > 1001 {
		batchLimit = 1001
	}

	contents := make([]listBucketEntry, 0, maxKeys)
	commonPrefixes := make([]listCommonPrefix, 0, maxKeys)
	seenPrefixes := make(map[string]struct{})
	cursorAfter := after
	lastToken := ""

	for {
		files, hasMore, err := fileRepo.ListByBucketWithPrefixPage(ctx, bucketID, prefix, cursorAfter, batchLimit)
		if err != nil {
			return nil, nil, false, "", err
		}
		if len(files) == 0 {
			return contents, commonPrefixes, false, "", nil
		}

		for _, file := range files {
			if file.Name > cursorAfter {
				cursorAfter = file.Name
			}

			item := listBucketPageItem{
				sortKey:  file.Name,
				entry:    fileListBucketEntry(file),
				hasEntry: true,
			}
			if delimiter != "" {
				remainder := strings.TrimPrefix(file.Name, prefix)
				if idx := strings.Index(remainder, delimiter); idx >= 0 {
					commonPrefix := prefix + remainder[:idx+len(delimiter)]
					skipAfter := commonPrefix + listCommonPrefixSkipSuffix
					if skipAfter > cursorAfter {
						cursorAfter = skipAfter
					}
					if commonPrefix <= after {
						continue
					}
					if _, ok := seenPrefixes[commonPrefix]; ok {
						continue
					}
					seenPrefixes[commonPrefix] = struct{}{}
					item = listBucketPageItem{sortKey: commonPrefix, commonPrefix: commonPrefix}
				}
			}

			if maxKeys == 0 {
				return contents, commonPrefixes, true, item.sortKey, nil
			}
			if len(contents)+len(commonPrefixes) == maxKeys {
				return contents, commonPrefixes, true, lastToken, nil
			}

			if item.hasEntry {
				contents = append(contents, item.entry)
			} else {
				commonPrefixes = append(commonPrefixes, listCommonPrefix{Prefix: item.commonPrefix})
			}
			lastToken = item.sortKey
		}

		if !hasMore {
			return contents, commonPrefixes, false, "", nil
		}
	}
}

// ListObjects godoc
// @Summary List objects
// @Tags s3
// @Produce xml
// @Param bucket path string true "Bucket key"
// @Success 200 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/s3/{bucket} [get]
func ListObjects(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketKey := c.Param("bucket")
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		maxKeys, ok := parseListMaxKeys(c)
		if !ok {
			return
		}
		prefix := c.Query("prefix")
		delimiter := c.Query("delimiter")
		marker := c.Query("marker")
		contents, commonPrefixes, isTruncated, nextMarker, err := buildListBucketPage(ctx, fileRepo, bucketDoc.ID, prefix, delimiter, marker, maxKeys)
		if err != nil {
			slog.ErrorContext(ctx, "list objects: list files", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		result := listBucketResult{
			Xmlns:          "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:           bucketKey,
			Prefix:         prefix,
			Marker:         marker,
			NextMarker:     nextMarker,
			MaxKeys:        maxKeys,
			Delimiter:      delimiter,
			IsTruncated:    isTruncated,
			Contents:       contents,
			CommonPrefixes: commonPrefixes,
			KeyCount:       len(contents) + len(commonPrefixes),
		}
		c.XML(http.StatusOK, result)
	}
}

func ListObjectsV2(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketKey := c.Param("bucket")
		bucketDoc, ok := lookupBucket(c, bucketRepo)
		if !ok {
			return
		}
		maxKeys, ok := parseListMaxKeys(c)
		if !ok {
			return
		}
		prefix := c.Query("prefix")
		delimiter := c.Query("delimiter")
		continuationToken := c.Query("continuation-token")
		startAfter := c.Query("start-after")
		after := startAfter
		if continuationToken != "" {
			after = continuationToken
		}
		contents, commonPrefixes, isTruncated, nextToken, err := buildListBucketPage(ctx, fileRepo, bucketDoc.ID, prefix, delimiter, after, maxKeys)
		if err != nil {
			slog.ErrorContext(ctx, "list objects v2: list files", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		result := listBucketResult{
			Xmlns:                 "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:                  bucketKey,
			Prefix:                prefix,
			MaxKeys:               maxKeys,
			Delimiter:             delimiter,
			IsTruncated:           isTruncated,
			Contents:              contents,
			CommonPrefixes:        commonPrefixes,
			KeyCount:              len(contents) + len(commonPrefixes),
			ContinuationToken:     continuationToken,
			NextContinuationToken: nextToken,
			StartAfter:            startAfter,
		}
		c.XML(http.StatusOK, result)
	}
}

func lookupObject(c *gin.Context, bucketRepo objectBucketReader, fileRepo objectFileReader) (*repository.File, int64, bool) {
	ctx := c.Request.Context()
	bucketKey := c.Param("bucket")
	name := strings.TrimPrefix(c.Param("object"), "/")
	bucketDoc, err := bucketRepo.GetByKey(ctx, bucketKey)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return nil, 0, false
		}
		slog.ErrorContext(ctx, "lookup object: get bucket", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return nil, 0, false
	}
	accessKey := c.GetString("accessKey")
	if accessKey == "" || bucketDoc.AccessKey != accessKey {
		writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
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
	c.Header("ETag", objectETag(*fileDoc))
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
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketKey := c.Param("bucket")
		name := strings.TrimPrefix(c.Param("object"), "/")
		if name == "" {
			writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "")
			return
		}
		bucketDoc, err := bucketRepo.GetByKey(ctx, bucketKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
				return
			}
			slog.ErrorContext(ctx, "put object: get bucket", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		accessKey := c.GetString("accessKey")
		if accessKey == "" || bucketDoc.AccessKey != accessKey {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return
		}
		sources, err := sourceRepo.ListByIDs(ctx, bucketDoc.SourceIDs)
		if err != nil {
			slog.ErrorContext(ctx, "put object: list sources", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if len(sources) == 0 {
			writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "")
			return
		}
		var existingFile *repository.File
		existingFile, err = fileRepo.GetByName(ctx, bucketDoc.ID, name)
		if err != nil && err != mongo.ErrNoDocuments {
			slog.ErrorContext(ctx, "put object: get existing file", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if err == mongo.ErrNoDocuments {
			existingFile = nil
		}
		selector := manager.SelectorForBucket(bucketDoc, sources)
		chunks, err := manager.UploadFileChunksWithStrategy(ctx, c.Request.Body, sources, chunkSize, nil, selector)
		if err != nil {
			slog.ErrorContext(ctx, "put object: upload chunks", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}

		fileDoc := repository.File{BucketID: bucketDoc.ID, Name: name, CreatedAt: time.Now().UTC(), Chunks: chunks}
		if existingFile == nil {
			if _, err := fileRepo.Create(ctx, fileDoc); err != nil {
				_ = manager.DeleteFileChunks(ctx, sourceRepo, chunks)
				slog.ErrorContext(ctx, "put object: save file", slog.String("error", err.Error()))
				writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
				return
			}
			c.Status(http.StatusOK)
			return
		}
		fileDoc.ID = existingFile.ID
		if _, err := fileRepo.UpdateByID(ctx, fileDoc); err != nil {
			_ = manager.DeleteFileChunks(ctx, sourceRepo, chunks)
			slog.ErrorContext(ctx, "put object: update file", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if err := manager.DeleteFileChunks(ctx, sourceRepo, existingFile.Chunks); err != nil {
			slog.WarnContext(ctx, "put object: delete old chunks", slog.String("error", err.Error()))
		}
		c.Status(http.StatusOK)
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
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketKey := c.Param("bucket")
		bucketDoc, err := bucketRepo.GetByKey(ctx, bucketKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
				return
			}
			slog.ErrorContext(ctx, "delete object: get bucket", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		accessKey := c.GetString("accessKey")
		if accessKey == "" || bucketDoc.AccessKey != accessKey {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return
		}
		name := strings.TrimPrefix(c.Param("object"), "/")
		if name == "" {
			c.Status(http.StatusNoContent)
			return
		}
		if _, err := deleteObjectByName(ctx, sourceRepo, fileRepo, bucketDoc.ID, name); err != nil {
			slog.ErrorContext(ctx, "delete object: delete file", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		c.Status(http.StatusNoContent)
	}
}

func DeleteObjects(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
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
			if _, err := deleteObjectByName(ctx, sourceRepo, fileRepo, bucketDoc.ID, key); err != nil {
				slog.ErrorContext(ctx, "delete objects: delete file", slog.String("key", key), slog.String("error", err.Error()))
				result.Errors = append(result.Errors, deleteObjectError{
					Key:     key,
					Code:    "InternalError",
					Message: "Internal error deleting object",
				})
				continue
			}
			if !req.Quiet {
				result.Deleted = append(result.Deleted, deletedObjectResult{Key: key})
			}
		}
		c.XML(http.StatusOK, result)
	}
}
