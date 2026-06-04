package services

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/repositories"
)

type NotificationService struct {
	repo     repositories.NotificationRepository
	telegram *TelegramNotifier
	email    *EmailNotifier
}

func NewNotificationService(repo repositories.NotificationRepository, telegram *TelegramNotifier, email *EmailNotifier) *NotificationService {
	return &NotificationService{repo: repo, telegram: telegram, email: email}
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
	notification := &models.Notification{Type: typ, Severity: severity, Title: title, Message: message, CreatedAt: time.Now().UTC()}
	if err := s.Create(ctx, notification); err != nil {
		return err
	}
	return s.dispatch(ctx, notification)
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

func (s *NotificationService) dispatch(ctx context.Context, notification *models.Notification) error {
	if s == nil || notification == nil {
		return nil
	}
	if !shouldDispatchNotification(notification.Type) {
		return nil
	}
	var errs []string
	if s.telegram != nil && s.telegram.Configured() {
		if err := s.telegram.SendNotification(ctx, notification.Severity, notification.Title, notification.Message); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func shouldDispatchNotification(typ string) bool {
	switch typ {
	case models.NotificationTypePanelOffline, models.NotificationTypePanelAuthError, models.NotificationTypeSyncFailed, models.NotificationTypeBackupImportCompleted, models.NotificationTypeBackupImportFailed, models.NotificationTypeBackupExportFailed, models.NotificationTypeClientTrafficLimit, models.NotificationTypeClientExpired:
		return true
	default:
		return false
	}
}
