package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dev-platform/backend/internal/config"
	"github.com/dev-platform/backend/internal/db"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
	"golang.org/x/oauth2/google"
)

const sessionCookie = "session_id"
const sessionTTL = 7 * 24 * time.Hour

type contextKey string

const UserContextKey contextKey = "user"

type Service struct {
	cfg    *config.Config
	store  *db.Store
	redis  *redis.Client
	oauth  map[string]*oauth2.Config
	states map[string]string
}

func NewService(cfg *config.Config, store *db.Store, redisClient *redis.Client) *Service {
	gitlabBase := strings.TrimSuffix(cfg.GitLabBaseURL, "/")
	s := &Service{
		cfg:    cfg,
		store:  store,
		redis:  redisClient,
		oauth:  make(map[string]*oauth2.Config),
		states: make(map[string]string),
	}
	if cfg.GitHubClientID != "" && cfg.GitHubClientSecret != "" {
		s.oauth["github"] = &oauth2.Config{
			ClientID:     cfg.GitHubClientID,
			ClientSecret: cfg.GitHubClientSecret,
			RedirectURL:  cfg.OAuthCallbackBase + "/api/v1/auth/github/callback",
			Endpoint:     github.Endpoint,
			Scopes:       []string{"read:user", "user:email"},
		}
	}
	if cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "" {
		s.oauth["google"] = &oauth2.Config{
			ClientID:     cfg.GoogleClientID,
			ClientSecret: cfg.GoogleClientSecret,
			RedirectURL:  cfg.OAuthCallbackBase + "/api/v1/auth/google/callback",
			Endpoint:     google.Endpoint,
			Scopes:       []string{"openid", "email", "profile"},
		}
	}
	if cfg.GitLabClientID != "" && cfg.GitLabClientSecret != "" {
		s.oauth["gitlab"] = &oauth2.Config{
			ClientID:     cfg.GitLabClientID,
			ClientSecret: cfg.GitLabClientSecret,
			RedirectURL:  cfg.OAuthCallbackBase + "/api/v1/auth/gitlab/callback",
			Endpoint: oauth2.Endpoint{
				AuthURL:  gitlabBase + "/oauth/authorize",
				TokenURL: gitlabBase + "/oauth/token",
			},
			Scopes: []string{"read_user", "email"},
		}
	}
	return s
}

func (s *Service) RegisterRoutes(r chi.Router) {
	r.Get("/auth/{provider}/login", s.handleLogin)
	r.Get("/auth/{provider}/callback", s.handleCallback)
	r.Post("/auth/logout", s.handleLogout)
}

func (s *Service) handleLogin(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	cfg, ok := s.oauth[provider]
	if !ok || cfg == nil {
		http.Error(w, "provider not configured", http.StatusBadRequest)
		return
	}

	state, err := randomToken(32)
	if err != nil {
		http.Error(w, "failed to create state", http.StatusInternalServerError)
		return
	}
	if err := s.redis.Set(r.Context(), stateKey(state), provider, 10*time.Minute).Err(); err != nil {
		http.Error(w, "failed to store state", http.StatusInternalServerError)
		return
	}

	url := cfg.AuthCodeURL(state, oauth2.AccessTypeOnline)
	http.Redirect(w, r, url, http.StatusFound)
}

func (s *Service) handleCallback(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	cfg, ok := s.oauth[provider]
	if !ok || cfg == nil {
		http.Error(w, "provider not configured", http.StatusBadRequest)
		return
	}

	state := r.URL.Query().Get("state")
	if state == "" {
		http.Error(w, "missing state", http.StatusBadRequest)
		return
	}
	storedProvider, err := s.redis.Get(r.Context(), stateKey(state)).Result()
	if err != nil || storedProvider != provider {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	_ = s.redis.Del(r.Context(), stateKey(state))

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := cfg.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, "token exchange failed", http.StatusBadRequest)
		return
	}

	profile, err := s.fetchProfile(r.Context(), provider, token, cfg)
	if err != nil {
		http.Error(w, "profile fetch failed", http.StatusBadRequest)
		return
	}

	user, err := s.store.UpsertOAuthUser(r.Context(), profile)
	if err != nil {
		http.Error(w, "user upsert failed", http.StatusInternalServerError)
		return
	}

	sessionID, err := randomToken(32)
	if err != nil {
		http.Error(w, "session create failed", http.StatusInternalServerError)
		return
	}
	if err := s.redis.Set(r.Context(), sessionKey(sessionID), user.ID.String(), sessionTTL).Err(); err != nil {
		http.Error(w, "session store failed", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL.Seconds()),
	})

	http.Redirect(w, r, s.cfg.FrontendURL+"/dashboard", http.StatusFound)
}

