package handlers

import (
	"context"
	"net/http"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
)

func abortMultipartUpload(c *gin.Context, bucketRepo objectBucketReader, sourceRepo *repository.SourceRepository, mpRepo multipartUploadAbortStore, factory manager.SourceClientFactory) {
	ctx := c.Request.Context()
	uploadID := c.Query("uploadId")

	mu, err := mpRepo.GetByUploadID(ctx, uploadID)
	if handleMultipartUploadLookupError(c, "abort multipart", err) {
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}
	if handleMultipartBucketMismatch(c, mu.BucketID == bucketDoc.ID) {
		return
	}

	err = manager.AbortMultipartUploadRecord(ctx, mpRepo, func(ctx context.Context, chunks []repository.FileChunk) error {
		return manager.DeleteFileChunksWithFactory(ctx, sourceRepo, chunks, factory)
	}, mu)
	if handleMultipartAbortError(c, err) {
		return
	}

	c.Status(http.StatusNoContent)
}
