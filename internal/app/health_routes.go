package app

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPN/hub/internal/database"
	"github.com/AmooVPN/hub/internal/version"
)

type systemHealthSnapshot struct {
	Status     string    `json:"status"`
	App        string    `json:"app"`
	SQLite     string    `json:"sqlite"`
	Redis      string    `json:"redis"`
	Migrations string    `json:"migrations"`
	Version    string    `json:"version"`
	Time       time.Time `json:"time"`
}

func (r *Runner) getHealth(c *fiber.Ctx) error {
	ctx := c.UserContext()
	health := r.collectSystemHealth(ctx)
	return c.Status(fiber.StatusOK).JSON(health)
}

func (r *Runner) getHealthLive(c *fiber.Ctx) error {
	return c.JSON(systemHealthSnapshot{Status: "ok", App: r.cfg.AppName, Version: version.Full(), Time: time.Now().UTC()})
}

func (r *Runner) getHealthReady(c *fiber.Ctx) error {
	ctx := c.UserContext()
	health := r.collectSystemHealth(ctx)
	status := fiber.StatusOK
	if health.Status != "ok" {
		status = fiber.StatusServiceUnavailable
	}
	return c.Status(status).JSON(health)
}

func (r *Runner) collectSystemHealth(ctx context.Context) systemHealthSnapshot {
	health := systemHealthSnapshot{Status: "ok", App: r.cfg.AppName, SQLite: "ok", Redis: "ok", Migrations: "ok", Version: version.Full(), Time: time.Now().UTC()}
	if err := pingSQLite(ctx, r.db); err != nil {
		health.SQLite = err.Error()
		health.Status = "degraded"
	}
	if r.redis != nil {
		pingCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		if err := r.redis.Ping(pingCtx).Err(); err != nil {
			health.Redis = err.Error()
			health.Status = "degraded"
		}
	} else {
		health.Redis = "unavailable"
		health.Status = "degraded"
	}
	if err := database.ValidateSchemaVersion(r.db); err != nil {
		health.Migrations = err.Error()
		health.Status = "degraded"
	}
	return health
}

func pingSQLite(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return errors.New("database is not configured")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return db.PingContext(ctx)
}
