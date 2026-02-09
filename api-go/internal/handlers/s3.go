package handlers

import (
	"bytes"
	"encoding/xml"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/s3aas/api-go/internal/gdrive"
	"github.com/example/s3aas/api-go/internal/manager"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type s3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

func writeS3Error(c *gin.Context, status int, code, message string) {
	c.XML(status, s3Error{Code: code, Message: message})
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
		if chunkSize <= 0 {
			chunkSize = 5 * 1024 * 1024
		}
		clients := make([]*gdrive.Client, len(sources))
		chunks := make([]repository.FileChunk, 0)
		buf := make([]byte, chunkSize)
		idx := 0
		for {
			n, err := c.Request.Body.Read(buf)
			if err != nil && err != io.EOF {
				log.Printf("put object: read chunk: %v", err)
				writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
				return
			}
			if n == 0 {
				break
			}
			src := sources[idx%len(sources)]
			if clients[idx%len(sources)] == nil {
				cli, err := gdrive.NewClient(ctx, []byte(src.Key))
				if err != nil {
					log.Printf("put object: create gdrive client: %v", err)
					writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
					return
				}
				clients[idx%len(sources)] = cli
			}
			driveName := primitive.NewObjectID().Hex()
			driveID, err := clients[idx%len(sources)].Upload(ctx, driveName, bytes.NewReader(buf[:n]))
			if err != nil {
				log.Printf("put object: upload chunk: %v", err)
				writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
				return
			}
			chunks = append(chunks, repository.FileChunk{SourceID: src.ID, Name: driveID, Order: idx, Size: int64(n)})
			idx++
			if err == io.EOF {
				break
			}
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
