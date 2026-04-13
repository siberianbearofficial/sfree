package handlers

import (
	"log/slog"
	"net/http"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/example/sfree/api-go/internal/s3sigv4"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// AWS4Auth validates AWS Signature V4 signed requests using bucket access keys.
func AWS4Auth(repo *repository.BucketRepository, secretKey string) gin.HandlerFunc {
	validator := &s3sigv4.Validator{}

	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if repo == nil {
			slog.ErrorContext(ctx, "aws4 auth: bucket repository is nil")
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if secretKey == "" {
			slog.ErrorContext(ctx, "aws4 auth: secret key is empty")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		accessKey, err := s3sigv4.AccessKeyFromRequest(c.Request)
		if err != nil || accessKey == "" {
			slog.WarnContext(ctx, "aws4 auth: failed to extract access key", slog.String("error", errString(err)))
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		encSecret, err := repo.GetSecretByAccessKey(c.Request.Context(), accessKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				slog.WarnContext(ctx, "aws4 auth: unknown access key")
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			slog.ErrorContext(ctx, "aws4 auth: failed to get secret", slog.String("error", err.Error()))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		secret, err := cryptoutil.Decrypt(encSecret, secretKey)
		if err != nil {
			slog.ErrorContext(ctx, "aws4 auth: decrypt secret", slog.String("error", err.Error()))
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if _, err = validator.Validate(c.Request.Context(), c.Request, accessKey, secret); err != nil {
			slog.WarnContext(ctx, "aws4 auth: signature validation failed", slog.String("error", err.Error()))
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		slog.DebugContext(ctx, "aws4 auth success")
		c.Set("accessKey", accessKey)
		c.Next()
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
