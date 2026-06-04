package app

import (
	"strings"
	"testing"
	"time"

	"github.com/AmooVPN/hub/internal/models"
)

func TestRenderAdminClientDetailPageShowsCoreSections(t *testing.T) {
	client := &models.Client{ID: 1, Username: "client-1", Status: "active", TrafficLimitBytes: 1024, SubscriptionToken: "abc123"}
	summary := clientSummary{Client: client, StatusText: "active", ExpiryText: "2026-01-01T00:00:00Z", TotalBytes: 256, RemainingBytes: 768}
	attachments := []models.ClientAttachment{{ID: 11, PanelID: 2, InboundID: 3, Enabled: true, RemoteEmail: "client@example.com"}}
	configs := []clientConfigRow{{PanelName: "panel-1", InboundRemark: "inbound-1", Protocol: "vless", Status: "active", CopyValue: "vless://example"}}
	audits := []models.AuditLog{{Action: "client_update", ActorType: "admin", CreatedAt: time.Now().UTC()}}
	page := renderAdminClientDetailPage(client, summary, attachments, configs, audits, "hub", "admin", "https://example.com")
	for _, expected := range []string{"Client detail", "Subscription URL", "Attached inbounds", "Configs", "Audit history", "/admin/clients/1/regenerate-token", "/admin/clients/1/sync-traffic"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}
