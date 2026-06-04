package repositories

import (
	"context"
	"testing"

	"github.com/AmooVPN/hub/internal/models"
)

func TestClientRepositoryUpdateSubscriptionToken(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewClientRepository(db)
	client := &models.Client{Username: "client-1", PasswordHash: "hash", Status: "active", SubscriptionToken: "old-token"}
	if err := repo.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := repo.UpdateSubscriptionToken(context.Background(), client.ID, "new-token"); err != nil {
		t.Fatalf("update token: %v", err)
	}
	loaded, err := repo.FindByUsername(context.Background(), client.Username)
	if err != nil {
		t.Fatalf("load client: %v", err)
	}
	if loaded.SubscriptionToken != "new-token" {
		t.Fatalf("unexpected token: %s", loaded.SubscriptionToken)
	}
}

func TestClientRepositoryUpdateStatus(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewClientRepository(db)
	client := &models.Client{Username: "client-1", PasswordHash: "hash", Status: "active", SubscriptionToken: "old-token"}
	if err := repo.Create(context.Background(), client); err != nil {
		t.Fatalf("create client: %v", err)
	}
	if err := repo.UpdateStatus(context.Background(), client.ID, "disabled"); err != nil {
		t.Fatalf("update status: %v", err)
	}
	loaded, err := repo.FindByUsername(context.Background(), client.Username)
	if err != nil {
		t.Fatalf("load client: %v", err)
	}
	if loaded.Status != "disabled" {
		t.Fatalf("unexpected status: %s", loaded.Status)
	}
}
