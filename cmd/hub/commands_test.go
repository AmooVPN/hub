package main

import (
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/AmooVPM/hub/internal/config"
	"github.com/AmooVPM/hub/internal/database"
)

type fakeRunner struct{ ran bool }

func (f *fakeRunner) Run() error {
	f.ran = true
	return nil
}

func TestExecuteDefaultsToServe(t *testing.T) {
	origLoadRunner := loadRunner
	t.Cleanup(func() { loadRunner = origLoadRunner })
	called := &fakeRunner{}
	loadRunner = func(*slog.Logger) (runnable, error) { return called, nil }
	if err := execute(nil, testLogger()); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !called.ran {
		t.Fatal("expected serve command to run")
	}
}

func TestExecuteMigrate(t *testing.T) {
	withCommandConfig(t, func(*config.Config) {
		if err := execute([]string{"migrate"}, testLogger()); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
}

func TestExecuteCreateAdmin(t *testing.T) {
	var cfg *config.Config
	withCommandConfig(t, func(testCfg *config.Config) {
		cfg = testCfg
		if err := execute([]string{"create-admin"}, testLogger()); err != nil {
			t.Fatalf("execute: %v", err)
		}
		db, err := openSQLite(cfg.DatabasePath)
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		defer db.Close()
		var count int64
		if err := db.QueryRow(`SELECT COUNT(*) FROM admin_users`).Scan(&count); err != nil {
			t.Fatalf("count admins: %v", err)
		}
		if count != 1 {
			t.Fatalf("expected 1 admin, got %d", count)
		}
	})
}

func TestExecuteHealthcheck(t *testing.T) {
	withCommandConfig(t, func(*config.Config) {
		if err := execute([]string{"healthcheck"}, testLogger()); err != nil {
			t.Fatalf("execute: %v", err)
		}
	})
}

func TestExecuteVersion(t *testing.T) {
	var buf strings.Builder
	if err := runVersion(&buf); err != nil {
		t.Fatalf("runVersion: %v", err)
	}
	if got := strings.TrimSpace(buf.String()); got == "" {
		t.Fatal("expected version output")
	}
}

func withCommandConfig(t *testing.T, fn func(*config.Config)) {
	t.Helper()
	dir := t.TempDir()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	t.Cleanup(func() { mr.Close() })
	cfg := &config.Config{
		AppName:              "hub",
		AppEnv:               "development",
		AppAddr:              "127.0.0.1:8080",
		AppBaseURL:           "http://localhost:8080",
		DatabasePath:         filepath.Join(dir, "hub.db"),
		RedisAddr:            mr.Addr(),
		SessionCookieName:    "hub_session",
		HUBSecretKey:         "secret-key-secret-key",
		BackupDir:            filepath.Join(dir, "backups"),
		InitialAdminUsername: "admin",
		InitialAdminPassword: "change-me-now-123",
		InitialAdminRole:     "owner",
	}
	origLoadConfig := loadConfig
	origOpenSQLite := openSQLite
	origOpenRedis := openRedis
	origRunMigrations := runMigrations
	origValidateSchema := validateSchema
	t.Cleanup(func() {
		loadConfig = origLoadConfig
		openSQLite = origOpenSQLite
		openRedis = origOpenRedis
		runMigrations = origRunMigrations
		validateSchema = origValidateSchema
	})
	loadConfig = func() (*config.Config, error) { return cfg, nil }
	openSQLite = database.OpenSQLite
	openRedis = func(addr, password string, dbNumber int) (*redis.Client, error) {
		return database.OpenRedis(addr, password, dbNumber)
	}
	runMigrations = database.RunMigrations
	validateSchema = database.ValidateSchemaVersion
	fn(cfg)
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
