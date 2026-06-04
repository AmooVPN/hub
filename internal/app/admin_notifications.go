package app

import (
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/models"
)

func (r *Runner) getAdminNotifications(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	items, err := r.notifications.ListRecent(c.UserContext(), 50)
	if err != nil {
		return err
	}
	unread, err := r.notifications.CountUnread(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminNotificationsPage(items, unread, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminNotificationBell(c *fiber.Ctx) error {
	if r.notifications == nil {
		return c.Type("html").SendString(renderAdminNotificationBell(0))
	}
	unread, err := r.notifications.CountUnread(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminNotificationBell(unread))
}

func (r *Runner) postAdminNotificationRead(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	_ = admin
	if r.notifications == nil {
		return c.Redirect("/admin/notifications", fiber.StatusFound)
	}
	id, err := strconv.ParseInt(c.Params("id"), 10, 64)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid notification id")
	}
	if err := r.notifications.MarkRead(c.UserContext(), id); err != nil {
		return err
	}
	return c.Redirect("/admin/notifications", fiber.StatusFound)
}

func (r *Runner) postAdminNotificationsReadAll(c *fiber.Ctx) error {
	if _, ok := currentAdmin(c); !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if r.notifications != nil {
		if err := r.notifications.MarkAllRead(c.UserContext()); err != nil {
			return err
		}
	}
	return c.Redirect("/admin/notifications", fiber.StatusFound)
}

func renderAdminNotificationBell(unread int64) string {
	badge := ""
	if unread > 0 {
		badge = `<span class="badge text-bg-danger ms-2">` + html.EscapeString(strconv.FormatInt(unread, 10)) + `</span>`
	}
	return `<a class="btn btn-outline-secondary btn-sm" href="/admin/notifications">Notifications` + badge + `</a>`
}

func renderAdminNotificationsPage(items []models.Notification, unread int64, appName, adminRole string) string {
	var rows strings.Builder
	for _, item := range items {
		readState := "Unread"
		if item.ReadAt != nil {
			readState = "Read"
		}
		actions := `<form method="post" action="/admin/notifications/` + strconv.FormatInt(item.ID, 10) + `/read" class="d-inline"><button class="btn btn-outline-secondary btn-sm" type="submit">Mark read</button></form>`
		if item.ReadAt != nil {
			actions = `<span class="text-body-secondary small">Read</span>`
		}
		rows.WriteString(`<tr><td><span class="badge text-bg-` + notificationTone(item.Severity) + `">` + html.EscapeString(item.Severity) + `</span></td><td>` + html.EscapeString(item.Title) + `</td><td>` + html.EscapeString(item.Message) + `</td><td>` + html.EscapeString(readState) + `</td><td>` + html.EscapeString(item.CreatedAt.Format(time.RFC3339)) + `</td><td class="text-nowrap">` + actions + `</td></tr>`)
 	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="6" class="text-body-secondary">No notifications found.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Notifications</h1><p class="text-body-secondary mb-0">Unread: ` + html.EscapeString(strconv.FormatInt(unread, 10)) + `</p></div><form method="post" action="/admin/notifications/read-all"><button class="btn btn-outline-primary btn-sm" type="submit">Mark all read</button></form></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Severity</th><th>Title</th><th>Message</th><th>Status</th><th>When</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "notifications", body)
}

func notificationTone(severity string) string {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case models.NotificationSeveritySuccess:
		return "success"
	case models.NotificationSeverityWarning:
		return "warning"
	case models.NotificationSeverityDanger:
		return "danger"
	default:
		return "info"
	}
}
