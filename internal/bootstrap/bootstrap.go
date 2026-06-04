package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	"github.com/AmooVPM/hub/internal/config"
	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/repositories"
	"github.com/AmooVPM/hub/internal/security"
)

var ErrInitialSetupCompleted = errors.New("initial setup is already complete")

func EnsureInitialAdmin(ctx context.Context, cfg *config.Config, admins repositories.AdminRepository, logger *slog.Logger) error {
	count, err := admins.Count(ctx)
	if err != nil {
		return err
	}
	if count != 0 {
		return nil
	}

	username := strings.TrimSpace(cfg.InitialAdminUsername)
	if username == "" {
		username = "admin"
	}
	password := cfg.InitialAdminPassword
	if password == "" {
		password = "change-me-now"
	}
	email := strings.TrimSpace(cfg.InitialAdminEmail)
	role := strings.TrimSpace(cfg.InitialAdminRole)
	if role == "" {
		role = "owner"
	}

	// if cfg.IsProduction() && weakPassword(cfg.InitialAdminPassword) {
	// 	return errors.New("initial admin password is too weak for production")
	// }

	hash, err := security.HashPassword(password)
	if err != nil {
		return err
	}

	admin := &models.AdminUser{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         normalizeRole(role),
		Active:       true,
	}
	if err := admin.Validate(); err != nil {
		return fmt.Errorf("invalid initial admin: %w", err)
	}
	if err := admins.Create(ctx, admin); err != nil {
		return err
	}
	logger.Info("created initial admin user", "username", admin.Username, "role", admin.Role)
	return nil
}

func CreateInitialAdmin(ctx context.Context, admins repositories.AdminRepository, username, password, email, role string) (*models.AdminUser, error) {
	count, err := admins.Count(ctx)
	if err != nil {
		return nil, err
	}
	if count != 0 {
		return nil, ErrInitialSetupCompleted
	}
	hash, err := security.HashPassword(password)
	if err != nil {
		return nil, err
	}
	admin := &models.AdminUser{
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         normalizeRole(role),
		Active:       true,
	}
	if err := admin.Validate(); err != nil {
		return nil, fmt.Errorf("invalid initial admin: %w", err)
	}
	if err := admins.Create(ctx, admin); err != nil {
		return nil, err
	}
	return admin, nil
}

func weakPassword(password string) bool {
	if utf8.RuneCountInString(password) < 12 {
		return true
	}
	for _, weak := range []string{"change-me-now", "password", "admin123", "admin", "changeme", "12345678"} {
		if strings.EqualFold(password, weak) {
			return true
		}
	}
	return false
}

func normalizeRole(role string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "owner", "admin", "support", "readonly":
		return strings.ToLower(strings.TrimSpace(role))
	default:
		return "owner"
	}
}
