package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

func TestRepositoryUniqueConstraints(t *testing.T) {
	db := openTestDatabase(t)
	adminRepo := NewAdminRepository(db)
	clientRepo := NewClientRepository(db)
	panelRepo := NewPanelRepository(db)

	admin := &models.AdminUser{Username: "admin-1", PasswordHash: "hash", Role: "owner", Active: true}
	if err := adminRepo.Create(context.Background(), admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if err := adminRepo.Create(context.Background(), &models.AdminUser{Username: "admin-1", PasswordHash: "hash", Role: "owner", Active: true}); err == nil {
		t.Fatal("expected duplicate admin username error")
	}

	client := &models.Client{Username: "client-1", PasswordHash: "hash", Status: "active", SubscriptionToken: "token-1"}
	if err := clientRepo.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := clientRepo.Create(context.Background(), &models.Client{Username: "client-1", PasswordHash: "hash", Status: "active", SubscriptionToken: "token-2"}); err == nil {
		t.Fatal("expected duplicate client username error")
	}

	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: models.PanelStatusOffline}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}
	if err := panelRepo.Create(context.Background(), &models.Panel{Name: "panel-2", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: models.PanelStatusOffline}); err == nil {
		t.Fatal("expected duplicate panel base url error")
	}
}

func TestRepositoryForeignKeyBehavior(t *testing.T) {
	db := openTestDatabase(t)
	panelRepo := NewPanelRepository(db)
	panel := &models.Panel{Name: "panel-1", BaseURL: "https://panel.example", Username: "admin", EncryptedPassword: "enc", Status: models.PanelStatusOffline}
	if err := panelRepo.Create(context.Background(), panel); err != nil {
		t.Fatalf("create panel: %v", err)
	}
	inboundRepo := NewInboundRepository(db)
	inbound := &models.Inbound{PanelID: panel.ID, RemoteInboundID: 1, Remark: "inbound-1", Protocol: "vless", Enabled: true, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	if err := inboundRepo.Upsert(context.Background(), inbound); err != nil {
		t.Fatalf("create inbound: %v", err)
	}
	var inboundID int64
	if err := db.QueryRowContext(context.Background(), `SELECT id FROM inbounds WHERE panel_id = ? AND remote_inbound_id = ?`, panel.ID, inbound.RemoteInboundID).Scan(&inboundID); err != nil {
		t.Fatalf("load inbound: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `INSERT INTO client_attachments (client_id, panel_id, inbound_id, enabled, upload_bytes, download_bytes, traffic_limit_bytes, created_at, updated_at) VALUES (?, ?, ?, 1, 0, 0, 0, ?, ?)`, 9999, panel.ID, inboundID, time.Now().UTC(), time.Now().UTC()); err == nil {
		t.Fatal("expected foreign key error")
	}
}

func TestRepositoryTransactionRollback(t *testing.T) {
	db := openTestDatabase(t)
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("begin tx: %v", err)
	}
	_, err = tx.ExecContext(context.Background(), `INSERT INTO admin_users (username, email, password_hash, role, active, created_at, updated_at) VALUES (?, ?, ?, ?, 1, ?, ?)`, "tx-admin", "", "hash", "owner", time.Now().UTC(), time.Now().UTC())
	if err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert in tx: %v", err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	var count int64
	if err := db.QueryRowContext(context.Background(), `SELECT COUNT(*) FROM admin_users WHERE username = ?`, "tx-admin").Scan(&count); err != nil {
		t.Fatalf("count admin: %v", err)
	}
	if count != 0 {
		t.Fatal("expected rolled back transaction to leave no rows")
	}
}
