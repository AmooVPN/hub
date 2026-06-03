package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupServiceCreateListDelete(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hub.db")
	backupDir := filepath.Join(dir, "backups")
	if err := os.WriteFile(dbPath, []byte("sqlite-bytes"), 0o600); err != nil {
		t.Fatalf("write db: %v", err)
	}
	service := NewBackupService()
	rec, err := service.Create(context.Background(), dbPath, backupDir, "hub")
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	path, err := service.Path(backupDir, rec.Name)
	if err != nil {
		t.Fatalf("path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("backup file missing: %v", err)
	}
	list, err := service.List(backupDir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != rec.Name {
		t.Fatalf("unexpected list: %+v", list)
	}
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer r.Close()
	if len(r.File) != 2 {
		t.Fatalf("expected 2 zip entries, got %d", len(r.File))
	}
	if err := service.Delete(backupDir, rec.Name); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected deleted backup, got %v", err)
	}
}

func TestBackupServiceValidateImportArchive(t *testing.T) {
	service := NewBackupService()
	data := buildBackupArchive(t, BackupMetadata{App: "hub", Version: backupFormatVersion, Database: "sqlite", SchemaVersion: backupSchemaVersion, CreatedAt: time.Now().UTC()}, []byte("SQLite format 3\x00rest-of-db"))
	meta, err := service.ValidateImportArchive(data)
	if err != nil {
		t.Fatalf("validate archive: %v", err)
	}
	if meta.App != "hub" || meta.Database != "sqlite" || meta.SchemaVersion != backupSchemaVersion {
		t.Fatalf("unexpected metadata: %+v", meta)
	}
}

func TestBackupServiceValidateImportArchiveRejectsInvalidApp(t *testing.T) {
	service := NewBackupService()
	data := buildBackupArchive(t, BackupMetadata{App: "other", Version: backupFormatVersion, Database: "sqlite", SchemaVersion: backupSchemaVersion, CreatedAt: time.Now().UTC()}, []byte("SQLite format 3\x00rest-of-db"))
	if _, err := service.ValidateImportArchive(data); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestBackupServicePrepareImportCreatesSafetyBackup(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hub.db")
	backupDir := filepath.Join(dir, "backups")
	if err := os.WriteFile(dbPath, []byte("sqlite-bytes"), 0o600); err != nil {
		t.Fatalf("write db: %v", err)
	}
	service := NewBackupService()
	data := buildBackupArchive(t, BackupMetadata{App: "hub", Version: backupFormatVersion, Database: "sqlite", SchemaVersion: backupSchemaVersion, CreatedAt: time.Now().UTC()}, []byte("SQLite format 3\x00rest-of-db"))
	result, err := service.PrepareImport(context.Background(), data, dbPath, backupDir, "hub")
	if err != nil {
		t.Fatalf("prepare import: %v", err)
	}
	if result.SafetyBackup.Name == "" || result.StagedName == "" {
		t.Fatalf("expected backup names in result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(backupDir, result.StagedName)); err != nil {
		t.Fatalf("staged archive missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(backupDir, result.SafetyBackup.Name)); err != nil {
		t.Fatalf("safety backup missing: %v", err)
	}
}

func buildBackupArchive(t *testing.T, meta BackupMetadata, db []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := writeZipFile(zw, "metadata.json", metaBytes); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
	if err := writeZipFile(zw, "hub.db", db); err != nil {
		t.Fatalf("write db: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close archive: %v", err)
	}
	return buf.Bytes()
}
