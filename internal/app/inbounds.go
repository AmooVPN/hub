package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/security"
	"github.com/AmooVPM/hub/internal/xui"
)

type inboundFilters struct {
	PanelID  int64
	Protocol string
	Network  string
	Security string
}

type selectOption struct {
	Value string
	Label string
}

func (r *Runner) getAdminInbounds(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	filters, err := parseInboundFilters(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	inbounds, err := r.listFilteredInbounds(c.UserContext(), filters)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderInboundListPage("Inbounds", inbounds, filters, r.inboundPanelNames(c.UserContext()), r.cfg.AppName, admin.Role, "/admin/inbounds"))
}

func (r *Runner) getAdminPanelInbounds(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	panel, err := r.loadPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	filters, err := parseInboundFilters(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}
	filters.PanelID = panel.ID
	inbounds, err := r.listFilteredInbounds(c.UserContext(), filters)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderInboundListPage(panel.Name+" Inbounds", inbounds, filters, map[int64]string{panel.ID: panel.Name}, r.cfg.AppName, admin.Role, "/admin/panels/"+strconv.FormatInt(panel.ID, 10)))
}

func (r *Runner) getAdminInboundDetail(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	inbound, panel, err := r.loadInboundWithPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderInboundDetailPage(inbound, panel, r.cfg.AppName, admin.Role))
}

func (r *Runner) postAdminInboundRefresh(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	inbound, panel, err := r.loadInboundWithPanel(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if err := r.syncPanelInbounds(c.UserContext(), panel); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "inbound_refresh", "inbound", &inbound.ID, map[string]any{"panel_id": panel.ID, "remote_inbound_id": inbound.RemoteInboundID})
	return c.Redirect(fmt.Sprintf("/admin/inbounds/%d", inbound.ID), fiber.StatusFound)
}

func (r *Runner) loadInboundWithPanel(ctx context.Context, idValue string) (*models.Inbound, *models.Panel, error) {
	id, err := parseParamID(idValue)
	if err != nil {
		return nil, nil, fiber.NewError(fiber.StatusBadRequest, "invalid inbound id")
	}
	inbound, err := r.inbounds.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	panel, err := r.panels.FindByID(ctx, inbound.PanelID)
	if err != nil {
		return nil, nil, err
	}
	return inbound, panel, nil
}

func (r *Runner) listFilteredInbounds(ctx context.Context, filters inboundFilters) ([]models.Inbound, error) {
	var items []models.Inbound
	var err error
	if filters.PanelID > 0 {
		items, err = r.inbounds.ListByPanel(ctx, filters.PanelID)
	} else {
		items, err = r.inbounds.List(ctx)
	}
	if err != nil {
		return nil, err
	}
	filtered := items[:0]
	for _, item := range items {
		if filters.Protocol != "" && !strings.EqualFold(item.Protocol, filters.Protocol) {
			continue
		}
		if filters.Network != "" && !strings.EqualFold(item.Network, filters.Network) {
			continue
		}
		if filters.Security != "" && !strings.EqualFold(item.Security, filters.Security) {
			continue
		}
		filtered = append(filtered, item)
	}
	return filtered, nil
}

func (r *Runner) inboundPanelNames(ctx context.Context) map[int64]string {
	panels, err := r.panels.List(ctx)
	if err != nil {
		return map[int64]string{}
	}
	result := make(map[int64]string, len(panels))
	for _, panel := range panels {
		result[panel.ID] = panel.Name
	}
	return result
}

