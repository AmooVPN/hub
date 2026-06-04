package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/AmooVPN/hub/internal/models"
)

func (r *Runner) startAutomaticBackups() {
	if r == nil || r.backups == nil || r.cfg == nil || !r.cfg.AutomaticBackupEnabled {
		return
	}
	interval, ok := automaticBackupInterval(r.cfg.AutomaticBackupSchedule)
	if !ok {
		if r.logger != nil {
			r.logger.Warn("automatic backup disabled due to invalid schedule", "schedule", r.cfg.AutomaticBackupSchedule)
		}
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		ctx := context.Background()
		for range ticker.C {
			if r.backgroundJobs != nil {
				_, err := r.backgroundJobs.Submit(ctx, models.JobTypeBackupExport, nil, 0, func(jobCtx context.Context) error {
					return r.runAutomaticBackup(jobCtx)
				})
				if err != nil && r.logger != nil {
					r.logger.Warn("automatic backup enqueue failed", "error", err)
				}
				continue
			}
			if err := r.runAutomaticBackup(ctx); err != nil && r.logger != nil {
				r.logger.Warn("automatic backup failed", "error", err)
			}
		}
	}()
}

func automaticBackupInterval(schedule string) (time.Duration, bool) {
	switch strings.ToLower(strings.TrimSpace(schedule)) {
	case "daily":
		return 24 * time.Hour, true
	case "weekly":
		return 7 * 24 * time.Hour, true
	case "monthly":
		return 30 * 24 * time.Hour, true
	default:
		return 0, false
	}
}

func (r *Runner) runAutomaticBackup(ctx context.Context) error {
	if r == nil || r.backups == nil || r.db == nil {
		return fmt.Errorf("backup runner is not configured")
	}
	if _, err := r.db.ExecContext(ctx, `PRAGMA wal_checkpoint(FULL);`); err != nil {
		return err
	}
	rec, err := r.backups.Create(ctx, r.cfg.DatabasePath, r.cfg.BackupDir, r.cfg.AppName)
	if err != nil {
		return err
	}
	if deleted, cleanupErr := r.backups.CleanupOldBackups(r.cfg.BackupDir, r.cfg.BackupRetentionCount, r.cfg.BackupRetentionDays, rec.Name); cleanupErr != nil {
		if r.logger != nil {
			r.logger.Warn("automatic backup retention cleanup failed", "error", cleanupErr)
		}
	} else if len(deleted) > 0 && r.logger != nil {
		r.logger.Info("automatic backup retention cleanup completed", "deleted", len(deleted))
	}
	if r.audit != nil {
		_ = r.logAudit(ctx, "system", nil, "automatic_backup", "backup", nil, map[string]any{"filename": rec.Name})
	}
	return nil
}
