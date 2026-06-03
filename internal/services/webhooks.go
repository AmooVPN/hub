package services

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

type WebhookEnvelope struct {
	Event     string    `json:"event"`
	Delivery  string    `json:"delivery"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

type WebhookDeliveryStatus string

const (
	WebhookDeliveryQueued   WebhookDeliveryStatus = "queued"
	WebhookDeliveryRunning  WebhookDeliveryStatus = "running"
	WebhookDeliverySuccess  WebhookDeliveryStatus = "success"
	WebhookDeliveryFailed   WebhookDeliveryStatus = "failed"
	WebhookDeliveryRetrying WebhookDeliveryStatus = "retrying"
)

func SignWebhookPayload(secret, event, delivery string, timestamp time.Time, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(event))
	mac.Write([]byte("."))
	mac.Write([]byte(delivery))
	mac.Write([]byte("."))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp.UTC().Unix())))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func NewWebhookEnvelope(event, delivery string, payload any) WebhookEnvelope {
	return WebhookEnvelope{Event: event, Delivery: delivery, Timestamp: time.Now().UTC(), Payload: payload}
}

func IsSupportedWebhookEvent(event string) bool { return models.IsValidWebhookEvent(event) }
