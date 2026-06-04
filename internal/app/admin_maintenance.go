package app

import (
	"fmt"
	"html"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/security"
)

func (r *Runner) getAdminMaintenance(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	return c.Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, "", ""))
}

func (r *Runner) postAdminMaintenanceCleanup(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	if r.cleanup == nil {
		return c.Status(fiber.StatusServiceUnavailable).Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, "Cleanup service is not configured.", "warning"))
	}
	report, err := r.cleanup.Run(c.UserContext(), r.cfg.BackupDir, r.cfg.BackupRetentionCount, r.cfg.BackupRetentionDays)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, err.Error(), "danger"))
	}
	return c.Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, "Cleanup complete: "+report.Summary(), "success"))
}

func (r *Runner) postAdminMaintenanceCheckDB(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	if r.db == nil {
		return c.Status(fiber.StatusServiceUnavailable).Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, "Database is not configured.", "warning"))
	}
	var result string
	if err := r.db.QueryRowContext(c.UserContext(), `PRAGMA integrity_check;`).Scan(&result); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, err.Error(), "danger"))
	}
	message := fmt.Sprintf("Integrity check result: %s", result)
	cls := "success"
	if strings.ToLower(strings.TrimSpace(result)) != "ok" {
		cls = "warning"
	}
	return c.Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, message, cls))
}

func (r *Runner) postAdminMaintenanceVacuumDB(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if !security.HasRole(admin.Role, security.RoleOwner) {
		return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("forbidden", r.cfg.AppName))
	}
	if r.db == nil {
		return c.Status(fiber.StatusServiceUnavailable).Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, "Database is not configured.", "warning"))
	}
	if _, err := r.db.ExecContext(c.UserContext(), `VACUUM;`); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, err.Error(), "danger"))
	}
	return c.Type("html").SendString(renderAdminMaintenancePage(r, admin.Role, "SQLite VACUUM completed.", "success"))
}

func renderAdminMaintenancePage(r *Runner, adminRole, message, alertClass string) string {
	var alert string
	if strings.TrimSpace(message) != "" {
		if strings.TrimSpace(alertClass) == "" {
			alertClass = "info"
		}
		alert = `<div class="alert alert-` + alertClass + `">` + html.EscapeString(message) + `</div>`
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Maintenance</h1><p class="text-body-secondary mb-0">Dangerous administrative operations</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/settings">Back</a></div>` + alert + `<div class="row g-3"><div class="col-12 col-lg-4"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Cleanup</div><div class="card-body"><p class="text-body-secondary">Deletes expired refresh tokens, old sync jobs, stale traffic snapshots, old webhook deliveries, and old backups according to retention.</p><form method="post" action="/admin/maintenance/cleanup"><button class="btn btn-outline-warning" type="submit">Run cleanup</button></form></div></div></div><div class="col-12 col-lg-4"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Database Check</div><div class="card-body"><p class="text-body-secondary">Runs ` + "`PRAGMA integrity_check;`" + ` on SQLite.</p><form method="post" action="/admin/maintenance/check-db"><button class="btn btn-outline-primary" type="submit">Check DB</button></form></div></div></div><div class="col-12 col-lg-4"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">VACUUM</div><div class="card-body"><p class="text-body-secondary">Reclaims free space in the SQLite database.</p><form method="post" action="/admin/maintenance/vacuum-db"><button class="btn btn-outline-danger" type="submit">Run VACUUM</button></form></div></div></div></div></div>`
	return renderAdminShell(r.cfg.AppName, adminRole, "maintenance", body)
}
