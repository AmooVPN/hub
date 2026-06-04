package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/security"
	"github.com/AmooVPN/hub/internal/services"
	"github.com/AmooVPN/hub/internal/xui"
)

type adminClientListRow struct {
	models.Client
	AttachmentCount int
	TotalBytes      int64
}

type clientListFilters struct {
	Query  string
	Status string
}

type inboundOption struct {
	PanelID int64
	Panel   string
	Inbound string
	Value   string
}

type inboundGroup struct {
	PanelID int64
	Panel   string
	Options []inboundOption
}

type attachmentBatchResult struct {
	Successes []string
	Failures  []string
}

type clientEditForm struct {
	Username         string
	DisplayName      string
	Email            string
	Status           string
	TrafficLimitText string
	ExpiryText       string
}

type clientCreateForm struct {
	clientEditForm
	Password string
}

func (r *Runner) getAdminClients(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	filters := clientListFilters{Query: strings.TrimSpace(c.Query("q")), Status: strings.ToLower(strings.TrimSpace(c.Query("status")))}
	clients, err := r.loadAdminClientList(c.UserContext(), filters)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminClientListPage(clients, filters, r.cfg.AppName, admin.Role))
}

func (r *Runner) getAdminClientNew(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	return c.Type("html").SendString(renderAdminClientCreatePage(clientCreateForm{}, "/admin/clients", r.cfg.AppName, admin.Role, nil))
}

func (r *Runner) postAdminClientCreate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	form := parseClientCreateForm(c)
	if form.Username == "" {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientCreatePage(form, "/admin/clients", r.cfg.AppName, admin.Role, []string{"username is required"}))
	}
	if form.Password == "" {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientCreatePage(form, "/admin/clients", r.cfg.AppName, admin.Role, []string{"password is required"}))
	}
	if form.Status != "" && form.Status != "active" && form.Status != "disabled" {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientCreatePage(form, "/admin/clients", r.cfg.AppName, admin.Role, []string{"invalid status"}))
	}
	limit, err := parseClientLimit(form.TrafficLimitText)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientCreatePage(form, "/admin/clients", r.cfg.AppName, admin.Role, []string{err.Error()}))
	}
	expiry, err := parseClientExpiry(form.ExpiryText)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientCreatePage(form, "/admin/clients", r.cfg.AppName, admin.Role, []string{err.Error()}))
	}
	client := &models.Client{Username: form.Username, DisplayName: form.DisplayName, Email: form.Email, Status: defaultClientStatus(form.Status), TrafficLimitBytes: limit, ExpiryTime: expiry, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := r.clientsSvc.Create(c.UserContext(), client, form.Password); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "client_create", "client", &client.ID, map[string]any{"username": client.Username, "status": client.Status})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d", client.ID), fiber.StatusFound)
}

func (r *Runner) getAdminClientDetail(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	attachments, err := r.loadClientAttachments(c.UserContext(), client.ID)
	if err != nil {
		return err
	}
	configs, err := r.loadClientConfigs(c.UserContext(), client.ID)
	if err != nil {
		return err
	}
	audits, err := r.loadClientAudits(c.UserContext(), client.ID)
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminClientDetailPage(client, summary, attachments, configs, audits, r.cfg.AppName, admin.Role, r.cfg.AppBaseURL))
}

func (r *Runner) getAdminClientEdit(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminClientEditPage(clientEditFormFromClient(client), "/admin/clients/"+c.Params("id"), r.cfg.AppName, admin.Role, nil))
}

func (r *Runner) postAdminClientUpdate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	form := parseClientEditForm(c)
	if form.Username == "" {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientEditPage(form, "/admin/clients/"+c.Params("id"), r.cfg.AppName, admin.Role, []string{"username is required"}))
	}
	if form.Status != "active" && form.Status != "disabled" {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientEditPage(form, "/admin/clients/"+c.Params("id"), r.cfg.AppName, admin.Role, []string{"invalid status"}))
	}
	updated := *client
	updated.Username = form.Username
	updated.DisplayName = form.DisplayName
	updated.Email = form.Email
	updated.Status = form.Status
	if form.TrafficLimitText != "" {
		limit, err := strconv.ParseInt(form.TrafficLimitText, 10, 64)
		if err != nil || limit < 0 {
			return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientEditPage(form, "/admin/clients/"+c.Params("id"), r.cfg.AppName, admin.Role, []string{"traffic limit must be a non-negative integer"}))
		}
		updated.TrafficLimitBytes = limit
	}
	if strings.TrimSpace(form.ExpiryText) == "" {
		updated.ExpiryTime = nil
	} else {
		expiry, err := time.Parse("2006-01-02", strings.TrimSpace(form.ExpiryText))
		if err != nil {
			return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientEditPage(form, "/admin/clients/"+c.Params("id"), r.cfg.AppName, admin.Role, []string{"expiry must be a valid date"}))
		}
		expiry = time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 0, 0, 0, 0, time.UTC)
		updated.ExpiryTime = &expiry
	}
	updated.UpdatedAt = time.Now().UTC()
	if err := r.clientsSvc.Update(c.UserContext(), &updated); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "client_update", "client", &updated.ID, map[string]any{"username": updated.Username, "status": updated.Status})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d", updated.ID), fiber.StatusFound)
}

func (r *Runner) postAdminClientDelete(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if err := r.clientsSvc.Delete(c.UserContext(), client.ID); err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "client_delete", "client", &client.ID, map[string]any{"username": client.Username})
	return c.Redirect("/admin/clients", fiber.StatusFound)
}

func parseClientEditForm(c *fiber.Ctx) clientEditForm {
	return clientEditForm{
		Username:         strings.TrimSpace(c.FormValue("username")),
		DisplayName:      strings.TrimSpace(c.FormValue("display_name")),
		Email:            strings.TrimSpace(c.FormValue("email")),
		Status:           strings.TrimSpace(c.FormValue("status")),
		TrafficLimitText: strings.TrimSpace(c.FormValue("traffic_limit_bytes")),
		ExpiryText:       strings.TrimSpace(c.FormValue("expiry_time")),
	}
}

func parseClientCreateForm(c *fiber.Ctx) clientCreateForm {
	return clientCreateForm{clientEditForm: parseClientEditForm(c), Password: strings.TrimSpace(c.FormValue("password"))}
}

func parseClientLimit(raw string) (int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	limit, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || limit < 0 {
		return 0, errors.New("traffic limit must be a non-negative integer")
	}
	return limit, nil
}

func parseClientExpiry(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	expiry, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, errors.New("expiry must be a valid date")
	}
	expiry = time.Date(expiry.Year(), expiry.Month(), expiry.Day(), 0, 0, 0, 0, time.UTC)
	return &expiry, nil
}

func defaultClientStatus(status string) string {
	if status == "disabled" {
		return status
	}
	return "active"
}

