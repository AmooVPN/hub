package services

import (
	"context"
	"database/sql"
	"testing"

	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/repositories"
)

func TestClientServiceCreateUpdateDeleteAndToken(t *testing.T) {
	db := openClientServiceDB(t)
	defer db.Close()
	if err := database.RunMigrations(db); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	repo := repositories.NewClientRepository(db)
	service := NewClientService(repo)

	client := &models.Client{Username: "client-1", Email: "a@example.com", Status: "active"}
	if err := service.Create(context.Background(), client, "password-123"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if client.ID == 0 || client.SubscriptionToken == "" || client.PasswordHash == "" {
		t.Fatalf("expected populated client, got %+v", client)
	}

	fetched, err := service.Get(context.Background(), client.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if fetched.Username != client.Username {
		t.Fatalf("unexpected fetched client: %+v", fetched)
	}

	fetched.DisplayName = "Updated"
	if err := service.Update(context.Background(), fetched); err != nil {
		t.Fatalf("update: %v", err)
	}
	if _, err := service.RegenerateToken(context.Background(), client.ID); err != nil {
		t.Fatalf("regenerate token: %v", err)
	}
	if _, err := service.ResetPassword(context.Background(), client.ID, "new-password"); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if err := service.Delete(context.Background(), client.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	deleted, err := service.Get(context.Background(), client.ID)
	if err != nil {
		t.Fatalf("get deleted: %v", err)
	}
	if deleted.Status != "deleted" {
		t.Fatalf("expected deleted status, got %+v", deleted)
	}
}

func openClientServiceDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:client-service-"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	return db
}
