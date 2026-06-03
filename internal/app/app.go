package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/redis/go-redis/v9"

	"github.com/AmooVPM/hub/internal/bootstrap"
	"github.com/AmooVPM/hub/internal/config"
	"github.com/AmooVPM/hub/internal/database"
	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/repositories"
	"github.com/AmooVPM/hub/internal/security"
)

type Runner struct {
	cfg         *config.Config
	db          *sql.DB
	redis       *redis.Client
	admins      repositories.AdminRepository
	clients     repositories.ClientRepository
	panels      repositories.PanelRepository
	audit       repositories.AuditRepository
	logger      *slog.Logger
	loginLocks  *attemptTracker
	server      *fiber.App
}

func New(logger *slog.Logger) (*Runner, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if err := cfg.EnsurePaths(); err != nil {
		return nil, err
	}

	db, err := database.OpenSQLite(cfg.DatabasePath)
	if err != nil {
		return nil, err
	}

	rdb, err := database.OpenRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, err
	}
	if err := database.ValidateSchemaVersion(db); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, err
	}

	admins := repositories.NewAdminRepository(db)
	clients := repositories.NewClientRepository(db)
	panels := repositories.NewPanelRepository(db)
	audit := repositories.NewAuditRepository(db)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := bootstrap.EnsureInitialAdmin(ctx, cfg, admins, logger); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, err
	}

	runner := &Runner{
		cfg:        cfg,
		db:         db,
		redis:      rdb,
		admins:     admins,
		clients:    clients,
		panels:     panels,
		audit:      audit,
		logger:     logger,
		loginLocks: newAttemptTracker(5, 15*time.Minute),
	}
	runner.server = runner.buildServer()
	return runner, nil
}

func (r *Runner) Run() error {
	return r.server.Listen(r.cfg.AppAddr)
}

func (r *Runner) buildServer() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:      r.cfg.AppName,
		ServerHeader:  r.cfg.AppName,
		ErrorHandler:  r.errorHandler,
		ReadTimeout:   5 * time.Second,
		WriteTimeout:  10 * time.Second,
		IdleTimeout:   60 * time.Second,
		BodyLimit:     int(1024 * 1024 * int64(r.cfg.MaxUploadSizeMB)),
		CaseSensitive: false,
		StrictRouting: false,
	})

	app.Use(recover.New())
	app.Get("/", r.home)
	app.Get("/healthz", r.health)
	app.Get("/admin/login", r.getAdminLogin)
	app.Post("/admin/login", r.postAdminLogin)
	app.Post("/admin/logout", r.postAdminLogout)
	app.Use("/admin", r.requireAdminSession)
	app.Get("/admin", security.RequirePermission(security.PermissionViewDashboard), r.getAdminDashboard)
	app.Get("/admin/users", security.RequirePermission(security.PermissionManageAdmins), r.getAdminUsers)
	app.Get("/admin/users/new", security.RequirePermission(security.PermissionManageAdmins), r.getAdminUserNew)
	app.Post("/admin/users", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserCreate)
	app.Get("/admin/users/:id/edit", security.RequirePermission(security.PermissionManageAdmins), r.getAdminUserEdit)
	app.Post("/admin/users/:id", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserUpdate)
	app.Post("/admin/users/:id/disable", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserDisable)
	app.Post("/admin/users/:id/enable", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserEnable)
	app.Post("/admin/users/:id/delete", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserDelete)
	app.Post("/admin/users/:id/reset-password", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserResetPassword)

	return app
}

func (r *Runner) errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
	}
	if c.Path() == "/healthz" {
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
	return c.Status(code).Type("html").SendString(renderPage("Error", `<div class="container py-5"><div class="alert alert-danger">` + html.EscapeString(err.Error()) + `</div></div>`))
}

func (r *Runner) home(c *fiber.Ctx) error {
	return c.Redirect("/admin/login", fiber.StatusFound)
}

func (r *Runner) health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":  "ok",
		"app":     r.cfg.AppName,
		"time":    time.Now().UTC(),
		"version": "1",
	})
}

