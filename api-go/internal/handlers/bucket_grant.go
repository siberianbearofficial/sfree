package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type createGrantRequest struct {
	Username string `json:"username" binding:"required"`
	Role     string `json:"role" binding:"required"`
}

type updateGrantRequest struct {
	Role string `json:"role" binding:"required"`
}

type grantResponse struct {
	ID        string    `json:"id"`
	BucketID  string    `json:"bucket_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	GrantedBy string    `json:"granted_by"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateGrant godoc
// @Summary Grant a user access to a bucket
// @Tags bucket-grants
// @Accept json
// @Produce json
// @Param id path string true "Bucket ID"
// @Param body body createGrantRequest true "Grant details"
// @Success 200 {object} grantResponse
// @Failure 400,401,404,409 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/grants [post]
func CreateGrant(bucketRepo *repository.BucketRepository, grantRepo *repository.BucketGrantRepository, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || grantRepo == nil || userRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleOwner)
		if acc == nil {
			return
		}

		var req createGrantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		role := repository.BucketRole(req.Role)
		if !repository.ValidRole(role) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, must be owner, editor, or viewer"})
			return
		}

		// Look up the target user by username.
		targetUser, err := userRepo.GetByUsername(ctx, req.Username)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.JSON(http.StatusBadRequest, gin.H{"error": "user not found"})
				return
			}
			slog.ErrorContext(ctx, "create grant: lookup user", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		// Cannot grant to the bucket owner (they already have implicit owner access).
		if targetUser.ID == acc.Bucket.UserID {
			c.JSON(http.StatusBadRequest, gin.H{"error": "cannot grant access to bucket owner"})
			return
		}

		granterID, ok := authenticatedUserID(c)
		if !ok {
			return
		}

		grant := repository.BucketGrant{
			BucketID:  acc.Bucket.ID,
			UserID:    targetUser.ID,
			Role:      role,
			GrantedBy: granterID,
			CreatedAt: time.Now().UTC(),
		}

		created, err := grantRepo.Create(ctx, grant)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				c.JSON(http.StatusConflict, gin.H{"error": "user already has access to this bucket"})
				return
			}
			slog.ErrorContext(ctx, "create grant: save", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, grantResponse{
			ID:        created.ID.Hex(),
			BucketID:  created.BucketID.Hex(),
			UserID:    created.UserID.Hex(),
			Username:  targetUser.Username,
			Role:      string(created.Role),
			GrantedBy: created.GrantedBy.Hex(),
			CreatedAt: created.CreatedAt,
		})
	}
}

// ListGrants godoc
// @Summary List access grants for a bucket
// @Tags bucket-grants
// @Produce json
// @Param id path string true "Bucket ID"
// @Success 200 {array} grantResponse
// @Failure 400,401,404 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/grants [get]
func ListGrants(bucketRepo *repository.BucketRepository, grantRepo *repository.BucketGrantRepository, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || grantRepo == nil || userRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleOwner)
		if acc == nil {
			return
		}

		grants, err := grantRepo.ListByBucket(ctx, acc.Bucket.ID)
		if err != nil {
			slog.ErrorContext(ctx, "list grants: query", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		resp := make([]grantResponse, 0, len(grants))
		for _, g := range grants {
			username := ""
			if u, err := userRepo.GetByID(ctx, g.UserID); err == nil {
				username = u.Username
			}
			resp = append(resp, grantResponse{
				ID:        g.ID.Hex(),
				BucketID:  g.BucketID.Hex(),
				UserID:    g.UserID.Hex(),
				Username:  username,
				Role:      string(g.Role),
				GrantedBy: g.GrantedBy.Hex(),
				CreatedAt: g.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, resp)
	}
}

// UpdateGrant godoc
// @Summary Update a grant's role
// @Tags bucket-grants
// @Accept json
// @Param id path string true "Bucket ID"
// @Param grant_id path string true "Grant ID"
// @Param body body updateGrantRequest true "New role"
// @Success 200 {string} string ""
// @Failure 400,401,404 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/grants/{grant_id} [patch]
func UpdateGrant(bucketRepo *repository.BucketRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || grantRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleOwner)
		if acc == nil {
			return
		}

		grantID, ok := routeObjectID(c, "grant_id")
		if !ok {
			return
		}

		var req updateGrantRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		role := repository.BucketRole(req.Role)
		if !repository.ValidRole(role) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid role, must be owner, editor, or viewer"})
			return
		}

		if err := grantRepo.UpdateRole(ctx, grantID, role); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "update grant", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

// DeleteGrant godoc
// @Summary Revoke a user's access to a bucket
// @Tags bucket-grants
// @Param id path string true "Bucket ID"
// @Param grant_id path string true "Grant ID"
// @Success 200 {string} string ""
// @Failure 400,401,404 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/grants/{grant_id} [delete]
func DeleteGrant(bucketRepo *repository.BucketRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || grantRepo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleOwner)
		if acc == nil {
			return
		}

		grantID, ok := routeObjectID(c, "grant_id")
		if !ok {
			return
		}

		if err := grantRepo.Delete(ctx, grantID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "delete grant", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}
