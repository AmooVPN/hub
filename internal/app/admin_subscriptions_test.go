package app

import (
	"strings"
	"testing"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

func TestRenderAdminSubscriptionDetailPageShowsCopyButtons(t *testing.T) {
	when := time.Now().UTC()
	page := renderAdminSubscriptionDetailPage(&adminSubscriptionRow{Client: models.Client{ID: 1, Username: "client-1", Status: "active", SubscriptionToken: "abc123"}, ActiveConfigs: 2, LastCacheAt: &when}, "hub", "admin", "https://example.com")
	if !strings.Contains(page, "/admin/subscriptions/1/regenerate") || !strings.Contains(page, "navigator.clipboard.writeText") {
		t.Fatal("expected subscription actions and copy buttons")
	}
}

func TestRenderAdminSubscriptionsPageShowsRows(t *testing.T) {
	page := renderAdminSubscriptionsPage([]adminSubscriptionRow{{Client: models.Client{ID: 1, Username: "client-1", Status: "active"}, ActiveConfigs: 2}}, "hub", "admin")
	if !strings.Contains(page, "/admin/subscriptions/1") || !strings.Contains(page, "Active configs") {
		t.Fatal("expected subscriptions list content")
	}
}
