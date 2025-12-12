package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/example/s3aas/api-go/internal/cryptoutil"
	"github.com/example/s3aas/api-go/internal/gdrive"
	"github.com/example/s3aas/api-go/internal/manager"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type createBucketRequest struct {
	Key string `json:"key" binding:"required"`
}

type createBucketResponse struct {
	Key          string    `json:"key"`
	AccessKey    string    `json:"access_key"`
	AccessSecret string    `json:"access_secret"`
	CreatedAt    time.Time `json:"created_at"`
}

type bucketResponse struct {
	ID        string    `json:"id"`
	Key       string    `json:"key"`
	AccessKey string    `json:"access_key"`
	CreatedAt time.Time `json:"created_at"`
}

type fileResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

// CreateBucket godoc
// @Summary Create bucket
// @Tags buckets
// @Accept json
// @Produce json
// @Param bucket body createBucketRequest true "Bucket to create"
// @Success 200 {object} createBucketResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 409 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets [post]
func CreateBucket(repo *repository.BucketRepository, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createBucketRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("create bucket: invalid request: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		if repo == nil {
			log.Print("create bucket: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		if secretKey == "" {
			log.Print("create bucket: secret key is empty")
			c.Status(http.StatusInternalServerError)
			return
		}
		accessKey := req.Key
		accessSecret, err := cryptoutil.GenerateSecret()
		if err != nil {
			log.Printf("create bucket: generate secret: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		encrypted, err := cryptoutil.Encrypt(accessSecret, secretKey)
		if err != nil {
			log.Printf("create bucket: encrypt secret: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		bucket := repository.Bucket{
			UserID:          userID,
			Key:             req.Key,
			AccessKey:       accessKey,
			AccessSecretEnc: encrypted,
			CreatedAt:       time.Now().UTC(),
		}
		created, err := repo.Create(c.Request.Context(), bucket)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				log.Printf("create bucket: key %s already exists: %v", req.Key, err)
				c.Status(http.StatusConflict)
				return
			}
			log.Printf("create bucket: failed to create bucket: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, createBucketResponse{
			Key:          created.Key,
			AccessKey:    created.AccessKey,
			AccessSecret: accessSecret,
			CreatedAt:    created.CreatedAt,
		})
	}
}

// ListBuckets godoc
// @Summary List buckets
// @Tags buckets
// @Produce json
// @Success 200 {array} bucketResponse
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets [get]
func ListBuckets(repo *repository.BucketRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			log.Print("list buckets: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		buckets, err := repo.ListByUser(c.Request.Context(), userID)
		if err != nil {
			log.Printf("list buckets: failed to list buckets: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		resp := make([]bucketResponse, 0, len(buckets))
		for _, b := range buckets {
			resp = append(resp, bucketResponse{
				ID:        b.ID.Hex(),
				Key:       b.Key,
				AccessKey: b.AccessKey,
				CreatedAt: b.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, resp)
	}
}

// DeleteBucket godoc
// @Summary Delete bucket
// @Tags buckets
// @Param id path string true "Bucket ID"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id} [delete]
func DeleteBucket(repo *repository.BucketRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			log.Print("delete bucket: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		idHex := c.Param("id")
		id, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := repo.Delete(c.Request.Context(), id, userID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("delete bucket: failed to delete: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

type uploadFileResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// UploadFile godoc
// @Summary Upload file to bucket
// @Tags buckets
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Bucket ID"
// @Param file formData file true "File to upload"
// @Success 200 {object} uploadFileResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/upload [post]
func UploadFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, chunkSize int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			log.Print("upload file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		idHex := c.Param("id")
		bucketID, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		bucketDoc, err := bucketRepo.GetByID(c.Request.Context(), bucketID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("upload file: get bucket: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if bucketDoc.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
		sources, err := sourceRepo.ListByUser(c.Request.Context(), userID)
		if err != nil {
			log.Printf("upload file: list sources: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if len(sources) == 0 {
			c.Status(http.StatusBadRequest)
			return
		}
		fh, err := c.FormFile("file")
		if err != nil {
			log.Printf("upload file: get file: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		f, err := fh.Open()
		if err != nil {
			log.Printf("upload file: open file: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		defer func() { _ = f.Close() }()

		ctx := c.Request.Context()
		clients := make([]*gdrive.Client, len(sources))
		chunks := make([]repository.FileChunk, 0)
		buf := make([]byte, chunkSize)
		idx := 0
		for {
			n, err := f.Read(buf)
			if err != nil && err != io.EOF {
				log.Printf("upload file: read chunk: %v", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			if n == 0 {
				break
			}
			src := sources[idx%len(sources)]
			if clients[idx%len(sources)] == nil {
				cli, err := gdrive.NewClient(ctx, []byte(src.Key))
				if err != nil {
					log.Printf("upload file: create gdrive client: %v", err)
					c.Status(http.StatusInternalServerError)
					return
				}
				clients[idx%len(sources)] = cli
			}
			name := primitive.NewObjectID().Hex()
			driveID, err := clients[idx%len(sources)].Upload(ctx, name, bytes.NewReader(buf[:n]))
			if err != nil {
				log.Printf("upload file: upload chunk: %v", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			chunks = append(chunks, repository.FileChunk{
				SourceID: src.ID,
				Name:     driveID,
				Order:    idx,
				Size:     int64(n),
			})
			idx++
			if err == io.EOF {
				break
			}
		}

		fileDoc := repository.File{
			BucketID:  bucketID,
			Name:      fh.Filename,
			CreatedAt: time.Now().UTC(),
			Chunks:    chunks,
		}
		created, err := fileRepo.Create(ctx, fileDoc)
		if err != nil {
			log.Printf("upload file: save file: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, uploadFileResponse{
			ID:        created.ID.Hex(),
			Name:      created.Name,
			CreatedAt: created.CreatedAt,
		})
	}
}

// ListFiles godoc
// @Summary List files in bucket
// @Tags buckets
// @Produce json
// @Param id path string true "Bucket ID"
// @Success 200 {array} fileResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files [get]
func ListFiles(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || fileRepo == nil {
			log.Print("list files: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		idHex := c.Param("id")
		bucketID, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		bucketDoc, err := bucketRepo.GetByID(c.Request.Context(), bucketID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("list files: get bucket: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if bucketDoc.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
		files, err := fileRepo.ListByBucket(c.Request.Context(), bucketID)
		if err != nil {
			log.Printf("list files: list files: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		resp := make([]fileResponse, 0, len(files))
		for _, f := range files {
			var size int64
			for _, ch := range f.Chunks {
				size += ch.Size
			}
			resp = append(resp, fileResponse{
				ID:        f.ID.Hex(),
				Name:      f.Name,
				CreatedAt: f.CreatedAt,
				Size:      size,
			})
		}
		c.JSON(http.StatusOK, resp)
	}
}

// DownloadFile godoc
// @Summary Download file
// @Tags buckets
// @Produce octet-stream
// @Param id path string true "Bucket ID"
// @Param file_id path string true "File ID"
// @Success 200 {file} file
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files/{file_id}/download [get]
func DownloadFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			log.Print("download file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		idHex := c.Param("id")
		fileHex := c.Param("file_id")
		bucketID, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		fileID, err := primitive.ObjectIDFromHex(fileHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		bucketDoc, err := bucketRepo.GetByID(c.Request.Context(), bucketID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("download file: get bucket: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if bucketDoc.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
		fileDoc, err := fileRepo.GetByID(c.Request.Context(), fileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("download file: get file: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if fileDoc.BucketID != bucketID {
			c.Status(http.StatusNotFound)
			return
		}
		var total int64
		for _, ch := range fileDoc.Chunks {
			total += ch.Size
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileDoc.Name))
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", strconv.FormatInt(total, 10))
		c.Status(http.StatusOK)
		if err := manager.StreamFile(c.Request.Context(), sourceRepo, fileDoc, c.Writer); err != nil {
			log.Printf("download file: %v", err)
		}
	}
}

// DeleteFile godoc
// @Summary Delete file
// @Tags buckets
// @Param id path string true "Bucket ID"
// @Param file_id path string true "File ID"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files/{file_id} [delete]
func DeleteFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			log.Print("delete file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		idHex := c.Param("id")
		fileHex := c.Param("file_id")
		bucketID, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		fileID, err := primitive.ObjectIDFromHex(fileHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		bucketDoc, err := bucketRepo.GetByID(c.Request.Context(), bucketID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("delete file: get bucket: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if bucketDoc.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
		fileDoc, err := fileRepo.GetByID(c.Request.Context(), fileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("delete file: get file: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if fileDoc.BucketID != bucketID {
			c.Status(http.StatusNotFound)
			return
		}
		ctx := c.Request.Context()
		clients := make(map[primitive.ObjectID]*gdrive.Client)
		for _, ch := range fileDoc.Chunks {
			cli, ok := clients[ch.SourceID]
			if !ok {
				src, err := sourceRepo.GetByID(ctx, ch.SourceID)
				if err != nil {
					log.Printf("delete file: get source: %v", err)
					c.Status(http.StatusInternalServerError)
					return
				}
				cli, err = gdrive.NewClient(ctx, []byte(src.Key))
				if err != nil {
					log.Printf("delete file: create client: %v", err)
					c.Status(http.StatusInternalServerError)
					return
				}
				clients[ch.SourceID] = cli
			}
			if err := cli.Delete(ctx, ch.Name); err != nil {
				log.Printf("delete file: delete chunk: %v", err)
				c.Status(http.StatusInternalServerError)
				return
			}
		}
		if err := fileRepo.Delete(ctx, fileID); err != nil {
			log.Printf("delete file: delete metadata: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}
