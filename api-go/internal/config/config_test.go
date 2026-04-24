package config

import (
	"strings"
	"testing"
)

func TestLoadLocal(t *testing.T) {
	t.Setenv("ENV", "")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mongo.Host != "localhost" {
		t.Fatalf("unexpected host %s", cfg.Mongo.Host)
	}
	if cfg.Mongo.Password != "example" {
		t.Fatalf("unexpected password %s", cfg.Mongo.Password)
	}
}

func TestLoadTestEnv(t *testing.T) {
	t.Setenv("ENV", "test")
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mongo.Host != "mongo" {
		t.Fatalf("unexpected host %s", cfg.Mongo.Host)
	}
	if cfg.Mongo.Database != "sfree_test" {
		t.Fatalf("unexpected db %s", cfg.Mongo.Database)
	}
}

func TestLoadRejectsInvalidDBPortOverride(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("DB_PORT", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "DB_PORT") {
		t.Fatalf("expected DB_PORT in error, got %v", err)
	}
}

func TestLoadRejectsInvalidSourceClientOverride(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("SOURCE_MAX_RETRIES", "three")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "SOURCE_MAX_RETRIES") {
		t.Fatalf("expected SOURCE_MAX_RETRIES in error, got %v", err)
	}
}

func TestLoadAppliesValidNumericOverrides(t *testing.T) {
	t.Setenv("ENV", "test")
	t.Setenv("DB_PORT", "27018")
	t.Setenv("SOURCE_MAX_RETRIES", "7")
	t.Setenv("RATE_LIMIT_PER_IP", "42")
	t.Setenv("UPLOAD_CHUNK_SIZE", "1024")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Mongo.Port != 27018 {
		t.Fatalf("unexpected port %d", cfg.Mongo.Port)
	}
	if cfg.SourceClient.MaxRetries != 7 {
		t.Fatalf("unexpected max retries %d", cfg.SourceClient.MaxRetries)
	}
	if cfg.RateLimit.PerIP != 42 {
		t.Fatalf("unexpected per-ip rate limit %d", cfg.RateLimit.PerIP)
	}
	if cfg.Upload.ChunkSize != 1024 {
		t.Fatalf("unexpected chunk size %d", cfg.Upload.ChunkSize)
	}
}
