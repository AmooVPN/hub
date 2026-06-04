package app

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

func (r *Runner) postAdminTelegramTest(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if r.telegram == nil || !r.telegram.Configured() {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Telegram is not configured.", "warning"))
	}
	if err := r.telegram.SendTest(c.UserContext(), "hub telegram test from "+admin.Username); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, err.Error(), "danger"))
	}
	return c.Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Telegram test message sent.", "success"))
}

func (r *Runner) postAdminEmailTest(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if r.email == nil || !r.email.Configured() {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Email is not configured.", "warning"))
	}
	recipient := strings.TrimSpace(admin.Email)
	if recipient == "" {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Your admin account does not have an email address.", "warning"))
	}
	if err := r.email.SendTest(c.UserContext(), recipient, "hub email test", "This is a test email from hub."); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminSettingsPage(r, admin.Role, err.Error(), "danger"))
	}
	return c.Type("html").SendString(renderAdminSettingsPage(r, admin.Role, "Test email sent to "+recipient+".", "success"))
}
