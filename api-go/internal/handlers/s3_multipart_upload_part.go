package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
)

func uploadPart(c *gin.Context, bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, mpRepo *repository.MultipartUploadRepository, chunkSize int, factory manager.SourceClientFactory) {
	ctx := c.Request.Context()
	uploadID := c.Query("uploadId")
	partNumStr := c.Query("partNumber")
	partNum, err := strconv.Atoi(partNumStr)
	if err != nil || partNum < 1 || partNum > 10000 {
		writeS3Error(c, http.StatusBadRequest, "InvalidArgument", "partNumber must be between 1 and 10000")
		return
	}

	mu, err := mpRepo.GetByUploadID(ctx, uploadID)
	if handleMultipartUploadLookupError(c, "upload part", err) {
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	if handleMultipartBucketMismatch(c, mu.BucketID == bucketDoc.ID) {
		return
	}

	objectSvc := manager.NewMultipartPartWriteServiceWithSourceClientFactory(sourceRepo, mpRepo, factory)
	result, err := objectSvc.UploadMultipartPartRecord(ctx, bucketDoc, mu, partNum, c.Request.Body, chunkSize)
	if handleMultipartUploadPartError(c, err) {
		return
	}
	if result.CleanupErr != nil {
		slog.WarnContext(ctx, "upload part: delete old part chunks", slog.String("error", result.CleanupErr.Error()))
	}

	c.Header("ETag", result.ETag)
	c.Status(http.StatusOK)
}
