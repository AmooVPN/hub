package app

import (
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/config"
)

type adminSettingsForm struct {
	AppBaseURL             string
	SessionCookieName      string
	SessionTTLHrs          string
	JWTAccessTTLMinutes    string
	JWTRefreshTTLDays      string
	BackupDir              string
	AutomaticBackupEnabled bool
	AutomaticBackupSchedule string
	BackupRetentionCount   string
	BackupRetentionDays    string
	MetricsEnabled         bool
	TrustProxy             bool
	TrustedProxies         string
	PanelURLStrictMode     bool
	PanelURLAllowPrivate   bool
	MaxUploadSizeMB        string
}

func (r *Runner) getAdminSettings(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "", ""))
}

func (r *Runner) postAdminSettings(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	form := parseAdminSettingsForm(c)
	if err := validateAdminSettingsForm(form); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, err.Error(), "danger"))
	}
	if err := applyAdminSettingsForm(r.cfg, form); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, err.Error(), "danger"))
	}
	if r.settings == nil {
		return c.Status(fiber.StatusServiceUnavailable).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Settings storage is not configured.", "warning"))
	}
	ctx := c.UserContext()
	for key, value := range r.cfg.SettingsMap() {
		if err := r.settings.Upsert(ctx, key, value); err != nil {
			return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, err.Error(), "danger"))
		}
	}
	if err := r.cfg.EnsurePaths(); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, err.Error(), "danger"))
	}
	return c.Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Settings saved.", "success"))
}

func (r *Runner) postAdminCleanup(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if r.cleanup == nil {
		return c.Status(fiber.StatusServiceUnavailable).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Cleanup service is not configured.", "warning"))
	}
	report, err := r.cleanup.Run(c.UserContext(), r.cfg.BackupDir, r.cfg.BackupRetentionCount, r.cfg.BackupRetentionDays)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, err.Error(), "danger"))
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "cleanup_run", "system", nil, map[string]any{"summary": report.Summary()})
	return c.Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Cleanup complete: "+report.Summary(), "success"))
}

func renderAdminSettingsPage(r *Runner, adminRole, message, alertClass string) string {
	alert := ""
	if strings.TrimSpace(message) != "" {
		if strings.TrimSpace(alertClass) == "" {
			alertClass = "info"
		}
		alert = `<div class="alert alert-` + html.EscapeString(alertClass) + `">` + html.EscapeString(message) + `</div>`
	}
	form := adminSettingsFormFromConfig(r.cfg)
	telegramStatus := "disabled"
	if r != nil && r.telegram != nil && r.telegram.Configured() {
		telegramStatus = "configured"
	}
	emailStatus := "disabled"
	if r != nil && r.email != nil && r.email.Configured() {
		emailStatus = "configured"
	}
	cleanupCard := `<div class="card shadow-sm"><div class="card-header fw-semibold">Maintenance</div><div class="card-body d-flex flex-wrap gap-2 align-items-center justify-content-between"><p class="text-body-secondary mb-0">Run database and file cleanup using current retention settings.</p><form method="post" action="/admin/settings/cleanup"><button class="btn btn-outline-warning" type="submit">Run cleanup now</button></form></div></div>`
	content := `<div class="container py-4 py-lg-5"><div class="d-flex flex-column gap-3">` + alert + settingsEditCard(form) + `<div class="card shadow-sm"><div class="card-header fw-semibold">Effective configuration</div><div class="card-body"><div class="table-responsive"><table class="table mb-0"><tbody>` + settingsRow("App base URL", r.cfg.AppBaseURL) + settingsRow("Session cookie", r.cfg.SessionCookieName) + settingsRow("Session TTL hours", fmt.Sprintf("%d", r.cfg.SessionTTLHrs)) + settingsRow("JWT access TTL minutes", fmt.Sprintf("%d", r.cfg.JWTAccessTTLMinutes)) + settingsRow("JWT refresh TTL days", fmt.Sprintf("%d", r.cfg.JWTRefreshTTLDays)) + settingsRow("Backup dir", r.cfg.BackupDir) + settingsRow("Automatic backups", fmt.Sprintf("%t", r.cfg.AutomaticBackupEnabled)) + settingsRow("Automatic backup schedule", r.cfg.AutomaticBackupSchedule) + settingsRow("Backup retention count", fmt.Sprintf("%d", r.cfg.BackupRetentionCount)) + settingsRow("Backup retention days", fmt.Sprintf("%d", r.cfg.BackupRetentionDays)) + settingsRow("Metrics enabled", fmt.Sprintf("%t", r.cfg.MetricsEnabled)) + settingsRow("Trust proxy", fmt.Sprintf("%t", r.cfg.TrustProxy)) + settingsRow("Trusted proxies", strings.Join(r.cfg.TrustedProxies, ", ")) + settingsRow("Panel URL strict mode", fmt.Sprintf("%t", r.cfg.PanelURLStrictMode)) + settingsRow("Panel URL allow private", fmt.Sprintf("%t", r.cfg.PanelURLAllowPrivate)) + settingsRow("Telegram notifications", telegramStatus) + settingsRow("Email notifications", emailStatus) + settingsRow("Max upload size MB", fmt.Sprintf("%d", r.cfg.MaxUploadSizeMB)) + `</tbody></table></div><p class="text-body-secondary small mt-3 mb-0">Secrets are intentionally hidden.</p></div></div>` + cleanupCard + `<div class="row g-3"><div class="col-12 col-lg-6"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Telegram</div><div class="card-body"><p class="text-body-secondary">Send a test message to the configured chat.</p><form method="post" action="/admin/settings/telegram/test"><button class="btn btn-outline-primary" type="submit">Send test message</button></form></div></div></div><div class="col-12 col-lg-6"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Email</div><div class="card-body"><p class="text-body-secondary">Send a test email to your admin address.</p><form method="post" action="/admin/settings/email/test"><button class="btn btn-outline-primary" type="submit">Send test email</button></form></div></div></div></div></div></div>`
	return renderAdminShell(r.cfg.AppName, adminRole, "settings", content)
}

