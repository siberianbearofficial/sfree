package handlers

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
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

type multiFileDownloadRequest struct {
	FileIDs []string `json:"file_ids"`
}

const (
	maxMultiFileDownloadCount    = 50
	maxMultiFileDownloadTotal    = 250 << 20
	maxMultiFileDownloadTotalStr = "250 MiB"
)

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
		files, err := fileRepo.ListByBucketByNameQuery(c.Request.Context(), bucketID, strings.TrimSpace(c.Query("q")))
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

// DownloadFilesArchive godoc
// @Summary Download multiple files as a zip archive
// @Tags buckets
// @Accept json
// @Produce application/zip
// @Param id path string true "Bucket ID"
// @Param body body multiFileDownloadRequest true "Selected file IDs"
// @Success 200 {file} file
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files/download [post]
func DownloadFilesArchive(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return DownloadFilesArchiveWithFactory(bucketRepo, sourceRepo, fileRepo, grantRepo, nil)
}

func DownloadFilesArchiveWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "download files archive: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var grantReader bucketAccessGrantReader
		if grantRepo != nil {
			grantReader = grantRepo
		}
		downloadFilesArchive(bucketRepo, sourceRepo, fileRepo, grantReader, factory)(c)
	}
}

func downloadFilesArchive(bucketRepo bucketAccessBucketReader, sourceRepo *repository.SourceRepository, fileRepo fileByIDReader, grantRepo bucketAccessGrantReader, factory manager.SourceClientFactory) gin.HandlerFunc {
	streamFile := fileStreamFuncForFactory(factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "download files archive: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleViewer)
		if acc == nil {
			return
		}

		var req multiFileDownloadRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.WarnContext(ctx, "download files archive: invalid request", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		if len(req.FileIDs) == 0 || len(req.FileIDs) > maxMultiFileDownloadCount {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file_ids must contain between 1 and %d file ids", maxMultiFileDownloadCount)})
			return
		}

		files := make([]*repository.File, 0, len(req.FileIDs))
		seen := make(map[string]struct{}, len(req.FileIDs))
		var totalSize int64
		for _, fileIDHex := range req.FileIDs {
			if _, ok := seen[fileIDHex]; ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("file_ids contains duplicate file id %q", fileIDHex)})
				return
			}
			seen[fileIDHex] = struct{}{}

			fileID, err := primitive.ObjectIDFromHex(fileIDHex)
			if err != nil {
				c.Status(http.StatusBadRequest)
				return
			}
			fileDoc, err := fileRepo.GetByID(ctx, fileID)
			if err != nil {
				if err == mongo.ErrNoDocuments {
					c.Status(http.StatusNotFound)
					return
				}
				slog.ErrorContext(ctx, "download files archive: get file", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			if fileDoc.BucketID != acc.Bucket.ID {
				c.Status(http.StatusNotFound)
				return
			}
			totalSize += manager.FileSize(*fileDoc)
			if totalSize > maxMultiFileDownloadTotal {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("selected files exceed the %s archive limit", maxMultiFileDownloadTotalStr)})
				return
			}
			files = append(files, fileDoc)
		}

		archiveName := sanitizeFilename(acc.Bucket.Key) + "-files.zip"
		if archiveName == "-files.zip" {
			archiveName = "files.zip"
		}
		w := newDeferredResponseWriter(c, func() {
			setArchiveDownloadHeaders(c, archiveName)
			c.Status(http.StatusOK)
		})
		zipTarget := newDeferredZipWriter(w)
		archive := zip.NewWriter(zipTarget)

		for _, fileDoc := range files {
			header := &zip.FileHeader{
				Name:     fileDoc.Name,
				Method:   zip.Store,
				Modified: fileDoc.CreatedAt,
			}
			entry, err := archive.CreateHeader(header)
			if err != nil {
				slog.ErrorContext(ctx, "download files archive: create entry", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			if err := streamFile(ctx, sourceRepo, fileDoc, newCommitOnFirstWriteWriter(entry, zipTarget)); err != nil {
				_ = archive.Close()
				if !w.isCommitted() {
					slog.ErrorContext(ctx, "download files archive: stream failed", slog.String("error", err.Error()))
					c.Status(http.StatusInternalServerError)
					return
				}
				slog.ErrorContext(ctx, "download files archive: stream failed after response commit", slog.String("error", err.Error()))
				return
			}
		}

		if err := archive.Close(); err != nil {
			if !w.isCommitted() {
				slog.ErrorContext(ctx, "download files archive: close archive", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			slog.ErrorContext(ctx, "download files archive: close archive after response commit", slog.String("error", err.Error()))
			return
		}
		if err := zipTarget.commit(); err != nil {
			if !w.isCommitted() {
				slog.ErrorContext(ctx, "download files archive: commit archive", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			slog.ErrorContext(ctx, "download files archive: commit archive after response commit", slog.String("error", err.Error()))
			return
		}
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

type deferredZipWriter struct {
	target    io.Writer
	buf       bytes.Buffer
	committed bool
}

func newDeferredZipWriter(target io.Writer) *deferredZipWriter {
	return &deferredZipWriter{target: target}
}

func (w *deferredZipWriter) Write(p []byte) (int, error) {
	if !w.committed {
		return w.buf.Write(p)
	}
	return w.target.Write(p)
}

func (w *deferredZipWriter) commit() error {
	if w.committed {
		return nil
	}
	w.committed = true
	if w.buf.Len() == 0 {
		return nil
	}
	_, err := w.target.Write(w.buf.Bytes())
	w.buf.Reset()
	return err
}

type commitOnFirstWriteWriter struct {
	target    io.Writer
	commit    func() error
	committed bool
}

func newCommitOnFirstWriteWriter(target io.Writer, committer interface{ commit() error }) *commitOnFirstWriteWriter {
	return &commitOnFirstWriteWriter{
		target: target,
		commit: committer.commit,
	}
}

func (w *commitOnFirstWriteWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if !w.committed {
		if err := w.commit(); err != nil {
			return 0, err
		}
		w.committed = true
	}
	return w.target.Write(p)
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
	fileDoc, err := fileRepo.GetByID(ctx, fileID)
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
	if err := shareLinkRepo.DeleteByFile(ctx, fileID); err != nil {
		slog.ErrorContext(ctx, "delete file: cleanup share links", slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}
	result, err := objectSvc.DeleteFile(ctx, bucketID, fileID)
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
