package app

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

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
	if cached, ok := r.loadSubscriptionCache(c.UserContext(), client.SubscriptionToken, format); ok {
		c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
		return c.SendString(cached)
	}
	result := raw
	c.Set(fiber.HeaderContentType, "text/plain; charset=utf-8")
	switch strings.ToLower(format) {
	case "", "base64":
		result = base64.StdEncoding.EncodeToString([]byte(raw))
	case "raw":
		result = raw
	case "clash":
		result = "# clash subscription placeholder\n" + raw
	case "singbox":
		result = "# sing-box subscription placeholder\n" + raw
	default:
		return fiber.NewError(fiber.StatusNotFound, "subscription format not found")
	}
	_ = r.storeSubscriptionCache(c.UserContext(), client.SubscriptionToken, format, result)
	return c.SendString(result)
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

func (r *Runner) invalidateSubscriptionCache(ctx context.Context, token string) {
	if r.redis == nil || strings.TrimSpace(token) == "" {
		return
	}
	_ = r.redis.Del(ctx, subscriptionCacheKey(token, "raw"), subscriptionCacheKey(token, "base64"), subscriptionCacheKey(token, "clash"), subscriptionCacheKey(token, "singbox")).Err()
}

func (r *Runner) loadSubscriptionCache(ctx context.Context, token, format string) (string, bool) {
	if r.redis == nil {
		return "", false
	}
	value, err := r.redis.Get(ctx, subscriptionCacheKey(token, format)).Result()
	if err != nil {
		return "", false
	}
	return value, true
}

func (r *Runner) storeSubscriptionCache(ctx context.Context, token, format, value string) error {
	if r.redis == nil || strings.TrimSpace(token) == "" {
		return nil
	}
	return r.redis.Set(ctx, subscriptionCacheKey(token, format), value, 5*time.Minute).Err()
}

func subscriptionCacheKey(token, format string) string {
	return fmt.Sprintf("hub:sub:%s:%s", token, strings.ToLower(format))
}
