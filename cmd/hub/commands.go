package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/AmooVPM/hub/internal/bootstrap"
	"github.com/AmooVPM/hub/internal/config"
	"github.com/AmooVPM/hub/internal/database"
	app "github.com/AmooVPM/hub/internal/app"
	"github.com/AmooVPM/hub/internal/repositories"
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
	case "create-admin":
		return runCreateAdmin(logger)
	case "healthcheck":
		return runHealthcheck(logger)
	case "backup":
		if len(rest) > 0 && rest[0] == "export" {
			return errors.New("backup export is not implemented yet")
		}
		if len(rest) > 0 && rest[0] == "import" {
			return errors.New("backup import is not implemented yet")
		}
		return errors.New("backup commands are not implemented yet")
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
	cfg, db, _, cleanup, err := openConfiguredDatabase()
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
	return ensureInitialAdmin(ctx, cfg, admins, logger)
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

func printUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: hub [serve|migrate|create-admin|healthcheck|help]")
}

func isKnownCommand(arg string) bool {
	switch strings.TrimSpace(arg) {
	case "serve", "migrate", "create-admin", "healthcheck", "backup", "help", "-h", "--help":
		return true
	default:
		return false
	}
}