func (r *Runner) getAdminLogin(c *fiber.Ctx) error {
	if _, ok := r.loadAdminFromRequest(c); ok {
		return c.Redirect("/admin", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderLoginPage("", r.cfg.AppName))
}

func (r *Runner) postAdminLogin(c *fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")
	key := c.IP() + ":" + username
	if r.loginLocks.blocked(key) {
		_ = r.logAudit(c.UserContext(), "admin", nil, "login_rate_limited", "admin", nil, map[string]any{"username": username})
		return c.Status(fiber.StatusTooManyRequests).Type("html").SendString(renderLoginPage("Too many attempts. Try again later.", r.cfg.AppName))
	}

	admin, err := r.admins.FindByUsername(c.UserContext(), username)
	if err != nil || admin == nil || !admin.Active || security.ComparePassword(password, admin.PasswordHash) != nil {
		r.loginLocks.fail(key)
		_ = r.logAudit(c.UserContext(), "admin", nil, "login_failed", "admin", nil, map[string]any{"username": username})
		return c.Status(fiber.StatusUnauthorized).Type("html").SendString(renderLoginPage("Invalid credentials.", r.cfg.AppName))
	}
	r.loginLocks.success(key)

	token, err := security.RandomToken(32)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()
	expires := time.Now().UTC().Add(time.Duration(r.cfg.SessionTTLHrs) * time.Hour)
	if err := r.redis.Set(ctx, adminSessionKey(token), admin.ID, time.Until(expires)).Err(); err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     r.cfg.SessionCookieName,
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   r.cfg.IsProduction(),
		SameSite: "Lax",
		Expires:  expires,
	})
	_ = r.logAudit(c.UserContext(), "admin", &admin.ID, "login_success", "admin", &admin.ID, map[string]any{"username": admin.Username})
	return c.Redirect("/admin", fiber.StatusFound)
}

func (r *Runner) postAdminLogout(c *fiber.Ctx) error {
	admin, ok := r.loadAdminFromRequest(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	if token := c.Cookies(r.cfg.SessionCookieName); token != "" {
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()
		_ = r.redis.Del(ctx, adminSessionKey(token)).Err()
	}
	c.Cookie(&fiber.Cookie{Name: r.cfg.SessionCookieName, Value: "", Path: "/", HTTPOnly: true, Secure: r.cfg.IsProduction(), SameSite: "Lax", Expires: time.Unix(0, 0)})
	_ = r.logAudit(c.UserContext(), "admin", &admin.ID, "logout", "admin", &admin.ID, nil)
	return c.Redirect("/admin/login", fiber.StatusFound)
}

func (r *Runner) requireAdminSession(c *fiber.Ctx) error {
	admin, ok := r.loadAdminFromRequest(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	c.Locals("admin", admin)
	return c.Next()
}

func (r *Runner) getAdminDashboard(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderDashboardPage(admin, r.cfg.AppName))
}

func (r *Runner) getAdminUsers(c *fiber.Ctx) error {
	admins, err := r.admins.List(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminUsersPage(admins, r.cfg.AppName))
}

func (r *Runner) getAdminUserNew(c *fiber.Ctx) error {
	return c.Type("html").SendString(renderAdminUserFormPage("Create Admin", "/admin/users", adminForm{}, false, r.cfg.AppName, nil))
}

func (r *Runner) postAdminUserCreate(c *fiber.Ctx) error {
	form, err := parseAdminForm(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Create Admin", "/admin/users", form, false, r.cfg.AppName, []string{err.Error()}))
	}
	if err := form.validate(true); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Create Admin", "/admin/users", form, false, r.cfg.AppName, []string{err.Error()}))
	}
	hash, err := security.HashPassword(form.Password)
	if err != nil {
		return err
	}
	admin := &models.AdminUser{Username: form.Username, Email: form.Email, PasswordHash: hash, Role: security.NormalizeRole(form.Role), Active: true}
	if err := r.admins.Create(c.UserContext(), admin); err != nil {
		return err
	}
	adminCtx, _ := currentAdmin(c)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(adminCtx), "admin_create", "admin", &admin.ID, map[string]any{"username": admin.Username, "role": admin.Role})
	return c.Redirect("/admin/users", fiber.StatusFound)
}

