package models

import (
	"strings"
	"time"
)

const (
	WebhookEventClientCreated                = "client.created"
	WebhookEventClientUpdated                = "client.updated"
	WebhookEventClientDisabled               = "client.disabled"
	WebhookEventClientEnabled                = "client.enabled"
	WebhookEventClientDeleted                = "client.deleted"
	WebhookEventClientExpired                = "client.expired"
	WebhookEventClientTrafficLimited         = "client.traffic_limited"
	WebhookEventPanelCreated                 = "panel.created"
	WebhookEventPanelUpdated                 = "panel.updated"
	WebhookEventPanelDeleted                 = "panel.deleted"
	WebhookEventPanelOnline                  = "panel.online"
	WebhookEventPanelOffline                 = "panel.offline"
	WebhookEventPanelAuthError               = "panel.auth_error"
	WebhookEventInboundSynced                = "inbound.synced"
	WebhookEventSubscriptionTokenRegenerated = "subscription.token_regenerated"
	WebhookEventBackupExported               = "backup.exported"
	WebhookEventBackupImported               = "backup.imported"
	WebhookEventSyncFailed                   = "sync.failed"
	NotificationSeverityInfo                 = "info"
	NotificationSeveritySuccess              = "success"
	NotificationSeverityWarning              = "warning"
	NotificationSeverityDanger               = "danger"
)

func IsValidWebhookEvent(event string) bool {
	switch strings.TrimSpace(event) {
	case WebhookEventClientCreated, WebhookEventClientUpdated, WebhookEventClientDisabled, WebhookEventClientEnabled, WebhookEventClientDeleted, WebhookEventClientExpired, WebhookEventClientTrafficLimited, WebhookEventPanelCreated, WebhookEventPanelUpdated, WebhookEventPanelDeleted, WebhookEventPanelOnline, WebhookEventPanelOffline, WebhookEventPanelAuthError, WebhookEventInboundSynced, WebhookEventSubscriptionTokenRegenerated, WebhookEventBackupExported, WebhookEventBackupImported, WebhookEventSyncFailed:
		return true
	default:
		return false
	}
}

func IsValidNotificationSeverity(severity string) bool {
	switch strings.TrimSpace(severity) {
	case NotificationSeverityInfo, NotificationSeveritySuccess, NotificationSeverityWarning, NotificationSeverityDanger:
		return true
	default:
		return false
	}
}

type Webhook struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Secret    string    `json:"-"`
	Active    bool      `json:"active"`
	Events    []string  `json:"events"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type WebhookDelivery struct {
	ID             int64      `json:"id"`
	WebhookID      int64      `json:"webhook_id"`
	EventType      string     `json:"event_type"`
	PayloadJSON    string     `json:"payload_json"`
	Status         string     `json:"status"`
	ResponseStatus *int       `json:"response_status,omitempty"`
	ResponseBody   string     `json:"response_body,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	Attempts       int        `json:"attempts"`
	NextRetryAt    *time.Time `json:"next_retry_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	DeliveredAt    *time.Time `json:"delivered_at,omitempty"`
}
