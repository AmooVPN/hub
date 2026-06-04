package app

import (
	"strings"
	"testing"
	"time"

	"github.com/AmooVPN/hub/internal/models"
)

func TestRenderPanelDetailPageV2ShowsHealthCheckButton(t *testing.T) {
	checkedAt := time.Now().UTC()
	panel := &models.Panel{ID: 42, Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", Status: models.PanelStatusOnline, LastCheckedAt: &checkedAt}
	page := renderPanelDetailPageV2(panel, "hub", "admin")
	if !strings.Contains(page, "/admin/panels/42/health") {
		t.Fatal("expected health check button")
	}
	if !strings.Contains(page, "Last checked") {
		t.Fatal("expected last checked field")
	}
}
