package security

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
)

const (
	RoleOwner    = "owner"
	RoleAdmin    = "admin"
	RoleSupport  = "support"
	RoleReadonly = "readonly"

	PermissionViewDashboard        = "dashboard:view"
	PermissionManageAdmins         = "admins:manage"
	PermissionManagePanels         = "panels:manage"
	PermissionManageClients        = "clients:manage"
	PermissionManageBackups        = "backups:manage"
	PermissionImportDatabase       = "database:import"
	PermissionExportDatabase       = "database:export"
	PermissionViewAuditLogs        = "audit:view"
	PermissionManageSettings       = "settings:manage"
	PermissionManageInbounds       = "inbounds:manage"
	PermissionManageSubscriptions  = "subscriptions:manage"
	PermissionViewClients          = "clients:view"
	PermissionViewConfigs          = "configs:view"
	PermissionResetClientPassword  = "clients:reset_password"
	PermissionToggleClient         = "clients:toggle"
	PermissionViewUsage            = "usage:view"
)

var rolePermissions = map[string]map[string]struct{}{
	RoleOwner: {
		PermissionViewDashboard:       {},
		PermissionManageAdmins:        {},
		PermissionManagePanels:        {},
		PermissionManageClients:       {},
		PermissionManageBackups:       {},
		PermissionImportDatabase:      {},
		PermissionExportDatabase:      {},
		PermissionViewAuditLogs:       {},
		PermissionManageSettings:      {},
		PermissionManageInbounds:      {},
		PermissionManageSubscriptions: {},
		PermissionViewClients:         {},
		PermissionViewConfigs:         {},
		PermissionResetClientPassword: {},
		PermissionToggleClient:        {},
		PermissionViewUsage:           {},
	},
	RoleAdmin: {
		PermissionViewDashboard:       {},
		PermissionManagePanels:        {},
		PermissionManageClients:       {},
		PermissionViewAuditLogs:       {},
		PermissionManageInbounds:      {},
		PermissionManageSubscriptions: {},
		PermissionViewClients:         {},
		PermissionViewConfigs:         {},
		PermissionResetClientPassword: {},
		PermissionToggleClient:        {},
		PermissionViewUsage:           {},
	},
	RoleSupport: {
		PermissionViewDashboard:       {},
		PermissionViewClients:         {},
		PermissionViewConfigs:         {},
		PermissionResetClientPassword: {},
		PermissionToggleClient:        {},
		PermissionViewUsage:           {},
	},
	RoleReadonly: {
		PermissionViewDashboard: {},
		PermissionManagePanels:  {},
		PermissionManageClients: {},
		PermissionViewUsage:     {},
	},
}

func NormalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case RoleOwner, RoleAdmin, RoleSupport, RoleReadonly:
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return RoleOwner
	}
}

func HasRole(role string, allowed ...string) bool {
	role = NormalizeRole(role)
	for _, candidate := range allowed {
		if role == NormalizeRole(candidate) {
			return true
		}
	}
	return false
}

func HasPermission(role, permission string) bool {
	permissions, ok := rolePermissions[NormalizeRole(role)]
	if !ok {
		return false
	}
	_, ok = permissions[permission]
	return ok
}

func RequirePermission(permission string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		admin, ok := c.Locals("admin").(*models.AdminUser)
		if !ok || admin == nil {
			return fiber.NewError(fiber.StatusUnauthorized, "admin session required")
		}
		if !HasPermission(admin.Role, permission) {
			return fiber.NewError(fiber.StatusForbidden, "forbidden")
		}
		return c.Next()
	}
}

func RequireRole(roles ...string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		admin, ok := c.Locals("admin").(*models.AdminUser)
		if !ok || admin == nil {
			return fiber.NewError(fiber.StatusUnauthorized, "admin session required")
		}
		if !HasRole(admin.Role, roles...) {
			return fiber.NewError(fiber.StatusForbidden, "forbidden")
		}
		return c.Next()
	}
}
