package app

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/security"
)

type dashboardSummary struct {
	TotalPanels      int64
	OnlinePanels     int64
	OfflinePanels    int64
	TotalInbounds    int64
	TotalClients     int64
	ActiveClients    int64
	DisabledClients  int64
	ExpiredClients   int64
	TotalTraffic     int64
	RunningSyncJobs  int64
	BackupCount      int64
	LastBackupAt     *time.Time
	LatestPanelCheck *time.Time
	SystemHealth     systemHealthSnapshot
	RecentSyncJobs   []dashboardSyncJob
	RecentAudits     []models.AuditLog
	RecentErrors     []dashboardError
	RecentNotices    []models.Notification
}

type dashboardSyncJob struct {
	JobType string
	Status  string
	Message string
	When    time.Time
}

type dashboardError struct {
	Name    string
	Message string
	When    time.Time
}

func (r *Runner) getAdminDashboard(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	summary, err := r.loadDashboardSummary(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderDashboardPage(admin, r.cfg.AppName, summary))
}

func (r *Runner) getAdminDashboardWidgets(c *fiber.Ctx) error {
	summary, err := r.loadDashboardSummary(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderDashboardWidgets(summary))
}

func (r *Runner) loadDashboardSummary(ctx context.Context) (dashboardSummary, error) {
	var summary dashboardSummary
	queries := []struct {
		query string
		into  *int64
	}{
		{`SELECT COUNT(*) FROM panels`, &summary.TotalPanels},
		{`SELECT COUNT(*) FROM panels WHERE status = 'online'`, &summary.OnlinePanels},
		{`SELECT COUNT(*) FROM panels WHERE status != 'online'`, &summary.OfflinePanels},
		{`SELECT COUNT(*) FROM inbounds`, &summary.TotalInbounds},
		{`SELECT COUNT(*) FROM clients`, &summary.TotalClients},
		{`SELECT COUNT(*) FROM clients WHERE status = 'active'`, &summary.ActiveClients},
		{`SELECT COUNT(*) FROM clients WHERE status = 'disabled'`, &summary.DisabledClients},
	}
	for _, item := range queries {
		if err := r.db.QueryRowContext(ctx, item.query).Scan(item.into); err != nil {
			return summary, err
		}
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients WHERE expiry_time IS NOT NULL AND expiry_time < ?`, time.Now().UTC()).Scan(&summary.ExpiredClients); err != nil {
		return summary, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(upload_bytes + download_bytes), 0) FROM client_attachments`).Scan(&summary.TotalTraffic); err != nil {
		return summary, err
	}
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_jobs WHERE status = ?`, models.SyncJobStatusRunning).Scan(&summary.RunningSyncJobs); err != nil {
		return summary, err
	}
	if err := r.loadRecentSyncJobs(ctx, &summary); err != nil {
		return summary, err
	}
	if r.audit != nil {
		audits, err := r.audit.ListRecent(ctx, 5)
		if err != nil {
			return summary, err
		}
		summary.RecentAudits = audits
	}
	summary.SystemHealth = r.collectSystemHealth(ctx)
	if r.backups != nil {
		items, err := r.backups.List(r.cfg.BackupDir)
		if err != nil {
			return summary, err
		}
		summary.BackupCount = int64(len(items))
		if len(items) > 0 {
			latest := items[0].ModifiedAt
			summary.LastBackupAt = &latest
		}
	}
	var latestPanelCheck sql.NullTime
	if err := r.db.QueryRowContext(ctx, `SELECT MAX(last_checked_at) FROM panels WHERE last_checked_at IS NOT NULL`).Scan(&latestPanelCheck); err != nil {
		return summary, err
	}
	if latestPanelCheck.Valid {
		t := latestPanelCheck.Time
		summary.LatestPanelCheck = &t
	}
	if err := r.loadRecentErrors(ctx, &summary); err != nil {
		return summary, err
	}
	if r.notifications != nil {
		notices, err := r.notifications.ListRecent(ctx, 5)
		if err != nil {
			return summary, err
		}
		summary.RecentNotices = notices
	}
	return summary, nil
}

func (r *Runner) loadRecentSyncJobs(ctx context.Context, summary *dashboardSummary) error {
	rows, err := r.db.QueryContext(ctx, `SELECT job_type, status, COALESCE(message, ''), COALESCE(finished_at, started_at, created_at) FROM sync_jobs ORDER BY id DESC LIMIT 5`)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item dashboardSyncJob
		if err := rows.Scan(&item.JobType, &item.Status, &item.Message, &item.When); err != nil {
			return err
		}
		summary.RecentSyncJobs = append(summary.RecentSyncJobs, item)
	}
	return rows.Err()
}

func (r *Runner) loadRecentErrors(ctx context.Context, summary *dashboardSummary) error {
	rows, err := r.db.QueryContext(ctx, `SELECT name, COALESCE(last_error, ''), COALESCE(updated_at, created_at) FROM panels WHERE last_error IS NOT NULL AND last_error != '' ORDER BY updated_at DESC LIMIT 5`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var item dashboardError
		if err := rows.Scan(&item.Name, &item.Message, &item.When); err != nil {
			return err
		}
		summary.RecentErrors = append(summary.RecentErrors, item)
	}
	return rows.Err()
}

func renderDashboardPage(admin *models.AdminUser, appName string, summary dashboardSummary) string {
	usersLink := ""
	if strings.EqualFold(admin.Role, "owner") || strings.EqualFold(admin.Role, "admin") {
		usersLink = `<a class="btn btn-outline-primary btn-sm" href="/admin/users">Manage admins</a>`
	}
	trafficLink := ""
	if security.HasPermission(admin.Role, security.PermissionManagePanels) {
		trafficLink = `<form method="post" action="/admin/sync/traffic"><button class="btn btn-outline-info btn-sm" type="submit">Sync traffic</button></form>`
	}
	body := `<script src="` + staticAssetURL("vendor/htmx/htmx.min.js") + `"></script><div class="d-flex flex-column gap-4"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3"><div><h1 class="h3 mb-1">Dashboard</h1><p class="text-body-secondary mb-0">Signed in as ` + html.EscapeString(admin.Username) + ` (` + html.EscapeString(admin.Role) + `)</p></div><div class="d-flex gap-2 flex-wrap">` + usersLink + trafficLink + `<a class="btn btn-outline-secondary btn-sm" href="/admin/settings">Settings</a><form method="post" action="/admin/logout"><button class="btn btn-outline-secondary btn-sm" type="submit">Logout</button></form></div></div><div id="dashboard-widgets" hx-get="/admin/dashboard/widgets" hx-trigger="load, every 30s" hx-swap="outerHTML">` + renderDashboardWidgets(summary) + `</div><div class="row g-3"><div class="col-12 col-xl-6" id="dashboard-panel-status" hx-get="/admin/dashboard/panel-status" hx-trigger="load, every 30s" hx-swap="outerHTML">` + renderDashboardMonitoring(summary) + `</div><div class="col-12 col-xl-6" id="dashboard-sync-jobs" hx-get="/admin/dashboard/sync-jobs" hx-trigger="load, every 30s" hx-swap="outerHTML">` + renderDashboardSyncJobs(summary.RecentSyncJobs) + `</div><div class="col-12 col-xl-6">` + renderDashboardAudits(summary.RecentAudits) + `</div><div class="col-12 col-xl-6"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Recent notifications</div><div class="card-body">` + renderDashboardNotifications(summary.RecentNotices) + `</div></div></div><div class="col-12"><div class="card"><div class="card-header fw-semibold">Recent errors</div><div class="card-body">` + renderDashboardErrors(summary.RecentErrors) + `</div></div></div></div></div>`
	return renderAdminShell(appName, admin.Role, "dashboard", body)
}

