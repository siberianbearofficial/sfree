package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/s3compat"
	"github.com/example/sfree/api-go/internal/telegram"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type createGDriveSourceRequest struct {
	Name string `json:"name" binding:"required"`
	Key  string `json:"key" binding:"required"`
}

type createTelegramSourceRequest struct {
	Name   string `json:"name" binding:"required"`
	Token  string `json:"token" binding:"required"`
	ChatID string `json:"chat_id" binding:"required"`
}

type createS3SourceRequest struct {
	Name            string `json:"name" binding:"required"`
	Endpoint        string `json:"endpoint" binding:"required"`
	Bucket          string `json:"bucket" binding:"required"`
	AccessKeyID     string `json:"access_key_id" binding:"required"`
	SecretAccessKey string `json:"secret_access_key" binding:"required"`
	Region          string `json:"region"`
	PathStyle       bool   `json:"path_style"`
}

type sourceResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

type sourceInfoFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

type sourceInfoResponse struct {
	ID           string           `json:"id"`
	Name         string           `json:"name"`
	Type         string           `json:"type"`
	Files        []sourceInfoFile `json:"files"`
	StorageTotal int64            `json:"storage_total"`
	StorageUsed  int64            `json:"storage_used"`
	StorageFree  int64            `json:"storage_free"`
}

// CreateGDriveSource godoc
// @Summary Create gdrive source
// @Tags sources
// @Accept json
// @Produce json
// @Param source body createGDriveSourceRequest true "Source to create"
// @Success 200 {object} sourceResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/gdrive [post]
func CreateGDriveSource(repo *repository.SourceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createGDriveSourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.WarnContext(c.Request.Context(), "create gdrive source: invalid request", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		saveSource(c, repo, repository.SourceTypeGDrive, req.Name, req.Key)
	}
}

// CreateTelegramSource godoc
// @Summary Create telegram source
// @Tags sources
// @Accept json
// @Produce json
// @Param source body createTelegramSourceRequest true "Source to create"
// @Success 200 {object} sourceResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/telegram [post]
func CreateTelegramSource(repo *repository.SourceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createTelegramSourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.WarnContext(c.Request.Context(), "create telegram source: invalid request", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		key, err := telegram.EncodeConfig(telegram.Config{Token: req.Token, ChatID: req.ChatID})
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "create telegram source: encode key", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		saveSource(c, repo, repository.SourceTypeTelegram, req.Name, key)
	}
}

// CreateS3Source godoc
// @Summary Create s3 source
// @Tags sources
// @Accept json
// @Produce json
// @Param source body createS3SourceRequest true "Source to create"
// @Success 200 {object} sourceResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/s3 [post]
func CreateS3Source(repo *repository.SourceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createS3SourceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.WarnContext(c.Request.Context(), "create s3 source: invalid request", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		key, err := s3compat.EncodeConfig(s3compat.Config{
			Endpoint:     req.Endpoint,
			Region:       req.Region,
			Bucket:       req.Bucket,
			AccessKeyID:  req.AccessKeyID,
			SecretAccess: req.SecretAccessKey,
			PathStyle:    req.PathStyle,
		})
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "create s3 source: encode key", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		saveSource(c, repo, repository.SourceTypeS3, req.Name, key)
	}
}

