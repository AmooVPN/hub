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

	"github.com/AmooVPN/hub/internal/config"
	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/security"
	"github.com/AmooVPN/hub/internal/services"
)

func TestRenderAdminMaintenancePageShowsActions(t *testing.T) {
	page := renderAdminMaintenancePage(&Runner{cfg: &config.Config{AppName: "hub"}}, security.RoleOwner, "", "")
	for _, expected := range []string{"Maintenance", "Run cleanup", "Check DB", "Run VACUUM", "Dangerous administrative operations"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}

func TestAdminMaintenanceRequiresOwner(t *testing.T) {
	app := maintenanceTestApp(t, security.RoleAdmin, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/maintenance", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", resp.StatusCode)
	}
}

func TestAdminMaintenancePageForOwner(t *testing.T) {
	app := maintenanceTestApp(t, security.RoleOwner, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/admin/maintenance", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	if !responseBodyContains(t, resp, "Maintenance") {
		t.Fatal("expected maintenance page")
	}
}

func TestAdminMaintenanceCleanupAction(t *testing.T) {
	db := openMaintenanceTestDatabase(t)
	app := maintenanceTestApp(t, security.RoleOwner, services.NewCleanupService(db, nil, nil), nil)
	req := httptest.NewRequest(http.MethodPost, "/admin/maintenance/cleanup", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	if !responseBodyContains(t, resp, "Cleanup complete:") {
		t.Fatal("expected cleanup success message")
	}
}

func TestAdminMaintenanceCheckDBAction(t *testing.T) {
	db := openMaintenanceTestDatabase(t)
	app := maintenanceTestApp(t, security.RoleOwner, nil, db)
	req := httptest.NewRequest(http.MethodPost, "/admin/maintenance/check-db", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	if !responseBodyContains(t, resp, "Integrity check result: ok") {
		t.Fatal("expected integrity check message")
	}
}

func TestAdminMaintenanceVacuumDBAction(t *testing.T) {
	db := openMaintenanceTestDatabase(t)
	app := maintenanceTestApp(t, security.RoleOwner, nil, db)
	req := httptest.NewRequest(http.MethodPost, "/admin/maintenance/vacuum-db", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	if !responseBodyContains(t, resp, "SQLite VACUUM completed.") {
		t.Fatal("expected vacuum success message")
	}
}

func maintenanceTestApp(t *testing.T, role string, cleanup *services.CleanupService, db *sql.DB) *fiber.App {
	t.Helper()
	r := &Runner{cfg: &config.Config{AppName: "hub"}, cleanup: cleanup, db: db}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin", &models.AdminUser{Role: role})
		return c.Next()
	})
	app.Get("/admin/maintenance", r.getAdminMaintenance)
	app.Post("/admin/maintenance/cleanup", r.postAdminMaintenanceCleanup)
	app.Post("/admin/maintenance/check-db", r.postAdminMaintenanceCheckDB)
	app.Post("/admin/maintenance/vacuum-db", r.postAdminMaintenanceVacuumDB)
	return app
}

func responseBodyContains(t *testing.T, resp *http.Response, expected string) bool {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return strings.Contains(string(body), expected)
}

func openMaintenanceTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "maintenance.db")
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
