package handlers

import (
	"log"
	"net/http"
	"time"

	"github.com/example/sfree/api-go/internal/cryptoutil"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

const passwordLength = 12

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
		var req createUserRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			log.Printf("create user: invalid request: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}
		if repo == nil {
			log.Print("create user: repository is nil")
			c.Status(http.StatusServiceUnavailable)
			return
		}
		password, err := generatePassword()
		if err != nil {
			log.Printf("create user: generate password: %v", err)
			c.Status(http.StatusInternalServerError)
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Printf("create user: failed to hash password: %v", err)
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
				log.Printf("create user: username %s already exists: %v", req.Username, err)
				c.Status(http.StatusConflict)
				return
			}
			log.Printf("create user: failed to create user: %v", err)
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