func (r *Runner) getAdminUserEdit(c *fiber.Ctx) error {
	adminID, err := parseParamID(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid admin id")
	}
	admin, err := r.admins.FindByID(c.UserContext(), adminID)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), adminFormFromAdmin(admin), true, r.cfg.AppName, nil))
}

func (r *Runner) postAdminUserUpdate(c *fiber.Ctx) error {
	adminID, err := parseParamID(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid admin id")
	}
	existing, err := r.admins.FindByID(c.UserContext(), adminID)
	if err != nil {
		return err
	}
	form, err := parseAdminForm(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), adminFormFromAdmin(existing), true, r.cfg.AppName, []string{err.Error()}))
	}
	if err := form.validate(false); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), form, true, r.cfg.AppName, []string{err.Error()}))
	}
	updated := *existing
	updated.Username = form.Username
	updated.Email = form.Email
	updated.Role = security.NormalizeRole(form.Role)
	if existing.Role == security.RoleOwner && updated.Role != security.RoleOwner {
		owners, err := r.admins.CountByRole(c.UserContext(), security.RoleOwner)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), adminFormFromAdmin(existing), true, r.cfg.AppName, []string{"last owner cannot be demoted"}))
		}
	}
	if err := r.admins.Update(c.UserContext(), &updated); err != nil {
		return err
	}
	adminCtx, _ := currentAdmin(c)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(adminCtx), "admin_update", "admin", &existing.ID, map[string]any{"username": updated.Username, "role": updated.Role})
	return c.Redirect("/admin/users", fiber.StatusFound)
}

func (r *Runner) postAdminUserDisable(c *fiber.Ctx) error {
	return r.toggleAdminActive(c, false)
}

func (r *Runner) postAdminUserEnable(c *fiber.Ctx) error {
	return r.toggleAdminActive(c, true)
}

func (r *Runner) toggleAdminActive(c *fiber.Ctx, active bool) error {
	adminID, err := parseParamID(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid admin id")
	}
	target, err := r.admins.FindByID(c.UserContext(), adminID)
	if err != nil {
		return err
	}
	if !active && target.Role == security.RoleOwner && target.Active {
		owners, err := r.admins.CountByRole(c.UserContext(), security.RoleOwner)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("last owner cannot be disabled", r.cfg.AppName))
		}
	}
	if err := r.admins.SetActive(c.UserContext(), adminID, active); err != nil {
		return err
	}
	adminCtx, _ := currentAdmin(c)
	action := "admin_disabled"
	if active {
		action = "admin_enabled"
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(adminCtx), action, "admin", &target.ID, map[string]any{"username": target.Username})
	return c.Redirect("/admin/users", fiber.StatusFound)
}

func (r *Runner) postAdminUserDelete(c *fiber.Ctx) error {
	adminID, err := parseParamID(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid admin id")
	}
	target, err := r.admins.FindByID(c.UserContext(), adminID)
	if err != nil {
		return err
	}
	if target.Role == security.RoleOwner && target.Active {
		owners, err := r.admins.CountByRole(c.UserContext(), security.RoleOwner)
		if err != nil {
			return err
		}
		if owners <= 1 {
			return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUsersPageMessage("last owner cannot be deleted", r.cfg.AppName))
		}
	}
	if err := r.admins.Delete(c.UserContext(), adminID); err != nil {
		return err
	}
	adminCtx, _ := currentAdmin(c)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(adminCtx), "admin_delete", "admin", &target.ID, map[string]any{"username": target.Username})
	return c.Redirect("/admin/users", fiber.StatusFound)
}

