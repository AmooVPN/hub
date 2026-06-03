package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
)

func (r *Runner) loadClientBySubscriptionToken(ctx context.Context, token string) (*models.Client, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, errors.New("missing token")
	}
	row := r.db.QueryRowContext(ctx, `SELECT id, username, password_hash, display_name, email, status, traffic_limit_bytes, expiry_time, subscription_token, created_at, updated_at FROM clients WHERE subscription_token = ?`, token)
	var client models.Client
	var displayName, email, subscriptionToken sql.NullString
	var expiryTime sql.NullTime
	if err := row.Scan(&client.ID, &client.Username, &client.PasswordHash, &displayName, &email, &client.Status, &client.TrafficLimitBytes, &expiryTime, &subscriptionToken, &client.CreatedAt, &client.UpdatedAt); err != nil {
		return nil, err
	}
	client.DisplayName = displayName.String
	client.Email = email.String
	client.SubscriptionToken = subscriptionToken.String
	if expiryTime.Valid {
		t := expiryTime.Time
		client.ExpiryTime = &t
	}
	return &client, nil
}

func (r *Runner) servePublicSubscription(c *fiber.Ctx, format string) error {
	client, err := r.loadClientBySubscriptionToken(c.UserContext(), c.Params("token"))
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "subscription not found")
	}
	if client.Status == "disabled" {
		return fiber.NewError(fiber.StatusForbidden, "subscription disabled")
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	raw := buildSubscriptionRaw(summary.Configs)
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	switch strings.ToLower(format) {
	case "", "base64":
		return c.SendString(base64.StdEncoding.EncodeToString([]byte(raw)))
	case "raw":
		return c.SendString(raw)
	case "clash":
		return c.SendString("# clash subscription placeholder\n" + raw)
	case "singbox":
		return c.SendString("# sing-box subscription placeholder\n" + raw)
	default:
		return fiber.NewError(fiber.StatusNotFound, "subscription format not found")
	}
}

func buildSubscriptionRaw(configs []clientConfigRow) string {
	if len(configs) == 0 {
		return ""
	}
	var b strings.Builder
	for i, cfg := range configs {
		if strings.TrimSpace(cfg.CopyValue) == "" {
			continue
		}
		if i > 0 && b.Len() > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(strings.TrimSpace(cfg.CopyValue))
	}
	return b.String()
}