func adminSettingsFormFromConfig(cfg *config.Config) adminSettingsForm {
	if cfg == nil {
		return adminSettingsForm{}
	}
	return adminSettingsForm{
		AppBaseURL:             cfg.AppBaseURL,
		SessionCookieName:      cfg.SessionCookieName,
		SessionTTLHrs:          strconv.Itoa(cfg.SessionTTLHrs),
		JWTAccessTTLMinutes:    strconv.Itoa(cfg.JWTAccessTTLMinutes),
		JWTRefreshTTLDays:      strconv.Itoa(cfg.JWTRefreshTTLDays),
		BackupDir:              cfg.BackupDir,
		AutomaticBackupEnabled: cfg.AutomaticBackupEnabled,
		AutomaticBackupSchedule: cfg.AutomaticBackupSchedule,
		BackupRetentionCount:   strconv.Itoa(cfg.BackupRetentionCount),
		BackupRetentionDays:    strconv.Itoa(cfg.BackupRetentionDays),
		MetricsEnabled:         cfg.MetricsEnabled,
		TrustProxy:             cfg.TrustProxy,
		TrustedProxies:         strings.Join(cfg.TrustedProxies, ", "),
		PanelURLStrictMode:     cfg.PanelURLStrictMode,
		PanelURLAllowPrivate:   cfg.PanelURLAllowPrivate,
		MaxUploadSizeMB:        strconv.Itoa(cfg.MaxUploadSizeMB),
	}
}

func parseAdminSettingsForm(c *fiber.Ctx) adminSettingsForm {
	return adminSettingsForm{
		AppBaseURL:             strings.TrimSpace(c.FormValue("app_base_url")),
		SessionCookieName:      strings.TrimSpace(c.FormValue("session_cookie_name")),
		SessionTTLHrs:          strings.TrimSpace(c.FormValue("session_ttl_hours")),
		JWTAccessTTLMinutes:    strings.TrimSpace(c.FormValue("jwt_access_ttl_minutes")),
		JWTRefreshTTLDays:      strings.TrimSpace(c.FormValue("jwt_refresh_ttl_days")),
		BackupDir:              strings.TrimSpace(c.FormValue("backup_dir")),
		AutomaticBackupEnabled: c.FormValue("automatic_backup_enabled") != "",
		AutomaticBackupSchedule: strings.TrimSpace(c.FormValue("automatic_backup_schedule")),
		BackupRetentionCount:   strings.TrimSpace(c.FormValue("backup_retention_count")),
		BackupRetentionDays:    strings.TrimSpace(c.FormValue("backup_retention_days")),
		MetricsEnabled:         c.FormValue("metrics_enabled") != "",
		TrustProxy:             c.FormValue("trust_proxy") != "",
		TrustedProxies:         strings.TrimSpace(c.FormValue("trusted_proxies")),
		PanelURLStrictMode:     c.FormValue("panel_url_strict_mode") != "",
		PanelURLAllowPrivate:   c.FormValue("panel_url_allow_private") != "",
		MaxUploadSizeMB:        strings.TrimSpace(c.FormValue("max_upload_size_mb")),
	}
}

