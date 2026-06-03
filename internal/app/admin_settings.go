package app

import (
	"fmt"
	"html"

	"github.com/gofiber/fiber/v2"
)

func (r *Runner) getAdminSettings(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	content := `<div class="container py-4 py-lg-5"><div class="row g-3"><div class="col-12"><div class="card shadow-sm"><div class="card-header fw-semibold">Effective configuration</div><div class="card-body"><div class="table-responsive"><table class="table mb-0"><tbody>` + settingsRow("App base URL", r.cfg.AppBaseURL) + settingsRow("Session cookie", r.cfg.SessionCookieName) + settingsRow("Session TTL hours", fmt.Sprintf("%d", r.cfg.SessionTTLHrs)) + settingsRow("JWT access TTL minutes", fmt.Sprintf("%d", r.cfg.JWTAccessTTLMinutes)) + settingsRow("JWT refresh TTL days", fmt.Sprintf("%d", r.cfg.JWTRefreshTTLDays)) + settingsRow("Backup dir", r.cfg.BackupDir) + settingsRow("Automatic backups", fmt.Sprintf("%t", r.cfg.AutomaticBackupEnabled)) + settingsRow("Automatic backup schedule", r.cfg.AutomaticBackupSchedule) + settingsRow("Backup retention count", fmt.Sprintf("%d", r.cfg.BackupRetentionCount)) + settingsRow("Backup retention days", fmt.Sprintf("%d", r.cfg.BackupRetentionDays)) + settingsRow("Metrics enabled", fmt.Sprintf("%t", r.cfg.MetricsEnabled)) + settingsRow("Max upload size MB", fmt.Sprintf("%d", r.cfg.MaxUploadSizeMB)) + `</tbody></table></div><p class="text-body-secondary small mt-3 mb-0">Secrets are intentionally hidden.</p></div></div></div></div></div>`
	return c.Type("html").SendString(renderAdminShell(r.cfg.AppName, admin.Role, "settings", content))
}

func settingsRow(label, value string) string {
	return `<tr><th class="text-nowrap">` + html.EscapeString(label) + `</th><td>` + html.EscapeString(value) + `</td></tr>`
}
