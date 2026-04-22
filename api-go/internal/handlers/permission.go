package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type bucketAccessBucketReader interface {
	GetByID(ctx context.Context, id primitive.ObjectID) (*repository.Bucket, error)
}

type bucketAccessGrantReader interface {
	GetByBucketAndUser(ctx context.Context, bucketID, userID primitive.ObjectID) (*repository.BucketGrant, error)
}

// bucketAccess holds the result of a permission check.
type bucketAccess struct {
	Bucket *repository.Bucket
	Role   repository.BucketRole
}

// requireBucketAccess verifies that the authenticated user has at least the
// given role on the bucket identified by the :id route parameter. On success it
// returns the bucket document and effective role. On failure it writes an HTTP
// error and returns nil.
func requireBucketAccess(
	c *gin.Context,
	bucketRepo bucketAccessBucketReader,
	grantRepo bucketAccessGrantReader,
	requiredRole repository.BucketRole,
) *bucketAccess {
	return requireBucketAccessFor(c, bucketRepo, bucketAccessGrantReaderOrNil(grantRepo), requiredRole)
}

func requireBucketAccessFor(
	c *gin.Context,
	bucketRepo bucketAccessBucketReader,
	grantRepo bucketAccessGrantReader,
	requiredRole repository.BucketRole,
) *bucketAccess {
	grantRepo = bucketAccessGrantReaderOrNil(grantRepo)

	bucketID, ok := routeObjectID(c, "id")
	if !ok {
		return nil
	}
	userID, ok := authenticatedUserID(c)
	if !ok {
		return nil
	}

	if bucketRepo == nil {
		slog.ErrorContext(c.Request.Context(), "bucket access: bucket repository is nil")
		c.Status(http.StatusInternalServerError)
		return nil
	}

	bucketDoc, err := bucketRepo.GetByID(c.Request.Context(), bucketID)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			c.Status(http.StatusNotFound)
			return nil
		}
		c.Status(http.StatusInternalServerError)
		return nil
	}

	// Bucket owner always has the owner role.
	if bucketDoc.UserID == userID {
		return &bucketAccess{Bucket: bucketDoc, Role: repository.RoleOwner}
	}

	// Check explicit grants.
	if grantRepo != nil {
		grant, err := grantRepo.GetByBucketAndUser(c.Request.Context(), bucketID, userID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return nil
			}
			slog.ErrorContext(c.Request.Context(), "bucket access: grant lookup failed",
				slog.String("bucket_id", bucketID.Hex()),
				slog.String("user_id", userID.Hex()),
				slog.String("error", err.Error()),
			)
			c.Status(http.StatusInternalServerError)
			return nil
		}
		if repository.RoleAtLeast(grant.Role, requiredRole) {
			return &bucketAccess{Bucket: bucketDoc, Role: grant.Role}
		}
	}

	// No access — return 404 to avoid leaking bucket existence.
	c.Status(http.StatusNotFound)
	return nil
}

func bucketAccessGrantReaderOrNil(grantRepo bucketAccessGrantReader) bucketAccessGrantReader {
	if grantRepo == nil {
		return nil
	}
	if repo, ok := grantRepo.(*repository.BucketGrantRepository); ok && repo == nil {
		return nil
	}
	return grantRepo
}
