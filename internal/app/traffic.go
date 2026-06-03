package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
)

type trafficSyncReport struct {
	Updated  int
	Skipped  int
	Failures []string
}

func (r *Runner) postAdminClientSyncTraffic(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	client, err := r.loadClientByParam(c.UserContext(), c.Params("id"))
	if err != nil {
		return err
	}
	report, err := r.syncClientTraffic(c.UserContext(), client)
	if err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminClientAttachmentsError(client, r.cfg.AppName, admin.Role, err.Error()))
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "traffic_sync", "client", &client.ID, map[string]any{"updated": report.Updated, "skipped": report.Skipped})
	return c.Redirect(fmt.Sprintf("/admin/clients/%d/attachments", client.ID), fiber.StatusFound)
}

func (r *Runner) postAdminSyncTraffic(c *fiber.Ctx) error {
	admin, ok := currentAdmin(c)
	if !ok {
		return c.Redirect("/admin/login", fiber.StatusFound)
	}
	report, err := r.syncAllTraffic(c.UserContext())
	if err != nil {
		return c.Status(fiber.StatusBadGateway).Type("html").SendString(renderAdminUsersPageMessage(err.Error(), r.cfg.AppName))
	}
	_ = r.logAudit(c.UserContext(), "admin", adminActorID(admin), "traffic_sync", "system", nil, map[string]any{"updated": report.Updated, "skipped": report.Skipped})
	return c.Redirect("/admin", fiber.StatusFound)
}

func (r *Runner) syncPanelTraffic(ctx context.Context, panel *models.Panel) error {
	if panel == nil {
		return errors.New("panel is nil")
	}
	jobID, err := r.startSyncJob(ctx, &panel.ID, models.JobTypeTrafficSync)
	if err != nil {
		return err
	}
	report, err := r.syncTrafficForAttachments(ctx, panel.ID, 0)
	status := "success"
	message := trafficSyncReportMessage(report)
	if err != nil {
		status = "failed"
		message = err.Error()
	}
	if len(report.Failures) > 0 {
		status = "failed"
		if message == "" {
			message = trafficSyncReportMessage(report)
		}
		if err == nil {
			err = errors.New(message)
		}
	}
	finishedAt := time.Now().UTC()
	if updErr := r.updateSyncJob(ctx, jobID, status, message, &finishedAt); updErr != nil && err == nil {
		err = updErr
	}
	return err
}

func (r *Runner) syncClientTraffic(ctx context.Context, client *models.Client) (*trafficSyncReport, error) {
	if client == nil {
		return nil, errors.New("client is nil")
	}
	jobID, err := r.startSyncJob(ctx, nil, models.JobTypeTrafficSync)
	if err != nil {
		return nil, err
	}
	report, syncErr := r.syncTrafficForAttachments(ctx, 0, client.ID)
	status := "success"
	message := trafficSyncReportMessage(report)
	if syncErr != nil {
		status = "failed"
		message = syncErr.Error()
	}
	if len(report.Failures) > 0 {
		status = "failed"
		if message == "" {
			message = trafficSyncReportMessage(report)
		}
		if syncErr == nil {
			syncErr = errors.New(message)
		}
	}
	finishedAt := time.Now().UTC()
	if updErr := r.updateSyncJob(ctx, jobID, status, message, &finishedAt); updErr != nil && syncErr == nil {
		syncErr = updErr
	}
	return report, syncErr
}

