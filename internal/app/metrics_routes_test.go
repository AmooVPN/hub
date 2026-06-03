package app

import (
	"database/sql"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/config"
	"github.com/AmooVPM/hub/internal/database"
	"github.com/AmooVPM/hub/internal/services"
)

func TestMetricsEndpointRequiresToken(t *testing.T) {
	r := &Runner{cfg: &config.Config{AppName: "hub", MetricsEnabled: true, MetricsToken: "secret"}, metrics: services.NewMetricsCollector()}
	app := fiber.New()
	app.Get("/metrics", r.getMetrics)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.StatusCode)
	}
}

func TestMetricsEndpointReturnsMetrics(t *testing.T) {
	db := openMetricsTestDatabase(t)
	backupDir := t.TempDir()
	backupService := services.NewBackupService()
	r := &Runner{cfg: &config.Config{AppName: "hub", MetricsEnabled: true, DatabasePath: filepath.Join(t.TempDir(), "hub.db"), BackupDir: backupDir}, db: db, backups: backupService, metrics: services.NewMetricsCollector()}
	app := fiber.New()
	app.Use(r.metricsMiddleware())
	app.Get("/metrics", r.getMetrics)
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "hub_http_requests_total") || !strings.Contains(body, "hub_panels_total") {
		t.Fatalf("unexpected metrics body: %s", body)
	}
}

func openMetricsTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
