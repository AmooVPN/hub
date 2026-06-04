package app

import (
	"strings"
	"testing"
	"time"

	"github.com/AmooVPN/hub/internal/security"
	"github.com/AmooVPN/hub/internal/services"
)

func TestRenderAdminBackupsPageHidesDeleteForAdmin(t *testing.T) {
	html := renderAdminBackupsPage([]services.BackupRecord{{Name: "hub-backup-2026-06-04-12-00-00.zip", Size: 123, ModifiedAt: time.Unix(0, 0)}}, security.RoleAdmin, "Hub", "/var/backups")
	if !strings.Contains(html, "Create backup") {
		t.Fatalf("expected export action")
	}
	if strings.Contains(html, `data-bs-target="#backupDeleteModal"`) {
		t.Fatalf("admin should not see delete action")
	}
}

func TestRenderAdminBackupsPageShowsDeleteForOwner(t *testing.T) {
	html := renderAdminBackupsPage([]services.BackupRecord{{Name: "hub-backup-2026-06-04-12-00-00.zip", Size: 123, ModifiedAt: time.Unix(0, 0)}}, security.RoleOwner, "Hub", "/var/backups")
	if !strings.Contains(html, `data-bs-target="#backupDeleteModal"`) {
		t.Fatalf("owner should see delete action")
	}
}

func TestRenderAdminBackupsPageShowsImportFormForOwner(t *testing.T) {
	html := renderAdminBackupsPage([]services.BackupRecord{}, security.RoleOwner, "Hub", "/var/backups")
	if !strings.Contains(html, "Validate import") {
		t.Fatalf("expected owner import form")
	}
}
