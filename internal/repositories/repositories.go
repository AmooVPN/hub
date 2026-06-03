package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

type AdminRepository interface {
	Count(context.Context) (int64, error)
	CountByRole(context.Context, string) (int64, error)
	Create(context.Context, *models.AdminUser) error
	FindByUsername(context.Context, string) (*models.AdminUser, error)
	FindByID(context.Context, int64) (*models.AdminUser, error)
	List(context.Context) ([]models.AdminUser, error)
	Update(context.Context, *models.AdminUser) error
	UpdatePassword(context.Context, int64, string) error
	SetActive(context.Context, int64, bool) error
	Delete(context.Context, int64) error
}

type ClientRepository interface {
	Count(context.Context) (int64, error)
	Create(context.Context, *models.Client) error
	FindByUsername(context.Context, string) (*models.Client, error)
	UpdatePassword(context.Context, int64, string) error
}

type PanelRepository interface {
	Create(context.Context, *models.Panel) error
	List(context.Context) ([]models.Panel, error)
}

type AuditRepository interface {
	Create(context.Context, *models.AuditLog) error
	ListRecent(context.Context, int) ([]models.AuditLog, error)
}

type sqliteAdminRepository struct{ db *sql.DB }
type sqliteClientRepository struct{ db *sql.DB }
type sqlitePanelRepository struct{ db *sql.DB }
type sqliteAuditRepository struct{ db *sql.DB }

func NewAdminRepository(db *sql.DB) AdminRepository { return &sqliteAdminRepository{db: db} }
func NewClientRepository(db *sql.DB) ClientRepository { return &sqliteClientRepository{db: db} }
func NewPanelRepository(db *sql.DB) PanelRepository { return &sqlitePanelRepository{db: db} }
func NewAuditRepository(db *sql.DB) AuditRepository { return &sqliteAuditRepository{db: db} }

func (r *sqliteAdminRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *sqliteAdminRepository) CountByRole(ctx context.Context, role string) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM admin_users WHERE role = ? AND active = 1`, role).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *sqliteAdminRepository) Create(ctx context.Context, admin *models.AdminUser) error {
	if admin == nil {
		return errors.New("admin is nil")
	}
	if admin.CreatedAt.IsZero() {
		admin.CreatedAt = time.Now().UTC()
	}
	if admin.UpdatedAt.IsZero() {
		admin.UpdatedAt = admin.CreatedAt
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO admin_users (username, email, password_hash, role, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, admin.Username, nullString(admin.Email), admin.PasswordHash, admin.Role, boolToInt(admin.Active), admin.CreatedAt.UTC(), admin.UpdatedAt.UTC())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	admin.ID = id
	return nil
}

func (r *sqliteAdminRepository) FindByUsername(ctx context.Context, username string) (*models.AdminUser, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, username, email, password_hash, role, active, created_at, updated_at FROM admin_users WHERE username = ?`, username)
	return scanAdmin(row)
}

func (r *sqliteAdminRepository) FindByID(ctx context.Context, id int64) (*models.AdminUser, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, username, email, password_hash, role, active, created_at, updated_at FROM admin_users WHERE id = ?`, id)
	return scanAdmin(row)
}

func (r *sqliteAdminRepository) List(ctx context.Context) ([]models.AdminUser, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, username, email, password_hash, role, active, created_at, updated_at FROM admin_users ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AdminUser
	for rows.Next() {
		item, err := scanAdmin(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *sqliteAdminRepository) Update(ctx context.Context, admin *models.AdminUser) error {
	if admin == nil {
		return errors.New("admin is nil")
	}
	if admin.UpdatedAt.IsZero() {
		admin.UpdatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `UPDATE admin_users SET username = ?, email = ?, role = ?, active = ?, updated_at = ? WHERE id = ?`, admin.Username, nullString(admin.Email), admin.Role, boolToInt(admin.Active), admin.UpdatedAt.UTC(), admin.ID)
	return err
}

func (r *sqliteAdminRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_users SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, time.Now().UTC(), id)
	return err
}

func (r *sqliteAdminRepository) SetActive(ctx context.Context, id int64, active bool) error {
	_, err := r.db.ExecContext(ctx, `UPDATE admin_users SET active = ?, updated_at = ? WHERE id = ?`, boolToInt(active), time.Now().UTC(), id)
	return err
}

func (r *sqliteAdminRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM admin_users WHERE id = ?`, id)
	return err
}

func scanAdmin(scanner interface{ Scan(...any) error }) (*models.AdminUser, error) {
	var admin models.AdminUser
	var email sql.NullString
	var active int
	if err := scanner.Scan(&admin.ID, &admin.Username, &email, &admin.PasswordHash, &admin.Role, &active, &admin.CreatedAt, &admin.UpdatedAt); err != nil {
		return nil, err
	}
	admin.Email = email.String
	admin.Active = active != 0
	return &admin, nil
}