func (s *Service) handleLogout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie(sessionCookie); err == nil {
		_ = s.redis.Del(r.Context(), sessionKey(cookie.Value))
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.CookieSecure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookie)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		userIDStr, err := s.redis.Get(r.Context(), sessionKey(cookie.Value)).Result()
		if err != nil {
			writeUnauthorized(w)
			return
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		user, err := s.store.GetUserByID(r.Context(), userID)
		if err != nil {
			writeUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func UserFromContext(ctx context.Context) (*db.User, bool) {
	user, ok := ctx.Value(UserContextKey).(*db.User)
	return user, ok
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":{"code":"unauthorized","message":"authentication required"}}`))
}

func sessionKey(id string) string { return "session:" + id }
func stateKey(id string) string   { return "oauth_state:" + id }

func randomToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func (s *Service) fetchProfile(ctx context.Context, provider string, token *oauth2.Token, cfg *oauth2.Config) (db.OAuthProfile, error) {
	switch provider {
	case "github":
		return s.fetchGitHubProfile(ctx, token)
	case "google":
		return s.fetchGoogleProfile(ctx, token)
	case "gitlab":
		return s.fetchGitLabProfile(ctx, token, cfg)
	default:
		return db.OAuthProfile{}, fmt.Errorf("unsupported provider")
	}
}

func (s *Service) fetchGitHubProfile(ctx context.Context, token *oauth2.Token) (db.OAuthProfile, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return db.OAuthProfile{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var payload struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return db.OAuthProfile{}, err
	}
	email := payload.Email
	if email == "" {
		email = payload.Login + "@users.noreply.github.com"
	}
	name := payload.Name
	if name == "" {
		name = payload.Login
	}
	return db.OAuthProfile{
		Provider:       "github",
		ProviderUserID: fmt.Sprintf("%d", payload.ID),
		Email:          email,
		Name:           name,
		AvatarURL:      payload.AvatarURL,
	}, nil
}

func (s *Service) fetchGoogleProfile(ctx context.Context, token *oauth2.Token) (db.OAuthProfile, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return db.OAuthProfile{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var payload struct {
		ID      string `json:"id"`
		Email   string `json:"email"`
		Name    string `json:"name"`
		Picture string `json:"picture"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return db.OAuthProfile{}, err
	}
	return db.OAuthProfile{
		Provider:       "google",
		ProviderUserID: payload.ID,
		Email:          payload.Email,
		Name:           payload.Name,
		AvatarURL:      payload.Picture,
	}, nil
}

func (s *Service) fetchGitLabProfile(ctx context.Context, token *oauth2.Token, cfg *oauth2.Config) (db.OAuthProfile, error) {
	base := strings.TrimSuffix(s.cfg.GitLabBaseURL, "/")
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get(base + "/api/v4/user")
	if err != nil {
		return db.OAuthProfile{}, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var payload struct {
		ID        int64  `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		AvatarURL string `json:"avatar_url"`
		Email     string `json:"email"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return db.OAuthProfile{}, err
	}
	email := payload.Email
	if email == "" {
		email = payload.Username + "@users.noreply.gitlab.com"
	}
	return db.OAuthProfile{
		Provider:       "gitlab",
		ProviderUserID: fmt.Sprintf("%d", payload.ID),
		Email:          email,
		Name:           payload.Name,
		AvatarURL:      payload.AvatarURL,
	}, nil
}

func LoginURL(baseURL, provider string) string {
	u, _ := url.Parse(baseURL)
	u.Path = "/api/v1/auth/" + provider + "/login"
	return u.String()
}
