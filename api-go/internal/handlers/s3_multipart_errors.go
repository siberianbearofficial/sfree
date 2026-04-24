package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

func writeNoSuchUploadError(c *gin.Context) {
	writeS3Error(c, http.StatusNotFound, "NoSuchUpload", "")
}

func handleMultipartUploadLookupError(c *gin.Context, operation string, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, mongo.ErrNoDocuments) {
		writeNoSuchUploadError(c)
		return true
	}
	slog.ErrorContext(c.Request.Context(), operation+": get upload", slog.String("error", err.Error()))
	writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
	return true
}

func handleMultipartBucketMismatch(c *gin.Context, matches bool) bool {
	if matches {
		return false
	}
	writeNoSuchUploadError(c)
	return true
}

func handleMultipartCompletionError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, manager.ErrMultipartUploadNotFound):
		writeNoSuchUploadError(c)
	case errors.Is(err, manager.ErrMultipartUploadHasNoParts):
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "at least one part is required")
	case errors.Is(err, manager.ErrMultipartUploadPartOrder):
		writeS3Error(c, http.StatusBadRequest, "InvalidPartOrder", "part numbers must be in ascending order")
	case errors.Is(err, manager.ErrMultipartUploadInvalidPart):
		var partErr manager.InvalidMultipartPartError
		if errors.As(err, &partErr) {
			message := fmt.Sprintf("part %d not uploaded", partErr.PartNumber)
			if partErr.Reason == "etag mismatch" {
				message = fmt.Sprintf("ETag mismatch for part %d", partErr.PartNumber)
			}
			writeS3Error(c, http.StatusBadRequest, "InvalidPart", message)
			return true
		}
		writeS3Error(c, http.StatusBadRequest, "InvalidPart", "")
	default:
		slog.ErrorContext(c.Request.Context(), "complete multipart: mutate object", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
	}
	return true
}

func handleMultipartUploadPartError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	switch {
	case errors.Is(err, manager.ErrMultipartUploadNotFound):
		writeNoSuchUploadError(c)
	case isBucketSourceResolutionError(err):
		writeS3Error(c, http.StatusBadRequest, "InvalidRequest", "no sources configured")
	default:
		slog.ErrorContext(c.Request.Context(), "upload part: store part", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
	}
	return true
}

func handleMultipartAbortError(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, manager.ErrMultipartUploadNotFound) {
		writeNoSuchUploadError(c)
		return true
	}
	slog.ErrorContext(c.Request.Context(), "abort multipart: cleanup", slog.String("error", err.Error()))
	writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
	return true
}
