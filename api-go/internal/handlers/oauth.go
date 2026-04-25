package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func newGitHubOAuthConfig(cfg *config.Config) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     cfg.GitHubOAuth.ClientID,
		ClientSecret: cfg.GitHubOAuth.ClientSecret,
		RedirectURL:  cfg.GitHubOAuth.RedirectURL,
		Scopes:       []string{"read:user"},
		Endpoint:     github.Endpoint,
	}
}

// GitHubLogin godoc
// @Summary Start GitHub OAuth login
// @Tags auth
// @Success 307 {string} string ""
// @Failure 500 {string} string ""
// @Failure 501 {string} string ""
// @Router /api/v1/auth/github [get]
func GitHubLogin(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cfg.GitHubOAuth.ClientID == "" {
			c.JSON(http.StatusNotImplemented, gin.H{"error": "GitHub OAuth is not configured"})
			return
		}
		oauthCfg := newGitHubOAuthConfig(cfg)
		state, err := generatePassword()
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.SetCookie("oauth_state", state, 600, "/", "", useSecureCookies(cfg, c.Request), true)
		c.Redirect(http.StatusTemporaryRedirect, oauthCfg.AuthCodeURL(state))
	}
}

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

type gitHubOAuthUserRepository interface {
	GetByGitHubID(ctx context.Context, githubID int64) (*repository.User, error)
	Create(ctx context.Context, user repository.User) (*repository.User, error)
}

type gitHubOAuthCallbackDeps struct {
	exchangeCode    func(ctx context.Context, code string) (*oauth2.Token, error)
	fetchGitHubUser func(ctx context.Context, token *oauth2.Token) (*http.Response, error)
	issueJWT        func(userID string, secret string) (string, error)
}

func newGitHubOAuthCallbackDeps(cfg *config.Config) gitHubOAuthCallbackDeps {
	oauthCfg := newGitHubOAuthConfig(cfg)
	return gitHubOAuthCallbackDeps{
		exchangeCode: func(ctx context.Context, code string) (*oauth2.Token, error) {
			return oauthCfg.Exchange(ctx, code)
		},
		fetchGitHubUser: func(ctx context.Context, token *oauth2.Token) (*http.Response, error) {
			return oauthCfg.Client(ctx, token).Get("https://api.github.com/user")
		},
		issueJWT: IssueJWT,
	}
}

// GitHubCallback godoc
// @Summary Complete GitHub OAuth login
// @Tags auth
// @Param code query string true "OAuth authorization code"
// @Param state query string true "OAuth state"
// @Success 307 {string} string ""
// @Failure 400 {string} string ""
// @Failure 500 {string} string ""
// @Router /api/v1/auth/github/callback [get]
func GitHubCallback(cfg *config.Config, userRepo gitHubOAuthUserRepository) gin.HandlerFunc {
	return gitHubCallback(cfg, userRepo, newGitHubOAuthCallbackDeps(cfg))
}

func gitHubCallback(cfg *config.Config, userRepo gitHubOAuthUserRepository, deps gitHubOAuthCallbackDeps) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		savedState, err := c.Cookie("oauth_state")
		if err != nil || savedState == "" || c.Query("state") != savedState {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth state"})
			return
		}
		c.SetCookie("oauth_state", "", -1, "/", "", useSecureCookies(cfg, c.Request), true)

		token, err := deps.exchangeCode(ctx, c.Query("code"))
		if err != nil {
			slog.ErrorContext(ctx, "oauth callback: token exchange failed", slog.String("error", err.Error()))
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to exchange code"})
			return
		}

		resp, err := deps.fetchGitHubUser(ctx, token)
		if err != nil {
			slog.ErrorContext(ctx, "oauth callback: github user fetch failed", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			slog.ErrorContext(ctx, "oauth callback: github user fetch returned non-2xx", slog.Int("status", resp.StatusCode))
			c.Status(http.StatusInternalServerError)
			return
		}

		var ghUser githubUser
		if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
			slog.ErrorContext(ctx, "oauth callback: failed to decode github user", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		// Find or create user by GitHub ID.
		user, err := userRepo.GetByGitHubID(ctx, ghUser.ID)
		if err != nil {
			if !errors.Is(err, mongo.ErrNoDocuments) {
				slog.ErrorContext(ctx, "oauth callback: failed to load github user", slog.String("error", err.Error()))
				c.Status(http.StatusInternalServerError)
				return
			}
			// User not found — create a new one.
			newUser := repository.User{
				Username:  ghUser.Login,
				GitHubID:  ghUser.ID,
				AvatarURL: ghUser.AvatarURL,
				CreatedAt: time.Now().UTC(),
			}
			user, err = userRepo.Create(ctx, newUser)
			if err != nil {
				if !mongo.IsDuplicateKeyError(err) {
					slog.ErrorContext(ctx, "oauth callback: failed to create user", slog.String("error", err.Error()))
					c.Status(http.StatusInternalServerError)
					return
				}
				// Username conflict — try with GitHub ID suffix.
				newUser.Username = fmt.Sprintf("%s-%d", ghUser.Login, ghUser.ID)
				user, err = userRepo.Create(ctx, newUser)
				if err != nil {
					slog.ErrorContext(ctx, "oauth callback: failed to create user", slog.String("error", err.Error()))
					c.Status(http.StatusInternalServerError)
					return
				}
			}
		}

		// Issue JWT.
		jwtSecret := cfg.JWTSecret
		if jwtSecret == "" {
			jwtSecret = cfg.AccessSecretKey
		}
		jwtToken, err := deps.issueJWT(user.ID.Hex(), jwtSecret)
		if err != nil {
			slog.ErrorContext(ctx, "oauth callback: failed to issue jwt", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		frontendURL := cfg.FrontendURL
		if frontendURL == "" {
			frontendURL = ""
		}
		setAuthCookie(c, cfg, jwtToken)

		redirectURL, _ := url.Parse(frontendURL + "/auth/callback")
		c.Redirect(http.StatusTemporaryRedirect, redirectURL.String())
	}
}

// IssueJWT creates a signed JWT for the given user ID.
func IssueJWT(userID string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(authSessionDuration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// TokenLogin godoc
// @Summary Issue auth token
// @Tags auth
// @Success 200 {object} map[string]string
// @Failure 401 {string} string ""
// @Failure 500 {string} string ""
// @Security BasicAuth
// @Router /api/v1/auth/token [post]
func TokenLogin(cfg *config.Config, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		userIDHex := c.GetString("userID")
		if userIDHex == "" {
			c.Status(http.StatusUnauthorized)
			return
		}
		jwtSecret := cfg.JWTSecret
		if jwtSecret == "" {
			jwtSecret = cfg.AccessSecretKey
		}
		token, err := IssueJWT(userIDHex, jwtSecret)
		if err != nil {
			slog.ErrorContext(ctx, "token login: failed to issue jwt", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"token": token})
	}
}
