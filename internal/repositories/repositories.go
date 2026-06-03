package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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
	Update(context.Context, *models.Client) error
	UpdateSubscriptionToken(context.Context, int64, string) error
	UpdateStatus(context.Context, int64, string) error
	UpdatePassword(context.Context, int64, string) error
}

type PanelRepository interface {
	Create(context.Context, *models.Panel) error
	List(context.Context) ([]models.Panel, error)
	FindByID(context.Context, int64) (*models.Panel, error)
	Update(context.Context, *models.Panel) error
	Delete(context.Context, int64) error
}

type InboundRepository interface {
	Upsert(context.Context, *models.Inbound) error
	MarkStaleByPanel(context.Context, int64, []int64) error
	List(context.Context) ([]models.Inbound, error)
	ListByPanel(context.Context, int64) ([]models.Inbound, error)
	FindByID(context.Context, int64) (*models.Inbound, error)
	TouchSyncedAt(context.Context, int64, time.Time) error
}

type AuditRepository interface {
	Create(context.Context, *models.AuditLog) error
	ListRecent(context.Context, int) ([]models.AuditLog, error)
	ListByTarget(context.Context, string, int64, int) ([]models.AuditLog, error)
}

type SyncJobRepository interface {
	Create(context.Context, *models.SyncJob) error
	Update(context.Context, *models.SyncJob) error
	FindByID(context.Context, int64) (*models.SyncJob, error)
	DeleteCompletedBefore(context.Context, time.Time) (int64, error)
}

type RefreshTokenRepository interface {
	Create(context.Context, *models.RefreshToken) error
	FindActiveByHash(context.Context, string) (*models.RefreshToken, error)
	RevokeByHash(context.Context, string) error
}

type sqliteAdminRepository struct{ db *sql.DB }
type sqliteClientRepository struct{ db *sql.DB }
type sqlitePanelRepository struct{ db *sql.DB }
type sqliteAuditRepository struct{ db *sql.DB }
type sqliteSyncJobRepository struct{ db *sql.DB }
type sqliteRefreshTokenRepository struct{ db *sql.DB }
type sqliteInboundRepository struct{ db *sql.DB }

func NewAdminRepository(db *sql.DB) AdminRepository   { return &sqliteAdminRepository{db: db} }
func NewClientRepository(db *sql.DB) ClientRepository { return &sqliteClientRepository{db: db} }
func NewPanelRepository(db *sql.DB) PanelRepository   { return &sqlitePanelRepository{db: db} }
func NewAuditRepository(db *sql.DB) AuditRepository   { return &sqliteAuditRepository{db: db} }
func NewSyncJobRepository(db *sql.DB) SyncJobRepository { return &sqliteSyncJobRepository{db: db} }
func NewRefreshTokenRepository(db *sql.DB) RefreshTokenRepository {
	return &sqliteRefreshTokenRepository{db: db}
}
func NewInboundRepository(db *sql.DB) InboundRepository { return &sqliteInboundRepository{db: db} }

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

func (r *sqliteClientRepository) Update(ctx context.Context, client *models.Client) error {
	if client == nil {
		return errors.New("client is nil")
	}
	if client.UpdatedAt.IsZero() {
		client.UpdatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `UPDATE clients SET username = ?, display_name = ?, email = ?, status = ?, traffic_limit_bytes = ?, expiry_time = ?, updated_at = ? WHERE id = ?`, client.Username, nullString(client.DisplayName), nullString(client.Email), client.Status, client.TrafficLimitBytes, nullTime(client.ExpiryTime), client.UpdatedAt.UTC(), client.ID)
	return err
}

