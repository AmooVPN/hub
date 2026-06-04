package app

import (
	"fmt"
	"html"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (r *Runner) getAdminSettings(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "", ""))
}

func renderAdminSettingsPage(r *Runner, adminRole, message, alertClass string) string {
	alert := ""
	if strings.TrimSpace(message) != "" {
		if strings.TrimSpace(alertClass) == "" {
			alertClass = "info"
		}
		alert = `<div class="alert alert-` + html.EscapeString(alertClass) + `">` + html.EscapeString(message) + `</div>`
	}
	telegramStatus := "disabled"
	if r != nil && r.telegram != nil && r.telegram.Configured() {
		telegramStatus = "configured"
	}
	emailStatus := "disabled"
	if r != nil && r.email != nil && r.email.Configured() {
		emailStatus = "configured"
	}
	content := `<div class="container py-4 py-lg-5"><div class="d-flex flex-column gap-3">` + alert + `<div class="card shadow-sm"><div class="card-header fw-semibold">Effective configuration</div><div class="card-body"><div class="table-responsive"><table class="table mb-0"><tbody>` + settingsRow("App base URL", r.cfg.AppBaseURL) + settingsRow("Session cookie", r.cfg.SessionCookieName) + settingsRow("Session TTL hours", fmt.Sprintf("%d", r.cfg.SessionTTLHrs)) + settingsRow("JWT access TTL minutes", fmt.Sprintf("%d", r.cfg.JWTAccessTTLMinutes)) + settingsRow("JWT refresh TTL days", fmt.Sprintf("%d", r.cfg.JWTRefreshTTLDays)) + settingsRow("Backup dir", r.cfg.BackupDir) + settingsRow("Automatic backups", fmt.Sprintf("%t", r.cfg.AutomaticBackupEnabled)) + settingsRow("Automatic backup schedule", r.cfg.AutomaticBackupSchedule) + settingsRow("Backup retention count", fmt.Sprintf("%d", r.cfg.BackupRetentionCount)) + settingsRow("Backup retention days", fmt.Sprintf("%d", r.cfg.BackupRetentionDays)) + settingsRow("Metrics enabled", fmt.Sprintf("%t", r.cfg.MetricsEnabled)) + settingsRow("Trust proxy", fmt.Sprintf("%t", r.cfg.TrustProxy)) + settingsRow("Trusted proxies", strings.Join(r.cfg.TrustedProxies, ", ")) + settingsRow("Telegram notifications", telegramStatus) + settingsRow("Email notifications", emailStatus) + settingsRow("Max upload size MB", fmt.Sprintf("%d", r.cfg.MaxUploadSizeMB)) + `</tbody></table></div><p class="text-body-secondary small mt-3 mb-0">Secrets are intentionally hidden.</p></div></div><div class="row g-3"><div class="col-12 col-lg-6"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Telegram</div><div class="card-body"><p class="text-body-secondary">Send a test message to the configured chat.</p><form method="post" action="/admin/settings/telegram/test"><button class="btn btn-outline-primary" type="submit">Send test message</button></form></div></div></div><div class="col-12 col-lg-6"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Email</div><div class="card-body"><p class="text-body-secondary">Send a test email to your admin address.</p><form method="post" action="/admin/settings/email/test"><button class="btn btn-outline-primary" type="submit">Send test email</button></form></div></div></div></div></div></div>`
	return renderAdminShell(r.cfg.AppName, adminRole, "settings", content)
}

func settingsRow(label, value string) string {
	return `<tr><th class="text-nowrap">` + html.EscapeString(label) + `</th><td>` + html.EscapeString(value) + `</td></tr>`
}
