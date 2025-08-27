package handlers

import (
	"bytes"
	"crypto/rand"
	"io"
	"log"
	"math/big"
	"net/http"
	"time"

	"github.com/example/s3aas/api-go/internal/gdrive"
	"github.com/example/s3aas/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

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

type bucketResponse struct {
	ID           string    `json:"id"`
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
// @Failure 401 {string} string ""
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

// ListBuckets godoc
// @Summary List buckets
// @Tags buckets
// @Produce json
// @Success 200 {array} bucketResponse
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets [get]
func ListBuckets(repo *repository.BucketRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			log.Print("list buckets: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		buckets, err := repo.List(c.Request.Context())
		if err != nil {
			log.Printf("list buckets: failed to list buckets: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		resp := make([]bucketResponse, 0, len(buckets))
		for _, b := range buckets {
			resp = append(resp, bucketResponse{
				ID:           b.ID.Hex(),
				Key:          b.Key,
				AccessKey:    b.AccessKey,
				AccessSecret: b.AccessSecret,
				CreatedAt:    b.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, resp)
	}
}

// DeleteBucket godoc
// @Summary Delete bucket
// @Tags buckets
// @Param id path string true "Bucket ID"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id} [delete]
func DeleteBucket(repo *repository.BucketRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if repo == nil {
			log.Print("delete bucket: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		idHex := c.Param("id")
		id, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		if err := repo.Delete(c.Request.Context(), id); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("delete bucket: failed to delete: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}

type uploadFileResponse struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// UploadFile godoc
// @Summary Upload file to bucket
// @Tags buckets
// @Accept multipart/form-data
// @Produce json
// @Param id path string true "Bucket ID"
// @Param file formData file true "File to upload"
// @Success 200 {object} uploadFileResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/upload [post]
func UploadFile(bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository, chunkSize int) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			log.Print("upload file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		idHex := c.Param("id")
		bucketID, err := primitive.ObjectIDFromHex(idHex)
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		if _, err := bucketRepo.GetByID(c.Request.Context(), bucketID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("upload file: get bucket: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		userID, err := primitive.ObjectIDFromHex(userIDHex)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		sources, err := sourceRepo.ListByUser(c.Request.Context(), userID)
		if err != nil {
			log.Printf("upload file: list sources: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if len(sources) == 0 {
			c.Status(http.StatusBadRequest)
			return
		}
		fh, err := c.FormFile("file")
		if err != nil {
			log.Printf("upload file: get file: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		f, err := fh.Open()
		if err != nil {
			log.Printf("upload file: open file: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		defer func() { _ = f.Close() }()

		ctx := c.Request.Context()
		clients := make([]*gdrive.Client, len(sources))
		chunks := make([]repository.FileChunk, 0)
		buf := make([]byte, chunkSize)
		idx := 0
		for {
			n, err := f.Read(buf)
			if err != nil && err != io.EOF {
				log.Printf("upload file: read chunk: %v", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			if n == 0 {
				break
			}
			src := sources[idx%len(sources)]
			if clients[idx%len(sources)] == nil {
				cli, err := gdrive.NewClient(ctx, []byte(src.Key))
				if err != nil {
					log.Printf("upload file: create gdrive client: %v", err)
					c.Status(http.StatusInternalServerError)
					return
				}
				clients[idx%len(sources)] = cli
			}
			name := primitive.NewObjectID().Hex()
			driveID, err := clients[idx%len(sources)].Upload(ctx, name, bytes.NewReader(buf[:n]))
			if err != nil {
				log.Printf("upload file: upload chunk: %v", err)
				c.Status(http.StatusInternalServerError)
				return
			}
			chunks = append(chunks, repository.FileChunk{
				SourceID: src.ID,
				Name:     driveID,
				Order:    idx,
				Size:     int64(n),
			})
			idx++
			if err == io.EOF {
				break
			}
		}

		fileDoc := repository.File{
			BucketID:  bucketID,
			Name:      fh.Filename,
			CreatedAt: time.Now().UTC(),
			Chunks:    chunks,
		}
		created, err := fileRepo.Create(ctx, fileDoc)
		if err != nil {
			log.Printf("upload file: save file: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, uploadFileResponse{
			ID:        created.ID.Hex(),
			Name:      created.Name,
			CreatedAt: created.CreatedAt,
		})
	}
}
