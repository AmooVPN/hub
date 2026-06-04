package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

type Migration struct {
	Version int
	Name    string
	SQL     []string
}

var migrations = []Migration{
	{
		Version: 1,
		Name:    "initial_schema",
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS schema_migrations (
				version INTEGER PRIMARY KEY,
				applied_at DATETIME NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS admin_users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				username TEXT NOT NULL UNIQUE,
				email TEXT,
				password_hash TEXT NOT NULL,
				role TEXT NOT NULL,
				active BOOLEAN NOT NULL DEFAULT 1,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS clients (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				username TEXT NOT NULL UNIQUE,
				password_hash TEXT NOT NULL,
				display_name TEXT,
				email TEXT,
				status TEXT NOT NULL,
				traffic_limit_bytes INTEGER DEFAULT 0,
				expiry_time DATETIME,
				subscription_token TEXT NOT NULL UNIQUE,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS panels (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				base_url TEXT NOT NULL UNIQUE,
				username TEXT NOT NULL,
				encrypted_password TEXT NOT NULL,
				version TEXT,
				status TEXT NOT NULL,
				last_sync_at DATETIME,
				last_error TEXT,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS inbounds (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				panel_id INTEGER NOT NULL,
				remote_inbound_id INTEGER NOT NULL,
				remark TEXT,
				protocol TEXT,
				port INTEGER,
				network TEXT,
				security TEXT,
				enabled BOOLEAN NOT NULL DEFAULT 1,
				raw_json TEXT,
				last_synced_at DATETIME,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				FOREIGN KEY(panel_id) REFERENCES panels(id) ON DELETE CASCADE
			);`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_inbounds_panel_remote ON inbounds(panel_id, remote_inbound_id);`,
			`CREATE TABLE IF NOT EXISTS client_attachments (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				client_id INTEGER NOT NULL,
				panel_id INTEGER NOT NULL,
				inbound_id INTEGER NOT NULL,
				remote_client_id TEXT,
				remote_email TEXT,
				enabled BOOLEAN NOT NULL DEFAULT 1,
				upload_bytes INTEGER DEFAULT 0,
				download_bytes INTEGER DEFAULT 0,
				traffic_limit_bytes INTEGER DEFAULT 0,
				expiry_time DATETIME,
				raw_config TEXT,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL,
				FOREIGN KEY(client_id) REFERENCES clients(id) ON DELETE CASCADE,
				FOREIGN KEY(panel_id) REFERENCES panels(id) ON DELETE CASCADE,
				FOREIGN KEY(inbound_id) REFERENCES inbounds(id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS client_refresh_tokens (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				client_id INTEGER NOT NULL,
				token_hash TEXT NOT NULL UNIQUE,
				user_agent TEXT,
				ip_address TEXT,
				revoked_at DATETIME,
				expires_at DATETIME NOT NULL,
				created_at DATETIME NOT NULL,
				FOREIGN KEY(client_id) REFERENCES clients(id) ON DELETE CASCADE
			);`,
			`CREATE TABLE IF NOT EXISTS traffic_snapshots (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				client_id INTEGER NOT NULL,
				attachment_id INTEGER,
				upload_bytes INTEGER NOT NULL,
				download_bytes INTEGER NOT NULL,
				total_bytes INTEGER NOT NULL,
				captured_at DATETIME NOT NULL,
				FOREIGN KEY(client_id) REFERENCES clients(id) ON DELETE CASCADE,
				FOREIGN KEY(attachment_id) REFERENCES client_attachments(id) ON DELETE SET NULL
			);`,
			`CREATE TABLE IF NOT EXISTS audit_logs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				actor_type TEXT NOT NULL,
				actor_id INTEGER,
				action TEXT NOT NULL,
				target_type TEXT,
				target_id INTEGER,
				metadata_json TEXT,
				created_at DATETIME NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS sync_jobs (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				panel_id INTEGER,
				job_type TEXT NOT NULL,
				status TEXT NOT NULL,
				message TEXT,
				started_at DATETIME,
				finished_at DATETIME,
				created_at DATETIME NOT NULL,
				FOREIGN KEY(panel_id) REFERENCES panels(id) ON DELETE SET NULL
			);`,
		},
	},
	{
		Version: 2,
		Name:    "add_inbound_stale_flag",
		SQL: []string{
			`ALTER TABLE inbounds ADD COLUMN stale BOOLEAN NOT NULL DEFAULT 0;`,
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_inbounds_panel_remote ON inbounds(panel_id, remote_inbound_id);`,
		},
	},
	{
		Version: 3,
		Name:    "add_client_attachment_unique_index",
		SQL: []string{
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_client_attachments_client_panel_inbound ON client_attachments(client_id, panel_id, inbound_id);`,
		},
	},
	{
		Version: 4,
		Name:    "add_sync_job_retry_count",
		SQL: []string{
			`ALTER TABLE sync_jobs ADD COLUMN retry_count INTEGER NOT NULL DEFAULT 0;`,
		},
	},
	{
		Version: 5,
		Name:    "add_panel_last_checked_at",
		SQL: []string{
			`ALTER TABLE panels ADD COLUMN last_checked_at DATETIME;`,
		},
	},
	{
		Version: 6,
		Name:    "add_webhook_tables",
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS webhooks (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				url TEXT NOT NULL,
				secret TEXT NOT NULL,
				active BOOLEAN NOT NULL DEFAULT 1,
				events TEXT NOT NULL,
				created_at DATETIME NOT NULL,
				updated_at DATETIME NOT NULL
			);`,
			`CREATE TABLE IF NOT EXISTS webhook_deliveries (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				webhook_id INTEGER NOT NULL,
				event_type TEXT NOT NULL,
				payload_json TEXT NOT NULL,
				status TEXT NOT NULL,
				response_status INTEGER,
				response_body TEXT,
				error_message TEXT,
				attempts INTEGER NOT NULL DEFAULT 0,
				next_retry_at DATETIME,
				created_at DATETIME NOT NULL,
				delivered_at DATETIME,
				FOREIGN KEY(webhook_id) REFERENCES webhooks(id) ON DELETE CASCADE
			);`,
		},
	},
	{
		Version: 7,
		Name:    "add_notifications_table",
		SQL: []string{
			`CREATE TABLE IF NOT EXISTS notifications (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				type TEXT NOT NULL,
				severity TEXT NOT NULL,
				title TEXT NOT NULL,
				message TEXT NOT NULL,
				read_at DATETIME,
				created_at DATETIME NOT NULL
			);`,
		},
	},
}

func RunMigrations(db *sql.DB) error {
	if len(migrations) == 0 {
		return nil
	}
	if err := validateMigrationRegistry(migrations); err != nil {
		return err
	}

	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Version < sorted[j].Version })

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		applied_at DATETIME NOT NULL
	);`); err != nil {
		return err
	}

	applied := map[int]struct{}{}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return err
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	latest := sorted[len(sorted)-1].Version
	for version := range applied {
		if version > latest {
			return fmt.Errorf("database schema version %d is newer than supported version %d", version, latest)
		}
	}

	for _, migration := range sorted {
		if _, ok := applied[migration.Version]; ok {
			continue
		}
		if err := applyMigration(ctx, db, migration); err != nil {
			return err
		}
	}

	return nil
}

func validateMigrationRegistry(registry []Migration) error {
	if len(registry) == 0 {
		return errors.New("no migrations registered")
	}
	seen := make(map[int]struct{}, len(registry))
	for _, migration := range registry {
		if migration.Version <= 0 {
			return fmt.Errorf("invalid migration version %d", migration.Version)
		}
		if strings.TrimSpace(migration.Name) == "" {
			return fmt.Errorf("migration %d has empty name", migration.Version)
		}
		if len(migration.SQL) == 0 {
			return fmt.Errorf("migration %d (%s) has no SQL statements", migration.Version, migration.Name)
		}
		if _, ok := seen[migration.Version]; ok {
			return fmt.Errorf("duplicate migration version %d", migration.Version)
		}
		seen[migration.Version] = struct{}{}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, migration Migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, statement := range migration.SQL {
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", migration.Version, migration.Name, err)
		}
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, migration.Version, time.Now().UTC()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return nil
}

func ValidateSchemaVersion(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := validateMigrationRegistry(migrations); err != nil {
		return err
	}

	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	defer rows.Close()

	applied := map[int]struct{}{}
	for rows.Next() {
		var version int
		if err := rows.Scan(&version); err != nil {
			return err
		}
		applied[version] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(applied) == 0 {
		return errors.New("no schema migrations applied")
	}

	registered := map[int]struct{}{}
	for _, migration := range migrations {
		registered[migration.Version] = struct{}{}
	}
	for version := range applied {
		if _, ok := registered[version]; !ok {
			return fmt.Errorf("database schema version %d is newer than supported version %d", version, latestMigrationVersion(migrations))
		}
	}
	for version := range registered {
		if _, ok := applied[version]; !ok {
			return fmt.Errorf("database schema version %d is missing", version)
		}
	}
	return nil
}

func latestMigrationVersion(registry []Migration) int {
	latest := 0
	for _, migration := range registry {
		if migration.Version > latest {
			latest = migration.Version
		}
	}
	return latest
}
