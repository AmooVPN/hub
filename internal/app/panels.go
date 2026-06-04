package app

import (
	"context"
	"errors"
	"fmt"
	"html"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/security"
	"github.com/AmooVPN/hub/internal/xui"
)

type panelListFilters struct {
	Status string
}

type panelForm struct {
	Name          string
	BaseURL       string
	Username      string
	Password      string
	APIToken      string
	ClearAPIToken bool
	Version       string
	Status        string
}

func (r *Runner) getAdminPanels(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	filters := panelListFilters{Status: strings.ToLower(strings.TrimSpace(c.Query("status")))}
	panels, err := r.panels.List(c.UserContext())
	if err != nil {
		return err
	}
	filtered := make([]models.Panel, 0, len(panels))
	for _, panel := range panels {
		if panelListMatchesFilters(&panel, filters) {
			filtered = append(filtered, panel)
		}
	}
	return c.Type("html").SendString(renderPanelListPage(filtered, filters, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminPanelNew(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderPanelFormPage("New Panel", "/admin/panels", panelForm{Status: models.PanelStatusUnknown}, r.cfg.AppName, admin.Role, nil, false))
}

func (r *Runner) postAdminPanelCreate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	form := parsePanelForm(c)
	if err := validatePanelForm(form, true); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderPanelFormPage("New Panel", "/admin/panels", form, r.cfg.AppName, admin.Role, []string{err.Error()}, false))
	}
	if err := validatePanelBaseURLPolicy(form.BaseURL, r.cfg.PanelURLStrictMode, r.cfg.PanelURLAllowPrivate); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderPanelFormPage("New Panel", "/admin/panels", form, r.cfg.AppName, admin.Role, []string{err.Error()}, false))
	}
	panel := &models.Panel{Name: form.Name, BaseURL: normalizeBaseURL(form.BaseURL), Username: form.Username, Version: form.Version, Status: defaultPanelStatus(form.Status), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	var err error
	if strings.TrimSpace(form.Password) != "" {
		panel.EncryptedPassword, err = security.Encrypt(form.Password, r.cfg.HUBSecretKey)
		if err != nil {
			return err
		}
	}
	if strings.TrimSpace(form.APIToken) != "" {
		panel.EncryptedAPIToken, err = security.Encrypt(form.APIToken, r.cfg.HUBSecretKey)
		if err != nil {
			return err
		}
	}
	if err := panel.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderPanelFormPage("New Panel", "/admin/panels", form, r.cfg.AppName, admin.Role, []string{err.Error()}, false))
	}
	if err := r.panels.Create(c.UserContext(), panel); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "panel_create", "panel", &panel.ID, map[string]any{"name": panel.Name, "base_url": panel.BaseURL})
	return c.Redirect(fmt.Sprintf("/admin/panels/%d", panel.ID), fiber.StatusFound)
}

func (r *Runner) getAdminPanelDetail(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderPanelDetailPageV2(panel, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminPanelEdit(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	form := panelForm{Name: panel.Name, BaseURL: panel.BaseURL, Username: panel.Username, Version: panel.Version, Status: panel.Status}
	return c.Type("html").SendString(renderPanelFormPage("Edit Panel", "/admin/panels/"+c.Params("id"), form, r.cfg.AppName, admin.Role, nil, true))
}

func (r *Runner) postAdminPanelUpdate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	form := parsePanelForm(c)
	if err := validatePanelForm(form, false); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderPanelFormPage("Edit Panel", "/admin/panels/"+c.Params("id"), form, r.cfg.AppName, admin.Role, []string{err.Error()}, true))
	}
	if err := validatePanelBaseURLPolicy(form.BaseURL, r.cfg.PanelURLStrictMode, r.cfg.PanelURLAllowPrivate); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderPanelFormPage("Edit Panel", "/admin/panels/"+c.Params("id"), form, r.cfg.AppName, admin.Role, []string{err.Error()}, true))
	}
	panel.Name = form.Name
	panel.BaseURL = normalizeBaseURL(form.BaseURL)
	panel.Username = form.Username
	panel.Version = form.Version
	panel.Status = defaultPanelStatus(form.Status)
	panel.UpdatedAt = time.Now().UTC()
	if strings.TrimSpace(form.Password) != "" {
		panel.EncryptedPassword, err = security.Encrypt(form.Password, r.cfg.HUBSecretKey)
		if err != nil {
			return err
		}
	}
	if form.ClearAPIToken {
		panel.EncryptedAPIToken = ""
	} else if strings.TrimSpace(form.APIToken) != "" {
		panel.EncryptedAPIToken, err = security.Encrypt(form.APIToken, r.cfg.HUBSecretKey)
		if err != nil {
			return err
		}
	}
	if err := panel.Validate(); err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderPanelFormPage("Edit Panel", "/admin/panels/"+c.Params("id"), form, r.cfg.AppName, admin.Role, []string{err.Error()}, true))
	}
	if err := r.panels.Update(c.UserContext(), panel); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "panel_update", "panel", &panel.ID, map[string]any{"name": panel.Name, "base_url": panel.BaseURL})
	return c.Redirect(fmt.Sprintf("/admin/panels/%d", panel.ID), fiber.StatusFound)
}

