package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type createBucketRequest struct {
	Key                  string         `json:"key" binding:"required"`
	SourceIDs            []string       `json:"source_ids" binding:"required,min=1"`
	DistributionStrategy string         `json:"distribution_strategy,omitempty"`
	SourceWeights        map[string]int `json:"source_weights,omitempty"`
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
	Role      string    `json:"role"`
	Shared    bool      `json:"shared"`
}

func bucketDocResponse(bucket repository.Bucket, role repository.BucketRole, shared bool) bucketResponse {
	return bucketResponse{
		ID:        bucket.ID.Hex(),
		Key:       bucket.Key,
		AccessKey: bucket.AccessKey,
		CreatedAt: bucket.CreatedAt,
		Role:      string(role),
		Shared:    shared,
	}
}

type fileResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	Size      int64     `json:"size"`
}

func validateSourceWeights(weights map[string]int, sourceIDs []primitive.ObjectID) error {
	if len(weights) == 0 {
		return nil
	}
	attached := make(map[primitive.ObjectID]struct{}, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		attached[sourceID] = struct{}{}
	}
	for sourceIDHex, weight := range weights {
		sourceID, err := primitive.ObjectIDFromHex(sourceIDHex)
		if err != nil {
			return fmt.Errorf("source_weights key %q must be a valid source id", sourceIDHex)
		}
		if _, ok := attached[sourceID]; !ok {
			return fmt.Errorf("source_weights key %q is not attached to this bucket", sourceIDHex)
		}
		if weight <= 0 {
			return fmt.Errorf("source_weights value for %q must be positive", sourceIDHex)
		}
		if weight > manager.MaxWeightedSourceWeight {
			return fmt.Errorf("source_weights value for %q must be <= %d", sourceIDHex, manager.MaxWeightedSourceWeight)
		}
	}
	return nil
}

