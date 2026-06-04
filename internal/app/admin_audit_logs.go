package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
)

type auditLogFilters struct {
	ActorType  string
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	DateFrom   string
	DateTo     string
}

func (r *Runner) getAdminAuditLogs(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	filters := auditLogFilters{
		ActorType:  strings.TrimSpace(c.Query("actor_type")),
		ActorID:    strings.TrimSpace(c.Query("actor_id")),
		Action:     strings.TrimSpace(c.Query("action")),
		TargetType: strings.TrimSpace(c.Query("target_type")),
		TargetID:   strings.TrimSpace(c.Query("target_id")),
		DateFrom:   strings.TrimSpace(c.Query("date_from")),
		DateTo:     strings.TrimSpace(c.Query("date_to")),
	}
	items, err := r.loadAuditLogs(c.UserContext(), filters, 100)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminAuditLogsPage(items, filters, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminAuditLogDetail(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid audit log id")
	}
	item, err := r.audit.FindByID(c.UserContext(), id)
	if err != nil {
		return err
	}
	if item == nil {
		return fiber.NewError(fiber.StatusNotFound, "audit log not found")
	}
	return c.Type("html").SendString(renderAdminAuditLogDetailPage(item, r.cfg.AppName, admin.Role))
}

func (r *Runner) loadAuditLogs(ctx context.Context, filters auditLogFilters, limit int) ([]models.AuditLog, error) {
	where := []string{"1=1"}
	args := make([]any, 0)
	if filters.ActorType != "" {
		where = append(where, "actor_type = ?")
		args = append(args, filters.ActorType)
	}
	if filters.ActorID != "" {
		if id, err := strconv.ParseInt(filters.ActorID, 10, 64); err == nil {
			where = append(where, "actor_id = ?")
			args = append(args, id)
		}
	}
	if filters.Action != "" {
		where = append(where, "action = ?")
		args = append(args, filters.Action)
	}
	if filters.TargetType != "" {
		where = append(where, "target_type = ?")
		args = append(args, filters.TargetType)
	}
	if filters.TargetID != "" {
		if id, err := strconv.ParseInt(filters.TargetID, 10, 64); err == nil {
			where = append(where, "target_id = ?")
			args = append(args, id)
		}
	}
	if filters.DateFrom != "" {
		if t, err := time.Parse(time.RFC3339, filters.DateFrom); err == nil {
			where = append(where, "created_at >= ?")
			args = append(args, t.UTC())
		}
	}
	if filters.DateTo != "" {
		if t, err := time.Parse(time.RFC3339, filters.DateTo); err == nil {
			where = append(where, "created_at <= ?")
			args = append(args, t.UTC())
		}
	}
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)
	query := `SELECT id, actor_type, actor_id, action, target_type, target_id, metadata_json, created_at FROM audit_logs WHERE ` + strings.Join(where, " AND ") + ` ORDER BY id DESC LIMIT ?`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.AuditLog, 0)
	for rows.Next() {
		audit, err := scanAuditLog(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *audit)
	}
	return items, rows.Err()
}

