package services

import (
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

func DetermineClientStatus(client *models.Client, totalBytes int64, now time.Time) string {
	if client == nil {
		return "unknown"
	}
	if client.Status == "disabled" {
		return "disabled"
	}
	if client.ExpiryTime != nil && !client.ExpiryTime.After(now) {
		return "expired"
	}
	if client.TrafficLimitBytes > 0 && totalBytes >= client.TrafficLimitBytes {
		return "limited"
	}
	return "active"
}