func (r *sqliteClientRepository) UpdateSubscriptionToken(ctx context.Context, id int64, token string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE clients SET subscription_token = ?, updated_at = ? WHERE id = ?`, token, time.Now().UTC(), id)
	return err
}

func (r *sqliteClientRepository) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE clients SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now().UTC(), id)
	return err
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

func (r *sqlitePanelRepository) FindByID(ctx context.Context, id int64) (*models.Panel, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, base_url, username, encrypted_password, version, status, last_sync_at, last_error, created_at, updated_at FROM panels WHERE id = ?`, id)
	var panel models.Panel
	var version, lastError sql.NullString
	var lastSyncAt sql.NullTime
	if err := row.Scan(&panel.ID, &panel.Name, &panel.BaseURL, &panel.Username, &panel.EncryptedPassword, &version, &panel.Status, &lastSyncAt, &lastError, &panel.CreatedAt, &panel.UpdatedAt); err != nil {
		return nil, err
	}
	panel.Version = version.String
	panel.LastError = lastError.String
	if lastSyncAt.Valid {
		t := lastSyncAt.Time
		panel.LastSyncAt = &t
	}
	return &panel, nil
}

func (r *sqlitePanelRepository) Update(ctx context.Context, panel *models.Panel) error {
	if panel == nil {
		return errors.New("panel is nil")
	}
	if panel.UpdatedAt.IsZero() {
		panel.UpdatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `UPDATE panels SET name = ?, base_url = ?, username = ?, encrypted_password = ?, version = ?, status = ?, last_sync_at = ?, last_error = ?, updated_at = ? WHERE id = ?`, panel.Name, panel.BaseURL, panel.Username, panel.EncryptedPassword, nullString(panel.Version), panel.Status, nullTime(panel.LastSyncAt), nullString(panel.LastError), panel.UpdatedAt.UTC(), panel.ID)
	return err
}

func (r *sqlitePanelRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM panels WHERE id = ?`, id)
	return err
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

func (r *sqliteAuditRepository) ListByTarget(ctx context.Context, targetType string, targetID int64, limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, actor_type, actor_id, action, target_type, target_id, metadata_json, created_at FROM audit_logs WHERE target_type = ? AND target_id = ? ORDER BY id DESC LIMIT ?`, targetType, targetID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.AuditLog
	for rows.Next() {
		var audit models.AuditLog
		var actorID, targetIDVal sql.NullInt64
		var targetTypeVal, metadata sql.NullString
		if err := rows.Scan(&audit.ID, &audit.ActorType, &actorID, &audit.Action, &targetTypeVal, &targetIDVal, &metadata, &audit.CreatedAt); err != nil {
			return nil, err
		}
		if actorID.Valid {
			v := actorID.Int64
			audit.ActorID = &v
		}
		if targetIDVal.Valid {
			v := targetIDVal.Int64
			audit.TargetID = &v
		}
		audit.TargetType = targetTypeVal.String
		audit.MetadataJSON = metadata.String
		items = append(items, audit)
	}
	return items, rows.Err()
}

