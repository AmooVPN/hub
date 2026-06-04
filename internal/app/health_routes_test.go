package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"github.com/AmooVPN/hub/internal/config"
	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/services"
)

func TestAutomaticBackupInterval(t *testing.T) {
	cases := map[string]int{
		"daily":   24,
		"weekly":  24 * 7,
		"monthly": 24 * 30,
	}
	for schedule, hours := range cases {
		interval, ok := automaticBackupInterval(schedule)
		if !ok {
			t.Fatalf("expected valid schedule %s", schedule)
		}
		if interval.Hours() != float64(hours) {
			t.Fatalf("unexpected interval for %s: %v", schedule, interval)
		}
	}
	if _, ok := automaticBackupInterval("nope"); ok {
		t.Fatal("expected invalid schedule")
	}
}

func TestCollectSystemHealthSQLiteOk(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hub.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	r := &Runner{cfg: &config.Config{AppName: "hub"}, db: db}
	health := r.collectSystemHealth(t.Context())
	if health.SQLite != "ok" {
		t.Fatalf("expected sqlite ok, got %+v", health)
	}
	if health.Status != "degraded" {
		t.Fatalf("expected degraded because redis unavailable, got %+v", health)
	}
}

func TestHealthLiveRoute(t *testing.T) {
	r := &Runner{cfg: &config.Config{AppName: "hub"}}
	app := fiber.New()
	app.Get("/health/live", r.getHealthLive)
	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["app"] != "hub" || body["status"] != "ok" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestHealthReadyRoute(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hub.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	r := &Runner{cfg: &config.Config{AppName: "hub"}, db: db}
	app := fiber.New()
	app.Get("/health/ready", r.getHealthReady)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected service unavailable, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["status"] != "degraded" {
		t.Fatalf("unexpected body: %+v", body)
	}
}

func TestHealthReadyRouteReportsRedisState(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hub.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("start miniredis: %v", err)
	}
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	r := &Runner{cfg: &config.Config{AppName: "hub"}, db: db, redis: rdb}
	app := fiber.New()
	app.Get("/health/ready", r.getHealthReady)
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["redis"] != "ok" {
		t.Fatalf("expected redis ok, got %+v", body)
	}
}

func TestMetricsEndpointReturnsExpectedFormat(t *testing.T) {
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
	if got := resp.Header.Get("Content-Type"); !strings.HasPrefix(got, "text/plain") {
		t.Fatalf("expected text/plain content type, got %q", got)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)
	for _, expected := range []string{"# HELP hub_http_requests_total", "# TYPE hub_panels_total gauge", "hub_backup_files_total"} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %q in metrics body: %s", expected, body)
		}
	}
}
