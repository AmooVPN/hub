package app

import (
	"path/filepath"
	"testing"

	"github.com/AmooVPM/hub/internal/config"
	"github.com/AmooVPM/hub/internal/database"
)

func TestAutomaticBackupInterval(t *testing.T) {
	cases := map[string]int{
		"daily":   24,
		"weekly":  24 * 7,
		"monthly": 24 * 30,
	}
	for schedule, hours := range cases {
		interval, ok := automaticBackupInterval(schedule)
		if !ok {
			t.Fatalf("expected valid schedule %s", schedule)
		}
		if interval.Hours() != float64(hours) {
			t.Fatalf("unexpected interval for %s: %v", schedule, interval)
		}
	}
	if _, ok := automaticBackupInterval("nope"); ok {
		t.Fatal("expected invalid schedule")
	}
}

func TestCollectSystemHealthSQLiteOk(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hub.db")
	db, err := database.OpenSQLite(dbPath)
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	defer db.Close()
	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	r := &Runner{cfg: &config.Config{AppName: "hub"}, db: db}
	health := r.collectSystemHealth(t.Context())
	if health.SQLite != "ok" {
		t.Fatalf("expected sqlite ok, got %+v", health)
	}
	if health.Status != "degraded" {
		t.Fatalf("expected degraded because redis unavailable, got %+v", health)
	}
}