func clientEditFormFromClient(client *models.Client) clientEditForm {
	form := clientEditForm{}
	if client == nil {
		return form
	}
	form.Username = client.Username
	form.DisplayName = client.DisplayName
	form.Email = client.Email
	form.Status = client.Status
	form.TrafficLimitText = strconv.FormatInt(client.TrafficLimitBytes, 10)
	if client.ExpiryTime != nil {
		form.ExpiryText = client.ExpiryTime.UTC().Format("2006-01-02")
	}
	return form
}

func renderAdminClientCreatePage(form clientCreateForm, action, appName, adminRole string, errors []string) string {
	var alert strings.Builder
	for _, errMsg := range errors {
		alert.WriteString(`<div class="alert alert-danger">` + html.EscapeString(errMsg) + `</div>`)
	}
	statusOptions := []string{"active", "disabled"}
	var opts strings.Builder
	for _, status := range statusOptions {
		selected := ""
		if form.Status == status {
			selected = ` selected`
		}
		opts.WriteString(`<option value="` + status + `"` + selected + `>` + html.EscapeString(strings.Title(status)) + `</option>`)
	}
	body := `<div class="container py-4 py-lg-5" style="max-width: 760px;"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap mb-3"><div><h1 class="h3 mb-1">Create client</h1><p class="text-body-secondary mb-0">Add a central client</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/clients">Back</a></div>` + alert.String() + `<div class="card shadow-sm"><div class="card-body"><form method="post" action="` + html.EscapeString(action) + `" class="vstack gap-3"><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" value="` + html.EscapeString(form.Username) + `" required></div><div><label class="form-label" for="password">Password</label><input class="form-control" id="password" name="password" type="password" required></div><div><label class="form-label" for="display_name">Display name</label><input class="form-control" id="display_name" name="display_name" value="` + html.EscapeString(form.DisplayName) + `"></div><div><label class="form-label" for="email">Email</label><input class="form-control" id="email" name="email" type="email" value="` + html.EscapeString(form.Email) + `"></div><div><label class="form-label" for="status">Status</label><select class="form-select" id="status" name="status">` + opts.String() + `</select></div><div><label class="form-label" for="traffic_limit_bytes">Traffic limit bytes</label><input class="form-control" id="traffic_limit_bytes" name="traffic_limit_bytes" inputmode="numeric" value="` + html.EscapeString(form.TrafficLimitText) + `"></div><div><label class="form-label" for="expiry_time">Expiry date</label><input class="form-control" id="expiry_time" name="expiry_time" type="date" value="` + html.EscapeString(form.ExpiryText) + `"><div class="form-text">Leave blank for no expiry.</div></div><div class="d-flex gap-2"><button class="btn btn-primary" type="submit">Create</button></div></form></div></div></div>`
	return renderAdminShell(appName, adminRole, "clients", body)
}

func (r *Runner) postAdminClientDisable(c *fiber.Ctx) error {
	return r.postAdminClientStatus(c, "disabled")
}

func (r *Runner) postAdminClientEnable(c *fiber.Ctx) error {
	return r.postAdminClientStatus(c, "active")
}

func (r *Runner) postAdminClientStatus(c *fiber.Ctx, status string) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	if err := r.clientsSvc.SetStatus(c.UserContext(), client.ID, status); err != nil {
		return err
	}
	client.Status = status
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "client_status_update", "client", &client.ID, map[string]any{"status": status})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d", client.ID), fiber.StatusFound)
}

func (r *Runner) postAdminClientResetPassword(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	newPassword := strings.TrimSpace(c.FormValue("password"))
	if newPassword == "" {
		newPassword, err = security.RandomToken(12)
		if err != nil {
			return err
		}
	}
	if _, err := r.clientsSvc.ResetPassword(c.UserContext(), client.ID, newPassword); err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "client_password_reset", "client", &client.ID, nil)
	return c.Type("html").SendString(renderPasswordResetResultPage(client.Username, newPassword, r.cfg.AppName))
}

func (r *Runner) postAdminClientRegenerateSubscriptionToken(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	oldToken := client.SubscriptionToken
	newToken, err := r.clientsSvc.RegenerateToken(c.UserContext(), client.ID)
	if err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), oldToken)
	client.SubscriptionToken = newToken
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "client_subscription_token_regenerate", "client", &client.ID, nil)
	return c.Redirect(fmt.Sprintf("/admin/clients/%d", client.ID), fiber.StatusFound)
}

func (r *Runner) getAdminClientAttachments(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	attachments, err := r.loadClientAttachments(c.UserContext(), client.ID)
	if err != nil {
		return err
	}
	options, err := r.loadInboundOptions(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderAdminClientAttachmentsPage(client, attachments, options, r.cfg.AppName, admin.Role, nil))
}

func (r *Runner) postAdminClientAttachmentCreate(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	selectedIDs := parseInboundSelection(c)
	if len(selectedIDs) == 0 {
		return c.Status(fiber.StatusBadRequest).Type("html").SendString(renderAdminClientAttachmentsError(client, r.cfg.AppName, admin.Role, "select at least one inbound"))
	}
	result := &attachmentBatchResult{}
	for _, inboundID := range selectedIDs {
		inbound, err := r.inbounds.FindByID(c.UserContext(), inboundID)
		if err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("Inbound %d: %v", inboundID, err))
			continue
		}
		panel, err := r.panels.FindByID(c.UserContext(), inbound.PanelID)
		if err != nil {
			result.Failures = append(result.Failures, inboundLabel(inbound, "")+": "+err.Error())
			continue
		}
		label := inboundLabel(inbound, panel.Name)
		if inbound.Stale {
			result.Failures = append(result.Failures, label+": inbound is stale")
			continue
		}
		if existing, err := r.findClientAttachmentByClientInbound(c.UserContext(), client.ID, inbound.ID); err == nil && existing != nil {
			result.Failures = append(result.Failures, label+": already attached")
			continue
		} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
			result.Failures = append(result.Failures, label+": "+err.Error())
			continue
		}
		attachment, err := r.attachClientToInbound(c.UserContext(), client, panel, inbound)
		if err != nil {
			result.Failures = append(result.Failures, label+": "+err.Error())
			continue
		}
		result.Successes = append(result.Successes, label)
		_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "attachment_create", "client_attachment", &attachment.ID, map[string]any{"client_id": client.ID, "panel_id": panel.ID, "inbound_id": inbound.ID})
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	attachments, err := r.loadClientAttachments(c.UserContext(), client.ID)
	if err != nil {
		return err
	}
	options, err := r.loadInboundOptions(c.UserContext())
	if err != nil {
		return err
	}
	messages := formatAttachmentBatchMessages(result)
	return c.Type("html").SendString(renderAdminClientAttachmentsPage(client, attachments, options, r.cfg.AppName, admin.Role, messages))
}