func (r *Runner) postAdminUserResetPassword(c *fiber.Ctx) error {
	adminID, err := parseParamID(c.Params("id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid admin id")
	}
	target, err := r.admins.FindByID(c.UserContext(), adminID)
	if err != nil {
		return err
	}
	password := strings.TrimSpace(c.FormValue("password"))
	if password == "" {
		password, err = security.RandomToken(16)
		if err != nil {
			return err
		}
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	if err := r.admins.UpdatePassword(c.UserContext(), adminID, hash); err != nil {
		return err
	}
	adminCtx, _ := currentAdmin(c)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(adminCtx), "admin_reset_password", "admin", &target.ID, map[string]any{"username": target.Username})
	return c.Type("html").SendString(renderPasswordResetResultPage(target.Username, password, r.cfg.AppName))
}

func (r *Runner) loadAdminFromRequest(c *fiber.Ctx) (*models.AdminUser, bool) {
	if admin, ok := currentAdmin(c); ok {
		return admin, true
	}
	token := c.Cookies(r.cfg.SessionCookieName)
	if token == "" {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()
	adminID, err := r.redis.Get(ctx, adminSessionKey(token)).Int64()
	if err != nil {
		return nil, false
	}
	admin, err := r.admins.FindByID(c.UserContext(), adminID)
	if err != nil || admin == nil || !admin.Active {
		return nil, false
	}
	return admin, true
}

func currentAdmin(c *fiber.Ctx) (*models.AdminUser, bool) {
	admin, ok := c.Locals("admin").(*models.AdminUser)
	if !ok || admin == nil {
		return nil, false
	}
	return admin, true
}

func adminActorID(admin *models.AdminUser) *int64 {
	if admin == nil {
		return nil
	}
	return &admin.ID
}

func parseParamID(value string) (int64, error) {
	return strconv.ParseInt(value, 10, 64)
}

func (r *Runner) logAudit(ctx context.Context, actorType string, actorID *int64, action string, targetType string, targetID *int64, metadata map[string]any) error {
	if r.audit == nil {
		return nil
	}
	meta := ""
	if len(metadata) != 0 {
		meta = fmt.Sprintf("%v", metadata)
	}
	return r.audit.Create(ctx, &models.AuditLog{ActorType: actorType, ActorID: actorID, Action: action, TargetType: targetType, TargetID: targetID, MetadataJSON: meta, CreatedAt: time.Now().UTC()})
}

func adminSessionKey(token string) string {
	return "session:admin:" + token
}

type attemptTracker struct {
	mu       sync.Mutex
	limit    int
	window   time.Duration
	attempts map[string][]time.Time
}

func newAttemptTracker(limit int, window time.Duration) *attemptTracker {
	return &attemptTracker{limit: limit, window: window, attempts: make(map[string][]time.Time)}
}

func (t *attemptTracker) fail(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-t.window)
	filtered := t.attempts[key][:0]
	for _, attempt := range t.attempts[key] {
		if attempt.After(cutoff) {
			filtered = append(filtered, attempt)
		}
	}
	filtered = append(filtered, time.Now())
	t.attempts[key] = filtered
}

func (t *attemptTracker) success(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.attempts, key)
}

func (t *attemptTracker) blocked(key string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	cutoff := time.Now().Add(-t.window)
	attempts := t.attempts[key]
	filtered := attempts[:0]
	for _, attempt := range attempts {
		if attempt.After(cutoff) {
			filtered = append(filtered, attempt)
		}
	}
	t.attempts[key] = filtered
	return len(filtered) >= t.limit
}

func renderPage(title, body string) string {
	return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><title>` + html.EscapeString(title) + `</title><link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet"></head><body class="bg-body-tertiary">` + body + `</body></html>`
}

