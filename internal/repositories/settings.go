package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

type SettingsRepository interface {
	LoadAll(context.Context) (map[string]string, error)
	Upsert(context.Context, string, string) error
}

type sqliteSettingsRepository struct{ db *sql.DB }

func NewSettingsRepository(db *sql.DB) SettingsRepository { return &sqliteSettingsRepository{db: db} }

func (r *sqliteSettingsRepository) LoadAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM app_settings ORDER BY key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, err
		}
		items[key] = value
	}
	return items, rows.Err()
}

func (r *sqliteSettingsRepository) Upsert(ctx context.Context, key, value string) error {
	if r == nil || r.db == nil {
		return errors.New("settings repository is not configured")
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO app_settings (key, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, time.Now().UTC())
	return err
}
