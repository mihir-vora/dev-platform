package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               string
	DatabaseURL        string
	RedisURL           string
	SessionSecret      string
	FrontendURL        string
	OAuthCallbackBase  string
	CookieSecure       bool
	WorkerConcurrency  int
	WorkerPollInterval time.Duration
	GitLabBaseURL      string

	GitHubClientID     string
	GitHubClientSecret string
	GoogleClientID     string
	GoogleClientSecret string
	GitLabClientID     string
	GitLabClientSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:               getEnv("API_PORT", getEnv("PORT", "8080")),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		RedisURL:           os.Getenv("REDIS_URL"),
		SessionSecret:      os.Getenv("SESSION_SECRET"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:3000"),
		OAuthCallbackBase:  getEnv("OAUTH_CALLBACK_BASE_URL", "http://localhost:8080"),
		CookieSecure:       getEnvBool("COOKIE_SECURE", false),
		GitLabBaseURL:      getEnv("GITLAB_BASE_URL", "https://gitlab.com"),
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GitLabClientID:     os.Getenv("GITLAB_CLIENT_ID"),
		GitLabClientSecret: os.Getenv("GITLAB_CLIENT_SECRET"),
		WorkerConcurrency:  getEnvInt("WORKER_CONCURRENCY", 3),
		WorkerPollInterval: getEnvDuration("WORKER_POLL_INTERVAL", time.Second),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}
	if len(cfg.SessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 characters")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return strings.EqualFold(v, "true") || v == "1"
}

func getEnvInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
