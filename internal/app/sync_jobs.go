package app

import (
	"context"
	"database/sql"
	"errors"
	"html"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
)

type syncJobListRow struct {
	ID        int64
	PanelName string
	JobType   string
	Status    string
	Message   string
	RetryCount int64
	When      time.Time
}

type syncJobRetryKey struct{}

func withSyncJobRetryCount(ctx context.Context, retryCount int64) context.Context {
	return context.WithValue(ctx, syncJobRetryKey{}, retryCount)
}

func syncJobRetryCountFromContext(ctx context.Context) int64 {
	if value, ok := ctx.Value(syncJobRetryKey{}).(int64); ok {
		return value
	}
	return 0
}

func (r *Runner) getAdminSyncJobs(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	rows, err := r.loadSyncJobList(c.UserContext(), c.Query("status"), c.Query("panel_id"))
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderSyncJobsListPage(rows, c.Query("status"), c.Query("panel_id"), r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminSyncJobDetail(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid job id")
	}
	row, err := r.loadSyncJobDetail(c.UserContext(), id)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderSyncJobDetailPage(row, r.cfg.AppName, admin.Role))
}

func (r *Runner) postAdminSyncJobRetry(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid job id")
	}
	job, err := r.jobs.Get(c.UserContext(), id)
	if err != nil {
		return err
	}
	if !strings.EqualFold(job.Status, models.SyncJobStatusFailed) {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderSyncJobsMessagePage("Only failed jobs can be retried.", r.cfg.AppName, admin.Role))
	}
	if job.JobType != "traffic_sync" && job.PanelID == nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderSyncJobsMessagePage("This job cannot be retried safely.", r.cfg.AppName, admin.Role))
	}
	if err := r.retrySyncJob(c.UserContext(), job); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderSyncJobsMessagePage(err.Error(), r.cfg.AppName, admin.Role))
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "job_retry", "sync_job", &job.ID, map[string]any{"job_type": job.JobType, "panel_id": job.PanelID})
	return c.Redirect(fmt.Sprintf("/admin/sync-jobs/%d", job.ID), fiber.StatusFound)
}

func (r *Runner) loadSyncJobList(ctx context.Context, statusFilter, panelFilter string) ([]syncJobListRow, error) {
	query := `SELECT sj.id, COALESCE(p.name, ''), sj.job_type, sj.status, COALESCE(sj.message, ''), sj.retry_count, COALESCE(sj.finished_at, sj.started_at, sj.created_at) FROM sync_jobs sj LEFT JOIN panels p ON p.id = sj.panel_id`
	clauses := make([]string, 0, 2)
	args := make([]any, 0, 2)
	if statusFilter = strings.TrimSpace(statusFilter); statusFilter != "" {
		clauses = append(clauses, `sj.status = ?`)
		args = append(args, statusFilter)
	}
	if panelFilter = strings.TrimSpace(panelFilter); panelFilter != "" {
		panelID, err := strconv.ParseInt(panelFilter, 10, 64)
		if err != nil {
			return nil, fiber.NewError(fiber.StatusBadRequest, "invalid panel filter")
		}
		clauses = append(clauses, `sj.panel_id = ?`)
		args = append(args, panelID)
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` ORDER BY sj.id DESC LIMIT 100`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]syncJobListRow, 0)
	for rows.Next() {
		var item syncJobListRow
		if err := rows.Scan(&item.ID, &item.PanelName, &item.JobType, &item.Status, &item.Message, &item.RetryCount, &item.When); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Runner) loadSyncJobDetail(ctx context.Context, id int64) (*syncJobListRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT sj.id, COALESCE(p.name, ''), sj.job_type, sj.status, COALESCE(sj.message, ''), sj.retry_count, COALESCE(sj.finished_at, sj.started_at, sj.created_at) FROM sync_jobs sj LEFT JOIN panels p ON p.id = sj.panel_id WHERE sj.id = ?`, id)
	var item syncJobListRow
	if err := row.Scan(&item.ID, &item.PanelName, &item.JobType, &item.Status, &item.Message, &item.RetryCount, &item.When); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fiber.NewError(fiber.StatusNotFound, "job not found")
		}
		return nil, err
	}
	return &item, nil
}

func renderSyncJobsListPage(items []syncJobListRow, statusFilter, panelFilter, appName, adminRole string) string {
	var rows strings.Builder
	for _, item := range items {
		rows.WriteString(`<tr><td><a href="/admin/sync-jobs/` + strconv.FormatInt(item.ID, 10) + `">#` + strconv.FormatInt(item.ID, 10) + `</a></td><td>` + html.EscapeString(defaultString(item.PanelName, "-")) + `</td><td>` + html.EscapeString(item.JobType) + `</td><td>` + syncJobStatusBadge(item.Status) + `</td><td>` + html.EscapeString(item.Message) + `</td><td>` + html.EscapeString(strconv.FormatInt(item.RetryCount, 10)) + `</td><td>` + html.EscapeString(item.When.Format(time.RFC3339)) + `</td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="7" class="text-body-secondary">No sync jobs found.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Sync Jobs</h1><p class="text-body-secondary mb-0">Recent sync and traffic jobs</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div><div class="card shadow-sm mb-3"><div class="card-body"><form method="get" class="row g-2 align-items-end"><div class="col-12 col-md-4"><label class="form-label" for="status">Status</label><input class="form-control" id="status" name="status" value="` + html.EscapeString(statusFilter) + `" placeholder="running"></div><div class="col-12 col-md-4"><label class="form-label" for="panel_id">Panel ID</label><input class="form-control" id="panel_id" name="panel_id" value="` + html.EscapeString(panelFilter) + `" placeholder="123"></div><div class="col-12 col-md-4"><button class="btn btn-primary w-100" type="submit">Filter</button></div></form></div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>ID</th><th>Panel</th><th>Type</th><th>Status</th><th>Message</th><th>Retries</th><th>When</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "sync-jobs", body)
}

