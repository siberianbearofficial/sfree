package handlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type createBucketRequest struct {
	Key       string   `json:"key" binding:"required"`
	SourceIDs []string `json:"source_ids" binding:"required,min=1"`
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
func CreateBucket(repo *repository.BucketRepository, sourceRepo *repository.SourceRepository, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req createBucketRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.WarnContext(ctx, "create bucket: invalid request", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		if repo == nil || sourceRepo == nil {
			slog.ErrorContext(ctx, "create bucket: repository is nil")
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
			slog.ErrorContext(ctx, "create bucket: secret key is empty")
			c.Status(http.StatusInternalServerError)
			return
		}
		accessKey := req.Key
		accessSecret, err := cryptoutil.GenerateSecret()
		if err != nil {
			slog.ErrorContext(ctx, "create bucket: generate secret", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		encrypted, err := cryptoutil.Encrypt(accessSecret, secretKey)
		if err != nil {
			slog.ErrorContext(ctx, "create bucket: encrypt secret", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		sourceIDs := make([]primitive.ObjectID, 0, len(req.SourceIDs))
		for _, sourceIDHex := range req.SourceIDs {
			sourceID, err := primitive.ObjectIDFromHex(sourceIDHex)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
			sourceDoc, err := sourceRepo.GetByID(c.Request.Context(), sourceID)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					c.Status(http.StatusBadRequest)
					return
				}
				slog.ErrorContext(ctx, "create bucket: get source", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			if sourceDoc.UserID != userID {
				c.Status(http.StatusBadRequest)
				return
			}
			sourceIDs = append(sourceIDs, sourceID)
		}
		bucket := repository.Bucket{
			UserID:          userID,
			Key:             req.Key,
			AccessKey:       accessKey,
			AccessSecretEnc: encrypted,
			SourceIDs:       sourceIDs,
			CreatedAt:       time.Now().UTC(),
		}
		created, err := repo.Create(c.Request.Context(), bucket)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				slog.WarnContext(ctx, "create bucket: key already exists", slog.String("key", req.Key))
				c.Status(http.StatusConflict)
				return
			}
			slog.ErrorContext(ctx, "create bucket: failed to create bucket", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "list buckets: repository is nil")
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
			slog.ErrorContext(ctx, "list buckets: failed to list buckets", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "delete bucket: repository is nil")
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
			slog.ErrorContext(ctx, "delete bucket: failed to delete", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "upload file: repository is nil")
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
			slog.ErrorContext(ctx, "upload file: get bucket", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if bucketDoc.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
		sources, err := sourceRepo.ListByIDs(c.Request.Context(), bucketDoc.SourceIDs)
		if err != nil {
			slog.ErrorContext(ctx, "upload file: list sources", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if len(sources) == 0 {
			c.Status(http.StatusBadRequest)
			return
		}
		fh, err := c.FormFile("file")
		if err != nil {
			slog.WarnContext(ctx, "upload file: get file", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		f, err := fh.Open()
		if err != nil {
			slog.WarnContext(ctx, "upload file: open file", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		defer func() { _ = f.Close() }()

		chunks, err := manager.UploadFileChunks(ctx, f, sources, chunkSize)
		if err != nil {
			slog.ErrorContext(ctx, "upload file: upload chunks", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		fileDoc := repository.File{
			BucketID:  bucketID,
			Name:      fh.Filename,
			CreatedAt: time.Now().UTC(),
			Chunks:    chunks,
		}
		created, err := fileRepo.Create(ctx, fileDoc)
		if err != nil {
			_ = manager.DeleteFileChunks(ctx, sourceRepo, chunks)
			slog.ErrorContext(ctx, "upload file: save file", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "list files: repository is nil")
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
			slog.ErrorContext(ctx, "list files: get bucket", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if bucketDoc.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
		files, err := fileRepo.ListByBucket(c.Request.Context(), bucketID)
		if err != nil {
			slog.ErrorContext(ctx, "list files: list files", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "download file: repository is nil")
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
			slog.ErrorContext(ctx, "download file: get bucket", slog.String("error", err.Error()))
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
			slog.ErrorContext(ctx, "download file: get file", slog.String("error", err.Error()))
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
			slog.ErrorContext(ctx, "download file: stream failed", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "delete file: repository is nil")
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
			slog.ErrorContext(ctx, "delete file: get bucket", slog.String("error", err.Error()))
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
			slog.ErrorContext(ctx, "delete file: get file", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if fileDoc.BucketID != bucketID {
			c.Status(http.StatusNotFound)
			return
		}
		if err := manager.DeleteFileChunks(ctx, sourceRepo, fileDoc.Chunks); err != nil {
			slog.ErrorContext(ctx, "delete file: delete chunk", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if err := fileRepo.Delete(ctx, fileID); err != nil {
			slog.ErrorContext(ctx, "delete file: delete metadata", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}
