package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func createMultipartUpload(c *gin.Context, bucketRepo *repository.BucketRepository, mpRepo *repository.MultipartUploadRepository) {
	ctx := c.Request.Context()
	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	objectKey := s3ObjectKey(c)
	if objectKey == "" {
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "empty object key")
		return
	}

	uploadID := primitive.NewObjectID().Hex()
	mu := repository.MultipartUpload{
		BucketID:     bucketDoc.ID,
		ObjectKey:    objectKey,
		UploadID:     uploadID,
		CreatedAt:    time.Now().UTC(),
		ContentType:  requestObjectContentType(c.Request),
		UserMetadata: requestObjectUserMetadata(c.Request),
	}
	if _, err := mpRepo.Create(ctx, mu); err != nil {
		slog.ErrorContext(ctx, "create multipart upload", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return
	}
	c.XML(http.StatusOK, initiateMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Bucket:   S3BucketKey(c),
		Key:      objectKey,
		UploadId: uploadID,
	})
}
