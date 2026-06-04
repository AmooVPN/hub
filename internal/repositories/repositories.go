package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/models"
	"gorm.io/gorm"
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
	FindByID(context.Context, int64) (*models.Client, error)
	FindByUsername(context.Context, string) (*models.Client, error)
	Delete(context.Context, int64) error
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
	FindByID(context.Context, int64) (*models.AuditLog, error)
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

type NotificationRepository interface {
	Create(context.Context, *models.Notification) error
	ListRecent(context.Context, int) ([]models.Notification, error)
	CountUnread(context.Context) (int64, error)
	FindByID(context.Context, int64) (*models.Notification, error)
	MarkRead(context.Context, int64) error
	MarkAllRead(context.Context) error
}

type WebhookRepository interface {
	Create(context.Context, *models.Webhook) error
	Update(context.Context, *models.Webhook) error
	Delete(context.Context, int64) error
	FindByID(context.Context, int64) (*models.Webhook, error)
	List(context.Context) ([]models.Webhook, error)
	ListActiveByEvent(context.Context, string) ([]models.Webhook, error)
}

type WebhookDeliveryRepository interface {
	Create(context.Context, *models.WebhookDelivery) error
	Update(context.Context, *models.WebhookDelivery) error
	FindByID(context.Context, int64) (*models.WebhookDelivery, error)
	ListByWebhook(context.Context, int64) ([]models.WebhookDelivery, error)
	ListDueForRetry(context.Context, time.Time, int) ([]models.WebhookDelivery, error)
}

type sqliteAdminRepository struct{ db *sql.DB }
type sqliteClientRepository struct{ db *sql.DB }
type sqlitePanelRepository struct{ db *gorm.DB }
type sqliteAuditRepository struct{ db *sql.DB }
type sqliteSyncJobRepository struct{ db *sql.DB }
type sqliteRefreshTokenRepository struct{ db *sql.DB }
type sqliteInboundRepository struct{ db *sql.DB }
type sqliteWebhookRepository struct{ db *sql.DB }
type sqliteWebhookDeliveryRepository struct{ db *sql.DB }
type sqliteNotificationRepository struct{ db *sql.DB }

func NewAdminRepository(db *sql.DB) AdminRepository   { return &sqliteAdminRepository{db: db} }
func NewClientRepository(db *sql.DB) ClientRepository { return &sqliteClientRepository{db: db} }
func NewPanelRepository(db *sql.DB) PanelRepository {
	gormDB, err := database.OpenGormSQLite(db)
	if err != nil {
		panic(fmt.Sprintf("open gorm sqlite: %v", err))
	}
	return &sqlitePanelRepository{db: gormDB}
}
func NewAuditRepository(db *sql.DB) AuditRepository     { return &sqliteAuditRepository{db: db} }
func NewSyncJobRepository(db *sql.DB) SyncJobRepository { return &sqliteSyncJobRepository{db: db} }
func NewRefreshTokenRepository(db *sql.DB) RefreshTokenRepository {
	return &sqliteRefreshTokenRepository{db: db}
}
func NewInboundRepository(db *sql.DB) InboundRepository { return &sqliteInboundRepository{db: db} }
func NewWebhookRepository(db *sql.DB) WebhookRepository { return &sqliteWebhookRepository{db: db} }
func NewWebhookDeliveryRepository(db *sql.DB) WebhookDeliveryRepository {
	return &sqliteWebhookDeliveryRepository{db: db}
}
func NewNotificationRepository(db *sql.DB) NotificationRepository {
	return &sqliteNotificationRepository{db: db}
}

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

func (r *sqliteClientRepository) FindByID(ctx context.Context, id int64) (*models.Client, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, display_name, email, status, traffic_limit_bytes, expiry_time, subscription_token, created_at, updated_at FROM clients WHERE id = ?`, id)
	return scanClient(row)
}

func (r *sqliteClientRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE clients SET status = 'deleted', updated_at = ? WHERE id = ?`, time.Now().UTC(), id)
	return err
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
	return r.db.WithContext(ctx).Create(panel).Error
}

func (r *sqlitePanelRepository) FindByID(ctx context.Context, id int64) (*models.Panel, error) {
	var panel models.Panel
	if err := r.db.WithContext(ctx).First(&panel, id).Error; err != nil {
		return nil, err
	}
	return &panel, nil
}

func (r *sqlitePanelRepository) Update(ctx context.Context, panel *models.Panel) error {
	if panel == nil {
		return errors.New("panel is nil")
	}
	return r.db.WithContext(ctx).Save(panel).Error
}

func (r *sqlitePanelRepository) Delete(ctx context.Context, id int64) error {
	return r.db.WithContext(ctx).Delete(&models.Panel{}, id).Error
}