func (r *Runner) postAdminClientAttachmentDelete(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	attachmentID, err := parseParamID(c.Params("attachment_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid attachment id")
	}
	attachment, err := r.loadClientAttachmentByID(c.UserContext(), attachmentID)
	if err != nil {
		return err
	}
	if attachment.ClientID != client.ID {
		return fiber.NewError(fiber.StatusNotFound, "attachment not found")
	}
	inbound, err := r.inbounds.FindByID(c.UserContext(), attachment.InboundID)
	if err != nil {
		return err
	}
	panel, err := r.panels.FindByID(c.UserContext(), attachment.PanelID)
	if err != nil {
		return err
	}
	if err := r.deleteRemoteAttachment(c.UserContext(), client, panel, inbound, attachment); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminClientAttachmentsError(client, r.cfg.AppName, admin.Role, err.Error()))
	}
	if err := r.deleteClientAttachment(c.UserContext(), attachment.ID); err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "attachment_delete", "client_attachment", &attachment.ID, map[string]any{"client_id": client.ID, "panel_id": panel.ID, "inbound_id": inbound.ID})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d/attachments", client.ID), fiber.StatusFound)
}

func (r *Runner) postAdminClientAttachmentDetach(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	attachmentID, err := parseParamID(c.Params("attachment_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid attachment id")
	}
	attachment, err := r.loadClientAttachmentByID(c.UserContext(), attachmentID)
	if err != nil {
		return err
	}
	if attachment.ClientID != client.ID {
		return fiber.NewError(fiber.StatusNotFound, "attachment not found")
	}
	inbound, err := r.inbounds.FindByID(c.UserContext(), attachment.InboundID)
	if err != nil {
		return err
	}
	panel, err := r.panels.FindByID(c.UserContext(), attachment.PanelID)
	if err != nil {
		return err
	}
	if err := r.deleteRemoteAttachment(c.UserContext(), client, panel, inbound, attachment); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminClientAttachmentsError(client, r.cfg.AppName, admin.Role, err.Error()))
	}
	if err := r.setClientAttachmentEnabled(c.UserContext(), attachment.ID, false); err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "attachment_detach", "client_attachment", &attachment.ID, map[string]any{"client_id": client.ID, "panel_id": panel.ID, "inbound_id": inbound.ID})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d/attachments", client.ID), fiber.StatusFound)
}

func (r *Runner) postAdminClientAttachmentRefresh(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	attachmentID, err := parseParamID(c.Params("attachment_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid attachment id")
	}
	attachment, err := r.loadClientAttachmentByID(c.UserContext(), attachmentID)
	if err != nil {
		return err
	}
	if attachment.ClientID != client.ID {
		return fiber.NewError(fiber.StatusNotFound, "attachment not found")
	}
	inbound, err := r.inbounds.FindByID(c.UserContext(), attachment.InboundID)
	if err != nil {
		return err
	}
	panel, err := r.panels.FindByID(c.UserContext(), attachment.PanelID)
	if err != nil {
		return err
	}
	attachment.RawConfig = generateAttachmentConfig(panel, inbound, attachment)
	attachment.UpdatedAt = time.Now().UTC()
	if err := r.saveClientAttachmentConfig(c.UserContext(), attachment.ID, attachment.RawConfig, attachment.UpdatedAt); err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "attachment_refresh", "client_attachment", &attachment.ID, map[string]any{"client_id": client.ID, "panel_id": panel.ID, "inbound_id": inbound.ID})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d/attachments", client.ID), fiber.StatusFound)
}

func (r *Runner) postAdminClientAttachmentSync(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	attachmentID, err := parseParamID(c.Params("attachment_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid attachment id")
	}
	attachment, err := r.loadClientAttachmentByID(c.UserContext(), attachmentID)
	if err != nil {
		return err
	}
	if attachment.ClientID != client.ID {
		return fiber.NewError(fiber.StatusNotFound, "attachment not found")
	}
	inbound, err := r.inbounds.FindByID(c.UserContext(), attachment.InboundID)
	if err != nil {
		return err
	}
	panel, err := r.panels.FindByID(c.UserContext(), attachment.PanelID)
	if err != nil {
		return err
	}
	traffic, err := r.syncAttachmentTraffic(c.UserContext(), panel, inbound, attachment)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminClientAttachmentsError(client, r.cfg.AppName, admin.Role, err.Error()))
	}
	if err := r.updateClientAttachmentTraffic(c.UserContext(), attachment.ID, traffic.Upload, traffic.Download); err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "attachment_sync", "client_attachment", &attachment.ID, map[string]any{"client_id": client.ID, "panel_id": panel.ID, "inbound_id": inbound.ID, "upload_bytes": traffic.Upload, "download_bytes": traffic.Download})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d/attachments", client.ID), fiber.StatusFound)
}

func (r *Runner) postAdminClientAttachmentDisable(c *fiber.Ctx) error {
	return r.postAdminClientAttachmentToggleEnabled(c, false)
}

func (r *Runner) postAdminClientAttachmentEnable(c *fiber.Ctx) error {
	return r.postAdminClientAttachmentToggleEnabled(c, true)
}

func (r *Runner) postAdminClientAttachmentToggleEnabled(c *fiber.Ctx, enabled bool) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	attachmentID, err := parseParamID(c.Params("attachment_id"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid attachment id")
	}
	attachment, err := r.loadClientAttachmentByID(c.UserContext(), attachmentID)
	if err != nil {
		return err
	}
	if attachment.ClientID != client.ID {
		return fiber.NewError(fiber.StatusNotFound, "attachment not found")
	}
	inbound, err := r.inbounds.FindByID(c.UserContext(), attachment.InboundID)
	if err != nil {
		return err
	}
	panel, err := r.panels.FindByID(c.UserContext(), attachment.PanelID)
	if err != nil {
		return err
	}
	if err := r.updateRemoteAttachmentEnabled(c.UserContext(), panel, inbound, attachment, enabled); err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminClientAttachmentsError(client, r.cfg.AppName, admin.Role, err.Error()))
	}
	if err := r.setClientAttachmentEnabled(c.UserContext(), attachment.ID, enabled); err != nil {
		return err
	}
	r.invalidateSubscriptionCache(c.UserContext(), client.SubscriptionToken)
	action := "attachment_disable"
	if enabled {
		action = "attachment_enable"
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), action, "client_attachment", &attachment.ID, map[string]any{"client_id": client.ID, "panel_id": panel.ID, "inbound_id": inbound.ID, "enabled": enabled})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d/attachments", client.ID), fiber.StatusFound)
}

func (r *Runner) loadAdminClientList(ctx context.Context, filters clientListFilters) ([]adminClientListRow, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT c.id, c.username, c.password_hash, c.display_name, c.email, c.status, c.traffic_limit_bytes, c.expiry_time, c.subscription_token, c.created_at, c.updated_at, COALESCE(COUNT(a.id), 0), COALESCE(SUM(COALESCE(a.upload_bytes, 0) + COALESCE(a.download_bytes, 0)), 0) FROM clients c LEFT JOIN client_attachments a ON a.client_id = c.id GROUP BY c.id ORDER BY c.id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []adminClientListRow
	now := time.Now().UTC()
	for rows.Next() {
		var row adminClientListRow
		var displayName, email, subscriptionToken sql.NullString
		var expiryTime sql.NullTime
		if err := rows.Scan(&row.ID, &row.Username, &row.PasswordHash, &displayName, &email, &row.Status, &row.TrafficLimitBytes, &expiryTime, &subscriptionToken, &row.CreatedAt, &row.UpdatedAt, &row.AttachmentCount, &row.TotalBytes); err != nil {
			return nil, err
		}
		row.DisplayName = displayName.String
		row.Email = email.String
		row.SubscriptionToken = subscriptionToken.String
		if expiryTime.Valid {
			t := expiryTime.Time
			row.ExpiryTime = &t
		}
		if !clientListMatchesFilters(&row.Client, row.TotalBytes, now, filters) {
			continue
		}
		items = append(items, row)
	}
	return items, rows.Err()
}

