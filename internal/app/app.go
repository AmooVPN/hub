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
	"github.com/AmooVPM/hub/internal/services"
)

type Runner struct {
	cfg            *config.Config
	db             *sql.DB
	redis          *redis.Client
	admins         repositories.AdminRepository
	clients        repositories.ClientRepository
	clientsSvc     *services.ClientService
	refreshes      repositories.RefreshTokenRepository
	panels         repositories.PanelRepository
	inbounds       repositories.InboundRepository
	jobs           *services.JobService
	backgroundJobs *services.BackgroundJobRunner
	metrics        *services.MetricsCollector
	panelHealth    *services.HealthService
	backups        *services.BackupService
	cleanup        *services.CleanupService
	audit          repositories.AuditRepository
	webhooks       *services.WebhookService
	notifications  *services.NotificationService
	telegram       *services.TelegramNotifier
	email          *services.EmailNotifier
	events         services.EventBus
	logger         *slog.Logger
	loginLocks     *attemptTracker
	server         *fiber.App
}

const clientSessionCookieName = "hub_client_session"

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
	refreshes := repositories.NewRefreshTokenRepository(db)
	panels := repositories.NewPanelRepository(db)
	inbounds := repositories.NewInboundRepository(db)
	syncJobs := repositories.NewSyncJobRepository(db)
	audit := repositories.NewAuditRepository(db)
	webhookDefs := repositories.NewWebhookRepository(db)
	webhookDeliveries := repositories.NewWebhookDeliveryRepository(db)
	notifications := repositories.NewNotificationRepository(db)
	backupService := services.NewBackupService()
	telegram := services.NewTelegramNotifier(cfg.TelegramBotToken, cfg.TelegramChatID)
	email := services.NewEmailNotifier(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := bootstrap.EnsureInitialAdmin(ctx, cfg, admins, logger); err != nil {
		_ = db.Close()
		_ = rdb.Close()
		return nil, err
	}

	runner := &Runner{
		cfg:            cfg,
		db:             db,
		redis:          rdb,
		admins:         admins,
		clients:        clients,
		clientsSvc:     services.NewClientService(clients),
		refreshes:      refreshes,
		panels:         panels,
		inbounds:       inbounds,
		jobs:           services.NewJobService(syncJobs),
		backgroundJobs: services.NewBackgroundJobRunner(syncJobs, 2),
		metrics:        services.NewMetricsCollector(),
		panelHealth:    services.NewHealthService(services.NewXUIHealthProbe(cfg.AppName, cfg.HUBSecretKey)),
		backups:        backupService,
		cleanup:        services.NewCleanupService(db, syncJobs, backupService),
		audit:          audit,
		webhooks:       services.NewWebhookService(webhookDefs, webhookDeliveries),
		notifications:  services.NewNotificationService(notifications, telegram, email),
		telegram:       telegram,
		email:          email,
		events:         services.NewEventBus(),
		logger:         logger,
		loginLocks:     newAttemptTracker(5, 15*time.Minute),
	}
	runner.server = runner.buildServer()
	runner.backgroundJobs.Start(context.Background())
	runner.startAutomaticBackups()
	runner.startAutomaticWebhookRetries()
	return runner, nil
}

func (r *Runner) Run() error {
	return r.server.Listen(r.cfg.AppAddr)
}

