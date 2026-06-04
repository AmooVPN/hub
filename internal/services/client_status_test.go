package services

import (
	"testing"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

func TestDetermineClientStatus(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	expired := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name   string
		client *models.Client
		total  int64
		want   string
	}{
		{name: "disabled", client: &models.Client{Status: "disabled"}, want: "disabled"},
		{name: "expired", client: &models.Client{Status: "active", ExpiryTime: &expired}, want: "expired"},
		{name: "limited", client: &models.Client{Status: "active", TrafficLimitBytes: 100}, total: 100, want: "limited"},
		{name: "active", client: &models.Client{Status: "active", TrafficLimitBytes: 100, ExpiryTime: &future}, total: 50, want: "active"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := DetermineClientStatus(tc.client, tc.total, now); got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
