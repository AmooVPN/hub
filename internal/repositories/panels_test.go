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

func TestPanelRepositoryPersistsLastCheckedAt(t *testing.T) {
	db := openPanelTestDatabase(t)
	repo := NewPanelRepository(db)
	checkedAt := time.Now().UTC()
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: models.PanelStatusOffline, LastCheckedAt: &checkedAt}
	if err := repo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}
	loaded, err := repo.FindByID(context.Background(), panel.ID)
	if err != nil {
		t.Fatalf("find panel: %v", err)
	}
	if loaded.LastCheckedAt == nil || !loaded.LastCheckedAt.Equal(checkedAt) {
		t.Fatalf("expected last checked at to persist, got %+v", loaded.LastCheckedAt)
	}
}

func openPanelTestDatabase(t *testing.T) *sql.DB {
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
