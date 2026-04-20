package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type s3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
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

func writeS3Error(c *gin.Context, status int, code, message string) {
	c.XML(status, s3Error{Code: code, Message: message})
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

func buildListBucketPage(files []repository.File, prefix, delimiter, after string, maxKeys int) ([]listBucketEntry, []listCommonPrefix, bool, string) {
	items := make([]listBucketPageItem, 0, len(files))
	seenPrefixes := make(map[string]struct{})
	for _, file := range files {
		if !strings.HasPrefix(file.Name, prefix) {
			continue
		}
		if delimiter != "" {
			remainder := strings.TrimPrefix(file.Name, prefix)
			if idx := strings.Index(remainder, delimiter); idx >= 0 {
				commonPrefix := prefix + remainder[:idx+len(delimiter)]
				if _, ok := seenPrefixes[commonPrefix]; !ok {
					seenPrefixes[commonPrefix] = struct{}{}
					items = append(items, listBucketPageItem{sortKey: commonPrefix, commonPrefix: commonPrefix})
				}
				continue
			}
		}
		items = append(items, listBucketPageItem{
			sortKey:  file.Name,
			entry:    fileListBucketEntry(file),
			hasEntry: true,
		})
	}

	start := 0
	if after != "" {
		for start < len(items) && items[start].sortKey <= after {
			start++
		}
	}
	remaining := len(items) - start
	if remaining < 0 {
		remaining = 0
	}
	pageSize := maxKeys
	if pageSize > remaining {
		pageSize = remaining
	}
	page := items[start : start+pageSize]
	isTruncated := start+pageSize < len(items)
	nextToken := ""
	if isTruncated && len(page) > 0 {
		nextToken = page[len(page)-1].sortKey
	}

	contents := make([]listBucketEntry, 0, len(page))
	commonPrefixes := make([]listCommonPrefix, 0, len(page))
	for _, item := range page {
		if item.hasEntry {
			contents = append(contents, item.entry)
			continue
		}
		commonPrefixes = append(commonPrefixes, listCommonPrefix{Prefix: item.commonPrefix})
	}
	return contents, commonPrefixes, isTruncated, nextToken
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
		files, err := fileRepo.ListByBucketWithPrefix(ctx, bucketDoc.ID, prefix)
		if err != nil {
			slog.ErrorContext(ctx, "list objects: list files", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		contents, commonPrefixes, isTruncated, nextMarker := buildListBucketPage(files, prefix, delimiter, marker, maxKeys)
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
		files, err := fileRepo.ListByBucketWithPrefix(ctx, bucketDoc.ID, prefix)
		if err != nil {
			slog.ErrorContext(ctx, "list objects v2: list files", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		contents, commonPrefixes, isTruncated, nextToken := buildListBucketPage(files, prefix, delimiter, after, maxKeys)
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

// lookupObject resolves the bucket and file from the request context.
// Returns the file document and total size, or writes an S3 error and returns false.
func lookupObject(c *gin.Context, bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository) (*repository.File, int64, bool) {
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

// setObjectHeaders writes standard S3 object headers (ETag, Last-Modified,
// Content-Length, Content-Type).
func setObjectHeaders(c *gin.Context, fileDoc *repository.File, total int64) {
	c.Header("ETag", objectETag(*fileDoc))
	c.Header("Last-Modified", fileDoc.CreatedAt.UTC().Format(http.TimeFormat))
	c.Header("Content-Length", strconv.FormatInt(total, 10))
	c.Header("Content-Type", "application/octet-stream")
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
	return func(c *gin.Context) {
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		fileDoc, total, ok := lookupObject(c, bucketRepo, fileRepo)
		if !ok {
			return
		}
		setObjectHeaders(c, fileDoc, total)
		c.Status(http.StatusOK)
		if err := manager.StreamFile(c.Request.Context(), sourceRepo, fileDoc, c.Writer); err != nil {
			slog.ErrorContext(c.Request.Context(), "get object: stream failed", slog.String("error", err.Error()))
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
		fileDoc, err := fileRepo.GetByName(ctx, bucketDoc.ID, name)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNoContent)
				return
			}
			slog.ErrorContext(ctx, "delete object: get file", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if err := fileRepo.Delete(ctx, fileDoc.ID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNoContent)
				return
			}
			slog.ErrorContext(ctx, "delete object: delete file", slog.String("error", err.Error()))
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if err := manager.DeleteFileChunks(ctx, sourceRepo, fileDoc.Chunks); err != nil {
			slog.WarnContext(ctx, "delete object: delete chunks", slog.String("error", err.Error()))
		}
		c.Status(http.StatusNoContent)
	}
}
