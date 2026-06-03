package services

import (
	"context"
	"errors"
	"testing"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/xui"
)

type fakeHealthProbe struct {
	result PanelHealthResult
	err    error
}

func (f fakeHealthProbe) Probe(context.Context, *models.Panel) (PanelHealthResult, error) {
	return f.result, f.err
}

func TestHealthServiceCheck(t *testing.T) {
	service := NewHealthService(fakeHealthProbe{result: PanelHealthResult{Status: models.PanelStatusOnline, Version: "1.0.0"}})
	result, err := service.Check(context.Background(), &models.Panel{})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if result.Status != models.PanelStatusOnline || result.Version != "1.0.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestHealthServiceRejectsMissingConfiguration(t *testing.T) {
	if _, err := NewHealthService(nil).Check(context.Background(), &models.Panel{}); err == nil {
		t.Fatal("expected missing config error")
	}
}

func TestClassifyHealthStatus(t *testing.T) {
	cases := map[error]string{
		xui.ErrXUIUnauthorized: models.PanelStatusAuthError,
		xui.ErrXUIUnavailable:  models.PanelStatusOffline,
		xui.ErrXUINotFound:     models.PanelStatusDegraded,
		errors.New("boom"):     models.PanelStatusError,
	}
	for err, want := range cases {
		if got := classifyHealthStatus(err); got != want {
			t.Fatalf("classifyHealthStatus(%v) = %s, want %s", err, got, want)
		}
	}
}
