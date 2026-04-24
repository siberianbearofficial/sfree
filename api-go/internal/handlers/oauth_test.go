package handlers

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/example/sfree/api-go/internal/config"
	"github.com/example/sfree/api-go/internal/repository"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/oauth2"
)

type stubGitHubOAuthUserRepo struct {
	getByGitHubIDFunc func(context.Context, int64) (*repository.User, error)
	createFunc        func(context.Context, repository.User) (*repository.User, error)
	createCalls       []repository.User
}

func (s *stubGitHubOAuthUserRepo) GetByGitHubID(ctx context.Context, githubID int64) (*repository.User, error) {
	if s.getByGitHubIDFunc == nil {
		return nil, mongo.ErrNoDocuments
	}
	return s.getByGitHubIDFunc(ctx, githubID)
}

func (s *stubGitHubOAuthUserRepo) Create(ctx context.Context, user repository.User) (*repository.User, error) {
	s.createCalls = append(s.createCalls, user)
	if s.createFunc == nil {
		if user.ID.IsZero() {
			user.ID = primitive.NewObjectID()
		}
		return &user, nil
	}
	return s.createFunc(ctx, user)
}

func TestGitHubCallbackRejectsInvalidState(t *testing.T) {
	t.Parallel()

	exchangeCalled := false
	w := serveOAuthCallback(t, &config.Config{}, &stubGitHubOAuthUserRepo{}, gitHubOAuthCallbackDeps{
		exchangeCode: func(context.Context, string) (*oauth2.Token, error) {
			exchangeCalled = true
			return nil, nil
		},
		fetchGitHubUser: func(context.Context, *oauth2.Token) (*http.Response, error) {
			t.Fatal("fetchGitHubUser should not be called")
			return nil, nil
		},
		issueJWT: func(string, string) (string, error) {
			t.Fatal("issueJWT should not be called")
			return "", nil
		},
	}, oauthCallbackRequest("/api/v1/auth/github/callback?code=test-code&state=wrong", "expected-state"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if exchangeCalled {
		t.Fatal("exchangeCode should not be called")
	}
}

func TestGitHubCallbackReturnsBadRequestOnExchangeFailure(t *testing.T) {
	t.Parallel()

	fetchCalled := false
	w := serveOAuthCallback(t, &config.Config{}, &stubGitHubOAuthUserRepo{}, gitHubOAuthCallbackDeps{
		exchangeCode: func(context.Context, string) (*oauth2.Token, error) {
			return nil, errors.New("exchange failed")
		},
		fetchGitHubUser: func(context.Context, *oauth2.Token) (*http.Response, error) {
			fetchCalled = true
			return nil, nil
		},
		issueJWT: func(string, string) (string, error) {
			t.Fatal("issueJWT should not be called")
			return "", nil
		},
	}, oauthCallbackRequest("/api/v1/auth/github/callback?code=test-code&state=expected-state", "expected-state"))

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if fetchCalled {
		t.Fatal("fetchGitHubUser should not be called after exchange failure")
	}
}

func TestGitHubCallbackRejectsNon2xxGitHubUserResponse(t *testing.T) {
	t.Parallel()

	repo := &stubGitHubOAuthUserRepo{}
	w := serveOAuthCallback(t, &config.Config{}, repo, gitHubOAuthCallbackDeps{
		exchangeCode: func(context.Context, string) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "token"}, nil
		},
		fetchGitHubUser: func(context.Context, *oauth2.Token) (*http.Response, error) {
			return oauthUserResponse(http.StatusForbidden, `{"id":99,"login":"ignored"}`), nil
		},
		issueJWT: func(string, string) (string, error) {
			t.Fatal("issueJWT should not be called")
			return "", nil
		},
	}, oauthCallbackRequest("/api/v1/auth/github/callback?code=test-code&state=expected-state", "expected-state"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if len(repo.createCalls) != 0 {
		t.Fatalf("expected no create calls, got %d", len(repo.createCalls))
	}
}

func TestGitHubCallbackRejectsInvalidGitHubUserJSON(t *testing.T) {
	t.Parallel()

	repo := &stubGitHubOAuthUserRepo{}
	getByIDCalled := false
	repo.getByGitHubIDFunc = func(context.Context, int64) (*repository.User, error) {
		getByIDCalled = true
		return nil, nil
	}

	w := serveOAuthCallback(t, &config.Config{}, repo, gitHubOAuthCallbackDeps{
		exchangeCode: func(context.Context, string) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "token"}, nil
		},
		fetchGitHubUser: func(context.Context, *oauth2.Token) (*http.Response, error) {
			return oauthUserResponse(http.StatusOK, `{`), nil
		},
		issueJWT: func(string, string) (string, error) {
			t.Fatal("issueJWT should not be called")
			return "", nil
		},
	}, oauthCallbackRequest("/api/v1/auth/github/callback?code=test-code&state=expected-state", "expected-state"))

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
	if getByIDCalled {
		t.Fatal("GetByGitHubID should not be called for invalid user JSON")
	}
	if len(repo.createCalls) != 0 {
		t.Fatalf("expected no create calls, got %d", len(repo.createCalls))
	}
}

