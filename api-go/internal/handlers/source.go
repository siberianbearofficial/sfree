package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/s3compat"
	"github.com/example/sfree/api-go/internal/telegram"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

type sourceHealthResponse struct {
	ID              string    `json:"id"`
	Type            string    `json:"type"`
	Status          string    `json:"status"`
	CheckedAt       time.Time `json:"checked_at"`
	LatencyMS       int64     `json:"latency_ms"`
	ReasonCode      string    `json:"reason_code"`
	Message         string    `json:"message"`
	QuotaTotalBytes *int64    `json:"quota_total_bytes" extensions:"x-nullable"`
	QuotaUsedBytes  *int64    `json:"quota_used_bytes" extensions:"x-nullable"`
	QuotaFreeBytes  *int64    `json:"quota_free_bytes" extensions:"x-nullable"`
}

type sourceGetter interface {
	GetByID(ctx context.Context, id primitive.ObjectID) (*repository.Source, error)
}

func ownedSourceFromRoute(c *gin.Context, repo sourceGetter, operation string) (*repository.Source, bool) {
	ctx := c.Request.Context()
	if nilSourceGetter(repo) {
		slog.ErrorContext(ctx, operation+": repository is nil")
		c.Status(http.StatusServiceUnavailable)
		return nil, false
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return nil, false
	}
	id, ok := routeObjectID(c, "id")
	if !ok {
		return nil, false
	}
	src, err := repo.GetByID(ctx, id)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.Status(http.StatusNotFound)
			return nil, false
		}
		slog.ErrorContext(ctx, operation+": get source", slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError)
		return nil, false
	}
	if src.UserID != userID {
		c.Status(http.StatusNotFound)
		return nil, false
	}
	return src, true
}

