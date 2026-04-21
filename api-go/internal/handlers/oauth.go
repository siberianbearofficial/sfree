package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
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

// GitHubLogin redirects the user to GitHub's OAuth consent page.
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
		c.SetCookie("oauth_state", state, 600, "/", "", true, true)
		c.Redirect(http.StatusTemporaryRedirect, oauthCfg.AuthCodeURL(state))
	}
}

type githubUser struct {
	ID        int64  `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

// GitHubCallback handles the OAuth callback from GitHub.
func GitHubCallback(cfg *config.Config, userRepo *repository.UserRepository) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx := c.Request.Context()
		savedState, err := c.Cookie("oauth_state")
		if err != nil || savedState == "" || c.Query("state") != savedState {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid oauth state"})
			return
		}
		// Clear the state cookie immediately to prevent replay attacks.
		c.SetCookie("oauth_state", "", -1, "/", "", true, true)

		oauthCfg := newGitHubOAuthConfig(cfg)
		token, err := oauthCfg.Exchange(ctx, c.Query("code"))
		if err != nil {
			slog.ErrorContext(ctx, "oauth callback: token exchange failed", slog.String("error", err.Error()))
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to exchange code"})
			return
		}

		client := oauthCfg.Client(ctx, token)
		resp, err := client.Get("https://api.github.com/user")
		if err != nil {
			slog.ErrorContext(ctx, "oauth callback: github user fetch failed", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		var ghUser githubUser
		if err := json.NewDecoder(resp.Body).Decode(&ghUser); err != nil {
			slog.ErrorContext(ctx, "oauth callback: failed to decode github user", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		// Find or create user by GitHub ID.
		user, err := userRepo.GetByGitHubID(ctx, ghUser.ID)
		if err != nil {
			// User not found — create a new one.
			newUser := repository.User{
				Username:  ghUser.Login,
				GitHubID:  ghUser.ID,
				AvatarURL: ghUser.AvatarURL,
				CreatedAt: time.Now().UTC(),
			}
			user, err = userRepo.Create(ctx, newUser)
			if err != nil {
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
		jwtToken, err := IssueJWT(user.ID.Hex(), jwtSecret)
		if err != nil {
			slog.ErrorContext(ctx, "oauth callback: failed to issue jwt", slog.String("error", err.Error()))
			c.Status(http.StatusInternalServerError)
			return
		}

		// Set JWT as HttpOnly cookie instead of URL query parameter to
		// prevent token leakage via logs, browser history, and Referer headers.
		frontendURL := cfg.FrontendURL
		if frontendURL == "" {
			frontendURL = ""
		}
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie("auth_token", jwtToken, 7*24*3600, "/", "", true, true)

		redirectURL, _ := url.Parse(frontendURL + "/auth/callback")
		q := redirectURL.Query()
		q.Set("username", user.Username)
		redirectURL.RawQuery = q.Encode()

		c.Redirect(http.StatusTemporaryRedirect, redirectURL.String())
	}
}

// IssueJWT creates a signed JWT for the given user ID.
func IssueJWT(userID string, secret string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// TokenLogin issues a JWT for an existing Basic Auth session.
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
