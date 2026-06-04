package config

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadParsesEnvironment(t *testing.T) {
	t.Setenv("APP_NAME", "hub")
	t.Setenv("APP_ENV", "production")
	t.Setenv("APP_ADDR", "0.0.0.0:9090")
	t.Setenv("APP_BASE_URL", "https://hub.example.com")
	t.Setenv("DATABASE_PATH", filepath.Join(t.TempDir(), "data", "hub.db"))
	t.Setenv("REDIS_ADDR", "redis:6379")
	t.Setenv("HUB_SECRET_KEY", "secret-key-secret-key")
	t.Setenv("BACKUP_DIR", filepath.Join(t.TempDir(), "backups"))
	t.Setenv("AUTOMATIC_BACKUP_SCHEDULE", "weekly")
	t.Setenv("TRUSTED_PROXIES", "10.0.0.1, 10.0.0.2")
	t.Setenv("TRUST_PROXY", "true")
	t.Setenv("INITIAL_ADMIN_USERNAME", "admin")
	t.Setenv("INITIAL_ADMIN_PASSWORD", "change-me-now-123")
	t.Setenv("INITIAL_ADMIN_ROLE", "owner")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.IsProduction() {
		t.Fatal("expected production mode")
	}
	if cfg.AppAddr != "0.0.0.0:9090" || cfg.AppBaseURL != "https://hub.example.com" {
		t.Fatalf("unexpected config: %+v", cfg)
	}
	if cfg.AutomaticBackupSchedule != "weekly" || !cfg.TrustProxy {
		t.Fatalf("unexpected schedule/proxy: %+v", cfg)
	}
	if !reflect.DeepEqual(cfg.TrustedProxies, []string{"10.0.0.1", "10.0.0.2"}) {
		t.Fatalf("unexpected trusted proxies: %+v", cfg.TrustedProxies)
	}
}

func TestLoadRejectsInvalidScheduleAndMissingSecret(t *testing.T) {
	t.Setenv("HUB_SECRET_KEY", "")
	t.Setenv("AUTOMATIC_BACKUP_SCHEDULE", "never")
	if _, err := Load(); err == nil {
		t.Fatal("expected load error")
	}
}

func TestEnsurePaths(t *testing.T) {
	dir := t.TempDir()
	cfg := &Config{DatabasePath: filepath.Join(dir, "data", "hub.db"), BackupDir: filepath.Join(dir, "backups")}
	if err := cfg.EnsurePaths(); err != nil {
		t.Fatalf("ensure paths: %v", err)
	}
}

func TestSplitEnvList(t *testing.T) {
	got := splitEnvList("a, b, ,c")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected split list: %+v", got)
	}
}
