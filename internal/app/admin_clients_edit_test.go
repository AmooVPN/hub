package app

import (
	"strings"
	"testing"

	"github.com/AmooVPM/hub/internal/models"
)

func TestRenderAdminClientEditPageShowsFields(t *testing.T) {
	page := renderAdminClientEditPage(clientEditForm{Username: "client-1", DisplayName: "Client One", Status: "active", TrafficLimitText: "1024"}, "/admin/clients/1", "hub", "admin", nil)
	for _, expected := range []string{"Edit client", "display_name", "traffic_limit_bytes", "/admin/clients/1"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}

func TestClientEditFormFromClient(t *testing.T) {
	client := &models.Client{Username: "client-1", DisplayName: "Client One", Email: "a@example.com", Status: "disabled", TrafficLimitBytes: 2048}
	form := clientEditFormFromClient(client)
	if form.Username != client.Username || form.TrafficLimitText != "2048" || form.Status != "disabled" {
		t.Fatalf("unexpected form: %+v", form)
	}
}

func TestRenderAdminClientCreatePageShowsPasswordField(t *testing.T) {
	page := renderAdminClientCreatePage(clientCreateForm{}, "/admin/clients", "hub", "admin", nil)
	for _, expected := range []string{"Create client", "password", "/admin/clients"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}
