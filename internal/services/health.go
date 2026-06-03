package services

import (
	"context"
	"errors"
	"time"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/security"
	"github.com/AmooVPM/hub/internal/xui"
)

type PanelHealthResult struct {
	Version string
	Status  string
	Message string
}

type PanelHealthProbe interface {
	Probe(context.Context, *models.Panel) (PanelHealthResult, error)
}

type HealthService struct {
	probe PanelHealthProbe
}

func NewHealthService(probe PanelHealthProbe) *HealthService {
	return &HealthService{probe: probe}
}

func (s *HealthService) Check(ctx context.Context, panel *models.Panel) (PanelHealthResult, error) {
	if s == nil || s.probe == nil {
		return PanelHealthResult{}, errors.New("health service is not configured")
	}
	if panel == nil {
		return PanelHealthResult{}, errors.New("panel is nil")
	}
	return s.probe.Probe(ctx, panel)
}

type XUIHealthProbe struct {
	AppName string
	Secret  string
}

func NewXUIHealthProbe(appName, secret string) *XUIHealthProbe {
	return &XUIHealthProbe{AppName: appName, Secret: secret}
}

func (p *XUIHealthProbe) Probe(ctx context.Context, panel *models.Panel) (PanelHealthResult, error) {
	if panel == nil {
		return PanelHealthResult{}, errors.New("panel is nil")
	}
	password, err := security.Decrypt(panel.EncryptedPassword, p.Secret)
	if err != nil {
		return PanelHealthResult{Status: models.PanelStatusError, Message: err.Error()}, err
	}
	client := xui.NewClient(panel.ID, panel.BaseURL, panel.Username, password)
	client.SetUserAgent(p.AppName)
	loginCtx, cancelLogin := context.WithTimeout(ctx, 10*time.Second)
	defer cancelLogin()
	if err := client.Login(loginCtx); err != nil {
		return PanelHealthResult{Status: classifyHealthStatus(err), Message: err.Error()}, err
	}
	listCtx, cancelList := context.WithTimeout(ctx, 20*time.Second)
	defer cancelList()
	if _, err := client.ListInbounds(listCtx); err != nil {
		return PanelHealthResult{Status: classifyHealthStatus(err), Message: err.Error()}, err
	}
	compat := client.Compatibility()
	return PanelHealthResult{Version: compat.Version, Status: models.PanelStatusOnline, Message: ""}, nil
}

func classifyHealthStatus(err error) string {
	if errors.Is(err, xui.ErrXUIUnauthorized) || errors.Is(err, xui.ErrXUIForbidden) {
		return models.PanelStatusAuthError
	}
	if errors.Is(err, xui.ErrXUITimeout) || errors.Is(err, xui.ErrXUIUnavailable) {
		return models.PanelStatusOffline
	}
	if errors.Is(err, xui.ErrXUINotFound) {
		return models.PanelStatusDegraded
	}
	return models.PanelStatusError
}