func (r *sqliteSyncJobRepository) Create(ctx context.Context, job *models.SyncJob) error {
	if job == nil {
		return errors.New("sync job is nil")
	}
	if job.CreatedAt.IsZero() {
		job.CreatedAt = time.Now().UTC()
	}
	if job.StartedAt == nil {
		now := job.CreatedAt
		job.StartedAt = &now
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO sync_jobs (panel_id, job_type, status, message, retry_count, started_at, finished_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`, job.PanelID, job.JobType, job.Status, nullString(job.Message), job.RetryCount, nullTime(job.StartedAt), nullTime(job.FinishedAt), job.CreatedAt.UTC())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	job.ID = id
	return nil
}

func (r *sqliteSyncJobRepository) Update(ctx context.Context, job *models.SyncJob) error {
	if job == nil {
		return errors.New("sync job is nil")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE sync_jobs SET panel_id = ?, job_type = ?, status = ?, message = ?, retry_count = ?, started_at = ?, finished_at = ? WHERE id = ?`, job.PanelID, job.JobType, job.Status, nullString(job.Message), job.RetryCount, nullTime(job.StartedAt), nullTime(job.FinishedAt), job.ID)
	return err
}

func (r *sqliteSyncJobRepository) FindByID(ctx context.Context, id int64) (*models.SyncJob, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, panel_id, job_type, status, message, retry_count, started_at, finished_at, created_at FROM sync_jobs WHERE id = ?`, id)
	return scanSyncJob(row)
}

func (r *sqliteSyncJobRepository) DeleteCompletedBefore(ctx context.Context, before time.Time) (int64, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, status, COALESCE(finished_at, started_at, created_at) FROM sync_jobs`)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	ids := make([]int64, 0)
	for rows.Next() {
		var id int64
		var status string
		var whenRaw string
		if err := rows.Scan(&id, &status, &whenRaw); err != nil {
			return 0, err
		}
		when, err := parseSQLiteTime(whenRaw)
		if err != nil {
			return 0, err
		}
		if (status == models.SyncJobStatusSuccess || status == models.SyncJobStatusFailed || status == models.SyncJobStatusCancelled) && when.Before(before.UTC()) {
			ids = append(ids, id)
		}
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(ids) == 0 {
		return 0, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM sync_jobs WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (r *sqliteInboundRepository) Upsert(ctx context.Context, inbound *models.Inbound) error {
	if inbound == nil {
		return errors.New("inbound is nil")
	}
	if inbound.CreatedAt.IsZero() {
		inbound.CreatedAt = time.Now().UTC()
	}
	if inbound.UpdatedAt.IsZero() {
		inbound.UpdatedAt = inbound.CreatedAt
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO inbounds (panel_id, remote_inbound_id, remark, protocol, port, network, security, enabled, stale, raw_json, last_synced_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(panel_id, remote_inbound_id) DO UPDATE SET remark = excluded.remark, protocol = excluded.protocol, port = excluded.port, network = excluded.network, security = excluded.security, enabled = excluded.enabled, stale = excluded.stale, raw_json = excluded.raw_json, last_synced_at = excluded.last_synced_at, updated_at = excluded.updated_at`, inbound.PanelID, inbound.RemoteInboundID, nullString(inbound.Remark), nullString(inbound.Protocol), inbound.Port, nullString(inbound.Network), nullString(inbound.Security), boolToInt(inbound.Enabled), boolToInt(inbound.Stale), nullString(inbound.RawJSON), nullTime(inbound.LastSyncedAt), inbound.CreatedAt.UTC(), inbound.UpdatedAt.UTC())
	return err
}

func (r *sqliteInboundRepository) MarkStaleByPanel(ctx context.Context, panelID int64, keep []int64) error {
	query := `UPDATE inbounds SET stale = 1, updated_at = ? WHERE panel_id = ?`
	args := []any{time.Now().UTC(), panelID}
	if len(keep) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(keep)), ",")
		query += " AND remote_inbound_id NOT IN (" + placeholders + ")"
		for _, id := range keep {
			args = append(args, id)
		}
	}
	_, err := r.db.ExecContext(ctx, query, args...)
	return err
}

func (r *sqliteInboundRepository) List(ctx context.Context) ([]models.Inbound, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, panel_id, remote_inbound_id, remark, protocol, port, network, security, enabled, stale, raw_json, last_synced_at, created_at, updated_at FROM inbounds ORDER BY id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.Inbound
	for rows.Next() {
		item, err := scanInbound(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *sqliteInboundRepository) ListByPanel(ctx context.Context, panelID int64) ([]models.Inbound, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, panel_id, remote_inbound_id, remark, protocol, port, network, security, enabled, stale, raw_json, last_synced_at, created_at, updated_at FROM inbounds WHERE panel_id = ? ORDER BY id DESC`, panelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []models.Inbound
	for rows.Next() {
		item, err := scanInbound(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *sqliteInboundRepository) FindByID(ctx context.Context, id int64) (*models.Inbound, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, panel_id, remote_inbound_id, remark, protocol, port, network, security, enabled, stale, raw_json, last_synced_at, created_at, updated_at FROM inbounds WHERE id = ?`, id)
	return scanInbound(row)
}

func (r *sqliteInboundRepository) TouchSyncedAt(ctx context.Context, id int64, syncedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE inbounds SET last_synced_at = ?, updated_at = ? WHERE id = ?`, syncedAt.UTC(), time.Now().UTC(), id)
	return err
}

func scanInbound(scanner interface{ Scan(...any) error }) (*models.Inbound, error) {
	var inbound models.Inbound
	var remark, protocol, network, security, rawJSON sql.NullString
	var enabled, stale int
	var lastSyncedAt sql.NullTime
	if err := scanner.Scan(&inbound.ID, &inbound.PanelID, &inbound.RemoteInboundID, &remark, &protocol, &inbound.Port, &network, &security, &enabled, &stale, &rawJSON, &lastSyncedAt, &inbound.CreatedAt, &inbound.UpdatedAt); err != nil {
		return nil, err
	}
	inbound.Remark = remark.String
	inbound.Protocol = protocol.String
	inbound.Network = network.String
	inbound.Security = security.String
	inbound.Enabled = enabled != 0
	inbound.Stale = stale != 0
	inbound.RawJSON = rawJSON.String
	if lastSyncedAt.Valid {
		t := lastSyncedAt.Time
		inbound.LastSyncedAt = &t
	}
	return &inbound, nil
}

func scanSyncJob(scanner interface{ Scan(...any) error }) (*models.SyncJob, error) {
	var job models.SyncJob
	var panelID sql.NullInt64
	var message sql.NullString
	var startedAt, finishedAt sql.NullTime
	if err := scanner.Scan(&job.ID, &panelID, &job.JobType, &job.Status, &message, &job.RetryCount, &startedAt, &finishedAt, &job.CreatedAt); err != nil {
		return nil, err
	}
	if panelID.Valid {
		v := panelID.Int64
		job.PanelID = &v
	}
	job.Message = message.String
	if startedAt.Valid {
		t := startedAt.Time
		job.StartedAt = &t
	}
	if finishedAt.Valid {
		t := finishedAt.Time
		job.FinishedAt = &t
	}
	return &job, nil
}

func (r *sqliteRefreshTokenRepository) Create(ctx context.Context, token *models.RefreshToken) error {
	if token == nil {
		return errors.New("refresh token is nil")
	}
	if token.CreatedAt.IsZero() {
		token.CreatedAt = time.Now().UTC()
	}
	_, err := r.db.ExecContext(ctx, `INSERT INTO client_refresh_tokens (client_id, token_hash, user_agent, ip_address, revoked_at, expires_at, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, token.ClientID, token.TokenHash, nullString(token.UserAgent), nullString(token.IPAddress), nullTime(token.RevokedAt), token.ExpiresAt.UTC(), token.CreatedAt.UTC())
	return err
}

func (r *sqliteRefreshTokenRepository) FindActiveByHash(ctx context.Context, hash string) (*models.RefreshToken, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, client_id, token_hash, user_agent, ip_address, revoked_at, expires_at, created_at FROM client_refresh_tokens WHERE token_hash = ? AND revoked_at IS NULL`, hash)
	var token models.RefreshToken
	var userAgent, ipAddress sql.NullString
	var revokedAt sql.NullTime
	if err := row.Scan(&token.ID, &token.ClientID, &token.TokenHash, &userAgent, &ipAddress, &revokedAt, &token.ExpiresAt, &token.CreatedAt); err != nil {
		return nil, err
	}
	token.UserAgent = userAgent.String
	token.IPAddress = ipAddress.String
	if revokedAt.Valid {
		t := revokedAt.Time
		token.RevokedAt = &t
	}
	if time.Now().UTC().After(token.ExpiresAt) {
		return nil, sql.ErrNoRows
	}
	return &token, nil
}

func (r *sqliteRefreshTokenRepository) RevokeByHash(ctx context.Context, hash string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE client_refresh_tokens SET revoked_at = ? WHERE token_hash = ? AND revoked_at IS NULL`, time.Now().UTC(), hash)
	return err
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

func parseSQLiteTime(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse sqlite time %q", value)
}