func (r *sqliteClientRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM clients`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *sqliteClientRepository) Create(ctx context.Context, client *models.Client) error {
	if client == nil {
		return errors.New("client is nil")
	}
	if client.CreatedAt.IsZero() {
		client.CreatedAt = time.Now().UTC()
	}
	if client.UpdatedAt.IsZero() {
		client.UpdatedAt = client.CreatedAt
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO clients (username, password_hash, display_name, email, status, traffic_limit_bytes, expiry_time, subscription_token, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, client.Username, client.PasswordHash, nullString(client.DisplayName), nullString(client.Email), client.Status, client.TrafficLimitBytes, nullTime(client.ExpiryTime), client.SubscriptionToken, client.CreatedAt.UTC(), client.UpdatedAt.UTC())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	client.ID = id
	return nil
}

func (r *sqliteClientRepository) FindByUsername(ctx context.Context, username string) (*models.Client, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, display_name, email, status, traffic_limit_bytes, expiry_time, subscription_token, created_at, updated_at FROM clients WHERE username = ?`, username)
	return scanClient(row)
}

func (r *sqliteClientRepository) UpdatePassword(ctx context.Context, id int64, passwordHash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE clients SET password_hash = ?, updated_at = ? WHERE id = ?`, passwordHash, time.Now().UTC(), id)
	return err
}

func scanClient(scanner interface{ Scan(...any) error }) (*models.Client, error) {
	var client models.Client
	var displayName, email, subscriptionToken sql.NullString
	var expiryTime sql.NullTime
	if err := scanner.Scan(&client.ID, &client.Username, &client.PasswordHash, &displayName, &email, &client.Status, &client.TrafficLimitBytes, &expiryTime, &subscriptionToken, &client.CreatedAt, &client.UpdatedAt); err != nil {
		return nil, err
	}
	client.DisplayName = displayName.String
	client.Email = email.String
	client.SubscriptionToken = subscriptionToken.String
	if expiryTime.Valid {
		t := expiryTime.Time
		client.ExpiryTime = &t
	}
	return &client, nil
}

func (r *sqlitePanelRepository) Create(ctx context.Context, panel *models.Panel) error {
	if panel == nil {
		return errors.New("panel is nil")
	}
	if panel.CreatedAt.IsZero() {
		panel.CreatedAt = time.Now().UTC()
	}
	if panel.UpdatedAt.IsZero() {
		panel.UpdatedAt = panel.CreatedAt
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO panels (name, base_url, username, encrypted_password, version, status, last_sync_at, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, panel.Name, panel.BaseURL, panel.Username, panel.EncryptedPassword, nullString(panel.Version), panel.Status, nullTime(panel.LastSyncAt), nullString(panel.LastError), panel.CreatedAt.UTC(), panel.UpdatedAt.UTC())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	panel.ID = id
	return nil
}

func (r *sqlitePanelRepository) List(ctx context.Context) ([]models.Panel, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, base_url, username, encrypted_password, version, status, last_sync_at, last_error, created_at, updated_at FROM panels ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.Panel
	for rows.Next() {
		var panel models.Panel
		var version, lastError sql.NullString
		var lastSyncAt sql.NullTime
		if err := rows.Scan(&panel.ID, &panel.Name, &panel.BaseURL, &panel.Username, &panel.EncryptedPassword, &version, &panel.Status, &lastSyncAt, &lastError, &panel.CreatedAt, &panel.UpdatedAt); err != nil {
			return nil, err
		}
		panel.Version = version.String
		panel.LastError = lastError.String
		if lastSyncAt.Valid {
			t := lastSyncAt.Time
			panel.LastSyncAt = &t
		}
		items = append(items, panel)
	}
	return items, rows.Err()
}

func (r *sqliteAuditRepository) Create(ctx context.Context, audit *models.AuditLog) error {
	if audit == nil {
		return errors.New("audit is nil")
	}
	if audit.CreatedAt.IsZero() {
		audit.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO audit_logs (actor_type, actor_id, action, target_type, target_id, metadata_json, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, audit.ActorType, audit.ActorID, audit.Action, nullString(audit.TargetType), audit.TargetID, nullString(audit.MetadataJSON), audit.CreatedAt.UTC())
	return err
}

func (r *sqliteAuditRepository) ListRecent(ctx context.Context, limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, actor_type, actor_id, action, target_type, target_id, metadata_json, created_at FROM audit_logs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AuditLog
	for rows.Next() {
		var audit models.AuditLog
		var actorID, targetID sql.NullInt64
		var targetType, metadata sql.NullString
		if err := rows.Scan(&audit.ID, &audit.ActorType, &actorID, &audit.Action, &targetType, &targetID, &metadata, &audit.CreatedAt); err != nil {
			return nil, err
		}
		if actorID.Valid {
			v := actorID.Int64
			audit.ActorID = &v
		}
		if targetID.Valid {
			v := targetID.Int64
			audit.TargetID = &v
		}
		audit.TargetType = targetType.String
		audit.MetadataJSON = metadata.String
		items = append(items, audit)
	}
	return items, rows.Err()
}

func nullString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullTime(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.UTC()
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func must[T any](value T, err error) T {
	if err != nil {
		panic(fmt.Sprintf("unexpected error: %v", err))
	}
	return value
}
