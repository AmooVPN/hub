package repositories

import (
	"context"
	"testing"

	"github.com/AmooVPN/hub/internal/models"
)

func TestAuditRepositoryListByTarget(t *testing.T) {
	db := openTestDatabase(t)
	repo := NewAuditRepository(db)
	targetID := int64(7)
	if err := repo.Create(context.Background(), &models.AuditLog{ActorType: "admin", Action: "client_update", TargetType: "client", TargetID: &targetID}); err != nil {
		t.Fatalf("create audit: %v", err)
	}
	items, err := repo.ListByTarget(context.Background(), "client", targetID, 10)
	if err != nil {
		t.Fatalf("list by target: %v", err)
	}
	if len(items) != 1 || items[0].Action != "client_update" {
		t.Fatalf("unexpected audit items: %+v", items)
	}
}
