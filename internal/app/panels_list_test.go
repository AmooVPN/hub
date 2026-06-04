package app

import (
	"strings"
	"testing"

	"github.com/AmooVPN/hub/internal/models"
)

func TestRenderPanelListPageShowsStatusFilter(t *testing.T) {
	panels := []models.Panel{{ID: 1, Name: "panel-1", BaseURL: "https://one.example", Username: "admin", Status: models.PanelStatusOnline}}
	page := renderPanelListPage(panels, panelListFilters{Status: models.PanelStatusOnline}, "hub", "admin")
	for _, expected := range []string{"<select class=\"form-select\" id=\"status\"", "option value=\"online\" selected", "text-bg-success\">online</span>"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}
