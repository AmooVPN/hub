package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	AppName              string
	AppEnv               string
	AppAddr              string
	AppBaseURL           string
	DatabasePath         string
	RedisAddr            string
	RedisPassword        string
	RedisDB              int
	SessionCookieName    string
	SessionTTLHrs        int
	JWTAccessTTLMinutes  int
	JWTRefreshTTLDays    int
	HUBSecretKey         string
	BackupDir            string
	BackupRetentionCount int
	BackupRetentionDays  int
	MaxUploadSizeMB      int
	InitialAdminUsername string
	InitialAdminPassword string
	InitialAdminEmail    string
	InitialAdminRole     string
}

func Load() (*Config, error) {
	c := &Config{
		AppName:              getEnv("APP_NAME", "hub"),
		AppEnv:               getEnv("APP_ENV", "development"),
		AppAddr:              getEnv("APP_ADDR", "0.0.0.0:8080"),
		AppBaseURL:           getEnv("APP_BASE_URL", "http://localhost:8080"),
		DatabasePath:         getEnv("DATABASE_PATH", "./data/hub.db"),
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        os.Getenv("REDIS_PASSWORD"),
		SessionCookieName:    getEnv("SESSION_COOKIE_NAME", "hub_session"),
		HUBSecretKey:         os.Getenv("HUB_SECRET_KEY"),
		BackupDir:            getEnv("BACKUP_DIR", "./backups"),
		BackupRetentionCount: 20,
		BackupRetentionDays:  30,
		InitialAdminUsername: getEnv("INITIAL_ADMIN_USERNAME", "admin"),
		InitialAdminPassword: getEnv("INITIAL_ADMIN_PASSWORD", "change-me-now"),
		InitialAdminEmail:    os.Getenv("INITIAL_ADMIN_EMAIL"),
		InitialAdminRole:     getEnv("INITIAL_ADMIN_ROLE", "owner"),
	}

	var err error
	if c.RedisDB, err = parseIntEnv("REDIS_DB", 0); err != nil {
		return nil, err
	}
	if c.SessionTTLHrs, err = parseIntEnv("SESSION_TTL_HOURS", 168); err != nil {
		return nil, err
	}
	if c.JWTAccessTTLMinutes, err = parseIntEnv("JWT_ACCESS_TTL_MINUTES", 15); err != nil {
		return nil, err
	}
	if c.JWTRefreshTTLDays, err = parseIntEnv("JWT_REFRESH_TTL_DAYS", 30); err != nil {
		return nil, err
	}
	if c.MaxUploadSizeMB, err = parseIntEnv("MAX_UPLOAD_SIZE_MB", 100); err != nil {
		return nil, err
	}
	if c.BackupRetentionCount, err = parseIntEnv("BACKUP_RETENTION_COUNT", 20); err != nil {
		return nil, err
	}
	if c.BackupRetentionDays, err = parseIntEnv("BACKUP_RETENTION_DAYS", 30); err != nil {
		return nil, err
	}

	if c.HUBSecretKey == "" {
		return nil, fmt.Errorf("HUB_SECRET_KEY is required")
	}

	return c, nil
}

func (c *Config) EnsurePaths() error {
	if err := os.MkdirAll(filepath.Dir(c.DatabasePath), 0o755); err != nil {
		return fmt.Errorf("create database directory: %w", err)
	}
	if err := os.MkdirAll(c.BackupDir, 0o755); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	return nil
}

func (c *Config) IsProduction() bool {
	return strings.EqualFold(c.AppEnv, "production")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func parseIntEnv(key string, fallback int) (int, error) {
	value := os.Getenv(key)
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", key, err)
	}
	return parsed, nil
}