func renderLoginPage(message, appName string) string {
	alert := ""
	if strings.TrimSpace(message) != "" {
		alert = `<div class="alert alert-warning">` + html.EscapeString(message) + `</div>`
	}
	return renderPage("Admin Login", `<main class="container py-5" style="max-width: 480px;"><div class="card shadow-sm"><div class="card-body p-4"><h1 class="h4 mb-1">` + html.EscapeString(appName) + `</h1><p class="text-body-secondary mb-4">Admin sign in</p>` + alert + `<form method="post" action="/admin/login" class="vstack gap-3"><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" autocomplete="username" required></div><div><label class="form-label" for="password">Password</label><input class="form-control" id="password" name="password" type="password" autocomplete="current-password" required></div><button class="btn btn-primary w-100" type="submit">Sign in</button></form></div></div></main>`)
}

func renderDashboardPage(admin *models.AdminUser, appName string) string {
	usersLink := ""
	if security.HasPermission(admin.Role, security.PermissionManageAdmins) {
		usersLink = `<a class="btn btn-outline-primary btn-sm" href="/admin/users">Manage admins</a>`
	}
	return renderPage("Admin Dashboard", `<main class="container py-5"><div class="d-flex flex-column gap-3"><div><h1 class="h3 mb-1">` + html.EscapeString(appName) + `</h1><p class="text-body-secondary mb-0">Signed in as ` + html.EscapeString(admin.Username) + ` (` + html.EscapeString(admin.Role) + `)</p></div><div class="card"><div class="card-body"><div class="d-flex justify-content-between align-items-center flex-wrap gap-2"><div><div class="fw-semibold">Admin dashboard</div><div class="text-body-secondary small">Initial milestone running</div></div><div class="d-flex gap-2">` + usersLink + `<form method="post" action="/admin/logout"><button class="btn btn-outline-secondary btn-sm" type="submit">Logout</button></form></div></div></div></div></div></main>`)
}

func renderAdminUsersPage(admins []models.AdminUser, appName string) string {
	var rows strings.Builder
	for _, admin := range admins {
		status := "active"
		if !admin.Active {
			status = "disabled"
		}
		actions := `<a class="btn btn-outline-secondary btn-sm" href="/admin/users/` + strconv.FormatInt(admin.ID, 10) + `/edit">Edit</a>`
		if admin.Active {
			actions += ` <form method="post" action="/admin/users/` + strconv.FormatInt(admin.ID, 10) + `/disable" class="d-inline"><button class="btn btn-outline-warning btn-sm" type="submit">Disable</button></form>`
		} else {
			actions += ` <form method="post" action="/admin/users/` + strconv.FormatInt(admin.ID, 10) + `/enable" class="d-inline"><button class="btn btn-outline-success btn-sm" type="submit">Enable</button></form>`
		}
		actions += ` <form method="post" action="/admin/users/` + strconv.FormatInt(admin.ID, 10) + `/reset-password" class="d-inline"><input type="hidden" name="password" value=""><button class="btn btn-outline-primary btn-sm" type="submit">Reset password</button></form>`
		actions += ` <form method="post" action="/admin/users/` + strconv.FormatInt(admin.ID, 10) + `/delete" class="d-inline" onsubmit="return confirm('Delete this admin?')"><button class="btn btn-outline-danger btn-sm" type="submit">Delete</button></form>`
		rows.WriteString(`<tr><td>` + html.EscapeString(admin.Username) + `</td><td>` + html.EscapeString(admin.Role) + `</td><td>` + html.EscapeString(status) + `</td><td class="text-nowrap">` + actions + `</td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="4" class="text-body-secondary">No admin users found.</td></tr>`)
	}
	return renderPage("Admin Users", `<main class="container py-5"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">` + html.EscapeString(appName) + `</h1><p class="text-body-secondary mb-0">Admin users</p></div><div class="d-flex gap-2"><a class="btn btn-primary btn-sm" href="/admin/users/new">New admin</a><a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div></div><div class="card"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Username</th><th>Role</th><th>Status</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div></main>`)
}

type adminForm struct {
	Username string
	Email    string
	Password string
	Role     string
}

func parseAdminForm(c *fiber.Ctx) (adminForm, error) {
	return adminForm{
		Username: strings.TrimSpace(c.FormValue("username")),
		Email:    strings.TrimSpace(c.FormValue("email")),
		Password: c.FormValue("password"),
		Role:     strings.TrimSpace(c.FormValue("role")),
	}, nil
}

