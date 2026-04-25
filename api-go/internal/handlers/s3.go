package handlers

import (
	"context"
	"encoding/xml"
	"errors"
	"log/slog"
	"net/http"

	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type s3Error struct {
	XMLName xml.Name `xml:"Error"`
	Code    string   `xml:"Code"`
	Message string   `xml:"Message"`
}

type objectBucketReader interface {
	GetByKey(ctx context.Context, key string) (*repository.Bucket, error)
}

func writeS3Error(c *gin.Context, status int, code, message string) {
	c.XML(status, s3Error{Code: code, Message: message})
}

func isBucketSourceResolutionError(err error) bool {
	return errors.Is(err, manager.ErrNoSources) || errors.Is(err, repository.ErrSourcesNotFound)
}

func lookupBucket(c *gin.Context, bucketRepo objectBucketReader) (*repository.Bucket, bool) {
	ctx := c.Request.Context()
	bucketDoc, err := bucketRepo.GetByKey(ctx, S3BucketKey(c))
	if err != nil {
		if err == mongo.ErrNoDocuments {
			writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
			return nil, false
		}
		slog.ErrorContext(ctx, "lookup bucket", slog.String("error", err.Error()))
		writeS3Error(c, http.StatusInternalServerError, "InternalError", "")
		return nil, false
	}
	accessKey := c.GetString("accessKey")
	if accessKey == "" || bucketDoc.AccessKey != accessKey {
		writeS3Error(c, http.StatusNotFound, "NoSuchBucket", "")
		return nil, false
	}
	return bucketDoc, true
}
