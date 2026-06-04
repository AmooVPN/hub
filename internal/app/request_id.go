package app

import (
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/models"
	"github.com/AmooVPN/hub/internal/services"
)

func (r *Runner) requestIDMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		requestID := c.Get("X-Request-ID")
		if requestID == "" {
			requestID = newRequestID()
		}
		correlationID := c.Get("X-Correlation-ID")
		if correlationID == "" {
			correlationID = requestID
		}
		c.Set("X-Request-ID", requestID)
		c.Set("X-Correlation-ID", correlationID)
		c.Locals("request_id", requestID)
		c.Locals("correlation_id", correlationID)
		c.SetUserContext(services.WithRequestID(c.UserContext(), requestID))
		return c.Next()
	}
}

func (r *Runner) clientAPILogMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		if r.logger != nil {
			fields := []any{"request_id", c.Locals("request_id"), "correlation_id", c.Locals("correlation_id"), "method", c.Method(), "path", c.Path(), "status", c.Response().StatusCode(), "duration_ms", time.Since(start).Milliseconds(), "ip", requestClientIP(c, r.cfg.TrustProxy)}
			if client, ok := c.Locals("client").(*models.Client); ok && client != nil {
				fields = append(fields, "client_id", client.ID, "client_username", client.Username)
			}
			if err != nil || c.Response().StatusCode() >= 400 {
				r.logger.Warn("client api request", fields...)
			} else {
				r.logger.Info("client api request", fields...)
			}
		}
		return err
	}
}

func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return hex.EncodeToString([]byte(time.Now().UTC().Format(time.RFC3339Nano)))
	}
	return hex.EncodeToString(buf[:])
}