func TestGitHubCallbackExistingUserSetsCookieAndRedirects(t *testing.T) {
	t.Parallel()

	existingUser := &repository.User{
		ID:        primitive.NewObjectID(),
		Username:  "existing-user",
		GitHubID:  42,
		AvatarURL: "https://avatars.example/existing.png",
	}
	repo := &stubGitHubOAuthUserRepo{
		getByGitHubIDFunc: func(context.Context, int64) (*repository.User, error) {
			return existingUser, nil
		},
	}

	w := serveOAuthCallback(t, testOAuthConfig(), repo, gitHubOAuthCallbackDeps{
		exchangeCode: func(context.Context, string) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "token"}, nil
		},
		fetchGitHubUser: func(context.Context, *oauth2.Token) (*http.Response, error) {
			return oauthUserResponse(http.StatusOK, `{"id":42,"login":"octocat","avatar_url":"https://avatars.example/octocat.png"}`), nil
		},
		issueJWT: func(userID, secret string) (string, error) {
			if userID != existingUser.ID.Hex() {
				t.Fatalf("issueJWT userID = %q, want %q", userID, existingUser.ID.Hex())
			}
			if secret != "jwt-secret" {
				t.Fatalf("issueJWT secret = %q, want %q", secret, "jwt-secret")
			}
			return "signed-token", nil
		},
	}, oauthCallbackRequest("/api/v1/auth/github/callback?code=test-code&state=expected-state", "expected-state"))

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", w.Code)
	}
	if len(repo.createCalls) != 0 {
		t.Fatalf("expected no create calls, got %d", len(repo.createCalls))
	}

	authCookie := findCookie(t, w.Result(), "auth_token")
	if authCookie.Value != "signed-token" {
		t.Fatalf("auth_token value = %q, want %q", authCookie.Value, "signed-token")
	}
	if !authCookie.HttpOnly {
		t.Fatal("auth_token should be HttpOnly")
	}
	if !authCookie.Secure {
		t.Fatal("auth_token should be Secure")
	}
	if authCookie.SameSite != http.SameSiteLaxMode {
		t.Fatalf("auth_token SameSite = %v, want %v", authCookie.SameSite, http.SameSiteLaxMode)
	}
	if authCookie.MaxAge != 7*24*3600 {
		t.Fatalf("auth_token MaxAge = %d, want %d", authCookie.MaxAge, 7*24*3600)
	}

	redirectURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if redirectURL.String() != "https://frontend.example/auth/callback?username=existing-user" {
		t.Fatalf("redirect location = %q", redirectURL.String())
	}
	if redirectURL.Query().Get("auth_token") != "" {
		t.Fatal("redirect URL should not include auth_token")
	}
}

func TestGitHubCallbackUsernameConflictFallsBackToGitHubIDSuffix(t *testing.T) {
	t.Parallel()

	createdUser := &repository.User{
		ID:       primitive.NewObjectID(),
		Username: "octocat-42",
		GitHubID: 42,
	}
	repo := &stubGitHubOAuthUserRepo{
		getByGitHubIDFunc: func(context.Context, int64) (*repository.User, error) {
			return nil, mongo.ErrNoDocuments
		},
	}
	createAttempt := 0
	repo.createFunc = func(_ context.Context, user repository.User) (*repository.User, error) {
		createAttempt++
		if createAttempt == 1 {
			return nil, mongo.WriteException{
				WriteErrors: []mongo.WriteError{{Code: 11000, Message: "duplicate key"}},
			}
		}
		createdUser.AvatarURL = user.AvatarURL
		return createdUser, nil
	}

	w := serveOAuthCallback(t, testOAuthConfig(), repo, gitHubOAuthCallbackDeps{
		exchangeCode: func(context.Context, string) (*oauth2.Token, error) {
			return &oauth2.Token{AccessToken: "token"}, nil
		},
		fetchGitHubUser: func(context.Context, *oauth2.Token) (*http.Response, error) {
			return oauthUserResponse(http.StatusOK, `{"id":42,"login":"octocat","avatar_url":"https://avatars.example/octocat.png"}`), nil
		},
		issueJWT: func(userID, secret string) (string, error) {
			if userID != createdUser.ID.Hex() {
				t.Fatalf("issueJWT userID = %q, want %q", userID, createdUser.ID.Hex())
			}
			if secret != "jwt-secret" {
				t.Fatalf("issueJWT secret = %q, want %q", secret, "jwt-secret")
			}
			return "signed-token", nil
		},
	}, oauthCallbackRequest("/api/v1/auth/github/callback?code=test-code&state=expected-state", "expected-state"))

	if w.Code != http.StatusTemporaryRedirect {
		t.Fatalf("expected 307, got %d", w.Code)
	}
	if len(repo.createCalls) != 2 {
		t.Fatalf("expected 2 create calls, got %d", len(repo.createCalls))
	}
	if repo.createCalls[0].Username != "octocat" {
		t.Fatalf("first username = %q, want %q", repo.createCalls[0].Username, "octocat")
	}
	if repo.createCalls[1].Username != "octocat-42" {
		t.Fatalf("second username = %q, want %q", repo.createCalls[1].Username, "octocat-42")
	}

	redirectURL, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatalf("parse redirect location: %v", err)
	}
	if redirectURL.Query().Get("username") != "octocat-42" {
		t.Fatalf("redirect username = %q, want %q", redirectURL.Query().Get("username"), "octocat-42")
	}
}

func serveOAuthCallback(t *testing.T, cfg *config.Config, repo gitHubOAuthUserRepository, deps gitHubOAuthCallbackDeps, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()

	router := gin.New()
	router.GET("/api/v1/auth/github/callback", gitHubCallback(cfg, repo, deps))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func oauthCallbackRequest(target string, state string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if state != "" {
		req.AddCookie(&http.Cookie{Name: "oauth_state", Value: state})
	}
	return req
}

func oauthUserResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func findCookie(t *testing.T, resp *http.Response, name string) *http.Cookie {
	t.Helper()

	for _, cookie := range resp.Cookies() {
		if cookie.Name == name {
			return cookie
		}
	}
	t.Fatalf("cookie %q not found", name)
	return nil
}

func testOAuthConfig() *config.Config {
	return &config.Config{
		JWTSecret:   "jwt-secret",
		FrontendURL: "https://frontend.example",
	}
}