func (r *Runner) buildServer() *fiber.App {
	app := fiber.New(fiber.Config{
		AppName:       r.cfg.AppName,
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
	app.Use(r.securityHeadersMiddleware())
	app.Use(r.requestIDMiddleware())
	app.Use(r.csrfMiddleware())
	app.Use(r.assetCacheMiddleware())
	app.Static("/static", staticAssetDir())
	if r.cfg.MetricsEnabled && r.metrics != nil {
		app.Use(r.metricsMiddleware())
	}
	app.Get("/", r.home)
	app.Get("/health", r.getHealth)
	app.Get("/health/live", r.getHealthLive)
	app.Get("/health/ready", r.getHealthReady)
	app.Get("/healthz", r.getHealthLive)
	app.Get("/metrics", r.getMetrics)
	app.Get("/admin/login", r.getAdminLogin)
	app.Post("/admin/login", r.postAdminLogin)
	app.Post("/admin/logout", r.postAdminLogout)
	app.Use("/admin", r.requireAdminSession)
	app.Get("/admin", security.RequirePermission(security.PermissionViewDashboard), r.getAdminDashboard)
	app.Get("/admin/dashboard/widgets", security.RequirePermission(security.PermissionViewDashboard), r.getAdminDashboardWidgets)
	app.Get("/admin/dashboard/panel-status", security.RequirePermission(security.PermissionViewDashboard), r.getAdminDashboardPanelStatus)
	app.Get("/admin/dashboard/sync-jobs", security.RequirePermission(security.PermissionViewDashboard), r.getAdminDashboardSyncJobs)
	app.Post("/admin/sync/traffic", security.RequirePermission(security.PermissionManagePanels), r.postAdminSyncTraffic)
	app.Get("/admin/sync-jobs", security.RequirePermission(security.PermissionViewDashboard), r.getAdminSyncJobs)
	app.Get("/admin/sync-jobs/:id", security.RequirePermission(security.PermissionViewDashboard), r.getAdminSyncJobDetail)
	app.Post("/admin/sync-jobs/:id/retry", security.RequirePermission(security.PermissionManagePanels), r.postAdminSyncJobRetry)
	app.Post("/admin/sync-jobs/:id/cancel", security.RequirePermission(security.PermissionManagePanels), r.postAdminSyncJobCancel)
	app.Get("/admin/settings", security.RequirePermission(security.PermissionManageSettings), r.getAdminSettings)
	app.Get("/admin/audit-logs", security.RequirePermission(security.PermissionViewAuditLogs), r.getAdminAuditLogs)
	app.Get("/admin/audit-logs/:id", security.RequirePermission(security.PermissionViewAuditLogs), r.getAdminAuditLogDetail)
	app.Get("/admin/backups", security.RequirePermission(security.PermissionViewDashboard), r.getAdminBackups)
	app.Get("/admin/backups/export", security.RequirePermission(security.PermissionViewDashboard), r.getAdminBackupsExport)
	app.Post("/admin/backups/import", security.RequirePermission(security.PermissionViewDashboard), r.postAdminBackupsImport)
	app.Get("/admin/backups/download/:filename", security.RequirePermission(security.PermissionViewDashboard), r.getAdminBackupsDownload)
	app.Post("/admin/backups/delete/:filename", security.RequirePermission(security.PermissionViewDashboard), r.postAdminBackupsDelete)
	app.Get("/admin/notifications", security.RequirePermission(security.PermissionViewDashboard), r.getAdminNotifications)
	app.Get("/admin/notifications/bell", security.RequirePermission(security.PermissionViewDashboard), r.getAdminNotificationBell)
	app.Post("/admin/notifications/:id/read", security.RequirePermission(security.PermissionViewDashboard), r.postAdminNotificationRead)
	app.Post("/admin/notifications/read-all", security.RequirePermission(security.PermissionViewDashboard), r.postAdminNotificationsReadAll)
	app.Post("/admin/settings/telegram/test", security.RequirePermission(security.PermissionManageSettings), r.postAdminTelegramTest)
	app.Post("/admin/settings/email/test", security.RequirePermission(security.PermissionManageSettings), r.postAdminEmailTest)
	app.Post("/admin/settings/cleanup", security.RequirePermission(security.PermissionManageSettings), r.postAdminCleanup)
	app.Get("/admin/maintenance", security.RequirePermission(security.PermissionManageSettings), r.getAdminMaintenance)
	app.Post("/admin/maintenance/cleanup", security.RequirePermission(security.PermissionManageSettings), r.postAdminMaintenanceCleanup)
	app.Post("/admin/maintenance/check-db", security.RequirePermission(security.PermissionManageSettings), r.postAdminMaintenanceCheckDB)
	app.Post("/admin/maintenance/vacuum-db", security.RequirePermission(security.PermissionManageSettings), r.postAdminMaintenanceVacuumDB)
	app.Get("/admin/webhooks", security.RequirePermission(security.PermissionManageSettings), r.getAdminWebhooks)
	app.Get("/admin/webhooks/new", security.RequirePermission(security.PermissionManageSettings), r.getAdminWebhookNew)
	app.Post("/admin/webhooks", security.RequirePermission(security.PermissionManageSettings), r.postAdminWebhookCreate)
	app.Get("/admin/webhooks/:id", security.RequirePermission(security.PermissionManageSettings), r.getAdminWebhookDetail)
	app.Get("/admin/webhooks/:id/edit", security.RequirePermission(security.PermissionManageSettings), r.getAdminWebhookEdit)
	app.Post("/admin/webhooks/:id", security.RequirePermission(security.PermissionManageSettings), r.postAdminWebhookUpdate)
	app.Post("/admin/webhooks/:id/delete", security.RequirePermission(security.PermissionManageSettings), r.postAdminWebhookDelete)
	app.Post("/admin/webhooks/:id/test", security.RequirePermission(security.PermissionManageSettings), r.postAdminWebhookTest)
	app.Get("/admin/webhooks/:id/deliveries", security.RequirePermission(security.PermissionManageSettings), r.getAdminWebhookDeliveries)
	app.Get("/admin/subscriptions", security.RequirePermission(security.PermissionManageSubscriptions), r.getAdminSubscriptions)
	app.Get("/admin/subscriptions/:client_id", security.RequirePermission(security.PermissionManageSubscriptions), r.getAdminSubscriptionDetail)
	app.Post("/admin/subscriptions/:client_id/regenerate", security.RequirePermission(security.PermissionManageSubscriptions), r.postAdminSubscriptionRegenerate)
	app.Post("/admin/subscriptions/:client_id/invalidate-cache", security.RequirePermission(security.PermissionManageSubscriptions), r.postAdminSubscriptionInvalidateCache)
	app.Get("/admin/panels", security.RequirePermission(security.PermissionManagePanels), r.getAdminPanels)
	app.Get("/admin/panels/new", security.RequirePermission(security.PermissionManagePanels), r.getAdminPanelNew)
	app.Post("/admin/panels", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelCreate)
	app.Get("/admin/panels/:id", security.RequirePermission(security.PermissionManagePanels), r.getAdminPanelDetail)
	app.Get("/admin/panels/:id/edit", security.RequirePermission(security.PermissionManagePanels), r.getAdminPanelEdit)
	app.Post("/admin/panels/:id", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelUpdate)
	app.Post("/admin/panels/:id/delete", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelDelete)
	app.Post("/admin/panels/:id/test", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelTest)
	app.Post("/admin/panels/:id/health", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelHealth)
	app.Post("/admin/panels/:id/sync", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelSync)
	app.Post("/admin/panels/:id/sync-traffic", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelSyncTraffic)
	app.Post("/admin/panels/:id/clear-session", security.RequirePermission(security.PermissionManagePanels), r.postAdminPanelClearSession)
	app.Get("/admin/inbounds", security.RequirePermission(security.PermissionManagePanels), r.getAdminInbounds)
	app.Get("/admin/panels/:id/inbounds", security.RequirePermission(security.PermissionManagePanels), r.getAdminPanelInbounds)
	app.Get("/admin/inbounds/:id", security.RequirePermission(security.PermissionManagePanels), r.getAdminInboundDetail)
	app.Post("/admin/inbounds/:id/refresh", security.RequirePermission(security.PermissionManagePanels), r.postAdminInboundRefresh)
	app.Get("/admin/clients", security.RequirePermission(security.PermissionViewClients), r.getAdminClients)
	app.Get("/admin/clients/new", security.RequirePermission(security.PermissionManageClients), r.getAdminClientNew)
	app.Post("/admin/clients", security.RequirePermission(security.PermissionManageClients), r.postAdminClientCreate)
	app.Get("/admin/clients/:id", security.RequirePermission(security.PermissionViewClients), r.getAdminClientDetail)
	app.Get("/admin/clients/:id/edit", security.RequirePermission(security.PermissionManageClients), r.getAdminClientEdit)
	app.Post("/admin/clients/:id", security.RequirePermission(security.PermissionManageClients), r.postAdminClientUpdate)
	app.Post("/admin/clients/:id/delete", security.RequirePermission(security.PermissionManageClients), r.postAdminClientDelete)
	app.Post("/admin/clients/:id/disable", security.RequirePermission(security.PermissionManageClients), r.postAdminClientDisable)
	app.Post("/admin/clients/:id/enable", security.RequirePermission(security.PermissionManageClients), r.postAdminClientEnable)
	app.Post("/admin/clients/:id/reset-password", security.RequirePermission(security.PermissionManageClients), r.postAdminClientResetPassword)
	app.Post("/admin/clients/:id/regenerate-token", security.RequirePermission(security.PermissionManageClients), r.postAdminClientRegenerateSubscriptionToken)
	app.Get("/admin/clients/:id/attachments", security.RequirePermission(security.PermissionManageClients), r.getAdminClientAttachments)
	app.Post("/admin/clients/:id/attachments", security.RequirePermission(security.PermissionManageClients), r.postAdminClientAttachmentCreate)
	app.Post("/admin/clients/:id/attachments/:attachment_id/delete", security.RequirePermission(security.PermissionManageClients), r.postAdminClientAttachmentDelete)
	app.Post("/admin/clients/:id/attachments/:attachment_id/detach", security.RequirePermission(security.PermissionManageClients), r.postAdminClientAttachmentDetach)
	app.Post("/admin/clients/:id/attachments/:attachment_id/refresh", security.RequirePermission(security.PermissionManageClients), r.postAdminClientAttachmentRefresh)
	app.Post("/admin/clients/:id/attachments/:attachment_id/sync", security.RequirePermission(security.PermissionManageClients), r.postAdminClientAttachmentSync)
	app.Post("/admin/clients/:id/attachments/:attachment_id/disable", security.RequirePermission(security.PermissionManageClients), r.postAdminClientAttachmentDisable)
	app.Post("/admin/clients/:id/attachments/:attachment_id/enable", security.RequirePermission(security.PermissionManageClients), r.postAdminClientAttachmentEnable)
	app.Post("/admin/clients/:id/sync-traffic", security.RequirePermission(security.PermissionManageClients), r.postAdminClientSyncTraffic)
	app.Get("/admin/users", security.RequirePermission(security.PermissionManageAdmins), r.getAdminUsers)
	app.Get("/admin/users/new", security.RequirePermission(security.PermissionManageAdmins), r.getAdminUserNew)
	app.Post("/admin/users", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserCreate)
	app.Get("/admin/users/:id/edit", security.RequirePermission(security.PermissionManageAdmins), r.getAdminUserEdit)
	app.Post("/admin/users/:id", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserUpdate)
	app.Post("/admin/users/:id/disable", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserDisable)
	app.Post("/admin/users/:id/enable", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserEnable)
	app.Post("/admin/users/:id/delete", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserDelete)
	app.Post("/admin/users/:id/reset-password", security.RequirePermission(security.PermissionManageAdmins), r.postAdminUserResetPassword)
	app.Get("/client/login", r.getClientLogin)
	app.Post("/client/login", r.postClientLogin)
	app.Post("/client/logout", r.postClientLogout)
	app.Use("/client", r.requireClientSession)
	app.Get("/client", r.getClientDashboard)
	app.Get("/client/profile", r.getClientProfile)
	app.Post("/client/profile/password", r.postClientProfilePassword)
	app.Get("/client/configs", r.getClientConfigs)
	app.Get("/client/subscription", r.getClientSubscription)
	app.Get("/client/usage", r.getClientUsage)
	app.Get("/sub/:token", r.getPublicSubscription)
	app.Get("/sub/:token/raw", r.getPublicSubscriptionRaw)
	app.Get("/sub/:token/base64", r.getPublicSubscriptionBase64)
	app.Get("/sub/:token/clash", r.getPublicSubscriptionClash)
	app.Get("/sub/:token/singbox", r.getPublicSubscriptionSingbox)
	app.Use("/api/v1/client", r.clientAPILogMiddleware())
	app.Post("/api/v1/client/auth/login", r.postClientAPILogin)
	app.Post("/api/v1/client/auth/refresh", r.postClientAPIRefresh)
	app.Post("/api/v1/client/auth/logout", r.postClientAPILogout)
	app.Get("/api/v1/client/me", r.requireClientAPIJWT, r.getClientAPIMe)
	app.Get("/api/v1/client/subscription", r.requireClientAPIJWT, r.getClientAPISubscription)
	app.Get("/api/v1/client/configs", r.requireClientAPIJWT, r.getClientAPIConfigs)
	app.Get("/api/v1/client/usage", r.requireClientAPIJWT, r.getClientAPIUsage)
	app.Get("/api/v1/client/status", r.requireClientAPIJWT, r.getClientAPIStatus)
	app.Get("/api/openapi.json", r.getOpenAPIJSON)
	app.Get("/api/openapi.yaml", r.getOpenAPIYAML)
	app.Get("/api/docs", r.getOpenAPIDocs)

	return app
}

func (r *Runner) errorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	var fiberErr *fiber.Error
	if errors.As(err, &fiberErr) {
		code = fiberErr.Code
	}
	requestID := services.RequestIDFromContext(c.UserContext())
	if requestID != "" {
		c.Set("X-Request-ID", requestID)
	}
	if strings.HasPrefix(c.Path(), "/api/") {
		return c.Status(code).JSON(fiber.Map{"error": fiber.Map{"code": strconv.Itoa(code), "message": err.Error(), "request_id": requestID}})
	}
	scope := "public"
	if strings.HasPrefix(c.Path(), "/admin/") {
		scope = "admin"
	} else if strings.HasPrefix(c.Path(), "/client/") {
		scope = "client"
	}
	return c.Status(code).Type("html").SendString(renderErrorPage(errorTitleForStatus(code), errorMessageForScope(code, scope), requestID))
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
	ip := requestClientIP(c, r.cfg.TrustProxy)
	key := ip + ":" + username
	if r.loginLocks.blocked(key) {
		_ = r.logAudit(c.UserContext(), "admin", nil, "login_rate_limited", "admin", nil, map[string]any{"username": username, "ip": ip, "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
		return c.Status(fiber.StatusTooManyRequests).Type("html").SendString(renderLoginPage("Too many attempts. Try again later.", r.cfg.AppName))
	}

	admin, err := r.admins.FindByUsername(c.UserContext(), username)
	if err != nil || admin == nil || !admin.Active || security.ComparePassword(password, admin.PasswordHash) != nil {
		r.loginLocks.fail(key)
		_ = r.logAudit(c.UserContext(), "admin", nil, "login_failed", "admin", nil, map[string]any{"username": username, "ip": ip, "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
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
	_ = r.logAudit(c.UserContext(), "admin", &admin.ID, "login_success", "admin", &admin.ID, map[string]any{"username": admin.Username, "ip": ip, "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
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

func (r *Runner) getAdminUsers(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	admins, err := r.admins.List(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminUsersPage(admins, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminUserNew(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderAdminUserFormPage("Create Admin", "/admin/users", adminForm{}, false, r.cfg.AppName, admin.Role, nil))
}

func (r *Runner) postAdminUserCreate(c *fiber.Ctx) error {
	form, err := parseAdminForm(c)
	if err != nil {
		admin, _ := currentAdmin(c)
		role := "owner"
		if admin != nil {
			role = admin.Role
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Create Admin", "/admin/users", form, false, r.cfg.AppName, role, []string{err.Error()}))
	}
	if err := form.validate(true); err != nil {
		admin, _ := currentAdmin(c)
		role := "owner"
		if admin != nil {
			role = admin.Role
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Create Admin", "/admin/users", form, false, r.cfg.AppName, role, []string{err.Error()}))
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
	current, _ := currentAdmin(c)
	role := "owner"
	if current != nil {
		role = current.Role
	}
	return c.Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), adminFormFromAdmin(admin), true, r.cfg.AppName, role, nil))
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
		current, _ := currentAdmin(c)
		role := "owner"
		if current != nil {
			role = current.Role
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), adminFormFromAdmin(existing), true, r.cfg.AppName, role, []string{err.Error()}))
	}
	if err := form.validate(false); err != nil {
		current, _ := currentAdmin(c)
		role := "owner"
		if current != nil {
			role = current.Role
		}
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), form, true, r.cfg.AppName, role, []string{err.Error()}))
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
			current, _ := currentAdmin(c)
			role := "owner"
			if current != nil {
				role = current.Role
			}
			return c.Status(fiber.StatusForbidden).Type("html").SendString(renderAdminUserFormPage("Edit Admin", "/admin/users/"+c.Params("id"), adminFormFromAdmin(existing), true, r.cfg.AppName, role, []string{"last owner cannot be demoted"}))
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

func (r *Runner) getClientLogin(c *fiber.Ctx) error {
	if _, ok := r.loadClientFromRequest(c); ok {
		return c.Redirect("/client", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderClientLoginPage("", r.cfg.AppName))
}

func (r *Runner) postClientLogin(c *fiber.Ctx) error {
	username := strings.TrimSpace(c.FormValue("username"))
	password := c.FormValue("password")
	key := "client:" + c.IP() + ":" + username
	if r.loginLocks.blocked(key) {
		return c.Status(fiber.StatusTooManyRequests).Type("html").SendString(renderClientLoginPage("Too many attempts. Try again later.", r.cfg.AppName))
	}

	client, err := r.clients.FindByUsername(c.UserContext(), username)
	if err != nil || client == nil || client.Status == "disabled" || client.Status == "deleted" || security.ComparePassword(password, client.PasswordHash) != nil {
		r.loginLocks.fail(key)
		return c.Status(fiber.StatusUnauthorized).Type("html").SendString(renderClientLoginPage("Invalid credentials.", r.cfg.AppName))
	}
	r.loginLocks.success(key)

	token, err := security.RandomToken(32)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()
	expires := time.Now().UTC().Add(time.Duration(r.cfg.SessionTTLHrs) * time.Hour)
	if err := r.redis.Set(ctx, clientSessionKey(token), client.ID, time.Until(expires)).Err(); err != nil {
		return err
	}
	c.Cookie(&fiber.Cookie{
		Name:     clientSessionCookieName,
		Value:    token,
		Path:     "/",
		HTTPOnly: true,
		Secure:   r.cfg.IsProduction(),
		SameSite: "Lax",
		Expires:  expires,
	})
	return c.Redirect("/client", fiber.StatusFound)
}

func (r *Runner) postClientLogout(c *fiber.Ctx) error {
	client, ok := r.loadClientFromRequest(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	if token := c.Cookies(clientSessionCookieName); token != "" {
		ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer cancel()
		_ = r.redis.Del(ctx, clientSessionKey(token)).Err()
	}
	c.Cookie(&fiber.Cookie{Name: clientSessionCookieName, Value: "", Path: "/", HTTPOnly: true, Secure: r.cfg.IsProduction(), SameSite: "Lax", Expires: time.Unix(0, 0)})
	_ = client
	return c.Redirect("/client/login", fiber.StatusFound)
}

func (r *Runner) requireClientSession(c *fiber.Ctx) error {
	client, ok := r.loadClientFromRequest(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	c.Locals("client", client)
	return c.Next()
}

func (r *Runner) getClientDashboard(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderClientDashboardPage(summary, r.cfg.AppName))
}

func (r *Runner) getClientProfile(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderClientProfilePage(client, r.cfg.AppName, nil))
}

func (r *Runner) postClientProfilePassword(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	currentPassword := c.FormValue("current_password")
	newPassword := strings.TrimSpace(c.FormValue("new_password"))
	confirmPassword := strings.TrimSpace(c.FormValue("confirm_password"))
	if err := security.ComparePassword(currentPassword, client.PasswordHash); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderClientProfilePage(client, r.cfg.AppName, []string{"current password is incorrect"}))
	}
	if len(newPassword) < 12 {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderClientProfilePage(client, r.cfg.AppName, []string{"new password must be at least 12 characters"}))
	}
	if newPassword != confirmPassword {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderClientProfilePage(client, r.cfg.AppName, []string{"password confirmation does not match"}))
	}
	hash, err := security.HashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := r.clients.UpdatePassword(c.UserContext(), client.ID, hash); err != nil {
		return err
	}
	client.PasswordHash = hash
	return c.Type("html").SendString(renderClientProfilePage(client, r.cfg.AppName, []string{"password updated"}))
}

func (r *Runner) getClientConfigs(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderClientConfigsPage(client, summary.Configs, r.cfg.AppName))
}

func (r *Runner) getClientSubscription(c *fiber.Ctx) error {
	if r.metrics != nil {
		r.metrics.IncSubscriptionRequest()
	}
	client, ok := currentClient(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderClientSubscriptionPage(summary, r.cfg.AppName))
}

func (r *Runner) getPublicSubscription(c *fiber.Ctx) error {
	if r.metrics != nil {
		r.metrics.IncSubscriptionRequest()
	}
	return r.servePublicSubscription(c, "base64")
}

func (r *Runner) getPublicSubscriptionRaw(c *fiber.Ctx) error {
	if r.metrics != nil {
		r.metrics.IncSubscriptionRequest()
	}
	return r.servePublicSubscription(c, "raw")
}

func (r *Runner) getPublicSubscriptionBase64(c *fiber.Ctx) error {
	if r.metrics != nil {
		r.metrics.IncSubscriptionRequest()
	}
	return r.servePublicSubscription(c, "base64")
}

func (r *Runner) getPublicSubscriptionClash(c *fiber.Ctx) error {
	if r.metrics != nil {
		r.metrics.IncSubscriptionRequest()
	}
	return r.servePublicSubscription(c, "clash")
}

func (r *Runner) getPublicSubscriptionSingbox(c *fiber.Ctx) error {
	if r.metrics != nil {
		r.metrics.IncSubscriptionRequest()
	}
	return r.servePublicSubscription(c, "singbox")
}

func (r *Runner) getClientUsage(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return c.Redirect("/client/login", fiber.StatusFound)
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderClientUsagePage(summary, r.cfg.AppName))
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

func currentClient(c *fiber.Ctx) (*models.Client, bool) {
	client, ok := c.Locals("client").(*models.Client)
	if !ok || client == nil {
		return nil, false
	}
	return client, true
}

func adminActorID(admin *models.AdminUser) *int64 {
	if admin == nil {
		return nil
	}
	return &admin.ID
}

func clientSessionKey(token string) string {
	return "session:client:" + token
}

func (r *Runner) loadClientFromRequest(c *fiber.Ctx) (*models.Client, bool) {
	if client, ok := currentClient(c); ok {
		return client, true
	}
	token := c.Cookies(clientSessionCookieName)
	if token == "" {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()
	clientID, err := r.redis.Get(ctx, clientSessionKey(token)).Int64()
	if err != nil {
		return nil, false
	}
	client, err := r.loadClientByID(c.UserContext(), clientID)
	if err != nil || client == nil || client.Status == "disabled" || client.Status == "deleted" {
		return nil, false
	}
	return client, true
}

func (r *Runner) loadClientByID(ctx context.Context, id int64) (*models.Client, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, display_name, email, status, traffic_limit_bytes, expiry_time, subscription_token, created_at, updated_at FROM clients WHERE id = ?`, id)
	return scanClientRow(row)
}

func scanClientRow(scanner interface{ Scan(...any) error }) (*models.Client, error) {
	var client models.Client
	var displayName, email, subscriptionToken sql.NullString
	var expiryTime sql.NullTime
	if err := scanner.Scan(&client.ID, &client.Username, &client.PasswordHash, &displayName, &email, &client.Status, &client.TrafficLimitBytes, &expiryTime, &subscriptionToken, &client.CreatedAt, &client.UpdatedAt); err != nil {
		return nil, err
	}
	client.DisplayName = displayName.String
	client.Email = email.String
	client.SubscriptionToken = subscriptionToken.String
	if expiryTime.Valid {
		t := expiryTime.Time
		client.ExpiryTime = &t
	}
	return &client, nil
}

type clientSummary struct {
	Client             *models.Client
	UploadBytes        int64
	DownloadBytes      int64
	TotalBytes         int64
	RemainingBytes     int64
	ActiveConfigs      int
	ExpiryText         string
	RemainingText      string
	StatusText         string
	SubscriptionURL    string
	RawSubscriptionURL string
	Base64URL          string
	ClashURL           string
	SingboxURL         string
	Configs            []clientConfigRow
}

type clientConfigRow struct {
	PanelName     string
	InboundRemark string
	Protocol      string
	Status        string
	RawConfig     string
	Enabled       bool
	CopyValue     string
}

func (r *Runner) loadClientSummary(ctx context.Context, client *models.Client) (clientSummary, error) {
	summary := clientSummary{Client: client}
	if client == nil {
		return summary, errors.New("client not found")
	}
	now := time.Now().UTC()
	if client.ExpiryTime != nil {
		summary.ExpiryText = formatTime(*client.ExpiryTime)
		if client.ExpiryTime.After(now) {
			summary.RemainingText = formatDuration(time.Until(*client.ExpiryTime))
		}
	}
	row := r.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(upload_bytes),0), COALESCE(SUM(download_bytes),0), COUNT(*) FROM client_attachments WHERE client_id = ?`, client.ID)
	if err := row.Scan(&summary.UploadBytes, &summary.DownloadBytes, &summary.ActiveConfigs); err != nil {
		return summary, err
	}
	summary.TotalBytes = summary.UploadBytes + summary.DownloadBytes
	if client.TrafficLimitBytes > 0 {
		remaining := client.TrafficLimitBytes - summary.TotalBytes
		if remaining < 0 {
			remaining = 0
		}
		summary.RemainingBytes = remaining
	}
	summary.StatusText = services.DetermineClientStatus(client, summary.TotalBytes, now)
	if summary.StatusText != "expired" && (client.TrafficLimitBytes <= 0 || summary.RemainingBytes > 0) {
		summary.Configs, _ = r.loadClientConfigs(ctx, client.ID)
		summary.ActiveConfigs = len(summary.Configs)
	} else {
		summary.ActiveConfigs = 0
	}
	summary.SubscriptionURL = r.buildClientURL("/sub/" + client.SubscriptionToken)
	summary.RawSubscriptionURL = r.buildClientURL("/sub/" + client.SubscriptionToken + "/raw")
	summary.Base64URL = r.buildClientURL("/sub/" + client.SubscriptionToken + "/base64")
	summary.ClashURL = r.buildClientURL("/sub/" + client.SubscriptionToken + "/clash")
	summary.SingboxURL = r.buildClientURL("/sub/" + client.SubscriptionToken + "/singbox")
	return summary, nil
}

func (r *Runner) loadClientConfigs(ctx context.Context, clientID int64) ([]clientConfigRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT COALESCE(p.name, ''), COALESCE(i.remark, ''), COALESCE(i.protocol, ''), ca.enabled, COALESCE(ca.raw_config, '') FROM client_attachments ca LEFT JOIN panels p ON p.id = ca.panel_id LEFT JOIN inbounds i ON i.id = ca.inbound_id WHERE ca.client_id = ? AND ca.enabled = 1 AND COALESCE(i.stale, 0) = 0 ORDER BY ca.id ASC`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []clientConfigRow
	for rows.Next() {
		var row clientConfigRow
		var enabled int
		if err := rows.Scan(&row.PanelName, &row.InboundRemark, &row.Protocol, &enabled, &row.RawConfig); err != nil {
			return nil, err
		}
		row.Enabled = enabled != 0
		if row.Enabled {
			row.Status = "active"
		} else {
			row.Status = "disabled"
		}
		row.CopyValue = row.RawConfig
		configs = append(configs, row)
	}
	return configs, rows.Err()
}

func (r *Runner) buildClientURL(path string) string {
	return strings.TrimRight(r.cfg.AppBaseURL, "/") + path
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
	audit := &models.AuditLog{ActorType: actorType, ActorID: actorID, Action: action, TargetType: targetType, TargetID: targetID, MetadataJSON: meta, CreatedAt: time.Now().UTC()}
	if err := r.audit.Create(ctx, audit); err != nil {
		return err
	}
	if r.events != nil {
		r.events.Publish(ctx, services.Event{Type: action, ActorType: actorType, ActorID: actorID, TargetType: targetType, TargetID: targetID, Payload: metadata, CreatedAt: audit.CreatedAt})
	}
	return nil
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
	return `<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><meta name="color-scheme" content="light dark"><meta name="theme-color" content="#0d6efd"><title>` + html.EscapeString(title) + `</title>` + themeScript() + `<link href="` + staticAssetURL("vendor/bootstrap/bootstrap.min.css") + `" rel="stylesheet"></head><body class="bg-body-tertiary"><div id="global-loading-indicator" class="position-fixed top-0 start-0 w-100" style="height:3px;z-index:2000;background:var(--bs-primary);opacity:0;transition:opacity .2s ease;" aria-hidden="true"></div>` + body + `<script>(function(){var pending=0;var indicator=document.getElementById('global-loading-indicator');function token(){var match=document.cookie.match(/(?:^|; )` + csrfCookieName + `=([^;]+)/);return match?decodeURIComponent(match[1]):"";}function syncIndicator(){if(!indicator){return;}indicator.style.opacity=pending>0?'1':'0';}document.addEventListener('htmx:beforeRequest',function(){pending++;syncIndicator();});document.addEventListener('htmx:afterRequest',function(){pending=Math.max(0,pending-1);syncIndicator();});document.addEventListener('submit',function(event){var form=event.target;if(form&&form.matches&&form.matches('form[method="post"], form[method="put"], form[method="delete"], form[method="patch"]')){pending++;syncIndicator();setTimeout(function(){pending=Math.max(0,pending-1);syncIndicator();},0);}});function apply(){var value=token();if(!value){return;}document.querySelectorAll('form').forEach(function(form){var method=(form.getAttribute('method')||'get').toLowerCase();if(method==='get'){return;}if(form.querySelector('input[name="` + csrfFormField + `"]')){return;}var input=document.createElement('input');input.type='hidden';input.name='` + csrfFormField + `';input.value=value;form.appendChild(input);});}apply();document.addEventListener('DOMContentLoaded',apply);document.addEventListener('htmx:afterSwap',apply);})();</script><script src="` + staticAssetURL("vendor/bootstrap/bootstrap.bundle.min.js") + `"></script></body></html>`
}

func renderLoginPage(message, appName string) string {
	alert := ""
	if strings.TrimSpace(message) != "" {
		alert = `<div class="alert alert-warning">` + html.EscapeString(message) + `</div>`
	}
	return renderPage("Admin Login", `<main class="container py-5" style="max-width: 480px;"><div class="card shadow-sm"><div class="card-body p-4"><h1 class="h4 mb-1">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary mb-4">Admin sign in</p>`+alert+`<form method="post" action="/admin/login" class="vstack gap-3"><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" autocomplete="username" required></div><div><label class="form-label" for="password">Password</label><input class="form-control" id="password" name="password" type="password" autocomplete="current-password" required></div><button class="btn btn-primary w-100" type="submit">Sign in</button></form></div></div></main>`)
}

func renderAdminUsersPage(admins []models.AdminUser, appName string, adminRole string) string {
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
	body := `<div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">Admin users</h1><p class="text-body-secondary mb-0">Manage administrative accounts</p></div><div class="d-flex gap-2"><a class="btn btn-primary btn-sm" href="/admin/users/new">New admin</a><a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div></div><div class="card"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Username</th><th>Role</th><th>Status</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "admin-users", `<div class="container py-4 py-lg-5">`+body+`</div>`)
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

func renderAdminUserFormPage(title, action string, form adminForm, editing bool, appName string, adminRole string, errors []string) string {
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
	body := `<div class="container py-4 py-lg-5" style="max-width: 720px;"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">` + html.EscapeString(title) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(appName) + `</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/users">Back</a></div>` + alert.String() + `<div class="card"><div class="card-body">` + formHTML + resetHTML + `</div></div></div></div>`
	return renderAdminShell(appName, adminRole, "admin-users", body)
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
	return renderPage("Password Reset", `<main class="container py-5" style="max-width: 640px;"><div class="card"><div class="card-body"><h1 class="h4">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary">Password reset for `+html.EscapeString(username)+`</p><div class="alert alert-success"><div class="fw-semibold mb-1">New password</div><code>`+html.EscapeString(password)+`</code></div><a class="btn btn-outline-secondary" href="/admin/users">Back to admin users</a></div></div></main>`)
}

func renderAdminUsersPageMessage(message, appName string) string {
	return renderPage("Admin Users", `<main class="container py-5"><div class="alert alert-danger">`+html.EscapeString(message)+`</div><a class="btn btn-outline-secondary" href="/admin/users">Back to admin users</a></main>`)
}

func renderClientLoginPage(message, appName string) string {
	alert := ""
	if strings.TrimSpace(message) != "" {
		alert = `<div class="alert alert-warning">` + html.EscapeString(message) + `</div>`
	}
	return renderPage("Client Login", `<main class="container py-5" style="max-width: 480px;"><div class="card shadow-sm"><div class="card-body p-4"><h1 class="h4 mb-1">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary mb-4">Client sign in</p>`+alert+`<form method="post" action="/client/login" class="vstack gap-3"><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" autocomplete="username" required></div><div><label class="form-label" for="password">Password</label><input class="form-control" id="password" name="password" type="password" autocomplete="current-password" required></div><button class="btn btn-primary w-100" type="submit">Sign in</button></form></div></div></main>`)
}

func renderClientDashboardPage(summary clientSummary, appName string) string {
	statusBadge := "secondary"
	if summary.StatusText == "active" {
		statusBadge = "success"
	} else if summary.StatusText == "expired" {
		statusBadge = "warning"
	}
	remainingTraffic := "Unlimited"
	if summary.Client.TrafficLimitBytes > 0 {
		remainingTraffic = formatBytes(summary.RemainingBytes)
	}
	return renderPage("Client Dashboard", `<main class="container py-4 py-lg-5"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary mb-0">Welcome, `+html.EscapeString(summary.Client.Username)+`</p></div><div class="d-flex gap-2"><a class="btn btn-outline-secondary btn-sm" href="/client/profile">Profile</a><a class="btn btn-outline-secondary btn-sm" href="/client/configs">Configs</a><a class="btn btn-outline-secondary btn-sm" href="/client/subscription">Subscription</a><a class="btn btn-outline-secondary btn-sm" href="/client/usage">Usage</a><form method="post" action="/client/logout"><button class="btn btn-outline-danger btn-sm" type="submit">Logout</button></form></div></div><div class="row g-3"><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Account status</div><div class="fs-5 fw-semibold"><span class="badge text-bg-`+statusBadge+`">`+html.EscapeString(summary.StatusText)+`</span></div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Expiry</div><div class="fs-5 fw-semibold">`+html.EscapeString(defaultString(summary.ExpiryText, "No expiry"))+`</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Traffic used</div><div class="fs-5 fw-semibold">`+html.EscapeString(formatBytes(summary.TotalBytes))+`</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Remaining traffic</div><div class="fs-5 fw-semibold">`+html.EscapeString(remainingTraffic)+`</div></div></div></div></div><div class="card"><div class="card-body d-flex flex-column gap-2"><div class="text-body-secondary small">Subscription link</div><div class="input-group"><input class="form-control" value="`+html.EscapeString(summary.SubscriptionURL)+`" readonly><button class="btn btn-outline-secondary" type="button" onclick="navigator.clipboard.writeText(this.previousElementSibling.value)">Copy</button></div></div></div></div></main>`)
}

func renderClientProfilePage(client *models.Client, appName string, messages []string) string {
	var alert strings.Builder
	for _, msg := range messages {
		alert.WriteString(`<div class="alert alert-info">` + html.EscapeString(msg) + `</div>`)
	}
	return renderPage("Client Profile", `<main class="container py-4 py-lg-5"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary mb-0">Profile for `+html.EscapeString(client.Username)+`</p></div><a class="btn btn-outline-secondary btn-sm" href="/client">Back</a></div>`+alert.String()+`<div class="row g-3"><div class="col-12 col-lg-5"><div class="card h-100"><div class="card-body vstack gap-2"><div><div class="text-body-secondary small">Username</div><div class="fw-semibold">`+html.EscapeString(client.Username)+`</div></div><div><div class="text-body-secondary small">Display name</div><div class="fw-semibold">`+html.EscapeString(client.DisplayName)+`</div></div><div><div class="text-body-secondary small">Email</div><div class="fw-semibold">`+html.EscapeString(client.Email)+`</div></div></div></div></div><div class="col-12 col-lg-7"><div class="card h-100"><div class="card-body"><form method="post" action="/client/profile/password" class="vstack gap-3"><div><label class="form-label" for="current_password">Current password</label><input class="form-control" id="current_password" name="current_password" type="password" required></div><div><label class="form-label" for="new_password">New password</label><input class="form-control" id="new_password" name="new_password" type="password" minlength="12" required></div><div><label class="form-label" for="confirm_password">Confirm password</label><input class="form-control" id="confirm_password" name="confirm_password" type="password" minlength="12" required></div><button class="btn btn-primary" type="submit">Update password</button></form></div></div></div></div></main>`)
}

func renderClientConfigsPage(client *models.Client, configs []clientConfigRow, appName string) string {
	var rows strings.Builder
	for _, cfg := range configs {
		copyValue := html.EscapeString(cfg.CopyValue)
		rows.WriteString(`<div class="col-12 col-lg-6"><div class="card h-100"><div class="card-body"><div class="d-flex justify-content-between gap-2"><div><div class="fw-semibold">` + html.EscapeString(cfg.PanelName) + `</div><div class="text-body-secondary small">` + html.EscapeString(cfg.InboundRemark) + `</div></div><span class="badge text-bg-` + mapStatusBadge(cfg.Status) + `">` + html.EscapeString(cfg.Status) + `</span></div><div class="mt-3"><div class="small text-body-secondary mb-1">Protocol</div><div>` + html.EscapeString(cfg.Protocol) + `</div></div><div class="mt-3"><textarea class="form-control" rows="4" readonly>` + copyValue + `</textarea></div><div class="mt-3 d-flex gap-2 flex-wrap"><button class="btn btn-outline-secondary btn-sm" type="button" onclick="navigator.clipboard.writeText(this.parentElement.previousElementSibling.value)">Copy</button><div class="border rounded d-flex align-items-center justify-content-center text-body-secondary small" style="min-width:120px;min-height:120px;">QR</div></div></div></div></div>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<div class="col-12"><div class="alert alert-warning mb-0">No active configs found.</div></div>`)
	}
	return renderPage("Client Configs", `<main class="container py-4 py-lg-5"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary mb-0">Configs for `+html.EscapeString(client.Username)+`</p></div><a class="btn btn-outline-secondary btn-sm" href="/client">Back</a></div><div class="row g-3">`+rows.String()+`</div></div></main>`)
}

func renderClientSubscriptionPage(summary clientSummary, appName string) string {
	return renderPage("Client Subscription", `<main class="container py-4 py-lg-5"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary mb-0">Subscription links for `+html.EscapeString(summary.Client.Username)+`</p></div><a class="btn btn-outline-secondary btn-sm" href="/client">Back</a></div><div class="row g-3"><div class="col-12"><div class="card"><div class="card-body vstack gap-3"><div><div class="text-body-secondary small">Main subscription URL</div><div class="input-group"><input class="form-control" value="`+html.EscapeString(summary.SubscriptionURL)+`" readonly><button class="btn btn-outline-secondary" type="button" onclick="navigator.clipboard.writeText(this.previousElementSibling.value)">Copy</button></div></div><div><div class="text-body-secondary small">Raw subscription URL</div><code class="d-block p-2 bg-body-tertiary rounded">`+html.EscapeString(summary.RawSubscriptionURL)+`</code></div><div><div class="text-body-secondary small">Base64 subscription URL</div><code class="d-block p-2 bg-body-tertiary rounded">`+html.EscapeString(summary.Base64URL)+`</code></div><div><div class="text-body-secondary small">Clash URL placeholder</div><code class="d-block p-2 bg-body-tertiary rounded">`+html.EscapeString(summary.ClashURL)+`</code></div><div><div class="text-body-secondary small">Sing-box URL placeholder</div><code class="d-block p-2 bg-body-tertiary rounded">`+html.EscapeString(summary.SingboxURL)+`</code></div></div></div></div></div></main>`)
}

func renderClientUsagePage(summary clientSummary, appName string) string {
	return renderPage("Client Usage", `<main class="container py-4 py-lg-5"><div class="d-flex flex-column gap-3"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap"><div><h1 class="h3 mb-1">`+html.EscapeString(appName)+`</h1><p class="text-body-secondary mb-0">Usage for `+html.EscapeString(summary.Client.Username)+`</p></div><a class="btn btn-outline-secondary btn-sm" href="/client">Back</a></div><div class="row g-3"><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Upload</div><div class="fs-5 fw-semibold">`+html.EscapeString(formatBytes(summary.UploadBytes))+`</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Download</div><div class="fs-5 fw-semibold">`+html.EscapeString(formatBytes(summary.DownloadBytes))+`</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Total</div><div class="fs-5 fw-semibold">`+html.EscapeString(formatBytes(summary.TotalBytes))+`</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card h-100"><div class="card-body"><div class="text-body-secondary small">Limit</div><div class="fs-5 fw-semibold">`+html.EscapeString(formatLimit(summary.Client.TrafficLimitBytes))+`</div></div></div></div></div><div class="card"><div class="card-body"><div class="text-body-secondary small">Remaining traffic</div><div class="fs-5 fw-semibold">`+html.EscapeString(formatRemainingTraffic(summary))+`</div></div></div></div></main>`)
}

func renderClientPortalLayout(title, appName, body string) string {
	return renderPage(title, body)
}

func formatBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return strconv.FormatInt(value, 10) + " B"
	}
	d := float64(value)
	for _, suffix := range []string{"KiB", "MiB", "GiB", "TiB"} {
		d /= unit
		if d < unit {
			return fmt.Sprintf("%.1f %s", d, suffix)
		}
	}
	return fmt.Sprintf("%.1f PiB", d/unit)
}

func formatLimit(value int64) string {
	if value <= 0 {
		return "Unlimited"
	}
	return formatBytes(value)
}

func formatRemainingTraffic(summary clientSummary) string {
	if summary.Client.TrafficLimitBytes <= 0 {
		return "Unlimited"
	}
	return formatBytes(summary.RemainingBytes)
}

func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "expired"
	}
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	if days > 0 {
		return fmt.Sprintf("%d days %d hours", days, hours)
	}
	return fmt.Sprintf("%d hours", hours)
}

func formatTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func mapStatusBadge(status string) string {
	switch strings.ToLower(status) {
	case "active":
		return "success"
	case "disabled":
		return "secondary"
	default:
		return "primary"
	}
}
