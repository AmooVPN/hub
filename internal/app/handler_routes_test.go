package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/security"
)

func TestAdminSettingsRouteWithAdminContext(t *testing.T) {
	r, _ := newHandlerTestRunner(t)
	app := newFiberTestApp()
	app.Use(withAdminContext(security.RoleOwner))
	app.Get("/admin/settings", r.getAdminSettings)

	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	page := string(body)
	if !strings.Contains(page, "Effective configuration") || !strings.Contains(page, "Secrets are intentionally hidden.") {
		t.Fatal("expected settings page content")
	}
}

func TestAdminSettingsUpdatePersistsValues(t *testing.T) {
	r, _ := newHandlerTestRunner(t)
	app := newFiberTestApp()
	app.Use(withAdminContext(security.RoleOwner))
	app.Post("/admin/settings", r.postAdminSettings)

	form := strings.NewReader("app_base_url=https%3A%2F%2Fhub.example.com%2Fapp&session_cookie_name=hub_session_new&session_ttl_hours=72&jwt_access_ttl_minutes=10&jwt_refresh_ttl_days=21&backup_dir=%2Ftmp%2Fhub-backups&automatic_backup_enabled=on&automatic_backup_schedule=weekly&backup_retention_count=12&backup_retention_days=14&metrics_enabled=on&trust_proxy=on&trusted_proxies=10.0.0.0%2F8%2C192.168.0.0%2F16&panel_url_strict_mode=on&max_upload_size_mb=55")
	req := httptest.NewRequest(http.MethodPost, "/admin/settings", form)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	if r.cfg.AppBaseURL != "https://hub.example.com/app" || r.cfg.SessionCookieName != "hub_session_new" || r.cfg.BackupRetentionCount != 12 || !r.cfg.AutomaticBackupEnabled {
		t.Fatalf("expected config to update, got %+v", r.cfg)
	}
	settings, err := r.settings.LoadAll(context.Background())
	if err != nil {
		t.Fatalf("load settings: %v", err)
	}
	if settings["APP_BASE_URL"] != "https://hub.example.com/app" || settings["SESSION_COOKIE_NAME"] != "hub_session_new" || settings["BACKUP_RETENTION_COUNT"] != "12" {
		t.Fatalf("expected settings to persist, got %+v", settings)
	}
}

func TestClientDashboardRouteWithClientContext(t *testing.T) {
	r, _ := newHandlerTestRunner(t)
	client := &models.Client{Username: "client-1", PasswordHash: mustHash(t, "client-password-123"), Status: "active", SubscriptionToken: "sub-token-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := r.clients.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	app := newFiberTestApp()
	app.Use(withClientContext(client))
	app.Get("/client", r.getClientDashboard)

	req := httptest.NewRequest(http.MethodGet, "/client", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	page := string(body)
	for _, expected := range []string{"Welcome, client-1", "Traffic used", "Subscription"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}

func TestClientAPIMeRequiresJWT(t *testing.T) {
	r, _ := newHandlerTestRunner(t)
	app := newFiberTestApp()
	app.Get("/api/v1/client/me", r.requireClientAPIJWT, r.getClientAPIMe)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/client/me", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.StatusCode)
	}
}

func TestClientAPIMeWithJWT(t *testing.T) {
	r, _ := newHandlerTestRunner(t)
	client := &models.Client{Username: "client-1", PasswordHash: mustHash(t, "client-password-123"), Status: "active", SubscriptionToken: "sub-token-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := r.clients.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	app := newFiberTestApp()
	app.Get("/api/v1/client/me", r.requireClientAPIJWT, r.getClientAPIMe)

	token := signClientAccessToken(t, r.cfg.HUBSecretKey, client.ID)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/client/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	var payload struct {
		Client struct {
			Username string `json:"username"`
		} `json:"client"`
		SubscriptionURL string `json:"subscription_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Client.Username != "client-1" || payload.SubscriptionURL == "" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