func renderDashboardWidgets(summary dashboardSummary) string {
	return `<div id="dashboard-widgets" class="row g-3">` + dashboardStatCard("Total panels", summary.TotalPanels, "primary") + dashboardStatCard("Online panels", summary.OnlinePanels, "success") + dashboardStatCard("Offline panels", summary.OfflinePanels, "danger") + dashboardStatCard("Running sync jobs", summary.RunningSyncJobs, "info") + dashboardStatCard("Total clients", summary.TotalClients, "primary") + dashboardStatCard("Active clients", summary.ActiveClients, "success") + dashboardStatCard("Expired clients", summary.ExpiredClients, "warning") + dashboardStatCard("Disabled clients", summary.DisabledClients, "secondary") + dashboardTrafficCard(summary.TotalTraffic) + `</div>`
}

func dashboardStatCard(title string, value int64, tone string) string {
	return `<div class="col-12 col-md-6 col-xl-3"><div class="card h-100 shadow-sm"><div class="card-body"><div class="text-body-secondary small">` + html.EscapeString(title) + `</div><div class="display-6 fw-semibold text-` + tone + `">` + html.EscapeString(fmt.Sprintf("%d", value)) + `</div></div></div></div>`
}

func dashboardTrafficCard(value int64) string {
	return `<div class="col-12 col-md-6 col-xl-3"><div class="card h-100 shadow-sm"><div class="card-body"><div class="text-body-secondary small">Total traffic</div><div class="display-6 fw-semibold">` + html.EscapeString(formatBytes(value)) + `</div></div></div></div>`
}

