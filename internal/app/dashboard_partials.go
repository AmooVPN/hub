package app

import "github.com/gofiber/fiber/v2"

func (r *Runner) getAdminDashboardPanelStatus(c *fiber.Ctx) error {
	summary, err := r.loadDashboardSummary(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderDashboardMonitoring(summary))
}

func (r *Runner) getAdminDashboardSyncJobs(c *fiber.Ctx) error {
	summary, err := r.loadDashboardSummary(c.UserContext())
	if err != nil {
		return err
	}
	return c.Type("html").SendString(renderDashboardSyncJobs(summary.RecentSyncJobs))
}