func (r *Runner) syncTrafficForAttachments(ctx context.Context, panelID, clientID int64) (*trafficSyncReport, error) {
	attachments, err := r.loadTrafficSyncAttachments(ctx, panelID, clientID)
	if err != nil {
		return nil, err
	}
	report := &trafficSyncReport{}
	for _, attachment := range attachments {
		if !attachment.Enabled {
			report.Skipped++
			continue
		}
		if err := r.withPanelSyncLock(ctx, attachment.PanelID, func() error {
			inbound, err := r.inbounds.FindByID(ctx, attachment.InboundID)
			if err != nil {
				return fmt.Errorf("attachment %d inbound: %w", attachment.ID, err)
			}
			if inbound.Stale {
				report.Skipped++
				return nil
			}
			panel, err := r.panels.FindByID(ctx, attachment.PanelID)
			if err != nil {
				return fmt.Errorf("attachment %d panel: %w", attachment.ID, err)
			}
			traffic, err := r.syncAttachmentTraffic(ctx, panel, inbound, &attachment)
			if err != nil {
				return fmt.Errorf("attachment %d traffic: %w", attachment.ID, err)
			}
			if err := r.updateClientAttachmentTraffic(ctx, attachment.ID, traffic.Upload, traffic.Download); err != nil {
				return fmt.Errorf("attachment %d update: %w", attachment.ID, err)
			}
			if err := r.recordTrafficSnapshot(ctx, attachment.ClientID, attachment.ID, traffic.Upload, traffic.Download); err != nil {
				return fmt.Errorf("attachment %d snapshot: %w", attachment.ID, err)
			}
			if client, err := r.loadClientByID(ctx, attachment.ClientID); err == nil && client != nil {
				r.invalidateSubscriptionCache(ctx, client.SubscriptionToken)
			}
			report.Updated++
			return nil
		}); err != nil {
			if strings.Contains(err.Error(), "already running") {
				report.Skipped++
				continue
			}
			report.Failures = append(report.Failures, err.Error())
			continue
		}
	}
	if len(report.Failures) > 0 {
		return report, errors.New(trafficSyncReportMessage(report))
	}
	return report, nil
}

func (r *Runner) syncAllTraffic(ctx context.Context) (*trafficSyncReport, error) {
	jobID, err := r.startSyncJob(ctx, nil, models.JobTypeTrafficSync)
	if err != nil {
		return nil, err
	}
	report, syncErr := r.syncTrafficForAttachments(ctx, 0, 0)
	status := "success"
	message := trafficSyncReportMessage(report)
	if syncErr != nil {
		status = "failed"
		message = syncErr.Error()
	}
	if len(report.Failures) > 0 {
		status = "failed"
		if message == "" {
			message = trafficSyncReportMessage(report)
		}
		if syncErr == nil {
			syncErr = errors.New(message)
		}
	}
	finishedAt := time.Now().UTC()
	if updErr := r.updateSyncJob(ctx, jobID, status, message, &finishedAt); updErr != nil && syncErr == nil {
		syncErr = updErr
	}
	return report, syncErr
}

func (r *Runner) loadTrafficSyncAttachments(ctx context.Context, panelID, clientID int64) ([]models.ClientAttachment, error) {
	query := `SELECT id, client_id, panel_id, inbound_id, remote_client_id, remote_email, enabled, upload_bytes, download_bytes, traffic_limit_bytes, expiry_time, raw_config, created_at, updated_at FROM client_attachments WHERE enabled = 1`
	args := []any{}
	if panelID > 0 {
		query += ` AND panel_id = ?`
		args = append(args, panelID)
	}
	if clientID > 0 {
		query += ` AND client_id = ?`
		args = append(args, clientID)
	}
	query += ` ORDER BY id ASC`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	attachments := make([]models.ClientAttachment, 0)
	for rows.Next() {
		attachment, err := scanClientAttachment(rows)
		if err != nil {
			return nil, err
		}
		attachments = append(attachments, *attachment)
	}
	return attachments, rows.Err()
}

func (r *Runner) startSyncJob(ctx context.Context, panelID *int64, jobType string) (int64, error) {
	if !models.IsValidJobType(jobType) {
		return 0, fmt.Errorf("invalid job type %q", jobType)
	}
	return r.jobs.Start(ctx, panelID, jobType, syncJobRetryCountFromContext(ctx))
}

func (r *Runner) recordTrafficSnapshot(ctx context.Context, clientID, attachmentID, uploadBytes, downloadBytes int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO traffic_snapshots (client_id, attachment_id, upload_bytes, download_bytes, total_bytes, captured_at) VALUES (?, ?, ?, ?, ?, ?)`, clientID, attachmentID, uploadBytes, downloadBytes, uploadBytes+downloadBytes, time.Now().UTC())
	return err
}

func trafficSyncReportMessage(report *trafficSyncReport) string {
	if report == nil {
		return ""
	}
	parts := make([]string, 0, 2)
	if report.Updated > 0 {
		parts = append(parts, fmt.Sprintf("updated=%d", report.Updated))
	}
	if report.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("skipped=%d", report.Skipped))
	}
	if len(report.Failures) > 0 {
		parts = append(parts, strings.Join(report.Failures, "; "))
	}
	return strings.Join(parts, " | ")
}
