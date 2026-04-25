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

type fileStreamFunc func(context.Context, *repository.SourceRepository, *repository.File, io.Writer) error

type fileByIDReader interface {
	GetByID(ctx context.Context, id primitive.ObjectID) (*repository.File, error)
}

type shareLinkByTokenReader interface {
	GetByToken(ctx context.Context, token string) (*repository.ShareLink, error)
}

var (
	streamDownloadFile fileStreamFunc = manager.StreamFile
)

type deferredResponseWriter struct {
	c         *gin.Context
	commit    func()
	committed bool
}

func newDeferredResponseWriter(c *gin.Context, commit func()) *deferredResponseWriter {
	return &deferredResponseWriter{c: c, commit: commit}
}

func (w *deferredResponseWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	w.commitNow()
	return w.c.Writer.Write(p)
}

func (w *deferredResponseWriter) commitNow() {
	if w.committed {
		return
	}
	w.commit()
	w.c.Writer.WriteHeaderNow()
	w.committed = true
}

func (w *deferredResponseWriter) isCommitted() bool {
	return w.committed
}

func fileStreamFuncForFactory(factory manager.SourceClientFactory) fileStreamFunc {
	if factory == nil {
		return streamDownloadFile
	}
	return func(ctx context.Context, sourceRepo *repository.SourceRepository, fileDoc *repository.File, w io.Writer) error {
		return manager.StreamFileWithFactory(ctx, sourceRepo, fileDoc, w, factory)
	}
}

func setAttachmentDownloadHeaders(c *gin.Context, filename string, total int64) {
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFilename(filename)))
	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Length", strconv.FormatInt(total, 10))
}

func setArchiveDownloadHeaders(c *gin.Context, filename string) {
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFilename(filename)))
	c.Header("Content-Type", "application/zip")
}