func renderDashboardMonitoring(summary dashboardSummary) string {
	backupTime := "No backups"
	if summary.LastBackupAt != nil {
		backupTime = summary.LastBackupAt.UTC().Format(time.RFC3339)
	}
	panelCheck := "No panel checks"
	if summary.LatestPanelCheck != nil {
		panelCheck = summary.LatestPanelCheck.UTC().Format(time.RFC3339)
	}
	return `<div class="row g-3"><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">SQLite</div><div class="fw-semibold text-` + monitorTone(summary.SystemHealth.SQLite) + `">` + html.EscapeString(summary.SystemHealth.SQLite) + `</div><div class="text-body-secondary small mt-2">Migrations: ` + html.EscapeString(summary.SystemHealth.Migrations) + `</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Redis</div><div class="fw-semibold text-` + monitorTone(summary.SystemHealth.Redis) + `">` + html.EscapeString(summary.SystemHealth.Redis) + `</div><div class="text-body-secondary small mt-2">App health: ` + html.EscapeString(summary.SystemHealth.Status) + `</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Backups</div><div class="fs-5 fw-semibold">` + html.EscapeString(fmt.Sprintf("%d", summary.BackupCount)) + `</div><div class="text-body-secondary small mt-2">Latest: ` + html.EscapeString(backupTime) + `</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Panel checks</div><div class="fs-5 fw-semibold">` + html.EscapeString(panelCheck) + `</div><div class="text-body-secondary small mt-2">Version: 0.1.0</div></div></div></div></div>`
}

func monitorTone(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "ok", "online":
		return "success"
	case "degraded", "warning":
		return "warning"
	default:
		return "danger"
	}
}

func renderDashboardSyncJobs(items []dashboardSyncJob) string {
	var rows strings.Builder
	for _, item := range items {
		rows.WriteString(`<tr><td>` + html.EscapeString(item.JobType) + `</td><td><span class="badge text-bg-secondary">` + html.EscapeString(item.Status) + `</span></td><td>` + html.EscapeString(item.Message) + `</td><td>` + html.EscapeString(item.When.Format(time.RFC3339)) + `</td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="4" class="text-body-secondary">No recent sync jobs.</td></tr>`)
	}
	return `<div class="card shadow-sm h-100"><div class="card-header fw-semibold">Recent sync jobs</div><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Type</th><th>Status</th><th>Message</th><th>When</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div>`
}

func renderDashboardAudits(items []models.AuditLog) string {
	var rows strings.Builder
	for _, item := range items {
		rows.WriteString(`<tr><td>` + html.EscapeString(item.Action) + `</td><td>` + html.EscapeString(item.ActorType) + `</td><td>` + html.EscapeString(item.TargetType) + `</td><td>` + html.EscapeString(item.CreatedAt.Format(time.RFC3339)) + `</td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="4" class="text-body-secondary">No recent audit logs.</td></tr>`)
	}
	return `<div class="card shadow-sm h-100"><div class="card-header fw-semibold">Recent audit logs</div><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Action</th><th>Actor</th><th>Target</th><th>When</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div>`
}

func renderDashboardErrors(items []dashboardError) string {
	if len(items) == 0 {
		return `<div class="text-body-secondary">No recent errors.</div>`
	}
	var out strings.Builder
	for _, item := range items {
		out.WriteString(`<div class="border rounded p-3 mb-2"><div class="fw-semibold">` + html.EscapeString(item.Name) + `</div><div class="text-danger small">` + html.EscapeString(item.Message) + `</div><div class="text-body-secondary small">` + html.EscapeString(item.When.Format(time.RFC3339)) + `</div></div>`)
	}
	return out.String()
}

func renderDashboardNotifications(items []models.Notification) string {
	if len(items) == 0 {
		return `<div class="text-body-secondary">No recent notifications.</div>`
	}
	var out strings.Builder
	for _, item := range items {
		out.WriteString(`<div class="border rounded p-3 mb-2"><div class="d-flex align-items-center justify-content-between gap-2"><div class="fw-semibold">` + html.EscapeString(item.Title) + `</div><span class="badge text-bg-` + notificationTone(item.Severity) + `">` + html.EscapeString(item.Severity) + `</span></div><div class="text-body-secondary small">` + html.EscapeString(item.Message) + `</div><div class="text-body-secondary small">` + html.EscapeString(item.CreatedAt.Format(time.RFC3339)) + `</div></div>`)
	}
	return out.String()
}