func respondBadSourceWeights(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	return true
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
		userID, ok := authenticatedUserID(c)
		if !ok {
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
		strategy := repository.DistributionStrategy(req.DistributionStrategy)
		if strategy == "" {
			strategy = repository.StrategyRoundRobin
		}
		if strategy != repository.StrategyRoundRobin && strategy != repository.StrategyWeighted {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid distribution_strategy"})
			return
		}
		if respondBadSourceWeights(c, validateSourceWeights(req.SourceWeights, sourceIDs)) {
			return
		}
		bucket := repository.Bucket{
			UserID:               userID,
			Key:                  req.Key,
			AccessKey:            accessKey,
			AccessSecretEnc:      encrypted,
			SourceIDs:            sourceIDs,
			DistributionStrategy: strategy,
			SourceWeights:        req.SourceWeights,
			CreatedAt:            time.Now().UTC(),
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
func ListBuckets(repo *repository.BucketRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "list buckets: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}

		// Owned buckets.
		buckets, err := repo.ListByUser(ctx, userID)
		if err != nil {
			slog.ErrorContext(ctx, "list buckets: failed to list buckets", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		resp := make([]bucketResponse, 0, len(buckets))
		for _, b := range buckets {
			resp = append(resp, bucketDocResponse(b, repository.RoleOwner, false))
		}

		// Shared-with-me buckets.
		if grantRepo != nil {
			grants, err := grantRepo.ListByUser(ctx, userID)
			if err != nil {
				slog.ErrorContext(ctx, "list buckets: failed to list grants", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			if len(grants) > 0 {
				bucketIDs := make([]primitive.ObjectID, 0, len(grants))
				grantByBucket := make(map[primitive.ObjectID]repository.BucketGrant, len(grants))
				for _, g := range grants {
					bucketIDs = append(bucketIDs, g.BucketID)
					grantByBucket[g.BucketID] = g
				}
				sharedBuckets, err := repo.ListByIDs(ctx, bucketIDs)
				if err != nil {
					slog.ErrorContext(ctx, "list buckets: failed to fetch shared buckets", slog.String("error", err.Error()))
					c.Status(http.StatusInternalServerError)
					return
				}
				for _, b := range sharedBuckets {
					g := grantByBucket[b.ID]
					resp = append(resp, bucketDocResponse(b, g.Role, true))
				}
			}
		}

		c.JSON(http.StatusOK, resp)
	}
}

// GetBucket godoc
// @Summary Get bucket
// @Tags buckets
// @Produce json
// @Param id path string true "Bucket ID"
// @Success 200 {object} bucketResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id} [get]
func GetBucket(repo *repository.BucketRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	if repo == nil {
		return func(c *gin.Context) {
			slog.ErrorContext(c.Request.Context(), "get bucket: repository is nil")
			c.Status(http.StatusServiceUnavailable)
		}
	}
	var grantReader bucketAccessGrantReader
	if grantRepo != nil {
		grantReader = grantRepo
	}
	return getBucket(repo, grantReader)
}

func getBucket(bucketRepo bucketAccessBucketReader, grantRepo bucketAccessGrantReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil {
			slog.ErrorContext(ctx, "get bucket: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccessFor(c, bucketRepo, grantRepo, repository.RoleViewer)
		if acc == nil {
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}

		c.JSON(http.StatusOK, bucketDocResponse(*acc.Bucket, acc.Role, acc.Bucket.UserID != userID))
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
func DeleteBucket(repo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return DeleteBucketWithFactory(repo, sourceRepo, fileRepo, mpRepo, grantRepo, nil)
}

func DeleteBucketWithFactory(repo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, grantRepo *repository.BucketGrantRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "delete bucket: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, repo, grantRepo, repository.RoleOwner)
		if acc == nil {
			return
		}

		if sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "delete bucket: cleanup repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		objectSvc := manager.NewBucketCleanupServiceWithSourceClientFactory(sourceRepo, fileRepo, mpRepo, factory)
		if _, err := objectSvc.DeleteBucketContents(ctx, acc.Bucket.ID); err != nil {
			slog.ErrorContext(ctx, "delete bucket: cleanup contents", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		if err := repo.Delete(ctx, acc.Bucket.ID, acc.Bucket.UserID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "delete bucket: failed to delete", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		// Clean up all grants for this bucket.
		if grantRepo != nil {
			_ = grantRepo.DeleteByBucket(ctx, acc.Bucket.ID)
		}
		c.Status(http.StatusOK)
	}
}

type updateDistributionRequest struct {
	DistributionStrategy string         `json:"distribution_strategy" binding:"required"`
	SourceWeights        map[string]int `json:"source_weights,omitempty"`
}

// UpdateBucketDistribution godoc
// @Summary Update bucket distribution strategy
// @Tags buckets
// @Accept json
// @Produce json
// @Param id path string true "Bucket ID"
// @Param body body updateDistributionRequest true "Distribution config"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/distribution [patch]
func UpdateBucketDistribution(repo *repository.BucketRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var req updateDistributionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		strategy := repository.DistributionStrategy(req.DistributionStrategy)
		if strategy != repository.StrategyRoundRobin && strategy != repository.StrategyWeighted {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid distribution_strategy"})
			return
		}

		acc := requireBucketAccess(c, repo, grantRepo, repository.RoleOwner)
		if acc == nil {
			return
		}
		if respondBadSourceWeights(c, validateSourceWeights(req.SourceWeights, acc.Bucket.SourceIDs)) {
			return
		}

		if err := repo.UpdateDistribution(c.Request.Context(), acc.Bucket.ID, acc.Bucket.UserID, strategy, req.SourceWeights); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(c.Request.Context(), "update distribution", slog.String("error", err.Error()))
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
		total := fileContentLength(fileDoc)
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
func DeleteFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return DeleteFileWithFactory(bucketRepo, sourceRepo, fileRepo, grantRepo, nil)
}

func DeleteFileWithFactory(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, grantRepo *repository.BucketGrantRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	objectSvc := manager.NewObjectDeleteServiceWithSourceClientFactory(sourceRepo, fileRepo, factory)
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "delete file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleEditor)
		if acc == nil {
			return
		}
		bucketID := acc.Bucket.ID

		fileID, ok := routeObjectID(c, "file_id")
		if !ok {
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
}
