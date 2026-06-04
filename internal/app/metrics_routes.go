package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/models"
)

func (r *Runner) metricsMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		if c.Path() != "/metrics" && r.metrics != nil {
			r.metrics.ObserveRequest(time.Since(start))
		}
		return err
	}
}

func (r *Runner) getMetrics(c *fiber.Ctx) error {
	if !r.cfg.MetricsEnabled {
		return fiber.ErrNotFound
	}
	if token := strings.TrimSpace(r.cfg.MetricsToken); token != "" && c.Get("X-Metrics-Token") != token {
		return fiber.NewError(fiber.StatusUnauthorized, "metrics access denied")
	}
	ctx := c.UserContext()
	panelsTotal, panelsOnline, panelsOffline, clientsTotal, clientsActive, clientsDisabled, syncJobsTotal, syncJobsFailed, backupCount, err := r.collectMetricsCounts(ctx)
	if err != nil {
		return err
	}
	collector := r.metrics.Snapshot()
	var out strings.Builder
	out.WriteString("# HELP hub_http_requests_total Total HTTP requests.\n")
	out.WriteString("# TYPE hub_http_requests_total counter\n")
	out.WriteString(fmt.Sprintf("hub_http_requests_total %d\n", collector.RequestTotal))
	out.WriteString("# HELP hub_http_request_duration_seconds Total HTTP request duration.\n")
	out.WriteString("# TYPE hub_http_request_duration_seconds counter\n")
	out.WriteString(fmt.Sprintf("hub_http_request_duration_seconds_sum %.6f\n", collector.RequestDuration.Seconds()))
	out.WriteString(fmt.Sprintf("hub_http_request_duration_seconds_count %d\n", collector.RequestTotal))
	out.WriteString("# HELP hub_panels_total Total panels.\n# TYPE hub_panels_total gauge\n")
	out.WriteString(fmt.Sprintf("hub_panels_total %d\n", panelsTotal))
	out.WriteString(fmt.Sprintf("hub_panels_online %d\n", panelsOnline))
	out.WriteString(fmt.Sprintf("hub_panels_offline %d\n", panelsOffline))
	out.WriteString(fmt.Sprintf("hub_clients_total %d\n", clientsTotal))
	out.WriteString(fmt.Sprintf("hub_clients_active %d\n", clientsActive))
	out.WriteString(fmt.Sprintf("hub_clients_disabled %d\n", clientsDisabled))
	out.WriteString(fmt.Sprintf("hub_sync_jobs_total %d\n", syncJobsTotal))
	out.WriteString(fmt.Sprintf("hub_sync_jobs_failed_total %d\n", syncJobsFailed))
	out.WriteString(fmt.Sprintf("hub_backup_exports_total %d\n", collector.BackupExportsTotal))
	out.WriteString(fmt.Sprintf("hub_backup_imports_total %d\n", collector.BackupImportsTotal))
	out.WriteString(fmt.Sprintf("hub_subscriptions_requests_total %d\n", collector.SubscriptionTotal))
	out.WriteString(fmt.Sprintf("hub_backup_files_total %d\n", backupCount))
	c.Set("Content-Type", "text/plain; charset=utf-8")
	return c.SendString(out.String())
}

func (r *Runner) collectMetricsCounts(ctx context.Context) (panelsTotal, panelsOnline, panelsOffline, clientsTotal, clientsActive, clientsDisabled, syncJobsTotal, syncJobsFailed, backupCount int64, err error) {
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM panels`).Scan(&panelsTotal); err != nil {
		return
	}
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM panels WHERE status = 'online'`).Scan(&panelsOnline); err != nil {
		return
	}
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM panels WHERE status != 'online'`).Scan(&panelsOffline); err != nil {
		return
	}
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients`).Scan(&clientsTotal); err != nil {
		return
	}
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients WHERE status = 'active'`).Scan(&clientsActive); err != nil {
		return
	}
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients WHERE status = 'disabled'`).Scan(&clientsDisabled); err != nil {
		return
	}
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_jobs`).Scan(&syncJobsTotal); err != nil {
		return
	}
	if err = r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sync_jobs WHERE status = ?`, models.SyncJobStatusFailed).Scan(&syncJobsFailed); err != nil {
		return
	}
	if r.backups != nil {
		items, listErr := r.backups.List(r.cfg.BackupDir)
		if listErr != nil {
			err = listErr
			return
		}
		backupCount = int64(len(items))
	}
	return
}
