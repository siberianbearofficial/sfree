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

type Config struct {
	Mongo           MongoConfig  `yaml:"mongo"`
	Upload          UploadConfig `yaml:"upload"`
	AccessSecretKey string       `yaml:"-"`
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
	overrideEnv(&cfg)
	return &cfg, nil
}

func overrideEnv(cfg *Config) {
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Mongo.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Mongo.Port = p
		}
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
	if v := os.Getenv("UPLOAD_CHUNK_SIZE"); v != "" {
		if s, err := strconv.Atoi(v); err == nil {
			cfg.Upload.ChunkSize = s
		}
	}
	if v := os.Getenv("ACCESS_SECRET_KEY"); v != "" {
		cfg.AccessSecretKey = v
	}
}