func clientListMatchesFilters(client *models.Client, totalBytes int64, now time.Time, filters clientListFilters) bool {
	if client == nil {
		return false
	}
	if filters.Query != "" {
		needle := strings.ToLower(filters.Query)
		if !strings.Contains(strings.ToLower(client.Username), needle) && !strings.Contains(strings.ToLower(client.DisplayName), needle) && !strings.Contains(strings.ToLower(client.Email), needle) {
			return false
		}
	}
	if filters.Status != "" && clientStatusLabel(client, totalBytes, now) != filters.Status {
		return false
	}
	return true
}

func clientStatusLabel(client *models.Client, totalBytes int64, now time.Time) string {
	return services.DetermineClientStatus(client, totalBytes, now)
}

func (r *Runner) loadClientByParam(ctx context.Context, idValue string) (*models.Client, error) {
	id, err := parseParamID(idValue)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "invalid client id")
	}
	return r.loadClientByID(ctx, id)
}

func (r *Runner) loadClientAttachments(ctx context.Context, clientID int64) ([]models.ClientAttachment, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, client_id, panel_id, inbound_id, remote_client_id, remote_email, enabled, upload_bytes, download_bytes, traffic_limit_bytes, expiry_time, raw_config, created_at, updated_at FROM client_attachments WHERE client_id = ? ORDER BY id DESC`, clientID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var attachments []models.ClientAttachment
	for rows.Next() {
		item, err := scanClientAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, *item)
	}
	return attachments, rows.Err()
}

func (r *Runner) loadClientAttachmentByID(ctx context.Context, id int64) (*models.ClientAttachment, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, client_id, panel_id, inbound_id, remote_client_id, remote_email, enabled, upload_bytes, download_bytes, traffic_limit_bytes, expiry_time, raw_config, created_at, updated_at FROM client_attachments WHERE id = ?`, id)
	return scanClientAttachment(row)
}

func (r *Runner) loadClientAudits(ctx context.Context, clientID int64) ([]models.AuditLog, error) {
	if r.audit == nil {
		return nil, nil
	}
	return r.audit.ListByTarget(ctx, "client", clientID, 20)
}

func (r *Runner) loadInboundOptions(ctx context.Context) ([]inboundGroup, error) {
	panels, err := r.panels.List(ctx)
	if err != nil {
		return nil, err
	}
	inbounds, err := r.inbounds.List(ctx)
	if err != nil {
		return nil, err
	}
	panelNames := make(map[int64]string, len(panels))
	for _, panel := range panels {
		panelNames[panel.ID] = panel.Name
	}
	groupMap := make(map[int64]*inboundGroup, len(panels))
	for _, panel := range panels {
		groupMap[panel.ID] = &inboundGroup{PanelID: panel.ID, Panel: panel.Name}
	}
	for _, inbound := range inbounds {
		group := groupMap[inbound.PanelID]
		if group == nil {
			group = &inboundGroup{PanelID: inbound.PanelID, Panel: "Panel #" + strconv.FormatInt(inbound.PanelID, 10)}
			groupMap[inbound.PanelID] = group
		}
		group.Options = append(group.Options, inboundOption{
			PanelID: inbound.PanelID,
			Panel:   panelNames[inbound.PanelID],
			Inbound: inbound.Remark,
			Value:   strconv.FormatInt(inbound.ID, 10),
		})
	}
	groups := make([]inboundGroup, 0, len(groupMap))
	for _, panel := range panels {
		if group := groupMap[panel.ID]; group != nil {
			sort.Slice(group.Options, func(i, j int) bool { return group.Options[i].Inbound < group.Options[j].Inbound })
			groups = append(groups, *group)
		}
	}
	return groups, nil
}

func (r *Runner) attachClientToInbound(ctx context.Context, client *models.Client, panel *models.Panel, inbound *models.Inbound) (*models.ClientAttachment, error) {
	password, apiToken, err := decryptPanelCredentials(panel.EncryptedPassword, panel.EncryptedAPIToken, r.cfg.HUBSecretKey)
	if err != nil {
		return nil, err
	}
	remoteID := remoteAttachmentIdentity(client, inbound)
	xuiClient := xui.NewClient(panel.ID, panel.BaseURL, panel.Username, password, apiToken)
	xuiClient.SetUserAgent(r.cfg.AppName)
	loginCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := xuiClient.Login(loginCtx); err != nil {
		return nil, err
	}
	addCtx, cancelAdd := context.WithTimeout(ctx, 20*time.Second)
	defer cancelAdd()
	result, err := xuiClient.AddClient(addCtx, int(inbound.RemoteInboundID), xui.XUIClientCreateRequest{Email: remoteID, ID: remoteID, Enable: true})
	if err != nil {
		return nil, err
	}
	if result == nil {
		return nil, errors.New("remote client creation returned no result")
	}
	now := time.Now().UTC()
	rawConfig, _ := json.Marshal(map[string]any{
		"remote_client_id": result.ID,
		"remote_email":     result.Email,
		"panel_id":         panel.ID,
		"inbound_id":       inbound.ID,
	})
	attachment := &models.ClientAttachment{
		ClientID:          client.ID,
		PanelID:           panel.ID,
		InboundID:         inbound.ID,
		RemoteClientID:    defaultString(result.ID, remoteID),
		RemoteEmail:       defaultString(result.Email, remoteID),
		Enabled:           true,
		UploadBytes:       0,
		DownloadBytes:     0,
		TrafficLimitBytes: 0,
		RawConfig:         string(rawConfig),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	attachment.RawConfig = generateAttachmentConfig(panel, inbound, attachment)
	if err := r.createClientAttachment(ctx, attachment); err != nil {
		_ = xuiClient.DeleteClient(context.Background(), int(inbound.RemoteInboundID), defaultString(result.ID, remoteID))
		return nil, err
	}
	return attachment, nil
}

func (r *Runner) deleteRemoteAttachment(ctx context.Context, client *models.Client, panel *models.Panel, inbound *models.Inbound, attachment *models.ClientAttachment) error {
	if attachment.RemoteClientID == "" {
		return nil
	}
	password, apiToken, err := decryptPanelCredentials(panel.EncryptedPassword, panel.EncryptedAPIToken, r.cfg.HUBSecretKey)
	if err != nil {
		return err
	}
	xuiClient := xui.NewClient(panel.ID, panel.BaseURL, panel.Username, password, apiToken)
	xuiClient.SetUserAgent(r.cfg.AppName)
	loginCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := xuiClient.Login(loginCtx); err != nil {
		return err
	}
	deleteCtx, cancelDelete := context.WithTimeout(ctx, 20*time.Second)
	defer cancelDelete()
	return xuiClient.DeleteClient(deleteCtx, int(inbound.RemoteInboundID), attachment.RemoteClientID)
}

func (r *Runner) createClientAttachment(ctx context.Context, attachment *models.ClientAttachment) error {
	if attachment == nil {
		return errors.New("attachment is nil")
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO client_attachments (client_id, panel_id, inbound_id, remote_client_id, remote_email, enabled, upload_bytes, download_bytes, traffic_limit_bytes, expiry_time, raw_config, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, attachment.ClientID, attachment.PanelID, attachment.InboundID, sqlStringOrNil(attachment.RemoteClientID), sqlStringOrNil(attachment.RemoteEmail), boolToDBInt(attachment.Enabled), attachment.UploadBytes, attachment.DownloadBytes, attachment.TrafficLimitBytes, timeOrNil(attachment.ExpiryTime), sqlStringOrNil(attachment.RawConfig), attachment.CreatedAt.UTC(), attachment.UpdatedAt.UTC())
	return err
}

func (r *Runner) findClientAttachmentByClientInbound(ctx context.Context, clientID, inboundID int64) (*models.ClientAttachment, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, client_id, panel_id, inbound_id, remote_client_id, remote_email, enabled, upload_bytes, download_bytes, traffic_limit_bytes, expiry_time, raw_config, created_at, updated_at FROM client_attachments WHERE client_id = ? AND inbound_id = ? LIMIT 1`, clientID, inboundID)
	return scanClientAttachment(row)
}

func (r *Runner) deleteClientAttachment(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM client_attachments WHERE id = ?`, id)
	return err
}

func (r *Runner) setClientAttachmentEnabled(ctx context.Context, id int64, enabled bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE client_attachments SET enabled = ?, updated_at = ? WHERE id = ?`, boolToDBInt(enabled), time.Now().UTC(), id)
	return err
}

func (r *Runner) updateClientAttachmentTraffic(ctx context.Context, id int64, uploadBytes, downloadBytes int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE client_attachments SET upload_bytes = ?, download_bytes = ?, updated_at = ? WHERE id = ?`, uploadBytes, downloadBytes, time.Now().UTC(), id)
	return err
}

