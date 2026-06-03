package models

const (
	PanelStatusUnknown   = "unknown"
	PanelStatusOnline    = "online"
	PanelStatusOffline   = "offline"
	PanelStatusAuthError = "auth_error"
	PanelStatusDegraded  = "degraded"
	PanelStatusSyncing   = "syncing"
	PanelStatusError     = "error"
)

func IsValidPanelStatus(status string) bool {
	switch status {
	case PanelStatusUnknown, PanelStatusOnline, PanelStatusOffline, PanelStatusAuthError, PanelStatusDegraded, PanelStatusSyncing, PanelStatusError:
		return true
	default:
		return false
	}
}
