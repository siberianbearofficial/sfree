package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const passwordLength = 12

type currentUserResponse struct {
	ID        string `json:"id"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url,omitempty"`
	GitHubID  int64  `json:"github_id,omitempty"`
}

// GetCurrentUser godoc
// @Summary Get current user
// @Tags auth
// @Success 200 {object} currentUserResponse
// @Failure 401 {string} string ""
// @Security BasicAuth
// @Router /api/v1/auth/me [get]
func GetCurrentUser(repo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		oid, ok := authenticatedUserID(c)
		if !ok {
			return
		}
		user, err := repo.GetByID(c.Request.Context(), oid)
		if err != nil {
			c.Status(http.StatusUnauthorized)
			return
		}
		c.JSON(http.StatusOK, currentUserResponse{
			ID:        user.ID.Hex(),
			Username:  user.Username,
			AvatarURL: user.AvatarURL,
			GitHubID:  user.GitHubID,
		})
	}
}

func generatePassword() (string, error) {
	return cryptoutil.RandomString(passwordLength)
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
}

type createUserResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	Password  string    `json:"password"`
}

// CreateUser godoc
// @Summary Create user
// @Tags users
// @Accept json
// @Produce json
// @Param user body createUserRequest true "User to create"
// @Success 200 {object} createUserResponse
// @Failure 400 {string} string ""
// @Failure 409 {string} string ""
// @Router /api/v1/users [post]
func CreateUser(repo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		var req createUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			slog.WarnContext(ctx, "create user: invalid request", slog.String("error", err.Error()))
			c.Status(http.StatusBadRequest)
			return
		}
		if repo == nil {
			slog.ErrorContext(ctx, "create user: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		password, err := generatePassword()
		if err != nil {
			slog.ErrorContext(ctx, "create user: generate password", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.ErrorContext(ctx, "create user: failed to hash password", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		user := repository.User{
			Username:     req.Username,
			PasswordHash: string(hash),
			CreatedAt:    time.Now().UTC(),
		}
		created, err := repo.Create(c.Request.Context(), user)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				slog.WarnContext(ctx, "create user: username already exists", slog.String("username", req.Username))
				c.Status(http.StatusConflict)
				return
			}
			slog.ErrorContext(ctx, "create user: failed to create user", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, createUserResponse{
			ID:        created.ID.Hex(),
			CreatedAt: created.CreatedAt,
			Password:  password,
		})
	}
}
