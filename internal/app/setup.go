package app

import (
	"errors"
	"html"
	"strings"
	"unicode/utf8"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/bootstrap"
	"github.com/AmooVPM/hub/internal/security"
)

type setupForm struct {
	Username        string
	Password        string
	ConfirmPassword  string
}

func (r *Runner) getSetup(c *fiber.Ctx) error {
	required, err := r.initialSetupRequired(c.UserContext())
	if err != nil {
		return err
	}
	if !required {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderInitialSetupPage(r.cfg.AppName, nil))
}

func (r *Runner) postSetup(c *fiber.Ctx) error {
	required, err := r.initialSetupRequired(c.UserContext())
	if err != nil {
		return err
	}
	if !required {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	form := parseSetupForm(c)
	if err := validateSetupForm(form); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderInitialSetupPage(r.cfg.AppName, []string{err.Error()}))
	}
	admin, err := bootstrap.CreateInitialAdmin(c.UserContext(), r.admins, form.Username, form.Password, "", security.RoleOwner)
	if err != nil {
		if errors.Is(err, bootstrap.ErrInitialSetupCompleted) {
			return c.Redirect("/admin/login", fiber.StatusFound)
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderInitialSetupPage(r.cfg.AppName, []string{err.Error()}))
	}
	token, err := security.RandomToken(32)
	if err != nil {
		return err
	}
	if err := r.issueAdminSession(c, admin, token); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", &admin.ID, "initial_setup", "admin", &admin.ID, map[string]any{"username": admin.Username, "role": admin.Role})
	return c.Redirect("/admin", fiber.StatusFound)
}

func parseSetupForm(c *fiber.Ctx) setupForm {
	return setupForm{
		Username:       strings.TrimSpace(c.FormValue("username")),
		Password:       c.FormValue("password"),
		ConfirmPassword: c.FormValue("confirm_password"),
	}
}

func validateSetupForm(form setupForm) error {
	if form.Username == "" {
		return errors.New("username is required")
	}
	if strings.TrimSpace(form.Password) == "" {
		return errors.New("password is required")
	}
	if utf8.RuneCountInString(form.Password) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	if form.Password != form.ConfirmPassword {
		return errors.New("password confirmation does not match")
	}
	return nil
}

func renderInitialSetupPage(appName string, errors []string) string {
	var alert strings.Builder
	for _, errMsg := range errors {
		alert.WriteString(`<div class="alert alert-danger">` + html.EscapeString(errMsg) + `</div>`)
	}
	body := `<main class="container py-5" style="max-width: 560px;"><div class="card shadow-sm"><div class="card-body p-4"><h1 class="h4 mb-1">` + html.EscapeString(appName) + `</h1><p class="text-body-secondary mb-4">Initial setup: create the first admin account</p>` + alert.String() + `<form method="post" action="/setup" class="vstack gap-3"><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" autocomplete="username" required></div><div><label class="form-label" for="password">Password</label><input class="form-control" id="password" name="password" type="password" autocomplete="new-password" minlength="12" required></div><div><label class="form-label" for="confirm_password">Confirm password</label><input class="form-control" id="confirm_password" name="confirm_password" type="password" autocomplete="new-password" minlength="12" required></div><button class="btn btn-primary w-100" type="submit">Create admin</button></form></div></div></main>`
	return renderPage("Initial Setup", body)
}
