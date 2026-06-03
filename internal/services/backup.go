package services

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	backupFormatVersion = "0.1.0"
	backupSchemaVersion = 1
)

type BackupMetadata struct {
	App           string    `json:"app"`
	Version       string    `json:"version"`
	Database      string    `json:"database"`
	SchemaVersion int       `json:"schema_version"`
	CreatedAt     time.Time `json:"created_at"`
}

type BackupRecord struct {
	Name        string
	Size        int64
	ModifiedAt  time.Time
	DownloadURL string
}

type BackupService struct{}

type ImportResult struct {
	Metadata     BackupMetadata
	SafetyBackup BackupRecord
	StagedName   string
	StagedPath   string
}

var writeStagedImportFile = os.WriteFile

func NewBackupService() *BackupService { return &BackupService{} }

func (s *BackupService) Create(ctx context.Context, dbPath, backupDir, appName string) (BackupRecord, error) {
	if s == nil {
		return BackupRecord{}, errors.New("backup service is not configured")
	}
	if err := ctx.Err(); err != nil {
		return BackupRecord{}, err
	}
	if err := checkpointSQLite(ctx, dbPath); err != nil {
		return BackupRecord{}, err
	}
	name := fmt.Sprintf("hub-backup-%s.zip", time.Now().UTC().Format("2006-01-02-15-04-05"))
	path := filepath.Join(backupDir, name)
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return BackupRecord{}, err
	}
	file, err := os.Create(path)
	if err != nil {
		return BackupRecord{}, err
	}
	defer file.Close()
	zw := zip.NewWriter(file)
	meta := BackupMetadata{App: appName, Version: backupFormatVersion, Database: "sqlite", SchemaVersion: backupSchemaVersion, CreatedAt: time.Now().UTC()}
	if err := writeZipFile(zw, "metadata.json", mustJSON(meta)); err != nil {
		_ = zw.Close()
		_ = os.Remove(path)
		return BackupRecord{}, err
	}
	dbBytes, err := os.ReadFile(dbPath)
	if err != nil {
		_ = zw.Close()
		_ = os.Remove(path)
		return BackupRecord{}, err
	}
	if err := writeZipFile(zw, "hub.db", dbBytes); err != nil {
		_ = zw.Close()
		_ = os.Remove(path)
		return BackupRecord{}, err
	}
	if err := zw.Close(); err != nil {
		_ = os.Remove(path)
		return BackupRecord{}, err
	}
	stat, err := os.Stat(path)
	if err != nil {
		return BackupRecord{}, err
	}
	return BackupRecord{Name: name, Size: stat.Size(), ModifiedAt: stat.ModTime()}, nil
}

func (s *BackupService) List(backupDir string) ([]BackupRecord, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	items := make([]BackupRecord, 0)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || filepath.Ext(name) != ".zip" || !strings.HasPrefix(name, "hub-backup-") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return nil, err
		}
		items = append(items, BackupRecord{Name: name, Size: info.Size(), ModifiedAt: info.ModTime()})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ModifiedAt.After(items[j].ModifiedAt) })
	return items, nil
}

func (s *BackupService) Delete(backupDir, filename string) error {
	if s == nil {
		return errors.New("backup service is not configured")
	}
	if filepath.Base(filename) != filename || !strings.HasSuffix(filename, ".zip") {
		return errors.New("invalid backup filename")
	}
	return os.Remove(filepath.Join(backupDir, filename))
}

func (s *BackupService) Path(backupDir, filename string) (string, error) {
	if filepath.Base(filename) != filename || !strings.HasSuffix(filename, ".zip") {
		return "", errors.New("invalid backup filename")
	}
	return filepath.Join(backupDir, filename), nil
}

