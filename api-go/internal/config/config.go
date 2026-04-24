package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"gopkg.in/yaml.v3"
)

type MongoConfig struct {
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
}

type UploadConfig struct {
	ChunkSize int `yaml:"chunk_size"`
}

type GitHubOAuthConfig struct {
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	RedirectURL  string `yaml:"redirect_url"`
}

type RateLimitConfig struct {
	PerIP  int `yaml:"per_ip"`  // requests per minute for unauthenticated (default 60)
	PerKey int `yaml:"per_key"` // requests per minute for authenticated (default 600)
}

type SourceClientConfig struct {
	TimeoutSeconds   int `yaml:"timeout_seconds"`     // per-request timeout (default 30)
	FailureThreshold int `yaml:"failure_threshold"`   // consecutive failures before circuit opens (default 5)
	RecoverySeconds  int `yaml:"recovery_seconds"`    // seconds before half-open probe (default 30)
	MaxRetries       int `yaml:"max_retries"`         // retry attempts after first call (default 3)
	RetryBaseDelayMs int `yaml:"retry_base_delay_ms"` // initial backoff in ms (default 100)
	RetryMaxDelayMs  int `yaml:"retry_max_delay_ms"`  // max backoff cap in ms (default 5000)
}

type Config struct {
	Mongo           MongoConfig        `yaml:"mongo"`
	Upload          UploadConfig       `yaml:"upload"`
	RateLimit       RateLimitConfig    `yaml:"rate_limit"`
	SourceClient    SourceClientConfig `yaml:"source_client"`
	AccessSecretKey string             `yaml:"access_secret_key"`
	JWTSecret       string             `yaml:"jwt_secret"`
	GitHubOAuth     GitHubOAuthConfig  `yaml:"github_oauth"`
	FrontendURL     string             `yaml:"frontend_url"`
}

func Load() (*Config, error) {
	env := os.Getenv("ENV")
	if env == "" {
		env = "local"
	}
	_, f, _, _ := runtime.Caller(0)
	base := filepath.Join(filepath.Dir(f), "..", "..")
	file := filepath.Join(base, "config", fmt.Sprintf("%s.yaml", env))
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := overrideEnv(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func overrideEnv(cfg *Config) error {
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Mongo.Host = v
	}
	if err := applyIntEnv("DB_PORT", &cfg.Mongo.Port); err != nil {
		return err
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Mongo.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Mongo.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Mongo.Database = v
	}
	if err := applyIntEnv("UPLOAD_CHUNK_SIZE", &cfg.Upload.ChunkSize); err != nil {
		return err
	}
	if v := os.Getenv("ACCESS_SECRET_KEY"); v != "" {
		cfg.AccessSecretKey = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.JWTSecret = v
	}
	if v := os.Getenv("GITHUB_CLIENT_ID"); v != "" {
		cfg.GitHubOAuth.ClientID = v
	}
	if v := os.Getenv("GITHUB_CLIENT_SECRET"); v != "" {
		cfg.GitHubOAuth.ClientSecret = v
	}
	if v := os.Getenv("GITHUB_REDIRECT_URL"); v != "" {
		cfg.GitHubOAuth.RedirectURL = v
	}
	if v := os.Getenv("FRONTEND_URL"); v != "" {
		cfg.FrontendURL = v
	}
	if err := applyIntEnv("RATE_LIMIT_PER_IP", &cfg.RateLimit.PerIP); err != nil {
		return err
	}
	if err := applyIntEnv("RATE_LIMIT_PER_KEY", &cfg.RateLimit.PerKey); err != nil {
		return err
	}
	if err := applyIntEnv("SOURCE_TIMEOUT_SECONDS", &cfg.SourceClient.TimeoutSeconds); err != nil {
		return err
	}
	if err := applyIntEnv("SOURCE_FAILURE_THRESHOLD", &cfg.SourceClient.FailureThreshold); err != nil {
		return err
	}
	if err := applyIntEnv("SOURCE_RECOVERY_SECONDS", &cfg.SourceClient.RecoverySeconds); err != nil {
		return err
	}
	if err := applyIntEnv("SOURCE_MAX_RETRIES", &cfg.SourceClient.MaxRetries); err != nil {
		return err
	}
	if err := applyIntEnv("SOURCE_RETRY_BASE_DELAY_MS", &cfg.SourceClient.RetryBaseDelayMs); err != nil {
		return err
	}
	if err := applyIntEnv("SOURCE_RETRY_MAX_DELAY_MS", &cfg.SourceClient.RetryMaxDelayMs); err != nil {
		return err
	}
	return nil
}

func applyIntEnv(key string, dst *int) error {
	v := os.Getenv(key)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("invalid %s value %q: %w", key, v, err)
	}
	*dst = n
	return nil
}
