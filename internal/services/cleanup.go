package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AmooVPM/hub/internal/repositories"
)

type CleanupReport struct {
	ExpiredRefreshTokens int64
	OldSyncJobs          int64
	OldTrafficSnapshots  int64
	OldWebhookDeliveries int64
	OldBackups           int64
}

type CleanupService struct {
	db       *sql.DB
	syncJobs repositories.SyncJobRepository
	backups  *BackupService
}

func NewCleanupService(db *sql.DB, syncJobs repositories.SyncJobRepository, backups *BackupService) *CleanupService {
	return &CleanupService{db: db, syncJobs: syncJobs, backups: backups}
}

func (s *CleanupService) Run(ctx context.Context, backupDir string, backupRetentionCount, backupRetentionDays int) (CleanupReport, error) {
	if s == nil || s.db == nil {
		return CleanupReport{}, errors.New("cleanup service is not configured")
	}
	if err := ctx.Err(); err != nil {
		return CleanupReport{}, err
	}
	var report CleanupReport
	now := time.Now().UTC()
	result, err := s.db.ExecContext(ctx, `DELETE FROM client_refresh_tokens WHERE expires_at < ?`, now)
	if err != nil {
		return report, err
	}
	if rows, err := result.RowsAffected(); err == nil {
		report.ExpiredRefreshTokens = rows
	}
	result, err = s.db.ExecContext(ctx, `DELETE FROM traffic_snapshots WHERE captured_at < ?`, now.Add(-30*24*time.Hour))
	if err != nil {
		return report, err
	}
	if rows, err := result.RowsAffected(); err == nil {
		report.OldTrafficSnapshots = rows
	}
	result, err = s.db.ExecContext(ctx, `DELETE FROM webhook_deliveries WHERE created_at < ?`, now.Add(-30*24*time.Hour))
	if err != nil {
		return report, err
	}
	if rows, err := result.RowsAffected(); err == nil {
		report.OldWebhookDeliveries = rows
	}
	if s.syncJobs != nil {
		deleted, err := s.syncJobs.DeleteCompletedBefore(ctx, now.Add(-30*24*time.Hour))
		if err != nil {
			return report, err
		}
		report.OldSyncJobs = deleted
	}
	if s.backups != nil {
		deleted, err := s.backups.CleanupOldBackups(backupDir, backupRetentionCount, backupRetentionDays)
		if err != nil {
			return report, err
		}
		report.OldBackups = int64(len(deleted))
	}
	return report, nil
}

func (r CleanupReport) Total() int64 {
	return r.ExpiredRefreshTokens + r.OldSyncJobs + r.OldTrafficSnapshots + r.OldWebhookDeliveries + r.OldBackups
}

func (r CleanupReport) Summary() string {
	return fmt.Sprintf("refresh tokens=%d, sync jobs=%d, traffic snapshots=%d, webhook deliveries=%d, backups=%d", r.ExpiredRefreshTokens, r.OldSyncJobs, r.OldTrafficSnapshots, r.OldWebhookDeliveries, r.OldBackups)
}