func validateAdminSettingsForm(form adminSettingsForm) error {
	if form.AppBaseURL == "" {
		return fmt.Errorf("App base URL is required")
	}
	if _, err := url.ParseRequestURI(form.AppBaseURL); err != nil {
		return fmt.Errorf("App base URL is invalid: %w", err)
	}
	if form.SessionCookieName == "" {
		return fmt.Errorf("Session cookie name is required")
	}
	if form.BackupDir == "" {
		return fmt.Errorf("Backup dir is required")
	}
	if form.AutomaticBackupSchedule != "" && !isBackupScheduleValueValid(form.AutomaticBackupSchedule) {
		return fmt.Errorf("Automatic backup schedule must be daily, weekly, or monthly")
	}
	for _, item := range []struct {
		label string
		value string
	}{
		{"Session TTL hours", form.SessionTTLHrs},
		{"JWT access TTL minutes", form.JWTAccessTTLMinutes},
		{"JWT refresh TTL days", form.JWTRefreshTTLDays},
		{"Backup retention count", form.BackupRetentionCount},
		{"Backup retention days", form.BackupRetentionDays},
		{"Max upload size MB", form.MaxUploadSizeMB},
	} {
		if item.value == "" {
			return fmt.Errorf("%s is required", item.label)
		}
		if _, err := strconv.Atoi(item.value); err != nil {
			return fmt.Errorf("%s must be a number", item.label)
		}
	}
	return nil
}

func applyAdminSettingsForm(cfg *config.Config, form adminSettingsForm) error {
	if cfg == nil {
		return fmt.Errorf("config is not configured")
	}
	parsedSessionTTL, _ := strconv.Atoi(form.SessionTTLHrs)
	parsedJWTAccessTTL, _ := strconv.Atoi(form.JWTAccessTTLMinutes)
	parsedJWTRefreshTTL, _ := strconv.Atoi(form.JWTRefreshTTLDays)
	parsedRetentionCount, _ := strconv.Atoi(form.BackupRetentionCount)
	parsedRetentionDays, _ := strconv.Atoi(form.BackupRetentionDays)
	parsedMaxUpload, _ := strconv.Atoi(form.MaxUploadSizeMB)
	cfg.AppBaseURL = form.AppBaseURL
	cfg.SessionCookieName = form.SessionCookieName
	cfg.SessionTTLHrs = parsedSessionTTL
	cfg.JWTAccessTTLMinutes = parsedJWTAccessTTL
	cfg.JWTRefreshTTLDays = parsedJWTRefreshTTL
	cfg.BackupDir = form.BackupDir
	cfg.AutomaticBackupEnabled = form.AutomaticBackupEnabled
	if form.AutomaticBackupSchedule != "" {
		cfg.AutomaticBackupSchedule = strings.ToLower(form.AutomaticBackupSchedule)
	}
	cfg.BackupRetentionCount = parsedRetentionCount
	cfg.BackupRetentionDays = parsedRetentionDays
	cfg.MetricsEnabled = form.MetricsEnabled
	cfg.TrustProxy = form.TrustProxy
	cfg.TrustedProxies = splitSettingsCSV(form.TrustedProxies)
	cfg.PanelURLStrictMode = form.PanelURLStrictMode
	cfg.PanelURLAllowPrivate = form.PanelURLAllowPrivate
	cfg.MaxUploadSizeMB = parsedMaxUpload
	return nil
}

