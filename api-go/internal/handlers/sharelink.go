package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

const shareTokenLength = 32

type createShareRequest struct {
	ExpiresIn *int `json:"expires_in"` // seconds until expiry, optional
}

type shareLinkCreator interface {
	Create(ctx context.Context, sl repository.ShareLink) (*repository.ShareLink, error)
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
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil || shareLinkRepo == nil {
			slog.ErrorContext(ctx, "create share link: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		var grantReader bucketAccessGrantReader
		if grantRepo != nil {
			grantReader = grantRepo
		}
		createShareLink(bucketRepo, fileRepo, shareLinkRepo, grantReader)(c)
	}
}

func createShareLink(bucketRepo bucketAccessBucketReader, fileRepo fileByIDReader, shareLinkRepo shareLinkCreator, grantRepo bucketAccessGrantReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil || shareLinkRepo == nil {
			slog.ErrorContext(ctx, "create share link: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccessFor(c, bucketRepo, grantRepo, repository.RoleEditor)
		if acc == nil {
			return
		}
		bucketID := acc.Bucket.ID

		fileID, ok := routeObjectID(c, "file_id")
		if !ok {
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}

		// Verify file exists in bucket.
		fileDoc, err := fileRepo.GetByID(ctx, fileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "create share link: get file", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if fileDoc.BucketID != bucketID {
			c.Status(http.StatusNotFound)
			return
		}

		var req createShareRequest
		if err := decodeOptionalCreateShareRequest(c, &req); err != nil {
			c.Status(http.StatusBadRequest)
			return
		}

		token, err := cryptoutil.RandomString(shareTokenLength)
		if err != nil {
			slog.ErrorContext(ctx, "create share link: generate token", slog.String("error", err.Error()))
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

		created, err := shareLinkRepo.Create(ctx, sl)
		if err != nil {
			slog.ErrorContext(ctx, "create share link: save", slog.String("error", err.Error()))
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

func decodeOptionalCreateShareRequest(c *gin.Context, req *createShareRequest) error {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return err
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	return json.Unmarshal(body, req)
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
		ctx := c.Request.Context()
		if shareLinkRepo == nil || bucketRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "get shared file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		getSharedFile(shareLinkRepo, sourceRepo, fileRepo)(c)
	}
}

func getSharedFile(shareLinkRepo shareLinkByTokenReader, sourceRepo *repository.SourceRepository, fileRepo fileByIDReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		if shareLinkRepo == nil || sourceRepo == nil || fileRepo == nil {
			slog.ErrorContext(ctx, "get shared file: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		token := c.Param("token")
		if token == "" {
			c.Status(http.StatusNotFound)
			return
		}

		sl, err := shareLinkRepo.GetByToken(ctx, token)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "get shared file: lookup token", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		// Check expiry.
		if sl.ExpiresAt != nil && time.Now().UTC().After(*sl.ExpiresAt) {
			c.Status(http.StatusGone)
			return
		}

		fileDoc, err := fileRepo.GetByID(ctx, sl.FileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "get shared file: get file", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		total := fileContentLength(fileDoc)
		if err := preflightFile(ctx, sourceRepo, fileDoc, total, streamDownloadFileRange); err != nil {
			slog.ErrorContext(ctx, "get shared file: stream", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		setAttachmentDownloadHeaders(c, fileDoc.Name, total)
		c.Status(http.StatusOK)
		if err := streamDownloadFile(ctx, sourceRepo, fileDoc, c.Writer); err != nil {
			slog.ErrorContext(ctx, "get shared file: stream failed after response commit", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if bucketRepo == nil || fileRepo == nil || shareLinkRepo == nil {
			slog.ErrorContext(ctx, "list share links: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}

		acc := requireBucketAccess(c, bucketRepo, grantRepo, repository.RoleEditor)
		if acc == nil {
			return
		}
		bucketID := acc.Bucket.ID

		fileID, ok := routeObjectID(c, "file_id")
		if !ok {
			return
		}

		fileDoc, err := fileRepo.GetByID(ctx, fileID)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "list share links: get file", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		if fileDoc.BucketID != bucketID {
			c.Status(http.StatusNotFound)
			return
		}

		links, err := shareLinkRepo.ListByFile(ctx, fileID)
		if err != nil {
			slog.ErrorContext(ctx, "list share links: list", slog.String("error", err.Error()))
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
		ctx := c.Request.Context()
		if shareLinkRepo == nil {
			slog.ErrorContext(ctx, "delete share link: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		id, ok := routeObjectID(c, "id")
		if !ok {
			return
		}
		userID, ok := authenticatedUserID(c)
		if !ok {
			return
		}

		if err := shareLinkRepo.Delete(ctx, id, userID); err != nil {
			if err == mongo.ErrNoDocuments {
				c.Status(http.StatusNotFound)
				return
			}
			slog.ErrorContext(ctx, "delete share link", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	}
}
