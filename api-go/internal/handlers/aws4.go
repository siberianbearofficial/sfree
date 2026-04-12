package handlers

import (
	"log"
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
		if repo == nil {
			log.Print("aws4 auth: bucket repository is nil")
			c.AbortWithStatus(http.StatusServiceUnavailable)
			return
		}
		if secretKey == "" {
			log.Print("aws4 auth: secret key is empty")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		accessKey, err := s3sigv4.AccessKeyFromRequest(c.Request)
		if err != nil || accessKey == "" {
			log.Printf("aws4 auth: failed to extract access key: %v", err)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		encSecret, err := repo.GetSecretByAccessKey(c.Request.Context(), accessKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				log.Print("aws4 auth: get secret by access key lead to ErrNoDocuments")
				c.AbortWithStatus(http.StatusUnauthorized)
				return
			}
			log.Printf("aws4 auth: failed to get secret: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}
		secret, err := cryptoutil.Decrypt(encSecret, secretKey)
		if err != nil {
			log.Printf("aws4 auth: decrypt secret: %v", err)
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if _, err = validator.Validate(c.Request.Context(), c.Request, accessKey, secret); err != nil {
			log.Printf("aws4 auth: signature validation failed: %v", err)
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		log.Print("aws4 auth success")
		c.Set("accessKey", accessKey)
		c.Next()
	}
}