func (r *Runner) saveClientAttachmentConfig(ctx context.Context, id int64, rawConfig string, updatedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE client_attachments SET raw_config = ?, updated_at = ? WHERE id = ?`, rawConfig, updatedAt.UTC(), id)
	return err
}

func (r *Runner) updateRemoteAttachmentEnabled(ctx context.Context, panel *models.Panel, inbound *models.Inbound, attachment *models.ClientAttachment, enabled bool) error {
	if attachment == nil || attachment.RemoteClientID == "" {
		return nil
	}
	password, apiToken, err := decryptPanelCredentials(panel.EncryptedPassword, panel.EncryptedAPIToken, r.cfg.HUBSecretKey)
	if err != nil {
		return err
	}
	xuiClient := xui.NewClient(panel.ID, panel.BaseURL, panel.Username, password, apiToken)
	xuiClient.SetUserAgent(r.cfg.AppName)
	loginCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := xuiClient.Login(loginCtx); err != nil {
		return err
	}
	updateCtx, cancelUpdate := context.WithTimeout(ctx, 20*time.Second)
	defer cancelUpdate()
	return xuiClient.UpdateClient(updateCtx, int(inbound.RemoteInboundID), attachment.RemoteClientID, xui.XUIClientUpdateRequest{Email: attachment.RemoteEmail, Enable: enabled})
}

func (r *Runner) syncAttachmentTraffic(ctx context.Context, panel *models.Panel, inbound *models.Inbound, attachment *models.ClientAttachment) (*xui.XUITraffic, error) {
	if attachment == nil || attachment.RemoteClientID == "" {
		return nil, errors.New("attachment is missing remote client id")
	}
	password, apiToken, err := decryptPanelCredentials(panel.EncryptedPassword, panel.EncryptedAPIToken, r.cfg.HUBSecretKey)
	if err != nil {
		return nil, err
	}
	xuiClient := xui.NewClient(panel.ID, panel.BaseURL, panel.Username, password, apiToken)
	xuiClient.SetUserAgent(r.cfg.AppName)
	loginCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := xuiClient.Login(loginCtx); err != nil {
		return nil, err
	}
	trafficCtx, cancelTraffic := context.WithTimeout(ctx, 20*time.Second)
	defer cancelTraffic()
	traffic, err := xuiClient.GetClientTraffic(trafficCtx, attachment.RemoteClientID)
	if err != nil {
		return nil, err
	}
	return traffic, nil
}

func generateAttachmentConfig(panel *models.Panel, inbound *models.Inbound, attachment *models.ClientAttachment) string {
	if panel == nil || inbound == nil || attachment == nil {
		return ""
	}
	host := panelHost(panel.BaseURL)
	if host == "" {
		host = panel.BaseURL
	}
	port := strconv.Itoa(inbound.Port)
	name := url.QueryEscape(defaultString(attachment.RemoteEmail, inbound.Remark))
	id := url.QueryEscape(defaultString(attachment.RemoteClientID, attachment.RemoteEmail))
	protocol := strings.ToLower(strings.TrimSpace(inbound.Protocol))
	network := strings.ToLower(strings.TrimSpace(defaultString(inbound.Network, "tcp")))
	security := strings.TrimSpace(inbound.Security)
	switch protocol {
	case "vless":
		query := url.Values{}
		query.Set("type", network)
		query.Set("encryption", "none")
		if security != "" {
			query.Set("security", security)
		}
		return fmt.Sprintf("vless://%s@%s:%s?%s#%s", id, host, port, query.Encode(), name)
	case "trojan":
		query := url.Values{}
		query.Set("type", network)
		if security != "" {
			query.Set("security", security)
		}
		return fmt.Sprintf("trojan://%s@%s:%s?%s#%s", id, host, port, query.Encode(), name)
	case "vmess":
		payload, _ := json.Marshal(map[string]any{
			"v":    "2",
			"ps":   defaultString(attachment.RemoteEmail, inbound.Remark),
			"add":  host,
			"port": inbound.Port,
			"id":   defaultString(attachment.RemoteClientID, attachment.RemoteEmail),
			"aid":  "0",
			"net":  network,
			"type": "none",
			"host": "",
			"path": "",
			"tls":  strings.EqualFold(security, "tls"),
		})
		return "vmess://" + base64.StdEncoding.EncodeToString(payload)
	case "shadowsocks", "ss":
		method := "aes-128-gcm"
		payload := method + ":" + defaultString(attachment.RemoteClientID, attachment.RemoteEmail) + "@" + host + ":" + port
		return "ss://" + base64.StdEncoding.EncodeToString([]byte(payload)) + "#" + name
	default:
		if strings.TrimSpace(attachment.RawConfig) != "" {
			return attachment.RawConfig
		}
		return fmt.Sprintf("placeholder://%s/%d/%d", protocol, panel.ID, inbound.ID)
	}
}

func panelHost(baseURL string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Host
}

func scanClientAttachment(scanner interface{ Scan(...any) error }) (*models.ClientAttachment, error) {
	var attachment models.ClientAttachment
	var remoteClientID, remoteEmail, rawConfig sql.NullString
	var expiryTime sql.NullTime
	var enabled int
	if err := scanner.Scan(&attachment.ID, &attachment.ClientID, &attachment.PanelID, &attachment.InboundID, &remoteClientID, &remoteEmail, &enabled, &attachment.UploadBytes, &attachment.DownloadBytes, &attachment.TrafficLimitBytes, &expiryTime, &rawConfig, &attachment.CreatedAt, &attachment.UpdatedAt); err != nil {
		return nil, err
	}
	attachment.RemoteClientID = remoteClientID.String
	attachment.RemoteEmail = remoteEmail.String
	attachment.Enabled = enabled != 0
	attachment.RawConfig = rawConfig.String
	if expiryTime.Valid {
		t := expiryTime.Time
		attachment.ExpiryTime = &t
	}
	return &attachment, nil
}

func remoteAttachmentIdentity(client *models.Client, inbound *models.Inbound) string {
	username := sanitizeAttachmentSegment(client.Username)
	if username == "" {
		username = "client"
	}
	return fmt.Sprintf("hub_%s_%d_%d", username, client.ID, inbound.ID)
}

func sanitizeAttachmentSegment(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastUnderscore = false
		default:
			if !lastUnderscore {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

func renderAdminClientListPage(clients []adminClientListRow, filters clientListFilters, appName, adminRole string) string {
	var rows strings.Builder
	now := time.Now().UTC()
	for _, client := range clients {
		status := clientStatusLabel(&client.Client, client.TotalBytes, now)
		rows.WriteString(`<tr><td><a href="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments">` + html.EscapeString(client.Username) + `</a></td><td>` + html.EscapeString(defaultString(client.DisplayName, "-")) + `</td><td>` + html.EscapeString(defaultString(client.Email, "-")) + `</td><td>` + clientStatusBadge(status) + `</td><td>` + html.EscapeString(strconv.FormatInt(int64(client.AttachmentCount), 10)) + `</td><td class="text-nowrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments">Manage</a></td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="6" class="text-body-secondary">No clients yet.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">Clients</h1><p class="text-body-secondary mb-0">Manage client attachments</p></div><div class="d-flex gap-2"><a class="btn btn-primary btn-sm" href="/admin/clients/new">New client</a></div></div><div class="card shadow-sm mb-3"><div class="card-body"><form method="get" action="/admin/clients" class="row g-2 align-items-end"><div class="col-12 col-md-6"><label class="form-label" for="q">Search</label><input class="form-control" id="q" name="q" value="` + html.EscapeString(filters.Query) + `" placeholder="Username, display name, or email"></div><div class="col-12 col-md-3"><label class="form-label" for="status">Status</label><select class="form-select" id="status" name="status"><option value="">All</option><option value="active"` + selectedOption(filters.Status, "active") + `>Active</option><option value="disabled"` + selectedOption(filters.Status, "disabled") + `>Disabled</option><option value="expired"` + selectedOption(filters.Status, "expired") + `>Expired</option><option value="limited"` + selectedOption(filters.Status, "limited") + `>Limited</option></select></div><div class="col-12 col-md-3 d-flex gap-2"><button class="btn btn-primary" type="submit">Filter</button><a class="btn btn-outline-secondary" href="/admin/clients">Reset</a></div></form></div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Username</th><th>Display name</th><th>Email</th><th>Status</th><th>Attachments</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "clients", body)
}

