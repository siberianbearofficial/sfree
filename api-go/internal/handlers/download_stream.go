package handlers

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const downloadPreflightBytes = int64(1)

type fileStreamFunc func(context.Context, *repository.SourceRepository, *repository.File, io.Writer) error
type fileRangeStreamFunc func(context.Context, *repository.SourceRepository, *repository.File, io.Writer, int64, int64) error

type fileByIDReader interface {
	GetByID(ctx context.Context, id primitive.ObjectID) (*repository.File, error)
}

type shareLinkByTokenReader interface {
	GetByToken(ctx context.Context, token string) (*repository.ShareLink, error)
}

var (
	streamDownloadFile      fileStreamFunc      = manager.StreamFile
	streamDownloadFileRange fileRangeStreamFunc = manager.StreamFileRange
)

func preflightFileRange(ctx context.Context, sourceRepo *repository.SourceRepository, fileDoc *repository.File, start, end int64, streamRange fileRangeStreamFunc) error {
	if end < start {
		return nil
	}
	preflightEnd := start + downloadPreflightBytes - 1
	if preflightEnd > end {
		preflightEnd = end
	}
	return streamRange(ctx, sourceRepo, fileDoc, io.Discard, start, preflightEnd)
}

func preflightFile(ctx context.Context, sourceRepo *repository.SourceRepository, fileDoc *repository.File, total int64, streamRange fileRangeStreamFunc) error {
	return preflightFileRange(ctx, sourceRepo, fileDoc, 0, total-1, streamRange)
}

func setAttachmentDownloadHeaders(c *gin.Context, filename string, total int64) {
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFilename(filename)))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(total, 10))
}
