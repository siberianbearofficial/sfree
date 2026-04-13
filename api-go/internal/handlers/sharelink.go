package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/manager"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

const shareTokenLength = 32

type createShareRequest struct {
	ExpiresIn *int `json:"expires_in"` // seconds until expiry, optional
}

type shareLinkResponse struct {
	ID        string     `json:"id"`
	FileID    string     `json:"file_id"`
	FileName  string     `json:"file_name"`
	Token     string     `json:"token"`
	URL       string     `json:"url"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// CreateShareLink godoc
// @Summary Create a public share link for a file
// @Tags shares
// @Accept json
// @Produce json
// @Param id path string true "Bucket ID"
// @Param file_id path string true "File ID"
// @Param body body createShareRequest false "Share options"
// @Success 200 {object} shareLinkResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files/{file_id}/share [post]
func CreateShareLink(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || fileRepo == nil || shareLinkRepo == nil {
			log.Print("create share link: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleEditor)
		if acc == nil {
			return
		}
		bucketID := acc.Bucket.ID

		fileID, err := primitive.ObjectIDFromHex(c.Param("file_id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		userIDHex := c.GetString("userID")
		userID, _ := primitive.ObjectIDFromHex(userIDHex)

		// Verify file exists in bucket.
		fileDoc, err := fileRepo.GetByID(c.Request.Context(), fileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("create share link: get file: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if fileDoc.BucketID != bucketID {
			c.Status(http.StatusNotFound)
			return
		}

		var req createShareRequest
		// Body is optional; ignore bind errors for empty body.
		_ = c.ShouldBindJSON(&req)

		token, err := cryptoutil.RandomString(shareTokenLength)
		if err != nil {
			log.Printf("create share link: generate token: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}

		now := time.Now().UTC()
		sl := repository.ShareLink{
			FileID:    fileID,
			BucketID:  bucketID,
			UserID:    userID,
			Token:     token,
			CreatedAt: now,
		}
		if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
			exp := now.Add(time.Duration(*req.ExpiresIn) * time.Second)
			sl.ExpiresAt = &exp
		}

		created, err := shareLinkRepo.Create(c.Request.Context(), sl)
		if err != nil {
			log.Printf("create share link: save: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}

		c.JSON(http.StatusOK, shareLinkResponse{
			ID:        created.ID.Hex(),
			FileID:    created.FileID.Hex(),
			FileName:  fileDoc.Name,
			Token:     created.Token,
			URL:       fmt.Sprintf("/share/%s", created.Token),
			ExpiresAt: created.ExpiresAt,
			CreatedAt: created.CreatedAt,
		})
	}
}

// GetSharedFile godoc
// @Summary Download a publicly shared file
// @Tags shares
// @Produce octet-stream
// @Param token path string true "Share token"
// @Success 200 {file} file
// @Failure 404 {string} string ""
// @Failure 410 {string} string ""
// @Failure 500 {string} string ""
// @Router /share/{token} [get]
func GetSharedFile(shareLinkRepo *repository.ShareLinkRepository, bucketRepo *repository.BucketRepository, sourceRepo *repository.SourceRepository, fileRepo *repository.FileRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shareLinkRepo == nil || bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			log.Print("get shared file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		token := c.Param("token")
		if token == "" {
			c.Status(http.StatusNotFound)
			return
		}

		sl, err := shareLinkRepo.GetByToken(c.Request.Context(), token)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("get shared file: lookup token: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}

		// Check expiry.
		if sl.ExpiresAt != nil && time.Now().UTC().After(*sl.ExpiresAt) {
			c.Status(http.StatusGone)
			return
		}

		fileDoc, err := fileRepo.GetByID(c.Request.Context(), sl.FileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("get shared file: get file: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}

		var total int64
		for _, ch := range fileDoc.Chunks {
			total += ch.Size
		}
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", sanitizeFilename(fileDoc.Name)))
		c.Header("Content-Type", "application/octet-stream")
		c.Header("Content-Length", strconv.FormatInt(total, 10))
		c.Status(http.StatusOK)
		if err := manager.StreamFile(c.Request.Context(), sourceRepo, fileDoc, c.Writer); err != nil {
			log.Printf("get shared file: stream: %v", err)
		}
	}
}

// ListShareLinks godoc
// @Summary List share links for a file
// @Tags shares
// @Produce json
// @Param id path string true "Bucket ID"
// @Param file_id path string true "File ID"
// @Success 200 {array} shareLinkResponse
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/buckets/{id}/files/{file_id}/shares [get]
func ListShareLinks(bucketRepo *repository.BucketRepository, fileRepo *repository.FileRepository, shareLinkRepo *repository.ShareLinkRepository, grantRepo *repository.BucketGrantRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bucketRepo == nil || fileRepo == nil || shareLinkRepo == nil {
			log.Print("list share links: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleEditor)
		if acc == nil {
			return
		}
		bucketID := acc.Bucket.ID

		fileID, err := primitive.ObjectIDFromHex(c.Param("file_id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		fileDoc, err := fileRepo.GetByID(c.Request.Context(), fileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("list share links: get file: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		if fileDoc.BucketID != bucketID {
			c.Status(http.StatusNotFound)
			return
		}

		links, err := shareLinkRepo.ListByFile(c.Request.Context(), fileID)
		if err != nil {
			log.Printf("list share links: list: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}

		resp := make([]shareLinkResponse, 0, len(links))
		for _, sl := range links {
			resp = append(resp, shareLinkResponse{
				ID:        sl.ID.Hex(),
				FileID:    sl.FileID.Hex(),
				FileName:  fileDoc.Name,
				Token:     sl.Token,
				URL:       fmt.Sprintf("/share/%s", sl.Token),
				ExpiresAt: sl.ExpiresAt,
				CreatedAt: sl.CreatedAt,
			})
		}
		c.JSON(http.StatusOK, resp)
	}
}

// DeleteShareLink godoc
// @Summary Revoke a share link
// @Tags shares
// @Param id path string true "Share link ID"
// @Success 200 {string} string ""
// @Failure 400 {string} string ""
// @Failure 401 {string} string ""
// @Failure 404 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/shares/{id} [delete]
func DeleteShareLink(shareLinkRepo *repository.ShareLinkRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		if shareLinkRepo == nil {
			log.Print("delete share link: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		id, err := primitive.ObjectIDFromHex(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
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

		if err := shareLinkRepo.Delete(c.Request.Context(), id, userID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			log.Printf("delete share link: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}
