package services

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmooVPM/hub/internal/database"
	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/repositories"
)

func TestBackgroundJobRunnerProcessesQueuedJobs(t *testing.T) {
	db := openBackgroundJobTestDatabase(t)
	panelRepo := repositories.NewPanelRepository(db)
	jobRepo := repositories.NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: models.PanelStatusOffline}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}
	runner := NewBackgroundJobRunner(jobRepo, 1)
	runner.Start(context.Background())
	t.Cleanup(func() { _ = runner.Close() })
	jobID, err := runner.Submit(context.Background(), models.JobTypeBackupExport, nil, 3, func(context.Context) error { return nil })
	if err != nil {
		t.Fatalf("submit job: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		job, err := jobRepo.FindByID(context.Background(), jobID)
		if err != nil {
			t.Fatalf("load job: %v", err)
		}
		if job.Status == models.SyncJobStatusSuccess {
			if job.StartedAt == nil || job.FinishedAt == nil {
				t.Fatalf("expected job timestamps: %+v", job)
			}
			if job.RetryCount != 3 {
				t.Fatalf("expected retry count to persist: %+v", job)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not complete")
}

func openBackgroundJobTestDatabase(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := database.RunMigrations(db); err != nil {
		_ = db.Close()
		t.Fatalf("run migrations: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}
