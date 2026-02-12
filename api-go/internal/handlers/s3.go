package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/s3aas/api-go/internal/manager"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type s3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

type listBucketResult struct {
	XMLName      xml.Name          `xml:"ListBucketResult"`
	Xmlns        string            `xml:"xmlns,attr"`
	Name         string            `xml:"Name"`
	Prefix       string            `xml:"Prefix"`
	MaxKeys      int               `xml:"MaxKeys"`
	IsTruncated  bool              `xml:"IsTruncated"`
	Contents     []listBucketEntry `xml:"Contents"`
	KeyCount     int               `xml:"KeyCount"`
	Continuation string            `xml:"ContinuationToken,omitempty"`
}

type listBucketEntry struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
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
		if bucketRepo == nil || fileRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		bucketKey := c.Param("bucket")
		ctx := c.Request.Context()
		bucketDoc, err := bucketRepo.GetByKey(ctx, bucketKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
				return
			}
			log.Printf("list objects: get bucket: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		accessKey := c.GetString("accessKey")
		if accessKey == "" || bucketDoc.AccessKey != accessKey {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return
		}
		files, err := fileRepo.ListByBucket(ctx, bucketDoc.ID)
		if err != nil {
			log.Printf("list objects: list files: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		entries := make([]listBucketEntry, 0, len(files))
		for _, file := range files {
			var size int64
			for _, chunk := range file.Chunks {
				size += chunk.Size
			}
			entries = append(entries, listBucketEntry{
				Key:          file.Name,
				LastModified: file.CreatedAt.UTC().Format(time.RFC3339),
				ETag:         objectETag(file),
				Size:         size,
				StorageClass: "STANDARD",
			})
		}
		result := listBucketResult{
			Xmlns:       "http://s3.amazonaws.com/doc/2006-03-01/",
			Name:        bucketKey,
			Prefix:      "",
			MaxKeys:     1000,
			IsTruncated: false,
			Contents:    entries,
			KeyCount:    len(entries),
		}
		c.XML(http.StatusOK, result)
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
		bucketKey := c.Param("bucket")
		name := strings.TrimPrefix(c.Param("object"), "/")
		ctx := c.Request.Context()
		bucketDoc, err := bucketRepo.GetByKey(ctx, bucketKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
				return
			}
			log.Printf("get object: get bucket: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		accessKey := c.GetString("accessKey")
		if accessKey == "" || bucketDoc.AccessKey != accessKey {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return
		}
		fileDoc, err := fileRepo.GetByName(ctx, bucketDoc.ID, name)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeS3Error(c, http.StatusNotFound, "NoSuchKey", "")
				return
			}
			log.Printf("get object: get file: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		var total int64
		for _, ch := range fileDoc.Chunks {
			total += ch.Size
		}
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", strconv.FormatInt(total, 10))
		c.Status(http.StatusOK)
		if err := manager.StreamFile(c.Request.Context(), sourceRepo, fileDoc, c.Writer); err != nil {
			log.Printf("get object: %v", err)
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
		ctx := c.Request.Context()
		bucketDoc, err := bucketRepo.GetByKey(ctx, bucketKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
				return
			}
			log.Printf("put object: get bucket: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		accessKey := c.GetString("accessKey")
		if accessKey == "" || bucketDoc.AccessKey != accessKey {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return
		}
		sources, err := sourceRepo.ListByUser(ctx, bucketDoc.UserID)
		if err != nil {
			log.Printf("put object: list sources: %v", err)
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
			log.Printf("put object: get existing file: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if err == mongo.ErrNoDocuments {
			existingFile = nil
		}
		chunks, err := manager.UploadFileChunks(ctx, c.Request.Body, sources, chunkSize)
		if err != nil {
			log.Printf("put object: upload chunks: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}

		fileDoc := repository.File{BucketID: bucketDoc.ID, Name: name, CreatedAt: time.Now().UTC(), Chunks: chunks}
		if existingFile == nil {
			if _, err := fileRepo.Create(ctx, fileDoc); err != nil {
				_ = manager.DeleteFileChunks(ctx, sourceRepo, chunks)
				log.Printf("put object: save file: %v", err)
				writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
				return
			}
			c.Status(http.StatusOK)
			return
		}
		fileDoc.ID = existingFile.ID
		if _, err := fileRepo.UpdateByID(ctx, fileDoc); err != nil {
			_ = manager.DeleteFileChunks(ctx, sourceRepo, chunks)
			log.Printf("put object: update file: %v", err)
			writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
			return
		}
		if err := manager.DeleteFileChunks(ctx, sourceRepo, existingFile.Chunks); err != nil {
			log.Printf("put object: delete old chunks: %v", err)
		}
		c.Status(http.StatusOK)
	}
}
