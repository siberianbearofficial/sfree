package handlers

import (
	"log"
	"net/http"

	"github.com/allaboutapps/aws4"
	"github.com/example/s3aas/api-go/internal/cryptoutil"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// AWS4Auth validates AWS Signature V4 signed requests using bucket access keys.
func AWS4Auth(repo *repository.BucketRepository, secretKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		accessKey := aws4.AccessKeyIDFromRequest(c.Request)
		if accessKey == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
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
		encSecret, err := repo.GetSecretByAccessKey(c.Request.Context(), accessKey)
		if err != nil {
			if err == mongo.ErrNoDocuments {
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
		signer := aws4.NewSignerWithStaticCredentials(accessKey, secret, "")
		if _, err = signer.Validate(c.Request); err != nil {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Set("accessKey", accessKey)
		c.Next()
	}
}
