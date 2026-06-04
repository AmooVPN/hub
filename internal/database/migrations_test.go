package database

import (
	"database/sql"
	"strings"
	"testing"
)

func TestRunMigrationsAndValidateSchemaVersion(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if err := ValidateSchemaVersion(db); err != nil {
		t.Fatalf("validate schema version: %v", err)
	}
}

func TestValidateSchemaVersionRejectsUnknownVersion(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (999, datetime('now'))`); err != nil {
		t.Fatalf("insert future version: %v", err)
	}
	if err := ValidateSchemaVersion(db); err == nil || !strings.Contains(err.Error(), "newer than supported") {
		t.Fatalf("expected newer-than-supported error, got %v", err)
	}
}

func TestRunMigrationsRejectsDuplicateRegistry(t *testing.T) {
	original := migrations
	t.Cleanup(func() { migrations = original })
	migrations = append(append([]Migration(nil), original...), Migration{Version: original[len(original)-1].Version, Name: "duplicate", SQL: []string{"SELECT 1;"}})

	db := openTestDB(t)
	defer db.Close()

	if err := RunMigrations(db); err == nil || !strings.Contains(err.Error(), "duplicate migration version") {
		t.Fatalf("expected duplicate migration error, got %v", err)
	}
}

func TestValidateSchemaVersionRejectsMissingVersion(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	if err := RunMigrations(db); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE version = 7`); err != nil {
		t.Fatalf("delete migration row: %v", err)
	}
	if err := ValidateSchemaVersion(db); err == nil || !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected missing version error, got %v", err)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:test-migrations-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}