func (r *Runner) postAdminPanelDelete(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if err := r.panels.Delete(c.UserContext(), panel.ID); err != nil {
		return err
	}
	_ = r.clearPanelSession(c.UserContext(), panel.ID)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "panel_delete", "panel", &panel.ID, map[string]any{"name": panel.Name})
	return c.Redirect("/admin/panels", fiber.StatusFound)
}

func (r *Runner) postAdminPanelTest(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	result, status, message := r.testPanelConnection(c.UserContext(), panel)
	panel.Status = status
	panel.LastError = message
	panel.UpdatedAt = time.Now().UTC()
	if result != nil {
		panel.Version = result.Version
	}
	if err := r.panels.Update(c.UserContext(), panel); err != nil {
		return err
	}
	if r.notifications != nil && status != models.PanelStatusOnline {
		severity := models.NotificationSeverityWarning
		typ := models.NotificationTypePanelOffline
		if status == models.PanelStatusAuthError {
			severity = models.NotificationSeverityDanger
			typ = models.NotificationTypePanelAuthError
		}
		_ = r.notifications.Notify(c.UserContext(), typ, severity, "Panel test failed", panel.Name+": "+message)
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "panel_test", "panel", &panel.ID, map[string]any{"status": status})
	return c.Redirect(fmt.Sprintf("/admin/panels/%d", panel.ID), fiber.StatusFound)
}

func (r *Runner) postAdminPanelHealth(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	result, err := r.panelHealth.Check(c.UserContext(), panel)
	panel.Status = result.Status
	panel.LastError = result.Message
	now := time.Now().UTC()
	panel.LastCheckedAt = &now
	panel.UpdatedAt = now
	if result.Version != "" {
		panel.Version = result.Version
	}
	if err := r.panels.Update(c.UserContext(), panel); err != nil {
		return err
	}
	if r.notifications != nil && result.Status != models.PanelStatusOnline {
		severity := models.NotificationSeverityWarning
		typ := models.NotificationTypePanelOffline
		if result.Status == models.PanelStatusAuthError {
			severity = models.NotificationSeverityDanger
			typ = models.NotificationTypePanelAuthError
		}
		_ = r.notifications.Notify(c.UserContext(), typ, severity, "Panel health check", panel.Name+": "+result.Message)
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "panel_health_check", "panel", &panel.ID, map[string]any{"status": panel.Status, "error": panel.LastError})
	if err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminUsersPageMessage(err.Error(), r.cfg.AppName))
	}
	return c.Redirect(fmt.Sprintf("/admin/panels/%d", panel.ID), fiber.StatusFound)
}

func (r *Runner) postAdminPanelSync(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	panel.Status = models.PanelStatusSyncing
	panel.UpdatedAt = time.Now().UTC()
	if err := r.panels.Update(c.UserContext(), panel); err != nil {
		return err
	}
	if err := r.syncPanelInbounds(c.UserContext(), panel); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "panel_sync", "panel", &panel.ID, map[string]any{"status": panel.Status})
	return c.Redirect(fmt.Sprintf("/admin/panels/%d", panel.ID), fiber.StatusFound)
}

