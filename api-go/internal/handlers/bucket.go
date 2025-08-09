package handlers

import (
	"crypto/rand"
	"log"
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

type createBucketRequest struct {
	Key string `json:"key" binding:"required"`
}

type createBucketResponse struct {
	Key          string    `json:"key"`
	AccessKey    string    `json:"access_key"`
	AccessSecret string    `json:"access_secret"`
	CreatedAt    time.Time `json:"created_at"`
}

// CreateBucket godoc
// @Summary Create bucket
// @Tags buckets
// @Accept json
// @Produce json
// @Param bucket body createBucketRequest true "Bucket to create"
// @Success 200 {object} createBucketResponse
// @Failure 400 {string} string ""
// @Failure 409 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets [post]
func CreateBucket(repo *repository.BucketRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req createBucketRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("create bucket: invalid request: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		if repo == nil {
			log.Print("create bucket: repository is nil")
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
				log.Printf("create bucket: key %s already exists: %v", req.Key, err)
				c.Status(http.StatusConflict)
				return
			}
			log.Printf("create bucket: failed to create bucket: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, createBucketResponse{
			Key:          created.Key,
			AccessKey:    created.AccessKey,
			AccessSecret: created.AccessSecret,
			CreatedAt:    created.CreatedAt,
		})
	}
}
