package app

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
)

type webhookForm struct {
	Name   string
	URL    string
	Secret string
	Events string
	Active bool
}

func (r *Runner) getAdminWebhooks(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	items, err := r.webhooks.ListWebhooks(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminWebhooksPage(items, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminWebhookNew(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderAdminWebhookFormPage("New Webhook", "/admin/webhooks", webhookForm{Active: true}, r.cfg.AppName, admin.Role, nil))
}

func (r *Runner) postAdminWebhookCreate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	form := parseWebhookForm(c)
	if err := validateWebhookForm(form); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminWebhookFormPage("New Webhook", "/admin/webhooks", form, r.cfg.AppName, admin.Role, []string{err.Error()}))
	}
	webhook := &models.Webhook{Name: form.Name, URL: form.URL, Secret: form.Secret, Active: form.Active, Events: parseWebhookEvents(form.Events), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := r.webhooks.CreateWebhook(c.UserContext(), webhook); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "webhook_create", "webhook", &webhook.ID, map[string]any{"name": webhook.Name, "url": webhook.URL})
	return c.Redirect(fmt.Sprintf("/admin/webhooks/%d", webhook.ID), fiber.StatusFound)
}

func (r *Runner) getAdminWebhookDetail(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	webhook, err := r.loadWebhook(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	deliveries, err := r.webhooks.ListDeliveries(c.UserContext(), webhook.ID)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminWebhookDetailPage(webhook, deliveries, r.cfg.AppName, admin.Role, ""))
}

func (r *Runner) getAdminWebhookEdit(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	webhook, err := r.loadWebhook(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminWebhookFormPage("Edit Webhook", "/admin/webhooks/"+c.Params("id"), webhookForm{Name: webhook.Name, URL: webhook.URL, Secret: webhook.Secret, Events: strings.Join(webhook.Events, ", "), Active: webhook.Active}, r.cfg.AppName, admin.Role, nil))
}

func (r *Runner) postAdminWebhookUpdate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	webhook, err := r.loadWebhook(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	form := parseWebhookForm(c)
	if err := validateWebhookForm(form); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminWebhookFormPage("Edit Webhook", "/admin/webhooks/"+c.Params("id"), form, r.cfg.AppName, admin.Role, []string{err.Error()}))
	}
	webhook.Name = form.Name
	webhook.URL = form.URL
	webhook.Secret = form.Secret
	webhook.Active = form.Active
	webhook.Events = parseWebhookEvents(form.Events)
	webhook.UpdatedAt = time.Now().UTC()
	if err := r.webhooks.UpdateWebhook(c.UserContext(), webhook); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "webhook_update", "webhook", &webhook.ID, map[string]any{"name": webhook.Name, "url": webhook.URL})
	return c.Redirect(fmt.Sprintf("/admin/webhooks/%d", webhook.ID), fiber.StatusFound)
}

func (r *Runner) postAdminWebhookDelete(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	webhook, err := r.loadWebhook(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if err := r.webhooks.DeleteWebhook(c.UserContext(), webhook.ID); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "webhook_delete", "webhook", &webhook.ID, map[string]any{"name": webhook.Name})
	return c.Redirect("/admin/webhooks", fiber.StatusFound)
}

func (r *Runner) postAdminWebhookTest(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	webhook, err := r.loadWebhook(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if _, err := r.webhooks.DeliverToWebhook(c.UserContext(), *webhook, "webhook.test", map[string]any{"message": "Webhook test", "webhook_id": webhook.ID}); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminWebhookDetailPage(webhook, mustLoadDeliveries(r, c.UserContext(), webhook.ID), r.cfg.AppName, admin.Role, err.Error()))
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "webhook_test", "webhook", &webhook.ID, nil)
	return c.Redirect(fmt.Sprintf("/admin/webhooks/%d", webhook.ID), fiber.StatusFound)
}

func (r *Runner) getAdminWebhookDeliveries(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	webhook, err := r.loadWebhook(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	deliveries, err := r.webhooks.ListDeliveries(c.UserContext(), webhook.ID)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminWebhookDetailPage(webhook, deliveries, r.cfg.AppName, admin.Role, ""))
}

func (r *Runner) loadWebhook(ctx context.Context, idValue string) (*models.Webhook, error) {
	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid webhook id")
	}
	return r.webhooks.GetWebhook(ctx, id)
}

func parseWebhookForm(c *fiber.Ctx) webhookForm {
	return webhookForm{
		Name:   strings.TrimSpace(c.FormValue("name")),
		URL:    strings.TrimSpace(c.FormValue("url")),
		Secret: c.FormValue("secret"),
		Events: strings.TrimSpace(c.FormValue("events")),
		Active: c.FormValue("active") != "",
	}
}

func validateWebhookForm(form webhookForm) error {
	if form.Name == "" {
		return errors.New("name is required")
	}
	if _, err := url.ParseRequestURI(form.URL); err != nil {
		return errors.New("url is invalid")
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(form.URL)), "http://") && !strings.HasPrefix(strings.ToLower(strings.TrimSpace(form.URL)), "https://") {
		return errors.New("url must use http or https")
	}
	if strings.TrimSpace(form.Secret) == "" {
		return errors.New("secret is required")
	}
	events := parseWebhookEvents(form.Events)
	if len(events) == 0 {
		return errors.New("at least one event is required")
	}
	for _, event := range events {
		if !models.IsValidWebhookEvent(event) {
			return fmt.Errorf("invalid event %q", event)
		}
	}
	return nil
}

