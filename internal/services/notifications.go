package services

import (
	"context"
	"errors"
	"time"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/repositories"
)

type NotificationService struct {
	repo repositories.NotificationRepository
}

func NewNotificationService(repo repositories.NotificationRepository) *NotificationService {
	return &NotificationService{repo: repo}
}

func (s *NotificationService) Create(ctx context.Context, notification *models.Notification) error {
	if s == nil || s.repo == nil {
		return errors.New("notification service is not configured")
	}
	if notification == nil {
		return errors.New("notification is nil")
	}
	if notification.CreatedAt.IsZero() {
		notification.CreatedAt = time.Now().UTC()
	}
	return s.repo.Create(ctx, notification)
}

func (s *NotificationService) Notify(ctx context.Context, typ, severity, title, message string) error {
	return s.Create(ctx, &models.Notification{Type: typ, Severity: severity, Title: title, Message: message, CreatedAt: time.Now().UTC()})
}

func (s *NotificationService) ListRecent(ctx context.Context, limit int) ([]models.Notification, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("notification service is not configured")
	}
	return s.repo.ListRecent(ctx, limit)
}

func (s *NotificationService) CountUnread(ctx context.Context) (int64, error) {
	if s == nil || s.repo == nil {
		return 0, errors.New("notification service is not configured")
	}
	return s.repo.CountUnread(ctx)
}

func (s *NotificationService) MarkRead(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return errors.New("notification service is not configured")
	}
	return s.repo.MarkRead(ctx, id)
}

func (s *NotificationService) MarkAllRead(ctx context.Context) error {
	if s == nil || s.repo == nil {
		return errors.New("notification service is not configured")
	}
	return s.repo.MarkAllRead(ctx)
}
