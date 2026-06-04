package services

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/AmooVPN/hub/internal/models"
)

func TestNotificationServiceDispatchesMappedEvents(t *testing.T) {
	repo := &stubNotificationRepo{}
	telegram := NewTelegramNotifier("token", "chat")
	var calls int
	telegram.Client = roundTripHTTPFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"ok":true}`)), Header: http.Header{}}, nil
	})
	service := NewNotificationService(repo, telegram, nil)
	if err := service.Notify(context.Background(), models.NotificationTypePanelOffline, models.NotificationSeverityWarning, "Panel offline", "panel-1 is offline"); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected 1 stored notification, got %d", len(repo.items))
	}
	if calls != 1 {
		t.Fatalf("expected telegram dispatch, got %d calls", calls)
	}
}
func TestNotificationServiceIgnoresUnmappedEvents(t *testing.T) {
	repo := &stubNotificationRepo{}
	service := NewNotificationService(repo, nil, nil)
	if err := service.Notify(context.Background(), "custom.event", models.NotificationSeverityInfo, "Title", "Message"); err != nil {
		t.Fatalf("notify failed: %v", err)
	}
	if len(repo.items) != 1 {
		t.Fatalf("expected notification to be stored")
	}
}

type stubNotificationRepo struct{ items []models.Notification }

func (s *stubNotificationRepo) Create(_ context.Context, n *models.Notification) error {
	if n == nil {
		return errors.New("nil")
	}
	n.ID = int64(len(s.items) + 1)
	s.items = append(s.items, *n)
	return nil
}
func (s *stubNotificationRepo) ListRecent(context.Context, int) ([]models.Notification, error) {
	return append([]models.Notification(nil), s.items...), nil
}
func (s *stubNotificationRepo) CountUnread(context.Context) (int64, error) { return 0, nil }
func (s *stubNotificationRepo) FindByID(context.Context, int64) (*models.Notification, error) {
	return nil, nil
}
func (s *stubNotificationRepo) MarkRead(context.Context, int64) error { return nil }
func (s *stubNotificationRepo) MarkAllRead(context.Context) error     { return nil }
