package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AmooVPM/hub/internal/config"
	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/repositories"
	"github.com/AmooVPM/hub/internal/security"
)

func EnsureInitialAdmin(ctx context.Context, cfg *config.Config, admins repositories.AdminRepository, logger *slog.Logger) error {
	count, err := admins.Count(ctx)
	if err != nil {
		return err
	}
	if count != 0 {
		return nil
	}

	if cfg.IsProduction() && weakPassword(cfg.InitialAdminPassword) {
		return errors.New("initial admin password is too weak for production")
	}

	hash, err := security.HashPassword(cfg.InitialAdminPassword)
	if err != nil {
		return err
	}

	admin := &models.AdminUser{
		Username:     cfg.InitialAdminUsername,
		Email:        cfg.InitialAdminEmail,
		PasswordHash: hash,
		Role:         normalizeRole(cfg.InitialAdminRole),
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

func weakPassword(password string) bool {
	if len(password) < 12 {
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