func renderSyncJobDetailPage(item *syncJobListRow, appName, adminRole string) string {
	retryButton := ""
	if strings.EqualFold(item.Status, models.SyncJobStatusFailed) {
		retryButton = `<form method="post" action="/admin/sync-jobs/` + strconv.FormatInt(item.ID, 10) + `/retry"><button class="btn btn-outline-primary btn-sm" type="submit">Retry</button></form>`
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Sync Job #` + strconv.FormatInt(item.ID, 10) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(defaultString(item.PanelName, "No panel")) + `</p></div><div class="d-flex gap-2">` + retryButton + `<a class="btn btn-outline-secondary btn-sm" href="/admin/sync-jobs">Back</a></div></div><div class="card shadow-sm"><div class="card-body"><dl class="row mb-0"><dt class="col-sm-3">Panel</dt><dd class="col-sm-9">` + html.EscapeString(defaultString(item.PanelName, "-")) + `</dd><dt class="col-sm-3">Type</dt><dd class="col-sm-9">` + html.EscapeString(item.JobType) + `</dd><dt class="col-sm-3">Status</dt><dd class="col-sm-9">` + syncJobStatusBadge(item.Status) + `</dd><dt class="col-sm-3">Retries</dt><dd class="col-sm-9">` + html.EscapeString(strconv.FormatInt(item.RetryCount, 10)) + `</dd><dt class="col-sm-3">Message</dt><dd class="col-sm-9">` + html.EscapeString(defaultString(item.Message, "-")) + `</dd><dt class="col-sm-3">When</dt><dd class="col-sm-9">` + html.EscapeString(item.When.Format(time.RFC3339)) + `</dd></dl></div></div></div>`
	return renderAdminShell(appName, adminRole, "sync-jobs", body)
}

func (r *Runner) retrySyncJob(ctx context.Context, job *models.SyncJob) error {
	if job == nil {
		return errors.New("job is nil")
	}
	if !strings.EqualFold(job.Status, models.SyncJobStatusFailed) {
		return errors.New("job must be failed before retry")
	}
	now := time.Now().UTC()
	switch job.JobType {
	case "panel_sync", "inbound_sync":
		if job.PanelID == nil {
			return errors.New("job is missing panel reference")
		}
		ctx = withSyncJobRetryCount(ctx, job.RetryCount+1)
		panel, err := r.panels.FindByID(ctx, *job.PanelID)
		if err != nil {
			return err
		}
		panel.Status = models.PanelStatusSyncing
		panel.UpdatedAt = now
		if err := r.panels.Update(ctx, panel); err != nil {
			return err
		}
		return r.syncPanelInbounds(ctx, panel)
	case "traffic_sync":
		if job.PanelID == nil {
			ctx = withSyncJobRetryCount(ctx, job.RetryCount+1)
			_, err := r.syncAllTraffic(ctx)
			return err
		}
		ctx = withSyncJobRetryCount(ctx, job.RetryCount+1)
		panel, err := r.panels.FindByID(ctx, *job.PanelID)
		if err != nil {
			return err
		}
		return r.syncPanelTraffic(ctx, panel)
	default:
		return errors.New("job type cannot be retried safely")
	}
}

func renderSyncJobsMessagePage(message, appName, adminRole string) string {
	body := `<div class="container py-5"><div class="alert alert-danger">` + html.EscapeString(message) + `</div><a class="btn btn-outline-secondary" href="/admin/sync-jobs">Back to sync jobs</a></div>`
	return renderAdminShell(appName, adminRole, "sync-jobs", body)
}

func syncJobStatusBadge(status string) string {
	class := "secondary"
	switch strings.ToLower(strings.TrimSpace(status)) {
	case models.SyncJobStatusSuccess:
		class = "success"
	case models.SyncJobStatusFailed:
		class = "danger"
	case models.SyncJobStatusRunning:
		class = "info"
	case models.SyncJobStatusQueued:
		class = "warning"
	case models.SyncJobStatusCancelled:
		class = "dark"
	}
	return `<span class="badge text-bg-` + class + `">` + html.EscapeString(status) + `</span>`
}
