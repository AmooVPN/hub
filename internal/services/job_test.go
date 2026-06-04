package services

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/repositories"
)

func TestJobServiceStartAndUpdate(t *testing.T) {
	db := openJobTestDatabase(t)
	panelRepo := repositories.NewPanelRepository(db)
	jobRepo := repositories.NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: "offline"}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}

	service := NewJobService(jobRepo)
	panelID := panel.ID
	jobID, err := service.Start(context.Background(), &panelID, "traffic_sync", 2)
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	if jobID == 0 {
		t.Fatal("expected job id")
	}

	finishedAt := time.Now().UTC()
	if err := service.Update(context.Background(), jobID, "success", "done", &finishedAt); err != nil {
		t.Fatalf("update job: %v", err)
	}

	job, err := jobRepo.FindByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "success" || job.Message != "done" || job.RetryCount != 2 {
		t.Fatalf("unexpected job state: %+v", job)
	}
	if job.StartedAt == nil || job.FinishedAt == nil {
		t.Fatalf("expected timestamps on job: %+v", job)
	}
}

func TestJobServiceJobCreation(t *testing.T) {
	db := openJobTestDatabase(t)
	panelRepo := repositories.NewPanelRepository(db)
	jobRepo := repositories.NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: "offline"}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}

	service := NewJobService(jobRepo)
	panelID := panel.ID
	jobID, err := service.Start(context.Background(), &panelID, "panel_sync", 0)
	if err != nil {
		t.Fatalf("start job: %v", err)
	}

	job, err := jobRepo.FindByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "running" || job.JobType != "panel_sync" || job.RetryCount != 0 {
		t.Fatalf("unexpected created job: %+v", job)
	}
}

func TestJobServiceJobStatusUpdate(t *testing.T) {
	db := openJobTestDatabase(t)
	panelRepo := repositories.NewPanelRepository(db)
	jobRepo := repositories.NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: "offline"}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}

	service := NewJobService(jobRepo)
	panelID := panel.ID
	jobID, err := service.Start(context.Background(), &panelID, "traffic_sync", 0)
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	finishedAt := time.Now().UTC()
	if err := service.Update(context.Background(), jobID, "success", "done", &finishedAt); err != nil {
		t.Fatalf("update job: %v", err)
	}

	job, err := jobRepo.FindByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "success" || job.Message != "done" {
		t.Fatalf("unexpected updated job: %+v", job)
	}
}

func TestJobServiceJobFailureState(t *testing.T) {
	db := openJobTestDatabase(t)
	panelRepo := repositories.NewPanelRepository(db)
	jobRepo := repositories.NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: "offline"}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}

	service := NewJobService(jobRepo)
	panelID := panel.ID
	jobID, err := service.Start(context.Background(), &panelID, "inbound_sync", 0)
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	finishedAt := time.Now().UTC()
	if err := service.Update(context.Background(), jobID, "failed", "remote panel unreachable", &finishedAt); err != nil {
		t.Fatalf("update failed job: %v", err)
	}

	job, err := jobRepo.FindByID(context.Background(), jobID)
	if err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.Status != "failed" || job.Message != "remote panel unreachable" {
		t.Fatalf("unexpected failed job: %+v", job)
	}
}

func TestJobServiceCleanupDeletesOldCompletedJobs(t *testing.T) {
	db := openJobTestDatabase(t)
	panelRepo := repositories.NewPanelRepository(db)
	jobRepo := repositories.NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: "offline"}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}

	service := NewJobService(jobRepo)
	panelID := panel.ID
	oldJobID, err := service.Start(context.Background(), &panelID, "traffic_sync", 0)
	if err != nil {
		t.Fatalf("start job: %v", err)
	}
	oldFinished := time.Now().UTC().Add(-45 * 24 * time.Hour)
	if err := service.Update(context.Background(), oldJobID, models.SyncJobStatusSuccess, "done", &oldFinished); err != nil {
		t.Fatalf("finish old job: %v", err)
	}

	newJobID, err := service.Start(context.Background(), &panelID, "traffic_sync", 0)
	if err != nil {
		t.Fatalf("start new job: %v", err)
	}
	newFinished := time.Now().UTC()
	if err := service.Update(context.Background(), newJobID, models.SyncJobStatusSuccess, "done", &newFinished); err != nil {
		t.Fatalf("finish new job: %v", err)
	}

	deleted, err := service.Cleanup(context.Background(), 30*24*time.Hour)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted == 0 {
		t.Fatal("expected old job to be deleted")
	}
	if _, err := jobRepo.FindByID(context.Background(), oldJobID); err == nil {
		t.Fatal("expected old job to be removed")
	}
	if _, err := jobRepo.FindByID(context.Background(), newJobID); err != nil {
		t.Fatalf("expected new job to remain: %v", err)
	}
}

func TestJobServiceRejectsMissingConfiguration(t *testing.T) {
	service := NewJobService(nil)
	if _, err := service.Start(context.Background(), nil, "traffic_sync", 0); err == nil {
		t.Fatal("expected start to fail without repo")
	}
	if err := service.Update(context.Background(), 1, "success", "", nil); err == nil {
		t.Fatal("expected update to fail without repo")
	}
}

func openJobTestDatabase(t *testing.T) *sql.DB {
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