func (r *Runner) postAdminPanelSyncTraffic(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if err := r.syncPanelTraffic(c.UserContext(), panel); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "traffic_sync", "panel", &panel.ID, map[string]any{"panel_id": panel.ID})
	return c.Redirect(fmt.Sprintf("/admin/panels/%d", panel.ID), fiber.StatusFound)
}

func (r *Runner) postAdminPanelClearSession(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if err := r.clearPanelSession(c.UserContext(), panel.ID); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "panel_session_cleared", "panel", &panel.ID, nil)
	return c.Redirect(fmt.Sprintf("/admin/panels/%d", panel.ID), fiber.StatusFound)
}

func (r *Runner) loadPanel(ctx context.Context, idValue string) (*models.Panel, error) {
	id, err := strconv.ParseInt(idValue, 10, 64)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid panel id")
	}
	panel, err := r.panels.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return panel, nil
}

func (r *Runner) clearPanelSession(ctx context.Context, panelID int64) error {
	if r.redis == nil {
		return nil
	}
	return r.redis.Del(ctx, panelSessionRedisKey(panelID)).Err()
}

func (r *Runner) testPanelConnection(ctx context.Context, panel *models.Panel) (*xui.XUICompatibility, string, string) {
	password, apiToken, err := decryptPanelCredentials(panel.EncryptedPassword, panel.EncryptedAPIToken, r.cfg.HUBSecretKey)
	if err != nil {
		return nil, models.PanelStatusError, err.Error()
	}
	client := xui.NewClient(panel.ID, panel.BaseURL, panel.Username, password, apiToken)
	client.SetUserAgent(r.cfg.AppName)
	loginCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := client.Login(loginCtx); err != nil {
		return nil, classifyPanelError(err), err.Error()
	}
	listCtx, cancelList := context.WithTimeout(ctx, 20*time.Second)
	defer cancelList()
	if _, err := client.ListInbounds(listCtx); err != nil {
		return nil, classifyPanelError(err), err.Error()
	}
	compat := client.Compatibility()
	return &compat, models.PanelStatusOnline, ""
}

func (r *Runner) syncPanelMetadata(ctx context.Context, panel *models.Panel) (string, string) {
	compat, status, message := r.testPanelConnection(ctx, panel)
	if compat != nil && compat.Version != "" {
		panel.Version = compat.Version
	}
	if status == models.PanelStatusOnline {
		panel.LastError = ""
	}
	return status, message
}

func parsePanelForm(c *fiber.Ctx) panelForm {
	return panelForm{
		Name:          strings.TrimSpace(c.FormValue("name")),
		BaseURL:       strings.TrimSpace(c.FormValue("base_url")),
		Username:      strings.TrimSpace(c.FormValue("username")),
		Password:      c.FormValue("password"),
		APIToken:      c.FormValue("api_token"),
		ClearAPIToken: c.FormValue("clear_api_token") != "",
		Version:       strings.TrimSpace(c.FormValue("version")),
		Status:        strings.TrimSpace(c.FormValue("status")),
	}
}

func validatePanelForm(form panelForm, requirePassword bool) error {
	if form.Name == "" {
		return errors.New("name is required")
	}
	if err := validatePanelBaseURL(form.BaseURL); err != nil {
		return err
	}
	if form.Username == "" {
		return errors.New("username is required")
	}
	if requirePassword && strings.TrimSpace(form.Password) == "" && strings.TrimSpace(form.APIToken) == "" {
		return errors.New("password or api token is required")
	}
	if form.Status != "" && !models.IsValidPanelStatus(form.Status) {
		return errors.New("invalid status")
	}
	return nil
}

func validatePanelBaseURL(raw string) error {
	parsed, err := url.ParseRequestURI(raw)
	if err != nil {
		return errors.New("base url is invalid")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("base url must use http or https")
	}
	if parsed.Host == "" {
		return errors.New("base url host is required")
	}
	return nil
}

func validatePanelBaseURLPolicy(raw string, strictMode, allowPrivate bool) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return errors.New("base url is invalid")
	}
	host := parsed.Hostname()
	if host == "" {
		return nil
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return nil
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		if strictMode || !allowPrivate {
			return errors.New("base url points to a private address")
		}
	}
	return nil
}