func (r *Runner) syncPanelInbounds(ctx context.Context, panel *models.Panel) error {
	if panel == nil {
		return errors.New("panel is nil")
	}
	return r.withPanelSyncLock(ctx, panel.ID, func() error {
		jobID, err := r.startSyncJob(ctx, &panel.ID, "inbound_sync")
		if err != nil {
			return err
		}
		if err := r.updateSyncJob(ctx, jobID, "running", "", nil); err != nil {
			return err
		}
		password, err := security.Decrypt(panel.EncryptedPassword, r.cfg.HUBSecretKey)
		if err != nil {
			return r.finishInboundSync(ctx, jobID, panel, models.PanelStatusError, err.Error(), err)
		}
		client := xui.NewClient(panel.ID, panel.BaseURL, panel.Username, password)
		client.SetUserAgent(r.cfg.AppName)
		loginCtx, cancelLogin := context.WithTimeout(ctx, 10*time.Second)
		defer cancelLogin()
		if err := client.Login(loginCtx); err != nil {
			return r.finishInboundSync(ctx, jobID, panel, classifyPanelError(err), err.Error(), err)
		}
		listCtx, cancelList := context.WithTimeout(ctx, 20*time.Second)
		defer cancelList()
		remoteInbounds, err := client.ListInbounds(listCtx)
		if err != nil {
			return r.finishInboundSync(ctx, jobID, panel, classifyPanelError(err), err.Error(), err)
		}
		now := time.Now().UTC()
		seen := make([]int64, 0, len(remoteInbounds))
		for _, remote := range remoteInbounds {
			rawJSON, _ := json.Marshal(remote)
			inbound := &models.Inbound{
				PanelID:         panel.ID,
				RemoteInboundID: remote.ID,
				Remark:          remote.Remark,
				Protocol:        remote.Protocol,
				Port:            remote.Port,
				Network:         remote.Network,
				Security:        remote.Security,
				Enabled:         remote.Enabled,
				Stale:           false,
				RawJSON:         string(rawJSON),
				LastSyncedAt:    &now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := r.inbounds.Upsert(ctx, inbound); err != nil {
				return r.finishInboundSync(ctx, jobID, panel, models.PanelStatusError, err.Error(), err)
			}
			seen = append(seen, remote.ID)
		}
		if err := r.inbounds.MarkStaleByPanel(ctx, panel.ID, seen); err != nil {
			return r.finishInboundSync(ctx, jobID, panel, models.PanelStatusError, err.Error(), err)
		}
		panel.Status = models.PanelStatusOnline
		if compat := client.Compatibility(); compat.Version != "" {
			panel.Version = compat.Version
		}
		panel.LastError = ""
		panel.LastSyncAt = &now
		panel.UpdatedAt = now
		if err := r.panels.Update(ctx, panel); err != nil {
			return r.finishInboundSync(ctx, jobID, panel, models.PanelStatusError, err.Error(), err)
		}
		_ = r.logAudit(ctx, "admin", nil, "inbound_sync", "panel", &panel.ID, map[string]any{"status": "success", "count": len(remoteInbounds)})
		finishedAt := time.Now().UTC()
		return r.updateSyncJob(ctx, jobID, "success", "", &finishedAt)
	})
}

func (r *Runner) updateSyncJob(ctx context.Context, jobID int64, status, message string, finishedAt *time.Time) error {
	return r.jobs.Update(ctx, jobID, status, message, finishedAt)
}

func (r *Runner) finishInboundSync(ctx context.Context, jobID int64, panel *models.Panel, status, message string, cause error) error {
	finishedAt := time.Now().UTC()
	if panel != nil {
		panel.Status = status
		panel.LastError = message
		panel.UpdatedAt = finishedAt
		_ = r.panels.Update(ctx, panel)
	}
	_ = r.updateSyncJob(ctx, jobID, "error", message, &finishedAt)
	if cause != nil {
		return cause
	}
	return nil
}

func parseInboundFilters(c *fiber.Ctx) (inboundFilters, error) {
	var filters inboundFilters
	if raw := strings.TrimSpace(c.Query("panel_id")); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return filters, fiber.NewError(fiber.StatusBadRequest, "invalid panel filter")
		}
		filters.PanelID = id
	}
	filters.Protocol = strings.TrimSpace(c.Query("protocol"))
	filters.Network = strings.TrimSpace(c.Query("network"))
	filters.Security = strings.TrimSpace(c.Query("security"))
	return filters, nil
}

