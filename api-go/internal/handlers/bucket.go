package handlers

import (
	"crypto/rand"
	"math/big"
	"net/http"
	"time"

	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// random string generator
const accessSecretLength = 80

var alphabet = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~")

func generateAccessSecret() string {
	b := make([]rune, accessSecretLength)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alphabet))))
		b[i] = alphabet[n.Int64()]
	}
	return string(b)
}

func CreateBucket(repo *repository.BucketRepository) gin.HandlerFunc {
	type request struct {
		Key string `json:"key" binding:"required"`
	}
	type response struct {
		Key          string    `json:"key"`
		AccessKey    string    `json:"access_key"`
		AccessSecret string    `json:"access_secret"`
		CreatedAt    time.Time `json:"created_at"`
	}

	return func(c *gin.Context) {
		var req request
		if err := c.ShouldBindJSON(&req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		if repo == nil {
			c.Status(http.StatusServiceUnavailable)
			return
		}
		accessKey := req.Key
		accessSecret := generateAccessSecret()
		bucket := repository.Bucket{
			Key:          req.Key,
			AccessKey:    accessKey,
			AccessSecret: accessSecret,
			CreatedAt:    time.Now().UTC(),
		}
		created, err := repo.Create(c.Request.Context(), bucket)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				c.Status(http.StatusConflict)
				return
			}
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, response{
			Key:          created.Key,
			AccessKey:    created.AccessKey,
			AccessSecret: created.AccessSecret,
			CreatedAt:    created.CreatedAt,
		})
	}
}