func (r *sqlitePanelRepository) List(ctx context.Context) ([]models.Panel, error) {
	var items []models.Panel
	if err := r.db.WithContext(ctx).Order("id ASC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
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

func (r *sqliteAuditRepository) FindByID(ctx context.Context, id int64) (*models.AuditLog, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, actor_type, actor_id, action, target_type, target_id, metadata_json, created_at FROM audit_logs WHERE id = ?`, id)
	return scanAudit(row)
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
		audit, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *audit)
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
		audit, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *audit)
	}
	return items, rows.Err()
}

func scanAudit(scanner interface{ Scan(...any) error }) (*models.AuditLog, error) {
	var audit models.AuditLog
	var actorID, targetID sql.NullInt64
	var targetType, metadata sql.NullString
	if err := scanner.Scan(&audit.ID, &audit.ActorType, &actorID, &audit.Action, &targetType, &targetID, &metadata, &audit.CreatedAt); err != nil {
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
	return &audit, nil
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

func nullInt(value *int) any {
	if value == nil {
		return nil
	}
	return *value
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
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05 -0700 MST", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("parse sqlite time %q", value)
}

func (r *sqliteWebhookRepository) Create(ctx context.Context, webhook *models.Webhook) error {
	if webhook == nil {
		return errors.New("webhook is nil")
	}
	if webhook.CreatedAt.IsZero() {
		webhook.CreatedAt = time.Now().UTC()
	}
	if webhook.UpdatedAt.IsZero() {
		webhook.UpdatedAt = webhook.CreatedAt
	}
	events, err := json.Marshal(webhook.Events)
	if err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO webhooks (name, url, secret, active, events, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, webhook.Name, webhook.URL, webhook.Secret, boolToInt(webhook.Active), string(events), webhook.CreatedAt.UTC(), webhook.UpdatedAt.UTC())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	webhook.ID = id
	return nil
}

func (r *sqliteWebhookRepository) Update(ctx context.Context, webhook *models.Webhook) error {
	if webhook == nil {
		return errors.New("webhook is nil")
	}
	if webhook.UpdatedAt.IsZero() {
		webhook.UpdatedAt = time.Now().UTC()
	}
	events, err := json.Marshal(webhook.Events)
	if err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `UPDATE webhooks SET name = ?, url = ?, secret = ?, active = ?, events = ?, updated_at = ? WHERE id = ?`, webhook.Name, webhook.URL, webhook.Secret, boolToInt(webhook.Active), string(events), webhook.UpdatedAt.UTC(), webhook.ID)
	return err
}

func (r *sqliteWebhookRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	return err
}

func (r *sqliteWebhookRepository) FindByID(ctx context.Context, id int64) (*models.Webhook, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, name, url, secret, active, events, created_at, updated_at FROM webhooks WHERE id = ?`, id)
	return scanWebhook(row)
}

func (r *sqliteWebhookRepository) List(ctx context.Context) ([]models.Webhook, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, url, secret, active, events, created_at, updated_at FROM webhooks ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.Webhook, 0)
	for rows.Next() {
		webhook, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *webhook)
	}
	return items, rows.Err()
}

func (r *sqliteWebhookRepository) ListActiveByEvent(ctx context.Context, event string) ([]models.Webhook, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, url, secret, active, events, created_at, updated_at FROM webhooks WHERE active = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.Webhook, 0)
	for rows.Next() {
		webhook, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		if containsString(webhook.Events, event) {
			items = append(items, *webhook)
		}
	}
	return items, rows.Err()
}

func scanWebhook(scanner interface{ Scan(...any) error }) (*models.Webhook, error) {
	var webhook models.Webhook
	var active int
	var events string
	if err := scanner.Scan(&webhook.ID, &webhook.Name, &webhook.URL, &webhook.Secret, &active, &events, &webhook.CreatedAt, &webhook.UpdatedAt); err != nil {
		return nil, err
	}
	webhook.Active = active != 0
	if err := json.Unmarshal([]byte(events), &webhook.Events); err != nil {
		webhook.Events = splitCSV(events)
	}
	return &webhook, nil
}

