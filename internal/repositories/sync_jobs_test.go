package repositories

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/AmooVPM/hub/internal/database"
	"github.com/AmooVPM/hub/internal/models"
)

func TestSyncJobRepositoryCreateUpdateFindByID(t *testing.T) {
	db := openTestDatabase(t)
	panelRepo := NewPanelRepository(db)
	repo := NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: "offline"}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}

	startedAt := time.Now().UTC().Add(-time.Minute)
	finishedAt := time.Now().UTC()
	panelID := panel.ID
	job := &models.SyncJob{
		PanelID:    &panelID,
		JobType:    "panel_sync",
		Status:     "running",
		Message:    "starting",
		StartedAt:  &startedAt,
		FinishedAt: nil,
	}
	if err := repo.Create(context.Background(), job); err != nil {
		t.Fatalf("create job: %v", err)
	}
	if job.ID == 0 {
		t.Fatal("expected job id to be set")
	}

	job.Status = "success"
	job.Message = "done"
	job.FinishedAt = &finishedAt
	if err := repo.Update(context.Background(), job); err != nil {
		t.Fatalf("update job: %v", err)
	}

	loaded, err := repo.FindByID(context.Background(), job.ID)
	if err != nil {
		t.Fatalf("find job: %v", err)
	}
	if loaded.PanelID == nil || *loaded.PanelID != panelID {
		t.Fatalf("unexpected panel id: %+v", loaded.PanelID)
	}
	if loaded.JobType != "panel_sync" || loaded.Status != "success" || loaded.Message != "done" {
		t.Fatalf("unexpected loaded job: %+v", loaded)
	}
	if loaded.StartedAt == nil || loaded.FinishedAt == nil {
		t.Fatalf("expected timestamps to be present: %+v", loaded)
	}
}

func TestSyncJobRepositoryDeleteCompletedBefore(t *testing.T) {
	db := openTestDatabase(t)
	panelRepo := NewPanelRepository(db)
	repo := NewSyncJobRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: "offline"}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}
	panelID := panel.ID
	oldJob := &models.SyncJob{PanelID: &panelID, JobType: "traffic_sync", Status: models.SyncJobStatusSuccess, FinishedAt: ptrTime(time.Now().UTC().Add(-40 * 24 * time.Hour)), CreatedAt: time.Now().UTC().Add(-40 * 24 * time.Hour)}
	newJob := &models.SyncJob{PanelID: &panelID, JobType: "traffic_sync", Status: models.SyncJobStatusRunning, CreatedAt: time.Now().UTC()}
	if err := repo.Create(context.Background(), oldJob); err != nil {
		t.Fatalf("create old job: %v", err)
	}
	if err := repo.Create(context.Background(), newJob); err != nil {
		t.Fatalf("create new job: %v", err)
	}
	deleted, err := repo.DeleteCompletedBefore(context.Background(), time.Now().UTC().Add(-30*24*time.Hour))
	if err != nil {
		t.Fatalf("delete completed before: %v", err)
	}
	if deleted == 0 {
		t.Fatal("expected completed job to be deleted")
	}
	if _, err := repo.FindByID(context.Background(), oldJob.ID); err == nil {
		t.Fatal("expected old job to be deleted")
	}
	if _, err := repo.FindByID(context.Background(), newJob.ID); err != nil {
		t.Fatalf("expected new job to remain: %v", err)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func openTestDatabase(t *testing.T) *sql.DB {
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