func parseWebhookEvents(raw string) []string {
	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func renderAdminWebhooksPage(items []models.Webhook, appName, adminRole string) string {
	var rows strings.Builder
	for _, item := range items {
		status := "Disabled"
		if item.Active {
			status = "Active"
		}
		rows.WriteString(`<tr><td><a href="/admin/webhooks/` + strconv.FormatInt(item.ID, 10) + `">` + html.EscapeString(item.Name) + `</a></td><td>` + html.EscapeString(item.URL) + `</td><td>` + html.EscapeString(strings.Join(item.Events, ", ")) + `</td><td>` + html.EscapeString(status) + `</td><td class="text-nowrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/webhooks/` + strconv.FormatInt(item.ID, 10) + `">View</a> <a class="btn btn-outline-primary btn-sm" href="/admin/webhooks/` + strconv.FormatInt(item.ID, 10) + `/edit">Edit</a></td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="5" class="text-body-secondary">No webhooks found.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Webhooks</h1><p class="text-body-secondary mb-0">Outgoing automation hooks</p></div><div class="d-flex gap-2"><a class="btn btn-primary btn-sm" href="/admin/webhooks/new">New webhook</a><a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Name</th><th>URL</th><th>Events</th><th>Status</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "webhooks", body)
}

func renderAdminWebhookFormPage(title, action string, form webhookForm, appName, adminRole string, errors []string) string {
	var alert strings.Builder
	for _, errMsg := range errors {
		alert.WriteString(`<div class="alert alert-danger">` + html.EscapeString(errMsg) + `</div>`)
	}
	checked := ""
	if form.Active {
		checked = " checked"
	}
	body := `<div class="container py-4 py-lg-5" style="max-width: 860px;"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(title) + `</h1><p class="text-body-secondary mb-0">Outgoing webhook configuration</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/webhooks">Back</a></div>` + alert.String() + `<div class="card shadow-sm"><div class="card-body"><form method="post" action="` + html.EscapeString(action) + `" class="vstack gap-3"><div><label class="form-label" for="name">Name</label><input class="form-control" id="name" name="name" value="` + html.EscapeString(form.Name) + `" required></div><div><label class="form-label" for="url">URL</label><input class="form-control" id="url" name="url" value="` + html.EscapeString(form.URL) + `" placeholder="https://example.com/webhook" required></div><div><label class="form-label" for="secret">Secret</label><input class="form-control" id="secret" name="secret" value="` + html.EscapeString(form.Secret) + `" required></div><div><label class="form-label" for="events">Events</label><textarea class="form-control" id="events" name="events" rows="4" placeholder="client.created, panel.offline" required>` + html.EscapeString(form.Events) + `</textarea><div class="form-text">Comma-separated event names.</div></div><div class="form-check"><input class="form-check-input" id="active" name="active" type="checkbox"` + checked + `><label class="form-check-label" for="active">Active</label></div><button class="btn btn-primary" type="submit">Save</button></form></div></div></div>`
	return renderAdminShell(appName, adminRole, "webhooks", body)
}

func renderAdminWebhookDetailPage(webhook *models.Webhook, deliveries []models.WebhookDelivery, appName, adminRole, message string) string {
	var alert string
	if strings.TrimSpace(message) != "" {
		alert = `<div class="alert alert-warning">` + html.EscapeString(message) + `</div>`
	}
	var rows strings.Builder
	for _, item := range deliveries {
		rows.WriteString(`<tr><td>` + html.EscapeString(item.EventType) + `</td><td><span class="badge text-bg-secondary">` + html.EscapeString(item.Status) + `</span></td><td>` + html.EscapeString(strconv.Itoa(item.Attempts)) + `</td><td>` + html.EscapeString(item.CreatedAt.Format(time.RFC3339)) + `</td><td>` + html.EscapeString(item.ErrorMessage) + `</td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="5" class="text-body-secondary">No deliveries yet.</td></tr>`)
	}
	status := "Disabled"
	if webhook.Active {
		status = "Active"
	}
	body := `<div class="container py-4 py-lg-5">` + alert + `<div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(webhook.Name) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(webhook.URL) + `</p></div><div class="d-flex gap-2 flex-wrap"><form method="post" action="/admin/webhooks/` + strconv.FormatInt(webhook.ID, 10) + `/test"><button class="btn btn-outline-primary btn-sm" type="submit">Test</button></form><a class="btn btn-outline-secondary btn-sm" href="/admin/webhooks/` + strconv.FormatInt(webhook.ID, 10) + `/edit">Edit</a><form method="post" action="/admin/webhooks/` + strconv.FormatInt(webhook.ID, 10) + `/delete" onsubmit="return confirm('Delete this webhook?')"><button class="btn btn-outline-danger btn-sm" type="submit">Delete</button></form><a class="btn btn-outline-secondary btn-sm" href="/admin/webhooks">Back</a></div></div><div class="row g-3 mb-3"><div class="col-12 col-lg-4"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Status</div><div class="fw-semibold">` + html.EscapeString(status) + `</div><div class="text-body-secondary small mt-2">Events: ` + html.EscapeString(strings.Join(webhook.Events, ", ")) + `</div></div></div></div><div class="col-12 col-lg-8"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Latest deliveries</div><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Event</th><th>Status</th><th>Attempts</th><th>When</th><th>Error</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div></div>`
	return renderAdminShell(appName, adminRole, "webhooks", body)
}

func mustLoadDeliveries(r *Runner, ctx context.Context, webhookID int64) []models.WebhookDelivery {
	deliveries, err := r.webhooks.ListDeliveries(ctx, webhookID)
	if err != nil {
		return nil
	}
	return deliveries
}
