package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
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

func TestAdminLoginRoute(t *testing.T) {
	r, cfg := newAuthTestRunner(t)
	app := fiber.New()
	app.Post("/admin/login", r.postAdminLogin)
	req := httptest.NewRequest(http.MethodPost, "/admin/login", strings.NewReader(url.Values{"username": {"admin"}, "password": {"change-me-now-123"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected redirect, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/admin" {
		t.Fatalf("expected admin redirect, got %q", loc)
	}
	if cookie := resp.Header.Values("Set-Cookie"); len(cookie) == 0 || !strings.Contains(cookie[0], cfg.SessionCookieName+"=") {
		t.Fatalf("expected session cookie, got %v", cookie)
	}
}

func TestClientLoginRoute(t *testing.T) {
	r, _ := newAuthTestRunner(t)
	client := &models.Client{Username: "client", PasswordHash: mustHash(t, "client-password-123"), Status: "active", SubscriptionToken: "token-1"}
	if err := r.clients.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	app := fiber.New()
	app.Post("/client/login", r.postClientLogin)
	req := httptest.NewRequest(http.MethodPost, "/client/login", strings.NewReader(url.Values{"username": {"client"}, "password": {"client-password-123"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/client" {
		t.Fatalf("unexpected login response: %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestDisabledClientLoginRejected(t *testing.T) {
	r, _ := newAuthTestRunner(t)
	client := &models.Client{Username: "client", PasswordHash: mustHash(t, "client-password-123"), Status: "disabled", SubscriptionToken: "token-1"}
	if err := r.clients.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	app := fiber.New()
	app.Post("/client/login", r.postClientLogin)
	req := httptest.NewRequest(http.MethodPost, "/client/login", strings.NewReader(url.Values{"username": {"client"}, "password": {"client-password-123"}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", resp.StatusCode)
	}
}

func TestLoadClientSummaryFiltersDisabledAndStaleConfigs(t *testing.T) {
	r, cfg := newAuthTestRunner(t)
	client := &models.Client{Username: "client", PasswordHash: mustHash(t, "client-password-123"), Status: "active", SubscriptionToken: "token-1", TrafficLimitBytes: 1000}
	if err := r.clients.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: mustEncrypt(t, "panel-password", cfg.HUBSecretKey), Status: models.PanelStatusOffline}
	if err := r.panels.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}
	activeInbound := &models.Inbound{PanelID: panel.ID, RemoteInboundID: 101, Remark: "active", Protocol: "vless", Enabled: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	disabledInbound := &models.Inbound{PanelID: panel.ID, RemoteInboundID: 102, Remark: "disabled", Protocol: "vless", Enabled: false, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	staleInbound := &models.Inbound{PanelID: panel.ID, RemoteInboundID: 103, Remark: "stale", Protocol: "vless", Enabled: true, Stale: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	for _, inbound := range []*models.Inbound{activeInbound, disabledInbound, staleInbound} {
		if err := r.inbounds.Upsert(context.Background(), inbound); err != nil {
			t.Fatalf("upsert inbound: %v", err)
		}
		if err := r.db.QueryRowContext(context.Background(), `SELECT id FROM inbounds WHERE panel_id = ? AND remote_inbound_id = ?`, panel.ID, inbound.RemoteInboundID).Scan(&inbound.ID); err != nil {
			t.Fatalf("load inbound id: %v", err)
		}
	}
	if err := r.createClientAttachment(context.Background(), &models.ClientAttachment{ClientID: client.ID, PanelID: panel.ID, InboundID: activeInbound.ID, RemoteClientID: "r1", RemoteEmail: "r1@example.com", Enabled: true, UploadBytes: 100, DownloadBytes: 50, RawConfig: "cfg-1", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create attachment 1: %v", err)
	}
	if err := r.createClientAttachment(context.Background(), &models.ClientAttachment{ClientID: client.ID, PanelID: panel.ID, InboundID: disabledInbound.ID, RemoteClientID: "r2", RemoteEmail: "r2@example.com", Enabled: false, UploadBytes: 200, DownloadBytes: 25, RawConfig: "cfg-2", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create attachment 2: %v", err)
	}
	if err := r.createClientAttachment(context.Background(), &models.ClientAttachment{ClientID: client.ID, PanelID: panel.ID, InboundID: staleInbound.ID, RemoteClientID: "r3", RemoteEmail: "r3@example.com", Enabled: true, UploadBytes: 25, DownloadBytes: 25, RawConfig: "cfg-3", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("create attachment 3: %v", err)
	}
	summary, err := r.loadClientSummary(context.Background(), client)
	if err != nil {
		t.Fatalf("load summary: %v", err)
	}
	if summary.TotalBytes != 425 || summary.RemainingBytes != 575 {
		t.Fatalf("unexpected traffic summary: %+v", summary)
	}
	if len(summary.Configs) != 1 {
		t.Fatalf("expected only one active config, got %+v", summary.Configs)
	}
}

func TestClientAttachmentCreatePartialFailure(t *testing.T) {
	r, cfg := newAuthTestRunner(t)
	client := &models.Client{Username: "client", PasswordHash: mustHash(t, "client-password-123"), Status: "active", SubscriptionToken: "token-1"}
	if err := r.clients.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		switch {
		case req.URL.Path == "/login" && req.Method == http.MethodPost:
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(req.URL.Path, "/client/add") && req.Method == http.MethodPost:
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "remote-1", "email": "remote-1@example.com", "enable": true})
		default:
			http.NotFound(w, req)
		}
	}))
	defer server.Close()
	panel := &models.Panel{Name: "panel-1", BaseURL: server.URL, Username: "admin", EncryptedPassword: mustEncrypt(t, "panel-password", cfg.HUBSecretKey), Status: models.PanelStatusOffline}
	if err := r.panels.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}
	activeInbound := &models.Inbound{PanelID: panel.ID, RemoteInboundID: 101, Remark: "active", Protocol: "vless", Enabled: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	staleInbound := &models.Inbound{PanelID: panel.ID, RemoteInboundID: 102, Remark: "stale", Protocol: "vless", Enabled: true, Stale: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	for _, inbound := range []*models.Inbound{activeInbound, staleInbound} {
		if err := r.inbounds.Upsert(context.Background(), inbound); err != nil {
			t.Fatalf("upsert inbound: %v", err)
		}
		if err := r.db.QueryRowContext(context.Background(), `SELECT id FROM inbounds WHERE panel_id = ? AND remote_inbound_id = ?`, panel.ID, inbound.RemoteInboundID).Scan(&inbound.ID); err != nil {
			t.Fatalf("load inbound id: %v", err)
		}
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("admin", &models.AdminUser{ID: 1, Username: "admin", Role: security.RoleOwner, Active: true})
		return c.Next()
	})
	app.Post("/admin/clients/:id/attachments", r.postAdminClientAttachmentCreate)
	form := url.Values{}
	form.Set("inbound_ids", fmt.Sprintf("%d,%d", activeInbound.ID, staleInbound.ID))
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/admin/clients/%d/attachments", client.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.RemoteAddr = "127.0.0.1:12345"
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "Attached:") || !strings.Contains(string(body), "Failed:") {
		t.Fatalf("expected partial success/failure, got %s", string(body))
	}
	var count int64
	if err := r.db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM client_attachments WHERE client_id = ?`, client.ID).Scan(&count); err != nil {
		t.Fatalf("count attachments: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one successful attachment, got %d", count)
	}
}

func newAuthTestRunner(t *testing.T) (*Runner, *config.Config) {
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
		AppName:              "hub",
		AppEnv:               "development",
		AppBaseURL:           "https://hub.example.com",
		DatabasePath:         filepath.Join(dir, "hub.db"),
		BackupDir:            filepath.Join(dir, "backups"),
		RedisAddr:            mr.Addr(),
		SessionCookieName:    "hub_session",
		SessionTTLHrs:        168,
		HUBSecretKey:         "secret-key-secret-key",
		BackupRetentionCount:  20,
		BackupRetentionDays:   30,
	}
	r := &Runner{
		cfg:            cfg,
		db:             db,
		redis:          rdb,
		admins:         repositories.NewAdminRepository(db),
		clients:        repositories.NewClientRepository(db),
		clientsSvc:     services.NewClientService(repositories.NewClientRepository(db)),
		panels:         repositories.NewPanelRepository(db),
		inbounds:       repositories.NewInboundRepository(db),
		loginLocks:     newAttemptTracker(5, 15*time.Minute),
		metrics:        services.NewMetricsCollector(),
		backups:        services.NewBackupService(),
		cleanup:        services.NewCleanupService(db, repositories.NewSyncJobRepository(db), services.NewBackupService()),
	}
	seedTestAdmin(t, r)
	return r, cfg
}

func mustHash(t *testing.T, password string) string {
	t.Helper()
	hash, err := security.HashPassword(password)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	return hash
}

func mustEncrypt(t *testing.T, plaintext, secret string) string {
	t.Helper()
	value, err := security.Encrypt(plaintext, secret)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	return value
}