func (f adminForm) validate(requirePassword bool) error {
	if f.Username == "" {
		return errors.New("username is required")
	}
	if !security.HasRole(f.Role, security.RoleOwner, security.RoleAdmin, security.RoleSupport, security.RoleReadonly) {
		return errors.New("invalid role")
	}
	if requirePassword && strings.TrimSpace(f.Password) == "" {
		return errors.New("password is required")
	}
	return nil
}

func adminFormFromAdmin(admin *models.AdminUser) adminForm {
	if admin == nil {
		return adminForm{}
	}
	return adminForm{Username: admin.Username, Email: admin.Email, Role: admin.Role}
}

func renderAdminUserFormPage(title, action string, form adminForm, editing bool, appName string, errors []string) string {
	var alert strings.Builder
	for _, errMsg := range errors {
		alert.WriteString(`<div class="alert alert-danger">` + html.EscapeString(errMsg) + `</div>`)
	}
	roleOptions := renderRoleOptions(form.Role)
	passwordLabel := "Password"
	passwordRequired := "required"
	if editing {
		passwordLabel = "Password"
		passwordRequired = ""
	}
	formHTML := `<form method="post" action="` + html.EscapeString(action) + `" class="vstack gap-3"><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" value="` + html.EscapeString(form.Username) + `" required></div><div><label class="form-label" for="email">Email</label><input class="form-control" id="email" name="email" type="email" value="` + html.EscapeString(form.Email) + `"></div><div><label class="form-label" for="role">Role</label><select class="form-select" id="role" name="role">` + roleOptions + `</select></div><div><label class="form-label" for="password">` + passwordLabel + `</label><input class="form-control" id="password" name="password" type="password" ` + passwordRequired + `></div><button class="btn btn-primary" type="submit">Save</button></form>`
	resetHTML := ""
	if editing {
		resetHTML = `<hr><form method="post" action="` + html.EscapeString(strings.Replace(action, "/edit", "/reset-password", 1)) + `" class="vstack gap-3"><div><label class="form-label" for="reset_password">Reset password</label><input class="form-control" id="reset_password" name="password" type="password" placeholder="Leave blank to auto-generate"></div><button class="btn btn-outline-primary" type="submit">Reset password</button></form>`
	}
	return renderPage(title, `<main class="container py-5" style="max-width: 720px;"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">` + html.EscapeString(appName) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(title) + `</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/users">Back</a></div>` + alert.String() + `<div class="card"><div class="card-body">` + formHTML + resetHTML + `</div></div></div></main>`)
}

func renderRoleOptions(selected string) string {
	roles := []string{security.RoleOwner, security.RoleAdmin, security.RoleSupport, security.RoleReadonly}
	var b strings.Builder
	for _, role := range roles {
		selectedAttr := ""
		if role == selected {
			selectedAttr = ` selected`
		}
		b.WriteString(`<option value="` + role + `"` + selectedAttr + `>` + strings.ToUpper(role[:1]) + role[1:] + `</option>`)
	}
	return b.String()
}

func renderPasswordResetResultPage(username, password, appName string) string {
	return renderPage("Password Reset", `<main class="container py-5" style="max-width: 640px;"><div class="card"><div class="card-body"><h1 class="h4">` + html.EscapeString(appName) + `</h1><p class="text-body-secondary">Password reset for ` + html.EscapeString(username) + `</p><div class="alert alert-success"><div class="fw-semibold mb-1">New password</div><code>` + html.EscapeString(password) + `</code></div><a class="btn btn-outline-secondary" href="/admin/users">Back to admin users</a></div></div></main>`)
}

func renderAdminUsersPageMessage(message, appName string) string {
	return renderPage("Admin Users", `<main class="container py-5"><div class="alert alert-danger">` + html.EscapeString(message) + `</div><a class="btn btn-outline-secondary" href="/admin/users">Back to admin users</a></main>`)
}
