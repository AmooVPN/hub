package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"html"
	"log/slog"
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
	app.Get("/admin", r.getAdminDashboard)

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
	if _, ok := r.loadAdminFromRequest(c); !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Next()
}

func (r *Runner) getAdminDashboard(c *fiber.Ctx) error {
	admin, ok := r.loadAdminFromRequest(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderDashboardPage(admin, r.cfg.AppName))
}

func (r *Runner) loadAdminFromRequest(c *fiber.Ctx) (*models.AdminUser, bool) {
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
	return renderPage("Admin Dashboard", `<main class="container py-5"><div class="d-flex flex-column gap-3"><div><h1 class="h3 mb-1">` + html.EscapeString(appName) + `</h1><p class="text-body-secondary mb-0">Signed in as ` + html.EscapeString(admin.Username) + `</p></div><div class="card"><div class="card-body"><div class="d-flex justify-content-between align-items-center"><div><div class="fw-semibold">Admin dashboard</div><div class="text-body-secondary small">Initial milestone running</div></div><form method="post" action="/admin/logout"><button class="btn btn-outline-secondary btn-sm" type="submit">Logout</button></form></div></div></div></div></main>`)
}
