package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
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

type bucketCreator interface {
	Create(context.Context, repository.Bucket) (*repository.Bucket, error)
}

type bucketDeleteStore interface {
	bucketAccessBucketReader
	Delete(ctx context.Context, id primitive.ObjectID, userID primitive.ObjectID) error
}

type bucketGrantBucketDeleter interface {
	DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error
}

type bucketContentsDeleter interface {
	DeleteBucketContents(ctx context.Context, bucketID primitive.ObjectID) (manager.DeleteBucketContentsResult, error)
}

type objectFileDeleter interface {
	DeleteFile(ctx context.Context, bucketID, fileID primitive.ObjectID) (manager.DeleteObjectResult, error)
}

type shareLinkBucketDeleter interface {
	DeleteByBucket(ctx context.Context, bucketID primitive.ObjectID) error
}

type shareLinkFileDeleter interface {
	DeleteByFile(ctx context.Context, fileID primitive.ObjectID) error
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

func isNilDependency(dep any) bool {
	if dep == nil {
		return true
	}
	value := reflect.ValueOf(dep)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
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
func CreateBucket(repo bucketCreator, sourceRepo sourceGetter, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req createBucketRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.WarnContext(ctx, "create bucket: invalid request", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		if isNilDependency(repo) || isNilDependency(sourceRepo) {
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
		seenSourceIDs := make(map[primitive.ObjectID]struct{}, len(req.SourceIDs))
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
			if _, ok := seenSourceIDs[sourceID]; ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("source_ids contains duplicate source id %q", sourceIDHex)})
				return
			}
			seenSourceIDs[sourceID] = struct{}{}
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
			resp = append(resp, bucketResponseFromAccess(b, repository.RoleOwner, false))
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
					resp = append(resp, bucketResponseFromAccess(b, g.Role, true))
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

		c.JSON(http.StatusOK, bucketResponseFromAccess(*acc.Bucket, acc.Role, acc.Bucket.UserID != userID))
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
func DeleteBucket(repo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return DeleteBucketWithFactory(repo, sourceRepo, fileRepo, mpRepo, shareLinkRepo, grantRepo, nil)
}

func DeleteBucketWithFactory(repo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository, factory manager.SourceClientFactory) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "delete bucket: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var objectSvc bucketContentsDeleter
		if sourceRepo != nil && fileRepo != nil {
			objectSvc = manager.NewBucketCleanupServiceWithSourceClientFactory(sourceRepo, fileRepo, mpRepo, factory)
		}
		var grantReader bucketAccessGrantReader
		var grantDeleter bucketGrantBucketDeleter
		if grantRepo != nil {
			grantReader = grantRepo
			grantDeleter = grantRepo
		}
		handleDeleteBucket(c, repo, shareLinkRepo, objectSvc, grantReader, grantDeleter)
	}
}

func handleDeleteBucket(c *gin.Context, repo bucketDeleteStore, shareLinkRepo shareLinkBucketDeleter, objectSvc bucketContentsDeleter, grantRepo bucketAccessGrantReader, grantDeleter bucketGrantBucketDeleter) {
	ctx := c.Request.Context()
	if repo == nil {
		slog.ErrorContext(ctx, "delete bucket: repository is nil")
		c.Status(http.StatusServiceUnavailable)
		return
	}

	acc := requireBucketAccessFor(c, repo, grantRepo, repository.RoleOwner)
	if acc == nil {
		return
	}
	if shareLinkRepo == nil || objectSvc == nil {
		slog.ErrorContext(ctx, "delete bucket: cleanup repository is nil")
		c.Status(http.StatusServiceUnavailable)
		return
	}

	if err := shareLinkRepo.DeleteByBucket(ctx, acc.Bucket.ID); err != nil {
		slog.ErrorContext(ctx, "delete bucket: cleanup share links", slog.String("error", err.Error()))
		c.Status(http.StatusInternalServerError)
		return
	}
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
	if grantDeleter != nil {
		_ = grantDeleter.DeleteByBucket(ctx, acc.Bucket.ID)
	}
	c.Status(http.StatusOK)
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