func selectedOption(current, value string) string {
	if current == value {
		return ` selected`
	}
	return ""
}

func clientStatusBadge(status string) string {
	switch status {
	case "active":
		return `<span class="badge text-bg-success">active</span>`
	case "disabled":
		return `<span class="badge text-bg-dark">disabled</span>`
	case "expired":
		return `<span class="badge text-bg-warning">expired</span>`
	case "limited":
		return `<span class="badge text-bg-danger">limited</span>`
	default:
		return `<span class="badge text-bg-secondary">` + html.EscapeString(status) + `</span>`
	}
}

func renderAdminClientAttachmentsPage(client *models.Client, attachments []models.ClientAttachment, options []inboundGroup, appName, adminRole string, messages []string) string {
	var alert strings.Builder
	inboundLabels := make(map[string]string)
	var batchMessage strings.Builder
	statusLabel := client.Status
	if strings.TrimSpace(statusLabel) == "" {
		statusLabel = "unknown"
	}
	expiryText := "No expiry"
	if client.ExpiryTime != nil {
		expiryText = client.ExpiryTime.UTC().Format(time.RFC3339)
	}
	trafficLimitText := formatBytes(client.TrafficLimitBytes)
	attachmentCountText := strconv.Itoa(len(attachments))
	for _, message := range messages {
		alert.WriteString(`<div class="alert alert-info">` + html.EscapeString(message) + `</div>`)
	}
	var selectOpts strings.Builder
	for _, group := range options {
		selectOpts.WriteString(`<optgroup label="` + html.EscapeString(defaultString(group.Panel, "Panel")) + `">`)
		for _, option := range group.Options {
			label := defaultString(option.Panel+" / "+option.Inbound, "Inbound")
			inboundLabels[option.Value] = label
			selectOpts.WriteString(`<option value="` + html.EscapeString(option.Value) + `">` + html.EscapeString(label) + `</option>`)
		}
		selectOpts.WriteString(`</optgroup>`)
	}
	var rows strings.Builder
	for _, attachment := range attachments {
		label := inboundLabels[strconv.FormatInt(attachment.InboundID, 10)]
		if label == "" {
			label = "Inbound #" + strconv.FormatInt(attachment.InboundID, 10)
		}
		toggleAction := "disable"
		toggleLabel := "Disable"
		btnClass := "btn-outline-warning"
		if !attachment.Enabled {
			toggleAction = "enable"
			toggleLabel = "Enable"
			btnClass = "btn-outline-success"
		}
		detachDisabled := ""
		if !attachment.Enabled {
			detachDisabled = " disabled"
		}
		rows.WriteString(`<tr><td>` + html.EscapeString(defaultString(label, "-")) + `</td><td>` + html.EscapeString(defaultString(attachment.RemoteClientID, "-")) + `</td><td>` + html.EscapeString(defaultString(attachment.RemoteEmail, "-")) + `</td><td>` + attachmentStateBadge(attachment.Enabled) + `</td><td class="text-nowrap"><div class="d-flex gap-2 flex-wrap"><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments/` + strconv.FormatInt(attachment.ID, 10) + `/refresh"><button class="btn btn-outline-secondary btn-sm" type="submit">Refresh config</button></form><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments/` + strconv.FormatInt(attachment.ID, 10) + `/sync"><button class="btn btn-outline-primary btn-sm" type="submit">Sync</button></form><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments/` + strconv.FormatInt(attachment.ID, 10) + `/detach"><button class="btn btn-outline-warning btn-sm" type="submit"` + detachDisabled + `>Detach</button></form><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments/` + strconv.FormatInt(attachment.ID, 10) + `/` + toggleAction + `"><button class="btn ` + btnClass + ` btn-sm" type="submit">` + toggleLabel + `</button></form><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments/` + strconv.FormatInt(attachment.ID, 10) + `/delete" onsubmit="return confirm('Delete this attachment?')"><button class="btn btn-outline-danger btn-sm" type="submit">Delete</button></form></div></td></tr>`)
	}
	if rows.Len() == 0 {
		rows.WriteString(`<tr><td colspan="5" class="text-body-secondary">No attachments yet.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(client.Username) + ` attachments</h1><p class="text-body-secondary mb-0">Manage client attachments</p></div><div class="d-flex gap-2"><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/sync-traffic"><button class="btn btn-outline-info btn-sm" type="submit">Sync traffic</button></form><a class="btn btn-outline-secondary btn-sm" href="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `">Client detail</a><a class="btn btn-outline-secondary btn-sm" href="/admin/clients">Back</a></div></div>` + alert.String() + batchMessage.String() + `<div class="row g-3 mb-3"><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Status</div><div class="fw-semibold">` + clientStatusBadge(statusLabel) + `</div><div class="text-body-secondary small mt-2">` + html.EscapeString(defaultString(client.DisplayName, "No display name")) + `</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Expiry</div><div class="fw-semibold">` + html.EscapeString(expiryText) + `</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Traffic limit</div><div class="fw-semibold">` + html.EscapeString(trafficLimitText) + `</div></div></div></div><div class="col-12 col-md-6 col-xl-3"><div class="card shadow-sm h-100"><div class="card-body"><div class="text-body-secondary small">Attachments</div><div class="fw-semibold">` + html.EscapeString(attachmentCountText) + `</div><div class="text-body-secondary small mt-2">` + html.EscapeString(defaultString(client.Email, "No email")) + `</div></div></div></div></div><div class="card shadow-sm mb-3"><div class="card-body"><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments" class="row g-2 align-items-end"><div class="col-12 col-md-9"><label class="form-label" for="inbound_ids">Inbounds</label><select class="form-select" id="inbound_ids" name="inbound_ids" multiple size="8" required>` + selectOpts.String() + `</select><div class="form-text">Use Ctrl/Cmd to select multiple inbounds.</div></div><div class="col-12 col-md-3"><button class="btn btn-primary w-100" type="submit">Attach selected</button></div></form></div></div><div class="card shadow-sm"><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Inbound</th><th>Remote ID</th><th>Remote Email</th><th>Status</th><th>Actions</th></tr></thead><tbody>` + rows.String() + `</tbody></table></div></div></div>`
	return renderAdminShell(appName, adminRole, "clients", body)
}

func renderAdminClientAttachmentsError(client *models.Client, appName, adminRole, message string) string {
	return renderAdminClientAttachmentsPage(client, nil, nil, appName, adminRole, []string{message})
}

func renderAdminClientDetailPage(client *models.Client, summary clientSummary, attachments []models.ClientAttachment, configs []clientConfigRow, audits []models.AuditLog, appName, adminRole, baseURL string) string {
	statusBadge := "secondary"
	switch strings.ToLower(summary.StatusText) {
	case "active":
		statusBadge = "success"
	case "expired":
		statusBadge = "warning"
	case "disabled":
		statusBadge = "dark"
	case "deleted":
		statusBadge = "secondary"
	}
	buttons := `<div class="d-flex gap-2 flex-wrap"><a class="btn btn-outline-secondary btn-sm" href="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/edit">Edit</a><a class="btn btn-outline-secondary btn-sm" href="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/attachments">Manage attachments</a><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/sync-traffic"><button class="btn btn-outline-info btn-sm" type="submit">Sync traffic</button></form><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/regenerate-token"><button class="btn btn-outline-primary btn-sm" type="submit">Regenerate subscription token</button></form><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/reset-password"><button class="btn btn-outline-warning btn-sm" type="submit">Reset password</button></form><form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/delete" onsubmit="return confirm('Delete this client?')"><button class="btn btn-outline-danger btn-sm" type="submit">Delete</button></form>`
	if client.Status == "disabled" || client.Status == "deleted" {
		buttons += `<form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/enable"><button class="btn btn-outline-success btn-sm" type="submit">Enable</button></form>`
	} else {
		buttons += `<form method="post" action="/admin/clients/` + strconv.FormatInt(client.ID, 10) + `/disable"><button class="btn btn-outline-danger btn-sm" type="submit">Disable</button></form>`
	}
	buttons += `<a class="btn btn-outline-secondary btn-sm" href="/admin/clients">Back</a></div>`
	var attachmentRows strings.Builder
	for _, attachment := range attachments {
		attachmentRows.WriteString(`<tr><td>` + html.EscapeString(strconv.FormatInt(attachment.ID, 10)) + `</td><td>` + html.EscapeString(strconv.FormatInt(attachment.PanelID, 10)) + `</td><td>` + html.EscapeString(strconv.FormatInt(attachment.InboundID, 10)) + `</td><td>` + attachmentStateBadge(attachment.Enabled) + `</td><td>` + html.EscapeString(defaultString(attachment.RemoteEmail, "-")) + `</td></tr>`)
	}
	if attachmentRows.Len() == 0 {
		attachmentRows.WriteString(`<tr><td colspan="5" class="text-body-secondary">No attachments yet.</td></tr>`)
	}
	var configRows strings.Builder
	for _, cfg := range configs {
		configRows.WriteString(`<tr><td>` + html.EscapeString(cfg.PanelName) + `</td><td>` + html.EscapeString(cfg.InboundRemark) + `</td><td>` + html.EscapeString(cfg.Protocol) + `</td><td>` + html.EscapeString(cfg.Status) + `</td><td class="text-nowrap"><input class="form-control form-control-sm d-inline-block w-auto" value="` + html.EscapeString(cfg.CopyValue) + `" readonly><button class="btn btn-outline-secondary btn-sm ms-2" type="button" onclick="navigator.clipboard.writeText(this.previousElementSibling.value)">Copy</button></td></tr>`)
	}
	if configRows.Len() == 0 {
		configRows.WriteString(`<tr><td colspan="5" class="text-body-secondary">No active configs.</td></tr>`)
	}
	var auditRows strings.Builder
	for _, audit := range audits {
		auditRows.WriteString(`<tr><td>` + html.EscapeString(audit.Action) + `</td><td>` + html.EscapeString(audit.ActorType) + `</td><td>` + html.EscapeString(defaultString(audit.TargetType, "-")) + `</td><td>` + html.EscapeString(audit.CreatedAt.Format(time.RFC3339)) + `</td></tr>`)
	}
	if auditRows.Len() == 0 {
		auditRows.WriteString(`<tr><td colspan="4" class="text-body-secondary">No audit history found.</td></tr>`)
	}
	body := `<div class="container py-4 py-lg-5"><div class="d-flex align-items-center justify-content-between flex-wrap gap-3 mb-3"><div><h1 class="h3 mb-1">` + html.EscapeString(client.Username) + `</h1><p class="text-body-secondary mb-0">Client detail</p></div>` + buttons + `</div><div class="row g-3"><div class="col-12 col-lg-6"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Identity & usage</div><div class="card-body"><dl class="row mb-0"><dt class="col-sm-4">Status</dt><dd class="col-sm-8"><span class="badge text-bg-` + statusBadge + `">` + html.EscapeString(summary.StatusText) + `</span></dd><dt class="col-sm-4">Expiry</dt><dd class="col-sm-8">` + html.EscapeString(defaultString(summary.ExpiryText, "No expiry")) + `</dd><dt class="col-sm-4">Usage</dt><dd class="col-sm-8">` + html.EscapeString(formatBytes(summary.TotalBytes)) + `</dd><dt class="col-sm-4">Traffic limit</dt><dd class="col-sm-8">` + html.EscapeString(formatBytes(client.TrafficLimitBytes)) + `</dd></dl></div></div></div><div class="col-12 col-lg-6"><div class="card shadow-sm h-100"><div class="card-header fw-semibold">Subscription</div><div class="card-body vstack gap-3"><div><div class="text-body-secondary small">Subscription URL</div><div class="input-group"><input class="form-control" value="` + html.EscapeString(strings.TrimRight(baseURL, "/")+"/sub/"+client.SubscriptionToken) + `" readonly><button class="btn btn-outline-secondary" type="button" onclick="navigator.clipboard.writeText(this.previousElementSibling.value)">Copy</button></div></div><div><div class="text-body-secondary small">Token</div><div class="input-group"><input class="form-control" value="` + html.EscapeString(client.SubscriptionToken) + `" readonly><button class="btn btn-outline-secondary" type="button" onclick="navigator.clipboard.writeText(this.previousElementSibling.value)">Copy</button></div></div></div></div></div><div class="col-12"><div class="card shadow-sm"><div class="card-header fw-semibold">Attached inbounds</div><div class="table-responsive"><table class="table mb-0"><thead><tr><th>ID</th><th>Panel</th><th>Inbound</th><th>Status</th><th>Email</th></tr></thead><tbody>` + attachmentRows.String() + `</tbody></table></div></div></div><div class="col-12"><div class="card shadow-sm"><div class="card-header fw-semibold">Configs</div><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Panel</th><th>Inbound</th><th>Protocol</th><th>Status</th><th>Copy</th></tr></thead><tbody>` + configRows.String() + `</tbody></table></div></div></div><div class="col-12"><div class="card shadow-sm"><div class="card-header fw-semibold">Audit history</div><div class="table-responsive"><table class="table mb-0"><thead><tr><th>Action</th><th>Actor</th><th>Target</th><th>When</th></tr></thead><tbody>` + auditRows.String() + `</tbody></table></div></div></div></div>`
	return renderAdminShell(appName, adminRole, "clients", body)
}

func renderAdminClientEditPage(form clientEditForm, action, appName, adminRole string, errors []string) string {
	var alert strings.Builder
	for _, errMsg := range errors {
		alert.WriteString(`<div class="alert alert-danger">` + html.EscapeString(errMsg) + `</div>`)
	}
	statusOptions := []string{"active", "disabled"}
	var opts strings.Builder
	for _, status := range statusOptions {
		selected := ""
		if form.Status == status {
			selected = ` selected`
		}
		opts.WriteString(`<option value="` + status + `"` + selected + `>` + html.EscapeString(strings.Title(status)) + `</option>`)
	}
	body := `<div class="container py-4 py-lg-5" style="max-width: 760px;"><div class="d-flex align-items-center justify-content-between gap-3 flex-wrap mb-3"><div><h1 class="h3 mb-1">Edit client</h1><p class="text-body-secondary mb-0">Update identity and limits</p></div><a class="btn btn-outline-secondary btn-sm" href="/admin/clients">Back</a></div>` + alert.String() + `<div class="card shadow-sm"><div class="card-body"><form method="post" action="` + html.EscapeString(action) + `" class="vstack gap-3"><div><label class="form-label" for="username">Username</label><input class="form-control" id="username" name="username" value="` + html.EscapeString(form.Username) + `" required></div><div><label class="form-label" for="display_name">Display name</label><input class="form-control" id="display_name" name="display_name" value="` + html.EscapeString(form.DisplayName) + `"></div><div><label class="form-label" for="email">Email</label><input class="form-control" id="email" name="email" type="email" value="` + html.EscapeString(form.Email) + `"></div><div><label class="form-label" for="status">Status</label><select class="form-select" id="status" name="status">` + opts.String() + `</select></div><div><label class="form-label" for="traffic_limit_bytes">Traffic limit bytes</label><input class="form-control" id="traffic_limit_bytes" name="traffic_limit_bytes" inputmode="numeric" value="` + html.EscapeString(form.TrafficLimitText) + `"></div><div><label class="form-label" for="expiry_time">Expiry date</label><input class="form-control" id="expiry_time" name="expiry_time" type="date" value="` + html.EscapeString(form.ExpiryText) + `"><div class="form-text">Leave blank to clear the expiry.</div></div><div class="d-flex gap-2 flex-wrap"><button class="btn btn-primary" type="submit">Save</button></div></form></div></div></div>`
	return renderAdminShell(appName, adminRole, "clients", body)
}

func attachmentStateBadge(enabled bool) string {
	if enabled {
		return `<span class="badge text-bg-success">enabled</span>`
	}
	return `<span class="badge text-bg-secondary">disabled</span>`
}

func parseInboundSelection(c *fiber.Ctx) []int64 {
	values := make([]int64, 0)
	if raw := strings.TrimSpace(c.FormValue("inbound_id")); raw != "" {
		if id, err := parseParamID(raw); err == nil {
			values = append(values, id)
		}
	}
	c.Context().PostArgs().VisitAll(func(key, value []byte) {
		if string(key) != "inbound_ids" {
			return
		}
		parts := strings.Split(string(value), ",")
		for _, part := range parts {
			if id, err := parseParamID(strings.TrimSpace(part)); err == nil {
				values = append(values, id)
			}
		}
	})
	seen := make(map[int64]struct{}, len(values))
	out := make([]int64, 0, len(values))
	for _, id := range values {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func inboundLabel(inbound *models.Inbound, panelName string) string {
	label := defaultString(inbound.Remark, "Inbound #"+strconv.FormatInt(inbound.ID, 10))
	if panelName == "" {
		return label
	}
	return panelName + " / " + label
}

func formatAttachmentBatchMessages(result *attachmentBatchResult) []string {
	if result == nil {
		return nil
	}
	messages := make([]string, 0, 2)
	if len(result.Successes) > 0 {
		messages = append(messages, "Attached: "+strings.Join(result.Successes, ", "))
	}
	if len(result.Failures) > 0 {
		messages = append(messages, "Failed: "+strings.Join(result.Failures, "; "))
	}
	return messages
}

func sqlStringOrNil(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func boolToDBInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func timeOrNil(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}
