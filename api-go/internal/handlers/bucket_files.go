package handlers

import (
	"context"
	"errors"
	"fmt"
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

type uploadFileResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type fileResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

type paginatedFileResponse struct {
	Items      []fileResponse `json:"items"`
	NextCursor string         `json:"next_cursor,omitempty"`
}

const (
	maxBatchDeleteFiles       = 100
	maxBucketFileListPageSize = 200
)

type batchDeleteFilesRequest struct {
	FileIDs []string `json:"file_ids"`
}

type batchDeleteFileResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type batchDeleteFileIssue struct {
	ID    string `json:"id"`
	Name  string `json:"name,omitempty"`
	Error string `json:"error"`
}

type batchDeleteFilesResponse struct {
	Deleted  []batchDeleteFileResponse `json:"deleted"`
	Failed   []batchDeleteFileIssue    `json:"failed"`
	Warnings []batchDeleteFileIssue    `json:"warnings,omitempty"`
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
func UploadFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository, chunkSize int) gin.HandlerFunc {
	return UploadFileWithFactory(bucketRepo, sourceRepo, fileRepo, grantRepo, chunkSize, nil)
}

func UploadFileWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository, chunkSize int, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectWriteServiceWithSourceClientFactory(sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "upload file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleEditor)
		if acc == nil {
			return
		}
		bucketDoc := acc.Bucket
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

		result, err := objectSvc.PutObject(ctx, bucketDoc, fh.Filename, f, chunkSize, objectContentType(fh.Header.Get("Content-Type")), nil)
		if err != nil {
			if isBucketSourceResolutionError(err) {
				c.Status(http.StatusBadRequest)
				return
			}
			slog.ErrorContext(ctx, "upload file: mutate file", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if result.CleanupErr != nil {
			slog.WarnContext(ctx, "upload file: delete old chunks", slog.String("error", result.CleanupErr.Error()))
		}
		c.JSON(http.StatusOK, uploadFileResponse{
			ID:        result.File.ID.Hex(),
			Name:      result.File.Name,
			CreatedAt: result.File.CreatedAt,
		})
	}
}

// ListFiles godoc
// @Summary List files in bucket
// @Tags buckets
// @Produce json
// @Param id path string true "Bucket ID"
// @Param q query string false "Filename search query"
// @Param limit query int false "Page size for bounded file listing"
// @Param cursor query string false "Continuation cursor for bounded file listing"
// @Success 200 {array} fileResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files [get]
func ListFiles(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "list files: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleViewer)
		if acc == nil {
			return
		}
		bucketID := acc.Bucket.ID
		query := strings.TrimSpace(c.Query("q"))
		limitParam := strings.TrimSpace(c.Query("limit"))
		if limitParam == "" {
			files, err := fileRepo.ListByBucketByNameQuery(c.Request.Context(), bucketID, query)
			if err != nil {
				slog.ErrorContext(ctx, "list files: list files", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			resp := make([]fileResponse, 0, len(files))
			for _, f := range files {
				resp = append(resp, fileResponse{
					ID:        f.ID.Hex(),
					Name:      f.Name,
					CreatedAt: f.CreatedAt,
					Size:      manager.FileSize(f),
				})
			}
			c.JSON(http.StatusOK, resp)
			return
		}

		limit, err := strconv.Atoi(limitParam)
		if err != nil || limit < 1 || limit > maxBucketFileListPageSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("limit must be between 1 and %d", maxBucketFileListPageSize)})
			return
		}
		files, hasMore, err := fileRepo.ListByBucketByNameQueryPage(c.Request.Context(), bucketID, query, c.Query("cursor"), limit)
		if err != nil {
			slog.ErrorContext(ctx, "list files: list files", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		resp := make([]fileResponse, 0, len(files))
		for _, f := range files {
			resp = append(resp, fileResponse{
				ID:        f.ID.Hex(),
				Name:      f.Name,
				CreatedAt: f.CreatedAt,
				Size:      manager.FileSize(f),
			})
		}
		page := paginatedFileResponse{Items: resp}
		if hasMore && len(files) > 0 {
			page.NextCursor = files[len(files)-1].Name
		}
		c.JSON(http.StatusOK, page)
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
func DownloadFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return DownloadFileWithFactory(bucketRepo, sourceRepo, fileRepo, grantRepo, nil)
}

func DownloadFileWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "download file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var grantReader bucketAccessGrantReader
		if grantRepo != nil {
			grantReader = grantRepo
		}
		downloadFile(bucketRepo, sourceRepo, fileRepo, grantReader, factory)(c)
	}
}

func downloadFile(bucketRepo bucketAccessBucketReader, sourceRepo *repository.SourceRepository, fileRepo fileByIDReader, grantRepo bucketAccessGrantReader, factory manager.SourceClientFactory) gin.HandlerFunc {
	streamFile := fileStreamFuncForFactory(factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "download file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccessFor(c, bucketRepo, grantRepo, repository.RoleViewer)
		if acc == nil {
			return
		}
		bucketID := acc.Bucket.ID

		fileID, ok := routeObjectID(c, "file_id")
		if !ok {
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
		total := manager.FileSize(*fileDoc)
		w := newDeferredResponseWriter(c, func() {
			setAttachmentDownloadHeaders(c, fileDoc.Name, total)
			c.Status(http.StatusOK)
		})
		if err := streamFile(c.Request.Context(), sourceRepo, fileDoc, w); err != nil {
			if !w.isCommitted() {
				slog.ErrorContext(ctx, "download file: stream failed", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			slog.ErrorContext(ctx, "download file: stream failed after response commit", slog.String("error", err.Error()))
			return
		}
		w.commitNow()
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
func DeleteFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return DeleteFileWithFactory(bucketRepo, sourceRepo, fileRepo, shareLinkRepo, grantRepo, nil)
}

func DeleteFileWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectDeleteServiceWithSourceClientFactory(sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil || shareLinkRepo == nil {
			slog.ErrorContext(ctx, "delete file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var grantReader bucketAccessGrantReader
		if grantRepo != nil {
			grantReader = grantRepo
		}
		handleDeleteFile(c, bucketRepo, fileRepo, shareLinkRepo, objectSvc, grantReader)
	}
}

// BatchDeleteFiles godoc
// @Summary Delete multiple bucket files
// @Tags buckets
// @Accept json
// @Produce json
// @Param id path string true "Bucket ID"
// @Param request body batchDeleteFilesRequest true "File IDs"
// @Success 200 {object} batchDeleteFilesResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files/batch-delete [post]
func BatchDeleteFiles(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return BatchDeleteFilesWithFactory(bucketRepo, sourceRepo, fileRepo, shareLinkRepo, grantRepo, nil)
}

func BatchDeleteFilesWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectDeleteServiceWithSourceClientFactory(sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil || shareLinkRepo == nil {
			slog.ErrorContext(ctx, "batch delete files: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var grantReader bucketAccessGrantReader
		if grantRepo != nil {
			grantReader = grantRepo
		}
		handleBatchDeleteFiles(c, bucketRepo, fileRepo, shareLinkRepo, objectSvc, grantReader)
	}
}

func handleBatchDeleteFiles(c *gin.Context, bucketRepo bucketAccessBucketReader, fileRepo fileByIDReader, shareLinkRepo shareLinkFileDeleter, objectSvc objectFileDeleter, grantRepo bucketAccessGrantReader) {
	ctx := c.Request.Context()
	if bucketRepo == nil || fileRepo == nil || shareLinkRepo == nil || objectSvc == nil {
		slog.ErrorContext(ctx, "batch delete files: repository is nil")
		c.Status(http.StatusServiceUnavailable)
		return
	}

	acc := requireBucketAccessFor(c, bucketRepo, grantRepo, repository.RoleEditor)
	if acc == nil {
		return
	}
	bucketID := acc.Bucket.ID

	var req batchDeleteFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_ids must be a non-empty JSON array"})
		return
	}
	fileIDs, ok := normalizeBatchDeleteFileIDs(req.FileIDs)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file_ids must contain between 1 and 100 valid file IDs"})
		return
	}

	resp := batchDeleteFilesResponse{
		Deleted: make([]batchDeleteFileResponse, 0, len(fileIDs)),
		Failed:  make([]batchDeleteFileIssue, 0),
	}

	for _, fileID := range fileIDs {
		fileDoc, result, err := deleteBucketFile(ctx, bucketID, fileID, fileRepo, shareLinkRepo, objectSvc)
		if err != nil {
			if errors.Is(err, manager.ErrObjectNotFound) {
				resp.Failed = append(resp.Failed, batchDeleteFileIssue{
					ID:    fileID.Hex(),
					Error: "File not found",
				})
				continue
			}
			slog.ErrorContext(ctx, "batch delete files: delete file", slog.String("file_id", fileID.Hex()), slog.String("error", err.Error()))
			resp.Failed = append(resp.Failed, batchDeleteFileIssue{
				ID:    fileID.Hex(),
				Name:  batchDeleteFileName(fileDoc),
				Error: "Delete failed",
			})
			continue
		}

		resp.Deleted = append(resp.Deleted, batchDeleteFileResponse{
			ID:   fileID.Hex(),
			Name: fileDoc.Name,
		})
		if result.CleanupErr != nil {
			slog.WarnContext(ctx, "batch delete files: chunk cleanup warning", slog.String("file_id", fileID.Hex()), slog.String("error", result.CleanupErr.Error()))
			resp.Warnings = append(resp.Warnings, batchDeleteFileIssue{
				ID:    fileID.Hex(),
				Name:  fileDoc.Name,
				Error: "File deleted, but chunk cleanup needs follow-up",
			})
		}
	}

	c.JSON(http.StatusOK, resp)
}

func handleDeleteFile(c *gin.Context, bucketRepo bucketAccessBucketReader, fileRepo fileByIDReader, shareLinkRepo shareLinkFileDeleter, objectSvc objectFileDeleter, grantRepo bucketAccessGrantReader) {
	ctx := c.Request.Context()
	if bucketRepo == nil || fileRepo == nil || shareLinkRepo == nil || objectSvc == nil {
		slog.ErrorContext(ctx, "delete file: repository is nil")
		c.Status(http.StatusServiceUnavailable)
		return
	}

	acc := requireBucketAccessFor(c, bucketRepo, grantRepo, repository.RoleEditor)
	if acc == nil {
		return
	}
	bucketID := acc.Bucket.ID

	fileID, ok := routeObjectID(c, "file_id")
	if !ok {
		return
	}
	_, result, err := deleteBucketFile(ctx, bucketID, fileID, fileRepo, shareLinkRepo, objectSvc)
	if err != nil {
		if errors.Is(err, manager.ErrObjectNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		slog.ErrorContext(ctx, "delete file: mutate file", slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}
	if result.CleanupErr != nil {
		slog.ErrorContext(ctx, "delete file: delete chunk", slog.String("error", result.CleanupErr.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func deleteBucketFile(ctx context.Context, bucketID, fileID primitive.ObjectID, fileRepo fileByIDReader, shareLinkRepo shareLinkFileDeleter, objectSvc objectFileDeleter) (*repository.File, manager.DeleteObjectResult, error) {
	fileDoc, err := fileRepo.GetByID(ctx, fileID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, manager.DeleteObjectResult{}, manager.ErrObjectNotFound
		}
		return nil, manager.DeleteObjectResult{}, err
	}
	if fileDoc.BucketID != bucketID {
		return nil, manager.DeleteObjectResult{}, manager.ErrObjectNotFound
	}
	if err := shareLinkRepo.DeleteByFile(ctx, fileID); err != nil {
		return fileDoc, manager.DeleteObjectResult{}, err
	}
	result, err := objectSvc.DeleteFile(ctx, bucketID, fileID)
	if err != nil {
		return fileDoc, result, err
	}
	return fileDoc, result, nil
}

func normalizeBatchDeleteFileIDs(raw []string) ([]primitive.ObjectID, bool) {
	if len(raw) == 0 || len(raw) > maxBatchDeleteFiles {
		return nil, false
	}
	seen := make(map[primitive.ObjectID]struct{}, len(raw))
	fileIDs := make([]primitive.ObjectID, 0, len(raw))
	for _, fileIDHex := range raw {
		fileID, err := primitive.ObjectIDFromHex(strings.TrimSpace(fileIDHex))
		if err != nil {
			return nil, false
		}
		if _, ok := seen[fileID]; ok {
			continue
		}
		seen[fileID] = struct{}{}
		fileIDs = append(fileIDs, fileID)
	}
	if len(fileIDs) == 0 {
		return nil, false
	}
	return fileIDs, true
}

func batchDeleteFileName(file *repository.File) string {
	if file == nil {
		return ""
	}
	return file.Name
}