func (s *BackupService) ValidateImportArchive(data []byte) (BackupMetadata, error) {
	if len(data) == 0 {
		return BackupMetadata{}, errors.New("backup archive is empty")
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return BackupMetadata{}, err
	}
	files := map[string][]byte{}
	for _, file := range r.File {
		name := file.Name
		if name == "" || path.Clean(name) != name || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || strings.ContainsRune(name, '\\') {
			return BackupMetadata{}, errors.New("backup archive contains invalid file paths")
		}
		if file.FileInfo().IsDir() {
			continue
		}
		if name != "metadata.json" && name != "hub.db" {
			continue
		}
		f, err := file.Open()
		if err != nil {
			return BackupMetadata{}, err
		}
		payload, readErr := io.ReadAll(f)
		_ = f.Close()
		if readErr != nil {
			return BackupMetadata{}, readErr
		}
		files[name] = payload
	}
	metaBytes, ok := files["metadata.json"]
	if !ok {
		return BackupMetadata{}, errors.New("metadata.json is required")
	}
	dbBytes, ok := files["hub.db"]
	if !ok {
		return BackupMetadata{}, errors.New("hub.db is required")
	}
	if !bytes.HasPrefix(dbBytes, []byte("SQLite format 3\x00")) {
		return BackupMetadata{}, errors.New("hub.db is not a valid sqlite database")
	}
	var meta BackupMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return BackupMetadata{}, fmt.Errorf("invalid metadata.json: %w", err)
	}
	if strings.TrimSpace(meta.App) != "hub" {
		return BackupMetadata{}, errors.New("backup archive is not for hub")
	}
	if strings.TrimSpace(meta.Database) != "sqlite" {
		return BackupMetadata{}, errors.New("backup archive is not sqlite")
	}
	if meta.SchemaVersion > backupSchemaVersion {
		return BackupMetadata{}, fmt.Errorf("unsupported backup schema version %d", meta.SchemaVersion)
	}
	if meta.SchemaVersion <= 0 {
		return BackupMetadata{}, errors.New("backup schema version is required")
	}
	return meta, nil
}

func (s *BackupService) PrepareImport(ctx context.Context, data []byte, dbPath, backupDir, appName string) (ImportResult, error) {
	meta, err := s.ValidateImportArchive(data)
	if err != nil {
		return ImportResult{}, err
	}
	safety, err := s.Create(ctx, dbPath, backupDir, appName)
	if err != nil {
		return ImportResult{}, err
	}
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return ImportResult{}, err
	}
	name := fmt.Sprintf("hub-import-%s.zip", time.Now().UTC().Format("2006-01-02-15-04-05"))
	path := filepath.Join(backupDir, name)
	if err := writeStagedImportFile(path, data, 0o600); err != nil {
		_ = s.Delete(backupDir, safety.Name)
		return ImportResult{}, err
	}
	return ImportResult{Metadata: meta, SafetyBackup: safety, StagedName: name, StagedPath: path}, nil
}

func (s *BackupService) CleanupOldBackups(backupDir string, keepCount, keepDays int, protectedNames ...string) ([]string, error) {
	items, err := s.List(backupDir)
	if err != nil {
		return nil, err
	}
	if keepCount <= 0 && keepDays <= 0 {
		return nil, nil
	}
	protected := make(map[string]struct{}, len(protectedNames))
	for _, name := range protectedNames {
		if strings.TrimSpace(name) == "" {
			continue
		}
		protected[name] = struct{}{}
	}
	kept := make(map[string]struct{}, len(items))
	if keepDays > 0 {
		cutoff := time.Now().UTC().Add(-time.Duration(keepDays) * 24 * time.Hour)
		for _, item := range items {
			if item.ModifiedAt.After(cutoff) || item.ModifiedAt.Equal(cutoff) {
				kept[item.Name] = struct{}{}
			}
		}
	}
	if keepCount > 0 {
		count := 0
		for _, item := range items {
			if _, ok := protected[item.Name]; ok {
				kept[item.Name] = struct{}{}
				continue
			}
			kept[item.Name] = struct{}{}
			count++
			if count >= keepCount {
				break
			}
		}
	}
	deleted := make([]string, 0)
	for _, item := range items {
		if _, ok := protected[item.Name]; ok {
			continue
		}
		if _, ok := kept[item.Name]; ok {
			continue
		}
		if err := s.Delete(backupDir, item.Name); err != nil && !os.IsNotExist(err) {
			return deleted, err
		}
		deleted = append(deleted, item.Name)
	}
	return deleted, nil
}

func checkpointSQLite(ctx context.Context, dbPath string) error {
	_ = ctx
	if dbPath == "" {
		return errors.New("database path is required")
	}
	return nil
}

func writeZipFile(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = io.Copy(w, bytes.NewReader(data))
	return err
}

func mustJSON(v any) []byte {
	data, _ := json.MarshalIndent(v, "", "  ")
	return data
}
