package handlers

import (
	"encoding/xml"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
)

func completeMultipartUpload(c *gin.Context, bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, mpRepo *repository.MultipartUploadRepository, factory manager.SourceClientFactory) {
	ctx := c.Request.Context()
	uploadID := c.Query("uploadId")

	mu, err := mpRepo.GetByUploadID(ctx, uploadID)
	if handleMultipartUploadLookupError(c, "complete multipart", err) {
		return
	}

	bucketDoc, ok := lookupBucket(c, bucketRepo)
	if !ok {
		return
	}

	var req completeMultipartUploadRequest
	if err := xml.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		writeS3Error(c, http.StatusBadRequest, "MalformedXML", "could not parse CompleteMultipartUpload body")
		return
	}
	requestedParts := make([]manager.CompleteMultipartPart, 0, len(req.Parts))
	for _, rp := range req.Parts {
		requestedParts = append(requestedParts, manager.CompleteMultipartPart{PartNumber: rp.PartNumber, ETag: rp.ETag})
	}

	objectSvc := manager.NewMultipartCompletionServiceWithSourceClientFactory(sourceRepo, fileRepo, mpRepo, factory)
	result, err := objectSvc.CompleteMultipartUploadRecord(ctx, bucketDoc.ID, mu, requestedParts)
	if handleMultipartCompletionError(c, err) {
		return
	}
	for _, cleanupErr := range result.CleanupErrs {
		slog.WarnContext(ctx, "complete multipart: cleanup", slog.String("error", cleanupErr.Error()))
	}
	c.XML(http.StatusOK, completeMultipartUploadResult{
		Xmlns:    "http://s3.amazonaws.com/doc/2006-03-01/",
		Location: fmt.Sprintf("/%s/%s", c.Param("bucket"), result.Upload.ObjectKey),
		Bucket:   c.Param("bucket"),
		Key:      result.Upload.ObjectKey,
		ETag:     result.ETag,
	})
}