func saveSource(c *gin.Context, repo *repository.SourceRepository, sourceType repository.SourceType, name, key string) {
	ctx := c.Request.Context()
	if repo == nil {
		c.Status(http.StatusServiceUnavailable)
		return
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return
	}
	source := repository.Source{
		UserID:    userID,
		Type:      sourceType,
		Name:      name,
		Key:       key,
		CreatedAt: time.Now().UTC(),
	}
	created, err := repo.Create(c.Request.Context(), source)
	if err != nil {
		slog.ErrorContext(ctx, "create source: failed to create", slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, sourceResponse{
		ID:        created.ID.Hex(),
		Name:      created.Name,
		Type:      string(created.Type),
		CreatedAt: created.CreatedAt,
	})
}

// ListSources godoc
// @Summary List sources
// @Tags sources
// @Produce json
// @Success 200 {array} sourceResponse
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources [get]
func ListSources(repo *repository.SourceRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "list sources: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}
		sources, err := repo.ListByUser(c.Request.Context(), userID)
		if err != nil {
			slog.ErrorContext(ctx, "list sources: failed to list", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		resp := make([]sourceResponse, 0, len(sources))
		for _, s := range sources {
			resp = append(resp, sourceResponse{
				ID:        s.ID.Hex(),
				Name:      s.Name,
				Type:      string(s.Type),
				CreatedAt: s.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, resp)
	}
}

// DeleteSource godoc
// @Summary Delete source
// @Tags sources
// @Param id path string true "Source ID"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 409 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/{id} [delete]
func DeleteSource(repo *repository.SourceRepository, bucketRepo *repository.BucketRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if repo == nil || bucketRepo == nil {
			slog.ErrorContext(ctx, "delete source: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}
		id, ok := routeObjectID(c, "id")
		if !ok {
			return
		}
		inUse, err := bucketRepo.HasSourceReference(c.Request.Context(), userID, id)
		if err != nil {
			slog.ErrorContext(ctx, "delete source: check buckets", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if inUse {
			c.Status(http.StatusConflict)
			return
		}
		if err := repo.Delete(c.Request.Context(), id, userID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "delete source: failed to delete", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

// GetSourceInfo godoc
// @Summary Get source info
// @Tags sources
// @Param id path string true "Source ID"
// @Success 200 {object} sourceInfoResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/{id}/info [get]
func GetSourceInfo(repo *repository.SourceRepository) gin.HandlerFunc {
	return getSourceInfo(repo, nil)
}

func getSourceInfo(repo *repository.SourceRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "get source info: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}
		id, ok := routeObjectID(c, "id")
		if !ok {
			return
		}
		src, err := repo.GetByID(c.Request.Context(), id)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "get source info: get source", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if src.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}
		infoClient, err := manager.NewSourceInfoClient(c.Request.Context(), src)
		if err != nil {
			if errors.Is(err, manager.ErrUnsupportedSourceType) {
				c.Status(http.StatusBadRequest)
				return
			}
			slog.ErrorContext(ctx, "get source info: create client", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		info, err := infoClient.Info(c.Request.Context())
		if err != nil {
			slog.ErrorContext(ctx, "get source info: load info", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		respFiles := make([]sourceInfoFile, 0, len(info.Files))
		for _, f := range info.Files {
			respFiles = append(respFiles, sourceInfoFile{ID: f.ID, Name: f.Name, Size: f.Size})
		}
		c.JSON(http.StatusOK, sourceInfoResponse{
			ID:           src.ID.Hex(),
			Name:         src.Name,
			Type:         string(src.Type),
			Files:        respFiles,
			StorageTotal: info.StorageTotal,
			StorageUsed:  info.StorageUsed,
			StorageFree:  info.StorageFree,
		})
	}
}

// DownloadSourceFile godoc
// @Summary Download a file from a source
// @Tags sources
// @Produce octet-stream
// @Param id path string true "Source ID"
// @Param file_id path string true "File ID (GDrive file ID or S3 object key)"
// @Success 200 {file} file
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/{id}/files/{file_id}/download [get]
func DownloadSourceFile(sourceRepo *repository.SourceRepository) gin.HandlerFunc {
	return downloadSourceFile(sourceRepo, nil)
}

func downloadSourceFile(sourceRepo *repository.SourceRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		if sourceRepo == nil {
			slog.ErrorContext(c.Request.Context(), "download source file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}
		id, ok := routeObjectID(c, "id")
		if !ok {
			return
		}
		fileID := c.Param("file_id")
		if fileID == "" {
			c.Status(http.StatusBadRequest)
			return
		}
		src, err := sourceRepo.GetByID(c.Request.Context(), id)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(c.Request.Context(), "download source file: get source", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if src.UserID != userID {
			c.Status(http.StatusNotFound)
			return
		}

		filename := fileID

		cli, err := manager.NewDirectSourceClient(c.Request.Context(), src)
		if err != nil {
			if errors.Is(err, manager.ErrSourceDownloadUnsupported) || errors.Is(err, manager.ErrUnsupportedSourceType) {
				c.JSON(http.StatusBadRequest, gin.H{"error": "download not supported for this source type"})
				return
			}
			slog.ErrorContext(c.Request.Context(), "download source file: create client", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		body, err := cli.Download(c.Request.Context(), fileID)
		if err != nil {
			slog.ErrorContext(c.Request.Context(), "download source file: download", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if body == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "download not supported for this source type"})
			return
		}
		defer func() { _ = body.Close() }()

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFilename(filename)))
		c.Header("Content-Type", "application/octet-stream")
		c.Status(http.StatusOK)
		if _, err := io.Copy(c.Writer, body); err != nil {
			slog.ErrorContext(c.Request.Context(), "download source file: stream", slog.String("error", err.Error()))
		}
	}
}
