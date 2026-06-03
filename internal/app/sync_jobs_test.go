package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/AmooVPM/hub/internal/models"
)

func TestRetrySyncJobRejectsNonFailedJobs(t *testing.T) {
	runner := &Runner{}
	err := runner.retrySyncJob(context.Background(), &models.SyncJob{Status: "success", JobType: "traffic_sync"})
	if err == nil || err.Error() != "job must be failed before retry" {
		t.Fatalf("expected failed-job guard, got %v", err)
	}
}

func TestRenderSyncJobDetailPageShowsRetryButtonForFailedJobs(t *testing.T) {
	when := time.Now().UTC()
	page := renderSyncJobDetailPage(&syncJobListRow{ID: 7, PanelName: "panel-1", JobType: "traffic_sync", Status: "failed", Message: "boom", When: when}, "hub", "admin")
	if !strings.Contains(page, `/admin/sync-jobs/7/retry`) {
		t.Fatal("expected retry button in failed job detail page")
	}
	if strings.Contains(renderSyncJobDetailPage(&syncJobListRow{ID: 7, PanelName: "panel-1", JobType: "traffic_sync", Status: "success", Message: "ok", When: when}, "hub", "admin"), `/admin/sync-jobs/7/retry`) {
		t.Fatal("did not expect retry button for successful job")
	}
}

func TestRenderDashboardWidgetsShowsRunningSyncJobs(t *testing.T) {
	page := renderDashboardWidgets(dashboardSummary{RunningSyncJobs: 3})
	if !strings.Contains(page, "Running sync jobs") || !strings.Contains(page, ">3<") {
		t.Fatal("expected running sync jobs widget")
	}
}
