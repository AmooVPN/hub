package services

import (
	"context"
	"errors"
	"time"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/repositories"
	"github.com/AmooVPN/hub/internal/security"
)

type ClientService struct {
	repo repositories.ClientRepository
}

func NewClientService(repo repositories.ClientRepository) *ClientService {
	return &ClientService{repo: repo}
}

func (s *ClientService) Create(ctx context.Context, client *models.Client, password string) error {
	if s == nil || s.repo == nil {
		return errors.New("client service is not configured")
	}
	if client == nil {
		return errors.New("client is nil")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}
	if client.SubscriptionToken == "" {
		token, err := security.RandomToken(32)
		if err != nil {
			return err
		}
		client.SubscriptionToken = token
	}
	client.PasswordHash = hash
	if client.Status == "" {
		client.Status = "active"
	}
	if client.CreatedAt.IsZero() {
		client.CreatedAt = time.Now().UTC()
	}
	if client.UpdatedAt.IsZero() {
		client.UpdatedAt = client.CreatedAt
	}
	if err := client.Validate(); err != nil {
		return err
	}
	return s.repo.Create(ctx, client)
}

func (s *ClientService) Get(ctx context.Context, id int64) (*models.Client, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("client service is not configured")
	}
	return s.repo.FindByID(ctx, id)
}

func (s *ClientService) Update(ctx context.Context, client *models.Client) error {
	if s == nil || s.repo == nil {
		return errors.New("client service is not configured")
	}
	if client == nil {
		return errors.New("client is nil")
	}
	if client.UpdatedAt.IsZero() {
		client.UpdatedAt = time.Now().UTC()
	}
	if err := client.Validate(); err != nil {
		return err
	}
	return s.repo.Update(ctx, client)
}

func (s *ClientService) Delete(ctx context.Context, id int64) error {
	if s == nil || s.repo == nil {
		return errors.New("client service is not configured")
	}
	return s.repo.Delete(ctx, id)
}

func (s *ClientService) SetStatus(ctx context.Context, id int64, status string) error {
	if s == nil || s.repo == nil {
		return errors.New("client service is not configured")
	}
	return s.repo.UpdateStatus(ctx, id, status)
}

func (s *ClientService) ResetPassword(ctx context.Context, id int64, password string) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("client service is not configured")
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return "", err
	}
	return password, s.repo.UpdatePassword(ctx, id, hash)
}

func (s *ClientService) RegenerateToken(ctx context.Context, id int64) (string, error) {
	if s == nil || s.repo == nil {
		return "", errors.New("client service is not configured")
	}
	client, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return "", err
	}
	token, err := security.RandomToken(32)
	if err != nil {
		return "", err
	}
	if err := s.repo.UpdateSubscriptionToken(ctx, id, token); err != nil {
		return "", err
	}
	if client != nil {
		client.SubscriptionToken = token
	}
	return token, nil
}
