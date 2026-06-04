package services

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/repositories"
)

func TestCleanupServiceRun(t *testing.T) {
	db := openCleanupDB(t)
	defer db.Close()
	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	repo := repositories.NewClientRepository(db)
	client := &models.Client{Username: "cleanup-client", PasswordHash: "hash", Status: "active", SubscriptionToken: "token"}
	if err := repo.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO client_refresh_tokens (client_id, token_hash, expires_at, created_at) VALUES (?, ?, ?, ?)`, client.ID, "expired", time.Now().UTC().Add(-time.Hour), time.Now().UTC().Add(-2*time.Hour)); err != nil {
		t.Fatalf("insert refresh token: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO traffic_snapshots (client_id, upload_bytes, download_bytes, total_bytes, captured_at) VALUES (?, 1, 1, 2, ?)`, client.ID, time.Now().UTC().Add(-31*24*time.Hour)); err != nil {
		t.Fatalf("insert traffic snapshot: %v", err)
	}
	webhookRepo := repositories.NewWebhookRepository(db)
	webhook := &models.Webhook{Name: "wh", URL: "https://example.com", Secret: "secret", Active: true, Events: []string{"client.created"}}
	if err := webhookRepo.Create(context.Background(), webhook); err != nil {
		t.Fatalf("create webhook: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO webhook_deliveries (webhook_id, event_type, payload_json, status, attempts, created_at) VALUES (?, ?, ?, ?, ?, ?)`, webhook.ID, "client.created", `{"a":1}`, "success", 1, time.Now().UTC().Add(-31*24*time.Hour)); err != nil {
		t.Fatalf("insert webhook delivery: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO sync_jobs (job_type, status, created_at, started_at, finished_at) VALUES (?, ?, ?, ?, ?)`, "panel.sync", models.SyncJobStatusSuccess, time.Now().UTC().Add(-31*24*time.Hour), time.Now().UTC().Add(-31*24*time.Hour), time.Now().UTC().Add(-31*24*time.Hour)); err != nil {
		t.Fatalf("insert sync job: %v", err)
	}
	backupDir := t.TempDir()
	oldBackup := filepath.Join(backupDir, "hub-backup-old.zip")
	if err := os.WriteFile(oldBackup, []byte("zip"), 0o600); err != nil {
		t.Fatalf("write backup: %v", err)
	}
	oldTime := time.Now().UTC().Add(-31 * 24 * time.Hour)
	if err := os.Chtimes(oldBackup, oldTime, oldTime); err != nil {
		t.Fatalf("chtimes backup: %v", err)
	}
	service := NewCleanupService(db, repositories.NewSyncJobRepository(db), NewBackupService())
	report, err := service.Run(context.Background(), backupDir, 0, 30)
	if err != nil {
		t.Fatalf("run cleanup: %v", err)
	}
	if report.ExpiredRefreshTokens != 1 || report.OldSyncJobs != 1 || report.OldTrafficSnapshots != 1 || report.OldWebhookDeliveries != 1 || report.OldBackups != 1 {
		t.Fatalf("unexpected cleanup report: %+v", report)
	}
}

func openCleanupDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:cleanup-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}
