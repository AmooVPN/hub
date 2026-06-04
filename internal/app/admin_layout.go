package app

import (
	"html"
	"strings"

	"github.com/AmooVPM/hub/internal/security"
)

type adminNavItem struct {
	Label   string
	Href    string
	Active  bool
	Allowed bool
}

func renderAdminShell(appName string, adminRole string, activePage string, content string) string {
	items := []adminNavItem{
		{Label: "Dashboard", Href: "/admin", Active: activePage == "dashboard", Allowed: true},
		{Label: "Panels", Href: "/admin/panels", Active: activePage == "panels", Allowed: security.HasPermission(adminRole, security.PermissionManagePanels)},
		{Label: "Inbounds", Href: "/admin/inbounds", Active: activePage == "inbounds", Allowed: security.HasPermission(adminRole, security.PermissionManageInbounds) || security.HasPermission(adminRole, security.PermissionViewDashboard)},
		{Label: "Clients", Href: "/admin/clients", Active: activePage == "clients", Allowed: security.HasPermission(adminRole, security.PermissionManageClients) || security.HasPermission(adminRole, security.PermissionViewClients)},
		{Label: "Subscriptions", Href: "/admin/subscriptions", Active: activePage == "subscriptions", Allowed: security.HasPermission(adminRole, security.PermissionManageSubscriptions) || security.HasPermission(adminRole, security.PermissionViewDashboard)},
		{Label: "Sync Jobs", Href: "/admin/sync-jobs", Active: activePage == "sync-jobs", Allowed: security.HasPermission(adminRole, security.PermissionViewDashboard)},
		{Label: "Backups", Href: "/admin/backups", Active: activePage == "backups", Allowed: security.HasRole(adminRole, security.RoleOwner, security.RoleAdmin)},
		{Label: "Notifications", Href: "/admin/notifications", Active: activePage == "notifications", Allowed: true},
		{Label: "Webhooks", Href: "/admin/webhooks", Active: activePage == "webhooks", Allowed: security.HasPermission(adminRole, security.PermissionManageSettings)},
		{Label: "Maintenance", Href: "/admin/maintenance", Active: activePage == "maintenance", Allowed: security.HasRole(adminRole, security.RoleOwner)},
		{Label: "Audit Logs", Href: "/admin/audit-logs", Active: activePage == "audit-logs", Allowed: security.HasPermission(adminRole, security.PermissionViewAuditLogs)},
		{Label: "Settings", Href: "/admin/settings", Active: activePage == "settings", Allowed: security.HasPermission(adminRole, security.PermissionManageSettings)},
		{Label: "Admin Users", Href: "/admin/users", Active: activePage == "admin-users", Allowed: security.HasPermission(adminRole, security.PermissionManageAdmins)},
	}

	var nav strings.Builder
	for _, item := range items {
		if !item.Allowed {
			continue
		}
		class := "nav-link text-body"
		if item.Active {
			class += " active fw-semibold bg-body-secondary"
		}
		nav.WriteString(`<a class="` + class + ` rounded px-3 py-2" href="` + item.Href + `">` + html.EscapeString(item.Label) + `</a>`)
	}

	return renderPage(appName+" Admin", `<script src="`+staticAssetURL("vendor/htmx/htmx.min.js")+`"></script><style>
.admin-shell{display:grid;grid-template-columns:280px 1fr;min-height:100vh}
.admin-sidebar{position:sticky;top:0;height:100vh;overflow:auto;background:var(--bs-body-bg);border-right:1px solid var(--bs-border-color)}
.admin-sidebar .nav-link{display:block}
.admin-sidebar .nav-link.active{background:var(--bs-secondary-bg);color:var(--bs-body-color)}
.admin-toggle{display:none}
@media (max-width: 991.98px){.admin-shell{grid-template-columns:1fr}.admin-sidebar{position:fixed;inset:0 auto 0 0;width:280px;transform:translateX(-100%);transition:transform .2s ease;z-index:1040}.admin-shell.sidebar-open .admin-sidebar{transform:translateX(0)}.admin-toggle{display:inline-flex}}
</style><div class="admin-shell" id="admin-shell"><aside class="admin-sidebar p-3"><div class="d-flex align-items-center justify-content-between mb-3"><div><div class="fw-semibold">` + html.EscapeString(appName) + `</div><div class="text-body-secondary small">Admin panel</div></div><button class="btn btn-outline-secondary btn-sm admin-toggle" type="button" onclick="document.getElementById('admin-shell').classList.remove('sidebar-open')">Close</button></div><nav class="nav nav-pills flex-column gap-1">` + nav.String() + `</nav></aside><div class="d-flex flex-column min-vh-100"><header class="border-bottom bg-body sticky-top"><div class="container-fluid py-3 d-flex align-items-center justify-content-between gap-3"><div class="d-flex align-items-center gap-2"><button class="btn btn-outline-secondary btn-sm admin-toggle" type="button" onclick="document.getElementById('admin-shell').classList.toggle('sidebar-open')">Menu</button><div class="fw-semibold">` + html.EscapeString(appName) + `</div></div><div class="d-flex align-items-center gap-3"><div id="admin-notification-bell" hx-get="/admin/notifications/bell" hx-trigger="load, every 30s" hx-swap="outerHTML" class="d-flex align-items-center"></div><div class="text-body-secondary small">` + html.EscapeString(strings.TrimSpace(activePage)) + `</div></div></div></header><main class="flex-grow-1">` + content + `</main></div></div>`)
}