func normalizeBaseURL(raw string) string {
	return strings.TrimRight(strings.TrimSpace(raw), "/")
}

func defaultPanelStatus(status string) string {
	if models.IsValidPanelStatus(status) {
		return status
	}
	return models.PanelStatusUnknown
}

func classifyPanelError(err error) string {
	if err == nil {
		return models.PanelStatusOnline
	}
	switch {
	case errors.Is(err, xui.ErrXUIUnauthorized), errors.Is(err, xui.ErrXUIForbidden):
		return models.PanelStatusAuthError
	case errors.Is(err, xui.ErrXUITimeout), errors.Is(err, xui.ErrXUIUnavailable):
		return models.PanelStatusOffline
	default:
		return models.PanelStatusError
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func panelSessionRedisKey(panelID int64) string {
	return fmt.Sprintf("hub:panel:%d:cookies", panelID)
}

func panelListMatchesFilters(panel *models.Panel, filters panelListFilters) bool {
	if panel == nil {
		return false
	}
	if filters.Status != "" && panel.Status != filters.Status {
		return false
	}
	return true
}

func renderPanelListPage(panels []models.Panel, filters panelListFilters, appName string, adminRole string) string {
	var rows strings.Builder
	for _, panel := range panels {
		rows.WriteString(`<tr><td>` + html.EscapeString(panel.Name) + `</td><td>` + html.EscapeString(panel.BaseURL) + `</td><td>` + html.EscapeString(panel.Username) + `</td><td>` + panelStatusBadge(panel.Status) + `</td><td>` + html.EscapeString(defaultString(panel.Version, "-")) + `</td><td>` + html.EscapeString(formatTimeOrDash(panel.LastSyncAt)) + `</td><td>` + html.EscapeString(formatTimeOrDash(panel.LastCheckedAt)) + `</td><td class="text-nowrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `">View</a> <a class="btn btn-outline-secondary btn-sm" href="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/edit">Edit</a></td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="8" class="text-body-secondary">No panels yet.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Panels</h1><p class="text-body-secondary mb-0">Remote 3x-ui panel connections</p></div><div class="d-flex gap-2"><a class="btn btn-primary btn-sm" href="/admin/panels/new">New panel</a><a class="btn btn-outline-secondary btn-sm" href="/admin">Back</a></div></div><div class="card shadow-sm mb-3"><div class="card-body"><form method="get" action="/admin/panels" class="row g-2 align-items-end"><div class="col-12 col-md-6"><label class="form-label" for="status">Status</label><select class="form-select" id="status" name="status"><option value="">All</option><option value="unknown"` + selectedOption(filters.Status, "unknown") + `>Unknown</option><option value="online"` + selectedOption(filters.Status, "online") + `>Online</option><option value="offline"` + selectedOption(filters.Status, "offline") + `>Offline</option><option value="auth_error"` + selectedOption(filters.Status, "auth_error") + `>Auth error</option><option value="degraded"` + selectedOption(filters.Status, "degraded") + `>Degraded</option><option value="syncing"` + selectedOption(filters.Status, "syncing") + `>Syncing</option><option value="error"` + selectedOption(filters.Status, "error") + `>Error</option></select></div><div class="col-12 col-md-6 d-flex gap-2"><button class="btn btn-primary" type="submit">Filter</button><a class="btn btn-outline-secondary" href="/admin/panels">Reset</a></div></form></div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Name</th><th>Base URL</th><th>User</th><th>Status</th><th>Version</th><th>Last Sync</th><th>Last Checked</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "panels", body)
}

func renderPanelDetailPage(panel *models.Panel, appName string, adminRole string) string {
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(panel.Name) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(panel.BaseURL) + `</p></div><div class="d-flex gap-2 flex-wrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/edit">Edit</a><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/test"><button class="btn btn-outline-primary btn-sm" type="submit">Test</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/sync"><button class="btn btn-outline-success btn-sm" type="submit">Sync</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/clear-session"><button class="btn btn-outline-warning btn-sm" type="submit">Clear session</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/delete" onsubmit="return confirm('Delete this panel?')"><button class="btn btn-outline-danger btn-sm" type="submit">Delete</button></form><a class="btn btn-outline-secondary btn-sm" href="/admin/panels">Back</a></div></div><div class="row g-3"><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Panel information</div><div class="card-body"><dl class="row mb-0"><dt class="col-sm-4">Status</dt><dd class="col-sm-8">` + panelStatusBadge(panel.Status) + `</dd><dt class="col-sm-4">Username</dt><dd class="col-sm-8">` + html.EscapeString(panel.Username) + `</dd><dt class="col-sm-4">Version</dt><dd class="col-sm-8">` + html.EscapeString(defaultString(panel.Version, "-")) + `</dd><dt class="col-sm-4">Last sync</dt><dd class="col-sm-8">` + html.EscapeString(formatTimeOrDash(panel.LastSyncAt)) + `</dd><dt class="col-sm-4">Last error</dt><dd class="col-sm-8 text-danger">` + html.EscapeString(defaultString(panel.LastError, "-")) + `</dd></dl></div></div></div><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Connection</div><div class="card-body"><div class="text-body-secondary small">Password is stored encrypted and never rendered back to the browser.</div><div class="mt-3"><a class="btn btn-outline-secondary btn-sm" href="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/edit">Update credentials</a></div></div></div></div></div></div>`
	return renderAdminShell(appName, adminRole, "panels", body)
}

func renderPanelDetailPageV2(panel *models.Panel, appName string, adminRole string) string {
	buttonBar := `<div class="d-flex gap-2 flex-wrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/edit">Edit</a><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/test"><button class="btn btn-outline-primary btn-sm" type="submit">Test</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/health"><button class="btn btn-outline-info btn-sm" type="submit">Health check</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/sync"><button class="btn btn-outline-success btn-sm" type="submit">Sync</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/sync-traffic"><button class="btn btn-outline-info btn-sm" type="submit">Sync traffic</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/clear-session"><button class="btn btn-outline-warning btn-sm" type="submit">Clear session</button></form><form method="post" action="/admin/panels/` + strconv.FormatInt(panel.ID, 10) + `/delete" onsubmit="return confirm('Delete this panel?')"><button class="btn btn-outline-danger btn-sm" type="submit">Delete</button></form><a class="btn btn-outline-secondary btn-sm" href="/admin/panels">Back</a></div>`
	authMethod := "Username / password"
	if strings.TrimSpace(panel.EncryptedAPIToken) != "" {
		authMethod = "API token"
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(panel.Name) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(panel.BaseURL) + `</p></div>` + buttonBar + `</div><div class="row g-3"><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Panel information</div><div class="card-body"><dl class="row mb-0"><dt class="col-sm-4">Status</dt><dd class="col-sm-8">` + panelStatusBadge(panel.Status) + `</dd><dt class="col-sm-4">Username</dt><dd class="col-sm-8">` + html.EscapeString(panel.Username) + `</dd><dt class="col-sm-4">Auth</dt><dd class="col-sm-8">` + html.EscapeString(authMethod) + `</dd><dt class="col-sm-4">Version</dt><dd class="col-sm-8">` + html.EscapeString(defaultString(panel.Version, "-")) + `</dd><dt class="col-sm-4">Last sync</dt><dd class="col-sm-8">` + html.EscapeString(formatTimeOrDash(panel.LastSyncAt)) + `</dd><dt class="col-sm-4">Last checked</dt><dd class="col-sm-8">` + html.EscapeString(formatTimeOrDash(panel.LastCheckedAt)) + `</dd><dt class="col-sm-4">Last error</dt><dd class="col-sm-8 text-danger">` + html.EscapeString(defaultString(panel.LastError, "-")) + `</dd></dl></div></div></div><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Panel details</div><div class="card-body"><dl class="row mb-0"><dt class="col-sm-4">Base URL</dt><dd class="col-sm-8">` + html.EscapeString(panel.BaseURL) + `</dd><dt class="col-sm-4">Created</dt><dd class="col-sm-8">` + html.EscapeString(panel.CreatedAt.UTC().Format(time.RFC3339)) + `</dd><dt class="col-sm-4">Updated</dt><dd class="col-sm-8">` + html.EscapeString(panel.UpdatedAt.UTC().Format(time.RFC3339)) + `</dd></dl></div></div></div></div></div>`
	return renderAdminShell(appName, adminRole, "panels", body)
}

func renderPanelFormPage(title, action string, form panelForm, appName string, adminRole string, errors []string, editing bool) string {
	var alert strings.Builder
	for _, errMsg := range errors {
		alert.WriteString(`<div class="alert alert-danger">` + html.EscapeString(errMsg) + `</div>`)
	}
	statusOptions := []string{models.PanelStatusUnknown, models.PanelStatusOnline, models.PanelStatusOffline, models.PanelStatusAuthError, models.PanelStatusDegraded, models.PanelStatusSyncing, models.PanelStatusError}
	var opts strings.Builder
	for _, status := range statusOptions {
		selected := ""
		if form.Status == status {
			selected = ` selected`
		}
		opts.WriteString(`<option value="` + status + `"` + selected + `>` + strings.ReplaceAll(status, "_", " ") + `</option>`)
	}
	passwordLabel := "Password"
	passwordNote := ""
	if editing {
		passwordNote = `<div class="form-text">Leave blank to keep current password.</div>`
	}
	body := `<div class="container py-4 py-lg-5" style="max-width: 760px;"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(title) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(appName) + `</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/panels">Back</a></div>` + alert.String() + `<div class="card shadow-sm"><div class="card-body"><form method="post" action="` + html.EscapeString(action) + `" class="vstack gap-3"><div><label class="form-label" for="name">Name</label><input class="form-control" id="name" name="name" value="` + html.EscapeString(form.Name) + `" required></div><div><label class="form-label" for="base_url">Base URL</label><input class="form-control" id="base_url" name="base_url" value="` + html.EscapeString(form.BaseURL) + `" placeholder="https://panel.example.com" required><div class="form-text text-warning">Public deployments should avoid private IP panel URLs.</div></div><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" value="` + html.EscapeString(form.Username) + `" required></div><div><label class="form-label" for="password">` + passwordLabel + `</label><input class="form-control" id="password" name="password" type="password"` + func() string {
		if editing {
			return ""
		}
		return " required"
	}() + `>` + passwordNote + `</div><div><label class="form-label" for="api_token">API token</label><input class="form-control" id="api_token" name="api_token" type="password"><div class="form-text">` + func() string {
		if editing {
			return "Leave blank to keep the current token. Check the box below to clear it. If provided, the token will be used instead of username/password for panel API requests."
		}
		return "Optional. If provided, the token will be used instead of username/password for panel API requests."
	}() + `</div>` + func() string {
		if !editing {
			return ""
		}
		return `<div class="form-check"><input class="form-check-input" type="checkbox" id="clear_api_token" name="clear_api_token"><label class="form-check-label" for="clear_api_token">Clear existing API token</label></div>`
	}() + `</div><div><label class="form-label" for="version">Version</label><input class="form-control" id="version" name="version" value="` + html.EscapeString(form.Version) + `"></div><div><label class="form-label" for="status">Status</label><select class="form-select" id="status" name="status">` + opts.String() + `</select></div><div class="d-flex gap-2"><button class="btn btn-primary" type="submit">Save</button></div></form></div></div></div>`
	return renderAdminShell(appName, adminRole, "panels", body)
}

func panelStatusBadge(status string) string {
	class := "secondary"
	switch status {
	case models.PanelStatusOnline:
		class = "success"
	case models.PanelStatusOffline:
		class = "danger"
	case models.PanelStatusAuthError:
		class = "warning"
	case models.PanelStatusDegraded:
		class = "primary"
	case models.PanelStatusSyncing:
		class = "info"
	case models.PanelStatusError:
		class = "dark"
	}
	return `<span class="badge text-bg-` + class + `">` + html.EscapeString(status) + `</span>`
}

func formatTimeOrDash(t *time.Time) string {
	if t == nil {
		return "-"
	}
	return t.UTC().Format(time.RFC3339)
}
