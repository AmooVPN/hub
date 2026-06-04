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