func renderInboundListPage(title string, inbounds []models.Inbound, filters inboundFilters, panelNames map[int64]string, appName, adminRole, backHref string) string {
	panelOptions := panelSelectOptions(panelNames)
	protocolOptions := stringSelectOptions(inbounds, func(inbound models.Inbound) string { return inbound.Protocol })
	networkOptions := stringSelectOptions(inbounds, func(inbound models.Inbound) string { return inbound.Network })
	securityOptions := stringSelectOptions(inbounds, func(inbound models.Inbound) string { return inbound.Security })
	var rows strings.Builder
	for _, inbound := range inbounds {
		panelName := panelNames[inbound.PanelID]
		rows.WriteString(`<tr><td>` + html.EscapeString(defaultString(panelName, "-")) + `</td><td>` + html.EscapeString(strconv.FormatInt(inbound.RemoteInboundID, 10)) + `</td><td>` + html.EscapeString(defaultString(inbound.Remark, "-")) + `</td><td>` + html.EscapeString(defaultString(inbound.Protocol, "-")) + `</td><td>` + html.EscapeString(strconv.Itoa(inbound.Port)) + `</td><td>` + html.EscapeString(defaultString(inbound.Network, "-")) + `</td><td>` + html.EscapeString(defaultString(inbound.Security, "-")) + `</td><td>` + yesNoBadge(inbound.Enabled) + `</td><td>` + staleBadge(inbound.Stale) + `</td><td>` + html.EscapeString(formatTimeOrDash(inbound.LastSyncedAt)) + `</td><td class="text-nowrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/inbounds/` + strconv.FormatInt(inbound.ID, 10) + `">View</a> <form class="d-inline" method="post" action="/admin/inbounds/` + strconv.FormatInt(inbound.ID, 10) + `/refresh"><button class="btn btn-outline-primary btn-sm" type="submit">Refresh</button></form></td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="11" class="text-body-secondary">No inbounds found.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(title) + `</h1><p class="text-body-secondary mb-0">Synced inbound mirror</p></div><div class="d-flex gap-2"><a class="btn btn-outline-secondary btn-sm" href="` + html.EscapeString(backHref) + `">Back</a></div></div><div class="card shadow-sm mb-3"><div class="card-body"><form method="get" class="row g-2 align-items-end"><div class="col-12 col-md-3"><label class="form-label" for="panel_id">Panel</label><select class="form-select" id="panel_id" name="panel_id"><option value="">All panels</option>` + optionHTML(panelOptions, strconv.FormatInt(filters.PanelID, 10)) + `</select></div><div class="col-12 col-md-3"><label class="form-label" for="protocol">Protocol</label><select class="form-select" id="protocol" name="protocol"><option value="">All protocols</option>` + optionHTML(protocolOptions, filters.Protocol) + `</select></div><div class="col-12 col-md-3"><label class="form-label" for="network">Network</label><select class="form-select" id="network" name="network"><option value="">All networks</option>` + optionHTML(networkOptions, filters.Network) + `</select></div><div class="col-12 col-md-3"><label class="form-label" for="security">Security</label><select class="form-select" id="security" name="security"><option value="">All security</option>` + optionHTML(securityOptions, filters.Security) + `</select></div><div class="col-12"><button class="btn btn-primary btn-sm" type="submit">Filter</button></div></form></div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Panel</th><th>Remote ID</th><th>Remark</th><th>Protocol</th><th>Port</th><th>Network</th><th>Security</th><th>Enabled</th><th>Stale</th><th>Last Synced</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "inbounds", body)
}

func renderInboundDetailPage(inbound *models.Inbound, panel *models.Panel, appName, adminRole string) string {
	rawJSON := html.EscapeString(defaultString(inbound.RawJSON, "{}"))
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(defaultString(inbound.Remark, "Inbound #"+strconv.FormatInt(inbound.ID, 10))) + `</h1><p class="text-body-secondary mb-0">` + html.EscapeString(panel.Name) + `</p></div><div class="d-flex gap-2 flex-wrap"><form method="post" action="/admin/inbounds/` + strconv.FormatInt(inbound.ID, 10) + `/refresh"><button class="btn btn-outline-primary btn-sm" type="submit">Refresh</button></form><a class="btn btn-outline-secondary btn-sm" href="/admin/inbounds">Back</a></div></div><div class="row g-3"><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Inbound information</div><div class="card-body"><dl class="row mb-0"><dt class="col-sm-4">Panel</dt><dd class="col-sm-8">` + html.EscapeString(panel.Name) + `</dd><dt class="col-sm-4">Remote ID</dt><dd class="col-sm-8">` + html.EscapeString(strconv.FormatInt(inbound.RemoteInboundID, 10)) + `</dd><dt class="col-sm-4">Protocol</dt><dd class="col-sm-8">` + html.EscapeString(defaultString(inbound.Protocol, "-")) + `</dd><dt class="col-sm-4">Port</dt><dd class="col-sm-8">` + html.EscapeString(strconv.Itoa(inbound.Port)) + `</dd><dt class="col-sm-4">Network</dt><dd class="col-sm-8">` + html.EscapeString(defaultString(inbound.Network, "-")) + `</dd><dt class="col-sm-4">Security</dt><dd class="col-sm-8">` + html.EscapeString(defaultString(inbound.Security, "-")) + `</dd><dt class="col-sm-4">Enabled</dt><dd class="col-sm-8">` + yesNoBadge(inbound.Enabled) + `</dd><dt class="col-sm-4">Stale</dt><dd class="col-sm-8">` + staleBadge(inbound.Stale) + `</dd><dt class="col-sm-4">Last synced</dt><dd class="col-sm-8">` + html.EscapeString(formatTimeOrDash(inbound.LastSyncedAt)) + `</dd></dl></div></div></div><div class="col-12 col-lg-6"><div class="card shadow-sm"><div class="card-header fw-semibold">Raw JSON</div><div class="card-body"><details><summary class="mb-2">View payload</summary><pre class="small bg-body-tertiary p-3 rounded border overflow-auto">` + rawJSON + `</pre></details></div></div></div></div></div>`
	return renderAdminShell(appName, adminRole, "inbounds", body)
}

func panelSelectOptions(panelNames map[int64]string) []selectOption {
	keys := make([]int64, 0, len(panelNames))
	for id := range panelNames {
		keys = append(keys, id)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	options := make([]selectOption, 0, len(keys))
	for _, id := range keys {
		options = append(options, selectOption{Value: strconv.FormatInt(id, 10), Label: panelNames[id]})
	}
	return options
}

func stringSelectOptions(inbounds []models.Inbound, valueFn func(models.Inbound) string) []selectOption {
	seen := make(map[string]struct{})
	values := make([]selectOption, 0)
	for _, inbound := range inbounds {
		value := strings.TrimSpace(valueFn(inbound))
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, selectOption{Value: value, Label: value})
	}
	return values
}

func optionHTML(options []selectOption, selectedValue string) string {
	var out strings.Builder
	for _, option := range options {
		attr := ""
		if option.Value == selectedValue {
			attr = ` selected`
		}
		out.WriteString(`<option value="` + html.EscapeString(option.Value) + `"` + attr + `>` + html.EscapeString(option.Label) + `</option>`)
	}
	return out.String()
}

func yesNoBadge(ok bool) string {
	if ok {
		return `<span class="badge text-bg-success">yes</span>`
	}
	return `<span class="badge text-bg-secondary">no</span>`
}

func staleBadge(stale bool) string {
	if stale {
		return `<span class="badge text-bg-warning">stale</span>`
	}
	return `<span class="badge text-bg-success">fresh</span>`
}
