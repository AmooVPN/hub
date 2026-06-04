package models

import "time"

const (
	NotificationTypePanelOffline           = "panel.offline"
	NotificationTypePanelAuthError         = "panel.auth_error"
	NotificationTypeSyncFailed             = "sync.failed"
	NotificationTypeBackupExportCompleted = "backup.exported"
	NotificationTypeBackupImportCompleted  = "backup.imported"
	NotificationTypeBackupImportFailed     = "backup.import.failed"
	NotificationTypeBackupExportFailed     = "backup.export.failed"
	NotificationTypeClientTrafficLimit     = "client.traffic_limited"
	NotificationTypeClientExpired          = "client.expired"
)

type Notification struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"`
	Severity  string     `json:"severity"`
	Title     string     `json:"title"`
	Message   string     `json:"message"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}
