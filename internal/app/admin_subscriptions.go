package app

import (
	"context"
	"database/sql"
	"fmt"
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/security"
)

type adminSubscriptionRow struct {
	models.Client
	ActiveConfigs int
	LastCacheAt   *time.Time
}

func (r *Runner) getAdminSubscriptions(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	rows, err := r.loadAdminSubscriptionRows(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminSubscriptionsPage(rows, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminSubscriptionDetail(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("client_id"))
	if err != nil {
		return err
	}
	row, err := r.loadAdminSubscriptionRow(c.UserContext(), client.ID)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminSubscriptionDetailPage(row, r.cfg.AppName, admin.Role, r.cfg.AppBaseURL))
}

func (r *Runner) postAdminSubscriptionRegenerate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("client_id"))
	if err != nil {
		return err
	}
	newToken, err := security.RandomToken(32)
	if err != nil {
		return err
	}
	oldToken := client.SubscriptionToken
	client.SubscriptionToken = newToken
	if err := r.clients.UpdateSubscriptionToken(c.UserContext(), client.ID, newToken); err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), oldToken)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "subscription_regenerate", "client", &client.ID, map[string]any{"old_token": "redacted"})
	return c.Redirect(fmt.Sprintf("/admin/subscriptions/%d", client.ID), fiber.StatusFound)
}

func (r *Runner) postAdminSubscriptionInvalidateCache(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("client_id"))
	if err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "subscription_cache_invalidate", "client", &client.ID, nil)
	return c.Redirect(fmt.Sprintf("/admin/subscriptions/%d", client.ID), fiber.StatusFound)
}

func (r *Runner) loadAdminSubscriptionRows(ctx context.Context) ([]adminSubscriptionRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id, c.username, c.password_hash, c.display_name, c.email, c.status, c.traffic_limit_bytes, c.expiry_time, c.subscription_token, c.created_at, c.updated_at, COALESCE(SUM(CASE WHEN a.enabled = 1 THEN 1 ELSE 0 END), 0) FROM clients c LEFT JOIN client_attachments a ON a.client_id = c.id GROUP BY c.id ORDER BY c.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]adminSubscriptionRow, 0)
	for rows.Next() {
		row, err := scanAdminSubscriptionRow(rows)
		if err != nil {
			return nil, err
		}
		if cachedAt, ok := r.loadSubscriptionCacheMeta(ctx, row.SubscriptionToken); ok {
			row.LastCacheAt = &cachedAt
		}
		items = append(items, *row)
	}
	return items, rows.Err()
}

func (r *Runner) loadAdminSubscriptionRow(ctx context.Context, clientID int64) (*adminSubscriptionRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT c.id, c.username, c.password_hash, c.display_name, c.email, c.status, c.traffic_limit_bytes, c.expiry_time, c.subscription_token, c.created_at, c.updated_at, COALESCE(SUM(CASE WHEN a.enabled = 1 THEN 1 ELSE 0 END), 0) FROM clients c LEFT JOIN client_attachments a ON a.client_id = c.id WHERE c.id = ? GROUP BY c.id`, clientID)
	item, err := scanAdminSubscriptionRow(row)
	if err != nil {
		return nil, err
	}
	if cachedAt, ok := r.loadSubscriptionCacheMeta(ctx, item.SubscriptionToken); ok {
		item.LastCacheAt = &cachedAt
	}
	return item, nil
}

func scanAdminSubscriptionRow(scanner interface{ Scan(...any) error }) (*adminSubscriptionRow, error) {
	var row adminSubscriptionRow
	var displayName, email, subscriptionToken sql.NullString
	var expiryTime sql.NullTime
	if err := scanner.Scan(&row.ID, &row.Username, &row.PasswordHash, &displayName, &email, &row.Status, &row.TrafficLimitBytes, &expiryTime, &subscriptionToken, &row.CreatedAt, &row.UpdatedAt, &row.ActiveConfigs); err != nil {
		return nil, err
	}
	row.DisplayName = displayName.String
	row.Email = email.String
	row.SubscriptionToken = subscriptionToken.String
	if expiryTime.Valid {
		t := expiryTime.Time
		row.ExpiryTime = &t
	}
	return &row, nil
}

func renderAdminSubscriptionsPage(items []adminSubscriptionRow, appName, adminRole string) string {
	var rows strings.Builder
	for _, item := range items {
		cacheText := "-"
		if item.LastCacheAt != nil {
			cacheText = html.EscapeString(item.LastCacheAt.UTC().Format(time.RFC3339))
		}
		rows.WriteString(`<tr><td><a href="/admin/subscriptions/` + strconv.FormatInt(item.ID, 10) + `">` + html.EscapeString(item.Username) + `</a></td><td>` + html.EscapeString(item.Status) + `</td><td>` + html.EscapeString(strconv.Itoa(item.ActiveConfigs)) + `</td><td>` + cacheText + `</td><td class="text-nowrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/subscriptions/` + strconv.FormatInt(item.ID, 10) + `">View</a></td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="5" class="text-body-secondary">No subscriptions found.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Subscriptions</h1><p class="text-body-secondary mb-0">Client subscription management</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Client</th><th>Status</th><th>Active configs</th><th>Last cache</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "subscriptions", body)
}

func renderAdminSubscriptionDetailPage(item *adminSubscriptionRow, appName, adminRole, baseURL string) string {
	cacheText := "-"
	if item.LastCacheAt != nil {
		cacheText = item.LastCacheAt.UTC().Format(time.RFC3339)
	}
	subscriptionURL := strings.TrimRight(baseURL, "/") + "/sub/" + item.SubscriptionToken
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(item.Username) + ` subscription</h1><p class="text-body-secondary mb-0">Manage token and cache</p></div><div class="d-flex gap-2 flex-wrap"><form method="post" action="/admin/subscriptions/` + strconv.FormatInt(item.ID, 10) + `/regenerate"><button class="btn btn-outline-primary btn-sm" type="submit">Regenerate token</button></form><form method="post" action="/admin/subscriptions/` + strconv.FormatInt(item.ID, 10) + `/invalidate-cache"><button class="btn btn-outline-warning btn-sm" type="submit">Invalidate cache</button></form><a class="btn btn-outline-secondary btn-sm" href="/admin/subscriptions">Back</a></div></div><div class="row g-3"><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Subscription details</div><div class="card-body"><dl class="row mb-0"><dt class="col-sm-4">Status</dt><dd class="col-sm-8">` + html.EscapeString(item.Status) + `</dd><dt class="col-sm-4">Active configs</dt><dd class="col-sm-8">` + html.EscapeString(strconv.Itoa(item.ActiveConfigs)) + `</dd><dt class="col-sm-4">Last cache</dt><dd class="col-sm-8">` + html.EscapeString(cacheText) + `</dd></dl></div></div></div><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Links</div><div class="card-body vstack gap-3"><div><div class="text-body-secondary small">Subscription URL</div><div class="input-group"><input class="form-control" value="` + html.EscapeString(subscriptionURL) + `" readonly><button class="btn btn-outline-secondary" type="button" onclick="navigator.clipboard.writeText(this.previousElementSibling.value)">Copy</button></div></div><div><div class="text-body-secondary small">Token</div><div class="input-group"><input class="form-control" value="` + html.EscapeString(item.SubscriptionToken) + `" readonly><button class="btn btn-outline-secondary" type="button" onclick="navigator.clipboard.writeText(this.previousElementSibling.value)">Copy</button></div></div></div></div></div></div></div>`
	return renderAdminShell(appName, adminRole, "subscriptions", body)
}
