package app

import (
	"strings"
	"testing"

	"github.com/AmooVPM/hub/internal/models"
)

func TestRenderPanelDetailPageV2ShowsHealthCheckButton(t *testing.T) {
	panel := &models.Panel{ID: 42, Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", Status: models.PanelStatusOnline}
	page := renderPanelDetailPageV2(panel, "hub", "admin")
	if !strings.Contains(page, "/admin/panels/42/health") {
		t.Fatal("expected health check button")
	}
}
