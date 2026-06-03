package services

import (
	"testing"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

func TestSignWebhookPayload(t *testing.T) {
	timestamp := time.Unix(1700000000, 0).UTC()
	sig1 := SignWebhookPayload("secret", models.WebhookEventBackupExported, "delivery-1", timestamp, []byte(`{"ok":true}`))
	sig2 := SignWebhookPayload("secret", models.WebhookEventBackupExported, "delivery-1", timestamp, []byte(`{"ok":true}`))
	if sig1 == "" || sig1 != sig2 {
		t.Fatalf("unexpected signature behavior: %q %q", sig1, sig2)
	}
}

func TestWebhookEventValidation(t *testing.T) {
	if !IsSupportedWebhookEvent(models.WebhookEventPanelOffline) {
		t.Fatal("expected supported webhook event")
	}
	if IsSupportedWebhookEvent("nope") {
		t.Fatal("did not expect unsupported webhook event")
	}
}
