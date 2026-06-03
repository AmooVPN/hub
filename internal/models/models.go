package models

import (
	"errors"
	"time"
)

type AdminUser struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"`
	Role         string    `json:"role"`
	Active       bool      `json:"active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AdminUserDTO struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email,omitempty"`
	Role      string    `json:"role"`
	Active    bool      `json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (u AdminUser) Validate() error {
	if u.Username == "" || u.PasswordHash == "" || u.Role == "" {
		return errors.New("invalid admin user")
	}
	return nil
}

func (u AdminUser) ToDTO() AdminUserDTO {
	return AdminUserDTO{ID: u.ID, Username: u.Username, Email: u.Email, Role: u.Role, Active: u.Active, CreatedAt: u.CreatedAt, UpdatedAt: u.UpdatedAt}
}

type Client struct {
	ID                int64      `json:"id"`
	Username          string     `json:"username"`
	PasswordHash      string     `json:"-"`
	DisplayName       string     `json:"display_name,omitempty"`
	Email             string     `json:"email,omitempty"`
	Status            string     `json:"status"`
	TrafficLimitBytes int64      `json:"traffic_limit_bytes"`
	ExpiryTime        *time.Time `json:"expiry_time,omitempty"`
	SubscriptionToken string     `json:"-"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type ClientDTO struct {
	ID                int64      `json:"id"`
	Username          string     `json:"username"`
	DisplayName       string     `json:"display_name,omitempty"`
	Email             string     `json:"email,omitempty"`
	Status            string     `json:"status"`
	TrafficLimitBytes int64      `json:"traffic_limit_bytes"`
	ExpiryTime        *time.Time `json:"expiry_time,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (c Client) Validate() error {
	if c.Username == "" || c.PasswordHash == "" || c.Status == "" || c.SubscriptionToken == "" {
		return errors.New("invalid client")
	}
	return nil
}

func (c Client) ToDTO() ClientDTO {
	return ClientDTO{ID: c.ID, Username: c.Username, DisplayName: c.DisplayName, Email: c.Email, Status: c.Status, TrafficLimitBytes: c.TrafficLimitBytes, ExpiryTime: c.ExpiryTime, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

type Panel struct {
	ID                int64      `json:"id"`
	Name              string     `json:"name"`
	BaseURL           string     `json:"base_url"`
	Username          string     `json:"username"`
	EncryptedPassword string     `json:"-"`
	Version           string     `json:"version,omitempty"`
	Status            string     `json:"status"`
	LastSyncAt        *time.Time `json:"last_sync_at,omitempty"`
	LastError         string     `json:"last_error,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (p Panel) Validate() error {
	if p.Name == "" || p.BaseURL == "" || p.Username == "" || p.EncryptedPassword == "" || !IsValidPanelStatus(p.Status) {
		return errors.New("invalid panel")
	}
	return nil
}

type Inbound struct {
	ID              int64      `json:"id"`
	PanelID         int64      `json:"panel_id"`
	RemoteInboundID int64      `json:"remote_inbound_id"`
	Remark          string     `json:"remark,omitempty"`
	Protocol        string     `json:"protocol,omitempty"`
	Port            int        `json:"port,omitempty"`
	Network         string     `json:"network,omitempty"`
	Security        string     `json:"security,omitempty"`
	Enabled         bool       `json:"enabled"`
	Stale           bool       `json:"stale"`
	RawJSON         string     `json:"raw_json,omitempty"`
	LastSyncedAt    *time.Time `json:"last_synced_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

type ClientAttachment struct {
	ID                int64      `json:"id"`
	ClientID          int64      `json:"client_id"`
	PanelID           int64      `json:"panel_id"`
	InboundID         int64      `json:"inbound_id"`
	RemoteClientID    string     `json:"remote_client_id,omitempty"`
	RemoteEmail       string     `json:"remote_email,omitempty"`
	Enabled           bool       `json:"enabled"`
	UploadBytes       int64      `json:"upload_bytes"`
	DownloadBytes     int64      `json:"download_bytes"`
	TrafficLimitBytes int64      `json:"traffic_limit_bytes"`
	ExpiryTime        *time.Time `json:"expiry_time,omitempty"`
	RawConfig         string     `json:"raw_config,omitempty"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type TrafficSnapshot struct {
	ID            int64     `json:"id"`
	ClientID      int64     `json:"client_id"`
	AttachmentID  *int64    `json:"attachment_id,omitempty"`
	UploadBytes   int64     `json:"upload_bytes"`
	DownloadBytes int64     `json:"download_bytes"`
	TotalBytes    int64     `json:"total_bytes"`
	CapturedAt    time.Time `json:"captured_at"`
}

type AuditLog struct {
	ID           int64     `json:"id"`
	ActorType    string    `json:"actor_type"`
	ActorID      *int64    `json:"actor_id,omitempty"`
	Action       string    `json:"action"`
	TargetType   string    `json:"target_type,omitempty"`
	TargetID     *int64    `json:"target_id,omitempty"`
	MetadataJSON string    `json:"metadata_json,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

type SyncJob struct {
	ID         int64      `json:"id"`
	PanelID    *int64     `json:"panel_id,omitempty"`
	JobType    string     `json:"job_type"`
	Status     string     `json:"status"`
	Message    string     `json:"message,omitempty"`
	StartedAt  *time.Time `json:"started_at,omitempty"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type RefreshToken struct {
	ID        int64      `json:"id"`
	ClientID  int64      `json:"client_id"`
	TokenHash string     `json:"-"`
	UserAgent string     `json:"user_agent,omitempty"`
	IPAddress string     `json:"ip_address,omitempty"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
}
