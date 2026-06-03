package models

import "strings"

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
