package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
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

func TestBackupServicePrepareImportRollsBackSafetyBackupOnStageFailure(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "hub.db")
	backupDir := filepath.Join(dir, "backups")
	if err := os.WriteFile(dbPath, []byte("sqlite-bytes"), 0o600); err != nil {
		t.Fatalf("write db: %v", err)
	}
	service := NewBackupService()
	data := buildBackupArchive(t, BackupMetadata{App: "hub", Version: backupFormatVersion, Database: "sqlite", SchemaVersion: backupSchemaVersion, CreatedAt: time.Now().UTC()}, []byte("SQLite format 3\x00rest-of-db"))
	originalWrite := writeStagedImportFile
	writeStagedImportFile = func(string, []byte, os.FileMode) error { return os.ErrPermission }
	t.Cleanup(func() { writeStagedImportFile = originalWrite })
	if _, err := service.PrepareImport(context.Background(), data, dbPath, backupDir, "hub"); err == nil {
		t.Fatal("expected stage failure")
	}
	backs, err := service.List(backupDir)
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backs) != 0 {
		t.Fatalf("expected rolled back safety backup, got %+v", backs)
	}
}

func TestBackupServiceCleanupOldBackupsByCount(t *testing.T) {
	dir := t.TempDir()
	service := NewBackupService()
	names := []string{
		"hub-backup-2026-06-01-00-00-00.zip",
		"hub-backup-2026-06-02-00-00-00.zip",
		"hub-backup-2026-06-03-00-00-00.zip",
		"hub-backup-2026-06-04-00-00-00.zip",
	}
	for i, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("write backup %s: %v", name, err)
		}
		modTime := time.Date(2026, 6, 1+i, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}
	deleted, err := service.CleanupOldBackups(dir, 2, 0)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	sort.Strings(deleted)
	if len(deleted) != 2 {
		t.Fatalf("expected 2 deleted files, got %v", deleted)
	}
	backs, err := service.List(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(backs) != 2 || backs[0].Name != names[3] || backs[1].Name != names[2] {
		t.Fatalf("unexpected remaining backups: %+v", backs)
	}
}

func TestBackupServiceCleanupOldBackupsKeepsProtectedNames(t *testing.T) {
	dir := t.TempDir()
	service := NewBackupService()
	protected := "hub-backup-2026-06-01-00-00-00.zip"
	for i, name := range []string{protected, "hub-backup-2026-06-02-00-00-00.zip"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(name), 0o600); err != nil {
			t.Fatalf("write backup %s: %v", name, err)
		}
		modTime := time.Date(2026, 6, 1+i, 0, 0, 0, 0, time.UTC)
		if err := os.Chtimes(path, modTime, modTime); err != nil {
			t.Fatalf("chtimes %s: %v", name, err)
		}
	}
	deleted, err := service.CleanupOldBackups(dir, 1, 0, protected)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if len(deleted) != 0 {
		t.Fatalf("expected no deletions when protected, got %v", deleted)
	}
	backs, err := service.List(dir)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(backs) != 2 {
		t.Fatalf("expected both backups to remain, got %+v", backs)
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
