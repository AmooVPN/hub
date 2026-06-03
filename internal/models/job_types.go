package models

const (
	JobTypePanelTest                = "panel_test"
	JobTypePanelSync                = "panel_sync"
	JobTypeInboundSync              = "inbound_sync"
	JobTypeTrafficSync              = "traffic_sync"
	JobTypeClientAttach             = "client_attach"
	JobTypeClientDetach             = "client_detach"
	JobTypeSubscriptionCacheRefresh = "subscription_cache_refresh"
	JobTypeBackupExport             = "backup_export"
	JobTypeBackupImport             = "backup_import"
	JobTypeBackupCleanup            = "backup_cleanup"
	JobTypePanelHealthCheck         = "panel_health_check"
	JobTypeWebhookDelivery          = "webhook_delivery"
	JobTypeNotificationDelivery     = "notification_delivery"
)

func IsValidJobType(jobType string) bool {
	switch jobType {
	case JobTypePanelTest, JobTypePanelSync, JobTypeInboundSync, JobTypeTrafficSync, JobTypeClientAttach, JobTypeClientDetach, JobTypeSubscriptionCacheRefresh, JobTypeBackupExport, JobTypeBackupImport, JobTypeBackupCleanup, JobTypePanelHealthCheck, JobTypeWebhookDelivery, JobTypeNotificationDelivery:
		return true
	default:
		return false
	}
}