func renderAdminAuditLogsPage(items []models.AuditLog, filters auditLogFilters, appName, adminRole string) string {
	var rows strings.Builder
	for _, item := range items {
		meta := item.MetadataJSON
		if len(meta) > 120 {
			meta = meta[:120] + "..."
		}
		rows.WriteString(`<tr><td><a href="/admin/audit-logs/` + strconv.FormatInt(item.ID, 10) + `">` + html.EscapeString(item.Action) + `</a></td><td>` + html.EscapeString(item.ActorType) + `</td><td>` + html.EscapeString(defaultAuditID(item.ActorID)) + `</td><td>` + html.EscapeString(item.TargetType) + `</td><td>` + html.EscapeString(defaultAuditID(item.TargetID)) + `</td><td>` + html.EscapeString(meta) + `</td><td>` + html.EscapeString(item.CreatedAt.Format(time.RFC3339)) + `</td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="7" class="text-body-secondary">No audit logs found.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Audit Logs</h1><p class="text-body-secondary mb-0">Search and inspect system activity</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div><div class="card shadow-sm mb-3"><div class="card-body"><form method="get" class="row g-2"><div class="col-12 col-md-3"><input class="form-control" name="actor_type" placeholder="Actor type" value="` + html.EscapeString(filters.ActorType) + `"></div><div class="col-12 col-md-2"><input class="form-control" name="actor_id" placeholder="Actor ID" value="` + html.EscapeString(filters.ActorID) + `"></div><div class="col-12 col-md-3"><input class="form-control" name="action" placeholder="Action" value="` + html.EscapeString(filters.Action) + `"></div><div class="col-12 col-md-2"><input class="form-control" name="target_type" placeholder="Target type" value="` + html.EscapeString(filters.TargetType) + `"></div><div class="col-12 col-md-2"><input class="form-control" name="target_id" placeholder="Target ID" value="` + html.EscapeString(filters.TargetID) + `"></div><div class="col-12 col-md-3"><input class="form-control" name="date_from" placeholder="From RFC3339" value="` + html.EscapeString(filters.DateFrom) + `"></div><div class="col-12 col-md-3"><input class="form-control" name="date_to" placeholder="To RFC3339" value="` + html.EscapeString(filters.DateTo) + `"></div><div class="col-12 d-flex gap-2"><button class="btn btn-primary" type="submit">Filter</button><a class="btn btn-outline-secondary" href="/admin/audit-logs">Reset</a></div></form></div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Action</th><th>Actor</th><th>Actor ID</th><th>Target</th><th>Target ID</th><th>Metadata</th><th>When</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "audit-logs", body)
}

func renderAdminAuditLogDetailPage(item *models.AuditLog, appName, adminRole string) string {
	pretty := item.MetadataJSON
	if strings.TrimSpace(pretty) != "" {
		var out bytes.Buffer
		if err := json.Indent(&out, []byte(pretty), "", "  "); err == nil {
			pretty = out.String()
		}
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Audit Log #` + strconv.FormatInt(item.ID, 10) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(item.Action) + `</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/audit-logs">Back</a></div><div class="card shadow-sm"><div class="card-body"><dl class="row mb-0"><dt class="col-sm-3">Actor type</dt><dd class="col-sm-9">` + html.EscapeString(item.ActorType) + `</dd><dt class="col-sm-3">Actor ID</dt><dd class="col-sm-9">` + html.EscapeString(defaultAuditID(item.ActorID)) + `</dd><dt class="col-sm-3">Target type</dt><dd class="col-sm-9">` + html.EscapeString(item.TargetType) + `</dd><dt class="col-sm-3">Target ID</dt><dd class="col-sm-9">` + html.EscapeString(defaultAuditID(item.TargetID)) + `</dd><dt class="col-sm-3">Created</dt><dd class="col-sm-9">` + html.EscapeString(item.CreatedAt.Format(time.RFC3339)) + `</dd></dl></div></div><div class="card shadow-sm mt-3"><div class="card-header fw-semibold">Metadata JSON</div><pre class="mb-0 p-3">` + html.EscapeString(pretty) + `</pre></div></div>`
	return renderAdminShell(appName, adminRole, "audit-logs", body)
}

func defaultAuditID(id *int64) string {
	if id == nil {
		return "-"
	}
	return strconv.FormatInt(*id, 10)
}

func scanAuditLog(scanner interface{ Scan(...any) error }) (*models.AuditLog, error) {
	var audit models.AuditLog
	var actorID, targetID sql.NullInt64
	var targetType, metadata sql.NullString
	if err := scanner.Scan(&audit.ID, &audit.ActorType, &actorID, &audit.Action, &targetType, &targetID, &metadata, &audit.CreatedAt); err != nil {
		return nil, err
	}
	if actorID.Valid {
		v := actorID.Int64
		audit.ActorID = &v
	}
	if targetID.Valid {
		v := targetID.Int64
		audit.TargetID = &v
	}
	audit.TargetType = targetType.String
	audit.MetadataJSON = metadata.String
	return &audit, nil
}
