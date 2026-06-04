package app

import (
	"context"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

func (r *Runner) startAutomaticWebhookRetries() {
	if r == nil || r.webhooks == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		ctx := context.Background()
		for range ticker.C {
			if r.backgroundJobs != nil {
				_, err := r.backgroundJobs.Submit(ctx, models.JobTypeWebhookDelivery, nil, 0, func(jobCtx context.Context) error {
					_, err := r.webhooks.RetryDueDeliveries(jobCtx, 25)
					return err
				})
				if err != nil && r.logger != nil {
					r.logger.Warn("webhook retry enqueue failed", "error", err)
				}
				continue
			}
			if _, err := r.webhooks.RetryDueDeliveries(ctx, 25); err != nil && r.logger != nil {
				r.logger.Warn("webhook retry failed", "error", err)
			}
		}
	}()
}
