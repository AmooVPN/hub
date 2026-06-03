package models

const (
	SyncJobStatusQueued    = "queued"
	SyncJobStatusRunning   = "running"
	SyncJobStatusSuccess   = "success"
	SyncJobStatusFailed    = "failed"
	SyncJobStatusCancelled = "cancelled"
)

func IsValidSyncJobStatus(status string) bool {
	switch status {
	case SyncJobStatusQueued, SyncJobStatusRunning, SyncJobStatusSuccess, SyncJobStatusFailed, SyncJobStatusCancelled:
		return true
	default:
		return false
	}
}