func settingsEditCard(form adminSettingsForm) string {
	return `<div class="card shadow-sm"><div class="card-header fw-semibold">Editable settings</div><div class="card-body"><form method="post" action="/admin/settings" class="row g-3"><div class="col-12 col-lg-6"><label class="form-label" for="app_base_url">App base URL</label><input class="form-control" id="app_base_url" name="app_base_url" value="` + html.EscapeString(form.AppBaseURL) + `" required></div><div class="col-12 col-lg-6"><label class="form-label" for="session_cookie_name">Session cookie name</label><input class="form-control" id="session_cookie_name" name="session_cookie_name" value="` + html.EscapeString(form.SessionCookieName) + `" required></div><div class="col-12 col-md-4"><label class="form-label" for="session_ttl_hours">Session TTL hours</label><input class="form-control" id="session_ttl_hours" name="session_ttl_hours" inputmode="numeric" value="` + html.EscapeString(form.SessionTTLHrs) + `" required></div><div class="col-12 col-md-4"><label class="form-label" for="jwt_access_ttl_minutes">JWT access TTL minutes</label><input class="form-control" id="jwt_access_ttl_minutes" name="jwt_access_ttl_minutes" inputmode="numeric" value="` + html.EscapeString(form.JWTAccessTTLMinutes) + `" required></div><div class="col-12 col-md-4"><label class="form-label" for="jwt_refresh_ttl_days">JWT refresh TTL days</label><input class="form-control" id="jwt_refresh_ttl_days" name="jwt_refresh_ttl_days" inputmode="numeric" value="` + html.EscapeString(form.JWTRefreshTTLDays) + `" required></div><div class="col-12"><label class="form-label" for="backup_dir">Backup dir</label><input class="form-control" id="backup_dir" name="backup_dir" value="` + html.EscapeString(form.BackupDir) + `" required></div><div class="col-12 col-md-4"><div class="form-check"><input class="form-check-input" id="automatic_backup_enabled" name="automatic_backup_enabled" type="checkbox"` + checkedAttr(form.AutomaticBackupEnabled) + `><label class="form-check-label" for="automatic_backup_enabled">Automatic backups</label></div></div><div class="col-12 col-md-4"><label class="form-label" for="automatic_backup_schedule">Automatic backup schedule</label><select class="form-select" id="automatic_backup_schedule" name="automatic_backup_schedule">` + scheduleOption("daily", form.AutomaticBackupSchedule) + scheduleOption("weekly", form.AutomaticBackupSchedule) + scheduleOption("monthly", form.AutomaticBackupSchedule) + `</select></div><div class="col-12 col-md-4"><label class="form-label" for="backup_retention_count">Backup retention count</label><input class="form-control" id="backup_retention_count" name="backup_retention_count" inputmode="numeric" value="` + html.EscapeString(form.BackupRetentionCount) + `" required></div><div class="col-12 col-md-4"><label class="form-label" for="backup_retention_days">Backup retention days</label><input class="form-control" id="backup_retention_days" name="backup_retention_days" inputmode="numeric" value="` + html.EscapeString(form.BackupRetentionDays) + `" required></div><div class="col-12 col-md-4"><div class="form-check"><input class="form-check-input" id="metrics_enabled" name="metrics_enabled" type="checkbox"` + checkedAttr(form.MetricsEnabled) + `><label class="form-check-label" for="metrics_enabled">Metrics enabled</label></div></div><div class="col-12 col-md-4"><div class="form-check"><input class="form-check-input" id="trust_proxy" name="trust_proxy" type="checkbox"` + checkedAttr(form.TrustProxy) + `><label class="form-check-label" for="trust_proxy">Trust proxy</label></div></div><div class="col-12"><label class="form-label" for="trusted_proxies">Trusted proxies</label><input class="form-control" id="trusted_proxies" name="trusted_proxies" value="` + html.EscapeString(form.TrustedProxies) + `" placeholder="10.0.0.0/8, 192.168.0.0/16"></div><div class="col-12 col-md-6"><div class="form-check"><input class="form-check-input" id="panel_url_strict_mode" name="panel_url_strict_mode" type="checkbox"` + checkedAttr(form.PanelURLStrictMode) + `><label class="form-check-label" for="panel_url_strict_mode">Panel URL strict mode</label></div></div><div class="col-12 col-md-6"><div class="form-check"><input class="form-check-input" id="panel_url_allow_private" name="panel_url_allow_private" type="checkbox"` + checkedAttr(form.PanelURLAllowPrivate) + `><label class="form-check-label" for="panel_url_allow_private">Allow private panel URLs</label></div></div><div class="col-12 col-md-4"><label class="form-label" for="max_upload_size_mb">Max upload size MB</label><input class="form-control" id="max_upload_size_mb" name="max_upload_size_mb" inputmode="numeric" value="` + html.EscapeString(form.MaxUploadSizeMB) + `" required></div><div class="col-12 d-flex gap-2"><button class="btn btn-primary" type="submit">Save settings</button></div></form></div></div>`
}

func checkedAttr(value bool) string {
	if value {
		return ` checked`
	}
	return ""
}

func scheduleOption(value, current string) string {
	selected := ""
	if strings.EqualFold(value, current) {
		selected = ` selected`
	}
	label := strings.Title(value)
	return `<option value="` + value + `"` + selected + `>` + label + `</option>`
}

func splitSettingsCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func isBackupScheduleValueValid(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "daily", "weekly", "monthly":
		return true
	default:
		return false
	}
}

func settingsRow(label, value string) string {
	return `<tr><th class="text-nowrap">` + html.EscapeString(label) + `</th><td>` + html.EscapeString(value) + `</td></tr>`
}
