package main

import (
	"archive/zip"
	"bytes"
	"context"
	"database/sql"
	"flag"
	"io"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AmooVPN/hub/internal/bootstrap"
	"github.com/AmooVPN/hub/internal/config"
	"github.com/AmooVPN/hub/internal/database"
	app "github.com/AmooVPN/hub/internal/app"
	"github.com/AmooVPN/hub/internal/repositories"
	"github.com/AmooVPN/hub/internal/services"
	"github.com/AmooVPN/hub/internal/version"
)

type runnable interface {
	Run() error
}

var (
	loadRunner           = func(logger *slog.Logger) (runnable, error) { return app.New(logger) }
	loadConfig           = config.Load
	openSQLite           = database.OpenSQLite
	openRedis            = database.OpenRedis
	runMigrations        = database.RunMigrations
	validateSchema       = database.ValidateSchemaVersion
	ensureInitialAdmin   = bootstrap.EnsureInitialAdmin
	newAdminRepository   = repositories.NewAdminRepository
	defaultLoggerFactory = func() *slog.Logger {
		return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
)

func execute(args []string, logger *slog.Logger) error {
	if logger == nil {
		logger = defaultLoggerFactory()
	}
	if len(args) == 0 {
		return runServe(logger)
	}
	cmd, rest := args[0], args[1:]
	switch cmd {
	case "serve":
		return runServe(logger)
	case "migrate":
		return runMigrate(logger)
	case "setup":
		return runSetup(rest, logger)
	case "create-admin":
		return runSetup(rest, logger)
	case "healthcheck":
		return runHealthcheck(logger)
	case "version":
		return runVersion(os.Stdout)
	case "backup":
		return runBackup(rest, logger)
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func runServe(logger *slog.Logger) error {
	runner, err := loadRunner(logger)
	if err != nil {
		return err
	}
	return runner.Run()
}

func runMigrate(logger *slog.Logger) error {
	_, db, _, cleanup, err := openConfiguredDatabase()
	if err != nil {
		return err
	}
	defer cleanup()
	if err := runMigrations(db); err != nil {
		return err
	}
	return validateSchema(db)
}

func runCreateAdmin(logger *slog.Logger) error {
	return runSetup(nil, logger)
}

func runSetup(args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	username := fs.String("username", "admin", "initial admin username")
	password := fs.String("password", "", "initial admin password")
	email := fs.String("email", "", "initial admin email")
	role := fs.String("role", "owner", "initial admin role")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*password) == "" {
		return errors.New("--password is required")
	}
	cfg, db, cleanup, err := openConfiguredSQLite()
	if err != nil {
		return err
	}
	defer cleanup()
	if err := runMigrations(db); err != nil {
		return err
	}
	if err := validateSchema(db); err != nil {
		return err
	}
	admins := newAdminRepository(db)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	admin, err := bootstrap.CreateInitialAdmin(ctx, admins, *username, *password, *email, *role)
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Info("created initial admin user", "username", admin.Username, "role", admin.Role, "database", cfg.DatabasePath)
	}
	_, _ = fmt.Fprintln(os.Stdout, "created initial admin user:", admin.Username)
	return nil
}

func runHealthcheck(logger *slog.Logger) error {
	_, db, redisClient, cleanup, err := openConfiguredDatabase()
	if err != nil {
		return err
	}
	defer cleanup()
	if err := runMigrations(db); err != nil {
		return err
	}
	if err := validateSchema(db); err != nil {
		return err
	}
	if redisClient == nil {
		return errors.New("redis connection is required")
	}
	return nil
}

func runBackup(args []string, logger *slog.Logger) error {
	if len(args) == 0 || args[0] == "export" {
		return runBackupExport(logger)
	}
	if args[0] == "import" {
		return runBackupImport(args[1:], logger)
	}
	return fmt.Errorf("unknown backup command %q", args[0])
}

func runBackupExport(logger *slog.Logger) error {
	cfg, db, cleanup, err := openConfiguredSQLite()
	if err != nil {
		return err
	}
	defer cleanup()
	_ = db
	service := services.NewBackupService()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	rec, err := service.Create(ctx, cfg.DatabasePath, cfg.BackupDir, cfg.AppName)
	if err != nil {
		return err
	}
	if logger != nil {
		logger.Info("created backup", "name", rec.Name, "size", rec.Size)
	}
	_, _ = fmt.Fprintln(os.Stdout, rec.Name)
	return nil
}

func runBackupImport(args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("backup import", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	file := fs.String("file", "", "backup file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	path := strings.TrimSpace(*file)
	if path == "" && fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	if path == "" {
		return errors.New("backup file is required")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return err
	}
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	if err := cfg.EnsurePaths(); err != nil {
		return err
	}
	service := services.NewBackupService()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	result, err := service.PrepareImport(ctx, data, cfg.DatabasePath, cfg.BackupDir, cfg.AppName)
	if err != nil {
		return err
	}
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	var dbBytes []byte
	for _, f := range archive.File {
		if f.Name != "hub.db" {
			continue
		}
		fhandle, err := f.Open()
		if err != nil {
			return err
		}
		dbBytes, err = io.ReadAll(fhandle)
		_ = fhandle.Close()
		if err != nil {
			return err
		}
		break
	}
	if len(dbBytes) == 0 {
		return errors.New("backup archive is missing hub.db")
	}
	if err := os.WriteFile(cfg.DatabasePath, dbBytes, 0o600); err != nil {
		return err
	}
	if logger != nil {
		logger.Info("imported backup", "archive", filepath.Base(abs), "safety_backup", result.SafetyBackup.Name)
	}
	_, _ = fmt.Fprintln(os.Stdout, "imported backup from", filepath.Base(abs))
	return nil
}

func openConfiguredSQLite() (*config.Config, *sql.DB, func(), error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, func() {}, err
	}
	if err := cfg.EnsurePaths(); err != nil {
		return nil, nil, func() {}, err
	}
	db, err := openSQLite(cfg.DatabasePath)
	if err != nil {
		return nil, nil, func() {}, err
	}
	cleanup := func() { _ = db.Close() }
	return cfg, db, cleanup, nil
}

func openConfiguredDatabase() (*config.Config, *sql.DB, io.Closer, func(), error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, nil, func() {}, err
	}
	if err := cfg.EnsurePaths(); err != nil {
		return nil, nil, nil, func() {}, err
	}
	db, err := openSQLite(cfg.DatabasePath)
	if err != nil {
		return nil, nil, nil, func() {}, err
	}
	rdb, err := openRedis(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		_ = db.Close()
		return nil, nil, nil, func() {}, err
	}
	cleanup := func() {
		_ = db.Close()
		_ = rdb.Close()
	}
	return cfg, db, rdb, cleanup, nil
}

func runVersion(w io.Writer) error {
	_, err := fmt.Fprintln(w, version.Full())
	return err
}

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: hub [serve|migrate|setup|create-admin|healthcheck|version|backup|help]")
}

func isKnownCommand(arg string) bool {
	switch strings.TrimSpace(arg) {
	case "serve", "migrate", "setup", "create-admin", "healthcheck", "version", "backup", "help", "-h", "--help":
		return true
	default:
		return false
	}
}