func (r *sqliteWebhookDeliveryRepository) Create(ctx context.Context, delivery *models.WebhookDelivery) error {
	if delivery == nil {
		return errors.New("webhook delivery is nil")
	}
	if delivery.CreatedAt.IsZero() {
		delivery.CreatedAt = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO webhook_deliveries (webhook_id, event_type, payload_json, status, response_status, response_body, error_message, attempts, next_retry_at, created_at, delivered_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, delivery.WebhookID, delivery.EventType, delivery.PayloadJSON, delivery.Status, nullInt(delivery.ResponseStatus), nullString(delivery.ResponseBody), nullString(delivery.ErrorMessage), delivery.Attempts, nullTime(delivery.NextRetryAt), delivery.CreatedAt.UTC(), nullTime(delivery.DeliveredAt))
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	delivery.ID = id
	return nil
}

func (r *sqliteWebhookDeliveryRepository) Update(ctx context.Context, delivery *models.WebhookDelivery) error {
	if delivery == nil {
		return errors.New("webhook delivery is nil")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE webhook_deliveries SET webhook_id = ?, event_type = ?, payload_json = ?, status = ?, response_status = ?, response_body = ?, error_message = ?, attempts = ?, next_retry_at = ?, delivered_at = ? WHERE id = ?`, delivery.WebhookID, delivery.EventType, delivery.PayloadJSON, delivery.Status, nullInt(delivery.ResponseStatus), nullString(delivery.ResponseBody), nullString(delivery.ErrorMessage), delivery.Attempts, nullTime(delivery.NextRetryAt), nullTime(delivery.DeliveredAt), delivery.ID)
	return err
}

func (r *sqliteWebhookDeliveryRepository) FindByID(ctx context.Context, id int64) (*models.WebhookDelivery, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, webhook_id, event_type, payload_json, status, response_status, response_body, error_message, attempts, next_retry_at, created_at, delivered_at FROM webhook_deliveries WHERE id = ?`, id)
	return scanWebhookDelivery(row)
}

func (r *sqliteWebhookDeliveryRepository) ListByWebhook(ctx context.Context, webhookID int64) ([]models.WebhookDelivery, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, webhook_id, event_type, payload_json, status, response_status, response_body, error_message, attempts, next_retry_at, created_at, delivered_at FROM webhook_deliveries WHERE webhook_id = ? ORDER BY id DESC`, webhookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.WebhookDelivery, 0)
	for rows.Next() {
		delivery, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *delivery)
	}
	return items, rows.Err()
}

func (r *sqliteWebhookDeliveryRepository) ListDueForRetry(ctx context.Context, at time.Time, limit int) ([]models.WebhookDelivery, error) {
	if limit <= 0 {
		limit = 25
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, webhook_id, event_type, payload_json, status, response_status, response_body, error_message, attempts, next_retry_at, created_at, delivered_at FROM webhook_deliveries WHERE status IN ('failed', 'retrying') AND next_retry_at IS NOT NULL AND next_retry_at <= ? ORDER BY next_retry_at ASC LIMIT ?`, at.UTC(), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.WebhookDelivery, 0)
	for rows.Next() {
		delivery, err := scanWebhookDelivery(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *delivery)
	}
	return items, rows.Err()
}

func (r *sqliteNotificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	if notification == nil {
		return errors.New("notification is nil")
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now().UTC()
	}
	result, err := r.db.ExecContext(ctx, `INSERT INTO notifications (type, severity, title, message, read_at, created_at) VALUES (?, ?, ?, ?, ?, ?)`, notification.Type, notification.Severity, notification.Title, notification.Message, nullTime(notification.ReadAt), notification.CreatedAt.UTC())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	notification.ID = id
	return nil
}

func (r *sqliteNotificationRepository) ListRecent(ctx context.Context, limit int) ([]models.Notification, error) {
	if limit <= 0 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, type, severity, title, message, read_at, created_at FROM notifications ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]models.Notification, 0)
	for rows.Next() {
		notification, err := scanNotification(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *notification)
	}
	return items, rows.Err()
}

func (r *sqliteNotificationRepository) CountUnread(ctx context.Context) (int64, error) {
	var count int64
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM notifications WHERE read_at IS NULL`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *sqliteNotificationRepository) FindByID(ctx context.Context, id int64) (*models.Notification, error) {
	row := r.db.QueryRowContext(ctx, `SELECT id, type, severity, title, message, read_at, created_at FROM notifications WHERE id = ?`, id)
	return scanNotification(row)
}

func (r *sqliteNotificationRepository) MarkRead(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE id = ? AND read_at IS NULL`, time.Now().UTC(), id)
	return err
}

func (r *sqliteNotificationRepository) MarkAllRead(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `UPDATE notifications SET read_at = ? WHERE read_at IS NULL`, time.Now().UTC())
	return err
}

func scanNotification(scanner interface{ Scan(...any) error }) (*models.Notification, error) {
	var notification models.Notification
	var readAt sql.NullTime
	if err := scanner.Scan(&notification.ID, &notification.Type, &notification.Severity, &notification.Title, &notification.Message, &readAt, &notification.CreatedAt); err != nil {
		return nil, err
	}
	if readAt.Valid {
		t := readAt.Time
		notification.ReadAt = &t
	}
	return &notification, nil
}

func scanWebhookDelivery(scanner interface{ Scan(...any) error }) (*models.WebhookDelivery, error) {
	var delivery models.WebhookDelivery
	var responseStatus sql.NullInt64
	var responseBody, errorMessage sql.NullString
	var nextRetryAt, deliveredAt sql.NullTime
	if err := scanner.Scan(&delivery.ID, &delivery.WebhookID, &delivery.EventType, &delivery.PayloadJSON, &delivery.Status, &responseStatus, &responseBody, &errorMessage, &delivery.Attempts, &nextRetryAt, &delivery.CreatedAt, &deliveredAt); err != nil {
		return nil, err
	}
	if responseStatus.Valid {
		v := int(responseStatus.Int64)
		delivery.ResponseStatus = &v
	}
	delivery.ResponseBody = responseBody.String
	delivery.ErrorMessage = errorMessage.String
	if nextRetryAt.Valid {
		t := nextRetryAt.Time
		delivery.NextRetryAt = &t
	}
	if deliveredAt.Valid {
		t := deliveredAt.Time
		delivery.DeliveredAt = &t
	}
	return &delivery, nil
}

func containsString(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(target)) {
			return true
		}
	}
	return false
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}
