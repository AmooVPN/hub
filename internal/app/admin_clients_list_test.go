package app

import (
	"strings"
	"testing"

	"github.com/AmooVPN/hub/internal/models"
)

func TestRenderAdminClientListPageShowsFiltersAndStatusBadges(t *testing.T) {
	clients := []adminClientListRow{{Client: models.Client{ID: 1, Username: "active-user", Status: "active"}, AttachmentCount: 2, TotalBytes: 50}, {Client: models.Client{ID: 2, Username: "disabled-user", Status: "disabled"}, AttachmentCount: 0, TotalBytes: 0}}
	page := renderAdminClientListPage(clients, clientListFilters{Query: "active", Status: "active"}, "hub", "admin")
	for _, expected := range []string{"<input class=\"form-control\" id=\"q\"", "<select class=\"form-select\" id=\"status\"", "text-bg-success\">active</span>", "Attachments"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}