func nilSourceGetter(repo sourceGetter) bool {
	if repo == nil {
		return true
	}
	value := reflect.ValueOf(repo)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
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
		if strings.TrimSpace(req.Key) == "" || !json.Valid([]byte(req.Key)) {
			slog.WarnContext(c.Request.Context(), "create gdrive source: invalid credentials")
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
		cfg, err := telegram.ValidateConfig(telegram.Config{Token: req.Token, ChatID: req.ChatID})
		if err != nil {
			slog.WarnContext(c.Request.Context(), "create telegram source: invalid config", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		key, err := telegram.EncodeConfig(cfg)
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
		cfg, err := s3compat.ValidateConfig(s3compat.Config{
			Endpoint:     req.Endpoint,
			Region:       req.Region,
			Bucket:       req.Bucket,
			AccessKeyID:  req.AccessKeyID,
			SecretAccess: req.SecretAccessKey,
			PathStyle:    req.PathStyle,
		})
		if err != nil {
			slog.WarnContext(c.Request.Context(), "create s3 source: invalid config", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		key, err := s3compat.EncodeConfig(cfg)
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
		sources, err := repo.ListMetadataByUser(c.Request.Context(), userID)
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
// @Description Returns provider file inventory plus provider-reported storage usage fields when cheaply available. These storage totals are not a universal quota contract; use the health endpoint for honest quota/capacity signals.
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

func GetSourceInfoWithFactory(repo *repository.SourceRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return getSourceInfo(repo, factory)
}

func getSourceInfo(repo *repository.SourceRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		src, ok := ownedSourceFromRoute(c, repo, "get source info")
		if !ok {
			return
		}
		info, err := manager.InspectSource(c.Request.Context(), src, factory)
		if err != nil {
			if err == manager.ErrUnsupportedSourceType {
				c.Status(http.StatusBadRequest)
				return
			}
			slog.ErrorContext(ctx, "get source info: inspect source", slog.String("error", err.Error()))
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

// GetSourceHealth godoc
// @Summary Check source health
// @Description Runs a lightweight on-demand provider probe. Google Drive returns native quota when available; S3-compatible and Telegram quota fields stay null because SFree does not invent provider-wide capacity where no cheap native signal exists.
// @Tags sources
// @Param id path string true "Source ID"
// @Success 200 {object} sourceHealthResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/{id}/health [get]
func GetSourceHealth(repo *repository.SourceRepository) gin.HandlerFunc {
	return getSourceHealth(repo, nil)
}

func GetSourceHealthWithFactory(repo *repository.SourceRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return getSourceHealth(repo, factory)
}

func getSourceHealth(repo sourceGetter, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		src, ok := ownedSourceFromRoute(c, repo, "get source health")
		if !ok {
			return
		}
		health, err := manager.CheckSourceHealth(c.Request.Context(), src, factory)
		if err != nil {
			if err == manager.ErrUnsupportedSourceType {
				c.Status(http.StatusBadRequest)
				return
			}
			slog.ErrorContext(ctx, "get source health: check source", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, sourceHealthResponse{
			ID:              health.SourceID,
			Type:            health.SourceType,
			Status:          string(health.Status),
			CheckedAt:       health.CheckedAt,
			LatencyMS:       health.LatencyMS,
			ReasonCode:      health.ReasonCode,
			Message:         health.Message,
			QuotaTotalBytes: health.Quota.TotalBytes,
			QuotaUsedBytes:  health.Quota.UsedBytes,
			QuotaFreeBytes:  health.Quota.FreeBytes,
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
	if sourceRepo == nil {
		return downloadSourceFile(nil, nil)
	}
	return downloadSourceFile(sourceRepo, nil)
}

// DownloadSourceFileByQuery godoc
// @Summary Download a file from a source
// @Tags sources
// @Produce octet-stream
// @Param id path string true "Source ID"
// @Param file_id query string true "File ID (GDrive file ID or S3 object key)"
// @Success 200 {file} file
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/sources/{id}/download [get]
func DownloadSourceFileByQuery(sourceRepo *repository.SourceRepository) gin.HandlerFunc {
	if sourceRepo == nil {
		return downloadSourceFile(nil, nil)
	}
	return downloadSourceFile(sourceRepo, nil)
}

func DownloadSourceFileByQueryWithFactory(sourceRepo *repository.SourceRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	if sourceRepo == nil {
		return downloadSourceFile(nil, factory)
	}
	return downloadSourceFile(sourceRepo, factory)
}

func DownloadSourceFileWithFactory(sourceRepo *repository.SourceRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	if sourceRepo == nil {
		return downloadSourceFile(nil, factory)
	}
	return downloadSourceFile(sourceRepo, factory)
}

func downloadSourceFile(sourceRepo sourceGetter, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		src, ok := ownedSourceFromRoute(c, sourceRepo, "download source file")
		if !ok {
			return
		}
		fileID := c.Query("file_id")
		if fileID == "" {
			fileID = c.Param("file_id")
		}
		if fileID == "" {
			c.Status(http.StatusBadRequest)
			return
		}

		filename := fileID
		body, err := manager.DownloadSourceFile(c.Request.Context(), src, fileID, factory)
		if err != nil {
			if err == manager.ErrUnsupportedSourceType {
				c.JSON(http.StatusBadRequest, gin.H{"error": "download not supported for this source type"})
				return
			}
			slog.ErrorContext(c.Request.Context(), "download source file: download", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if body == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "download not supported for this source type"})
			return
		}
		defer func() { _ = body.Close() }()

		prefix := make([]byte, 1)
		n, err := io.ReadFull(body, prefix)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			slog.ErrorContext(c.Request.Context(), "download source file: preflight", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		stream := io.Reader(body)
		if n > 0 {
			stream = io.MultiReader(bytes.NewReader(prefix[:n]), body)
		}

		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFilename(filename)))
		c.Header("Content-Type", "application/octet-stream")
		c.Status(http.StatusOK)
		if _, err := io.Copy(c.Writer, stream); err != nil {
			slog.ErrorContext(c.Request.Context(), "download source file: stream", slog.String("error", err.Error()))
		}
	}
}
