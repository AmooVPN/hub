package app

import (
	"context"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/redis/go-redis/v9"

	"github.com/AmooVPN/hub/internal/config"
	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/repositories"
	"github.com/AmooVPN/hub/internal/security"
	"github.com/AmooVPN/hub/internal/services"
)

func newHandlerTestRunner(t *testing.T) (*Runner, *config.Config) {
	r, cfg := newEmptyHandlerTestRunner(t)
	seedTestAdmin(t, r)
	return r, cfg
}

func newEmptyHandlerTestRunner(t *testing.T) (*Runner, *config.Config) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.OpenSQLite(filepath.Join(dir, "hub.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("migrations: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		_ = db.Close()
		t.Fatalf("start redis: %v", err)
	}
	t.Cleanup(func() {
		_ = db.Close()
		mr.Close()
	})
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := &config.Config{
		AppName:             "hub",
		AppEnv:              "development",
		AppBaseURL:          "https://hub.example.com",
		DatabasePath:        filepath.Join(dir, "hub.db"),
		BackupDir:           filepath.Join(dir, "backups"),
		RedisAddr:           mr.Addr(),
		SessionCookieName:   "hub_session",
		SessionTTLHrs:       168,
		JWTAccessTTLMinutes: 15,
		JWTRefreshTTLDays:   30,
		HUBSecretKey:        "secret-key-secret-key",
		BackupRetentionCount: 20,
		BackupRetentionDays:  30,
		MaxUploadSizeMB:     10,
	}
	r := &Runner{
		cfg:        cfg,
		db:         db,
		redis:      rdb,
		admins:     repositories.NewAdminRepository(db),
		clients:    repositories.NewClientRepository(db),
		clientsSvc: services.NewClientService(repositories.NewClientRepository(db)),
		refreshes:  repositories.NewRefreshTokenRepository(db),
		settings:   repositories.NewSettingsRepository(db),
		panels:     repositories.NewPanelRepository(db),
		inbounds:   repositories.NewInboundRepository(db),
		metrics:    services.NewMetricsCollector(),
		backups:    services.NewBackupService(),
		cleanup:    services.NewCleanupService(db, repositories.NewSyncJobRepository(db), services.NewBackupService()),
		loginLocks: newAttemptTracker(5, 15*time.Minute),
	}
	return r, cfg
}

func seedTestAdmin(t *testing.T, r *Runner) {
	t.Helper()
	hash, err := security.HashPassword("change-me-now-123")
	if err != nil {
		t.Fatalf("hash admin password: %v", err)
	}
	admin := &models.AdminUser{Username: "admin", PasswordHash: hash, Role: "owner", Active: true}
	if err := r.admins.Create(context.Background(), admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
}

func newFiberTestApp() *fiber.App {
	return fiber.New()
}

func withAdminContext(role string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("admin", &models.AdminUser{ID: 1, Username: "admin", Role: role, Active: true})
		return c.Next()
	}
}

func withClientContext(client *models.Client) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("client", client)
		return c.Next()
	}
}

func signClientAccessToken(t *testing.T, secret string, clientID int64) string {
	t.Helper()
	token, err := security.SignAccessToken(strconv.FormatInt(clientID, 10), 15*time.Minute, secret)
	if err != nil {
		t.Fatalf("sign access token: %v", err)
	}
	return token
}
