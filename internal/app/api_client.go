package app

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/security"
	"github.com/AmooVPM/hub/internal/services"
)

type apiErrorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id,omitempty"`
	} `json:"error"`
}

type clientAPILoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type clientAPIRefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type clientAPIAuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
}

type clientAPIMeResponse struct {
	Client             models.ClientDTO `json:"client"`
	Status             string           `json:"status"`
	ExpiryTime         string           `json:"expiry_time,omitempty"`
	RemainingTime      string           `json:"remaining_time,omitempty"`
	TrafficLimitBytes  int64            `json:"traffic_limit_bytes"`
	UploadBytes        int64            `json:"upload_bytes"`
	DownloadBytes      int64            `json:"download_bytes"`
	TotalBytes         int64            `json:"total_bytes"`
	RemainingBytes     int64            `json:"remaining_bytes"`
	ActiveConfigs      int              `json:"active_configs"`
	SubscriptionURL    string           `json:"subscription_url"`
	RawSubscriptionURL string           `json:"raw_subscription_url"`
	Base64URL          string           `json:"base64_subscription_url"`
	ClashURL           string           `json:"clash_url"`
	SingboxURL         string           `json:"singbox_url"`
}

type clientAPISubscriptionResponse struct {
	SubscriptionURL string            `json:"subscription_url"`
	Formats         map[string]string `json:"formats"`
	Active          bool              `json:"active"`
}

type clientAPIConfigItem struct {
	ID            int64  `json:"id"`
	PanelName     string `json:"panel_name"`
	InboundRemark string `json:"inbound_remark"`
	Protocol      string `json:"protocol"`
	Enabled       bool   `json:"enabled"`
	Config        string `json:"config"`
}

type clientAPIConfigsResponse struct {
	Configs []clientAPIConfigItem `json:"configs"`
}

type clientAPIUsageResponse struct {
	UploadBytes           int64 `json:"upload_bytes"`
	DownloadBytes         int64 `json:"download_bytes"`
	TotalBytes            int64 `json:"total_bytes"`
	TrafficLimitBytes     int64 `json:"traffic_limit_bytes"`
	RemainingTrafficBytes int64 `json:"remaining_traffic_bytes"`
}

type clientAPIStatusResponse struct {
	Status                string `json:"status"`
	IsActive              bool   `json:"is_active"`
	IsExpired             bool   `json:"is_expired"`
	ExpiryTime            string `json:"expiry_time,omitempty"`
	RemainingSeconds      int64  `json:"remaining_seconds"`
	RemainingDays         int64  `json:"remaining_days"`
	TrafficLimitBytes     int64  `json:"traffic_limit_bytes"`
	UsedTrafficBytes      int64  `json:"used_traffic_bytes"`
	RemainingTrafficBytes int64  `json:"remaining_traffic_bytes"`
}

func (r *Runner) postClientAPILogin(c *fiber.Ctx) error {
	var req clientAPILoginRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return apiError(c, fiber.StatusBadRequest, "invalid_request", "Invalid request body")
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" || req.Password == "" {
		return apiError(c, fiber.StatusBadRequest, "invalid_request", "Username and password are required")
	}
	ip := requestClientIP(c, r.cfg.TrustProxy)
	key := "api:client:" + ip + ":" + req.Username
	if r.loginLocks.blocked(key) {
		_ = r.logAudit(c.UserContext(), "client", nil, "api_login_rate_limited", "client", nil, map[string]any{"username": req.Username, "ip": ip, "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
		return apiError(c, fiber.StatusTooManyRequests, "rate_limited", "Too many attempts. Try again later.")
	}

	client, err := r.clients.FindByUsername(c.UserContext(), req.Username)
	if err != nil || client == nil || client.Status == "disabled" || client.Status == "deleted" || security.ComparePassword(req.Password, client.PasswordHash) != nil {
		r.loginLocks.fail(key)
		_ = r.logAudit(c.UserContext(), "client", nil, "api_login_failed", "client", nil, map[string]any{"username": req.Username, "ip": ip, "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
		return apiError(c, fiber.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
	}
	r.loginLocks.success(key)

	accessToken, refreshToken, err := r.issueClientTokens(c.UserContext(), client, c.Get("User-Agent"), ip)
	if err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "client", &client.ID, "api_login_success", "client", &client.ID, map[string]any{"username": client.Username, "ip": ip, "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
	return c.Status(fiber.StatusOK).JSON(clientAPIAuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int((15 * time.Minute).Seconds()),
	})
}

func (r *Runner) postClientAPIRefresh(c *fiber.Ctx) error {
	var req clientAPIRefreshRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return apiError(c, fiber.StatusBadRequest, "invalid_request", "Invalid request body")
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return apiError(c, fiber.StatusBadRequest, "invalid_request", "Refresh token is required")
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()
	refreshHash := security.HashRefreshToken(req.RefreshToken)
	stored, err := r.refreshes.FindActiveByHash(ctx, refreshHash)
	if err != nil {
		return apiError(c, fiber.StatusUnauthorized, "invalid_refresh_token", "Invalid refresh token")
	}
	client, err := r.loadClientByID(ctx, stored.ClientID)
	if err != nil || client == nil || client.Status == "disabled" || client.Status == "deleted" {
		return apiError(c, fiber.StatusUnauthorized, "invalid_refresh_token", "Invalid refresh token")
	}
	if err := r.refreshes.RevokeByHash(ctx, refreshHash); err != nil {
		return err
	}
	accessToken, refreshToken, err := r.issueClientTokens(ctx, client, c.Get("User-Agent"), requestClientIP(c, r.cfg.TrustProxy))
	if err != nil {
		return err
	}
	_ = r.logAudit(ctx, "client", &client.ID, "api_refresh", "refresh_token", nil, map[string]any{"ip": requestClientIP(c, r.cfg.TrustProxy), "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
	return c.Status(fiber.StatusOK).JSON(clientAPIAuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int((15 * time.Minute).Seconds()),
	})
}

func (r *Runner) postClientAPILogout(c *fiber.Ctx) error {
	var req clientAPIRefreshRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return apiError(c, fiber.StatusBadRequest, "invalid_request", "Invalid request body")
	}
	req.RefreshToken = strings.TrimSpace(req.RefreshToken)
	if req.RefreshToken == "" {
		return apiError(c, fiber.StatusBadRequest, "invalid_request", "Refresh token is required")
	}
	ctx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
	defer cancel()
	if err := r.refreshes.RevokeByHash(ctx, security.HashRefreshToken(req.RefreshToken)); err != nil {
		return err
	}
	_ = r.logAudit(ctx, "client", nil, "api_logout", "refresh_token", nil, map[string]any{"ip": requestClientIP(c, r.cfg.TrustProxy), "proto": requestForwardedProto(c, r.cfg.TrustProxy), "host": requestForwardedHost(c, r.cfg.TrustProxy)})
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

func (r *Runner) getClientAPIMe(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Authorization required")
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "client", &client.ID, "api_me", "client", &client.ID, nil)
	return c.Status(fiber.StatusOK).JSON(clientAPIMeResponse{
		Client:             client.ToDTO(),
		Status:             summary.StatusText,
		ExpiryTime:         summary.ExpiryText,
		RemainingTime:      summary.RemainingText,
		TrafficLimitBytes:  client.TrafficLimitBytes,
		UploadBytes:        summary.UploadBytes,
		DownloadBytes:      summary.DownloadBytes,
		TotalBytes:         summary.TotalBytes,
		RemainingBytes:     summary.RemainingBytes,
		ActiveConfigs:      summary.ActiveConfigs,
		SubscriptionURL:    summary.SubscriptionURL,
		RawSubscriptionURL: summary.RawSubscriptionURL,
		Base64URL:          summary.Base64URL,
		ClashURL:           summary.ClashURL,
		SingboxURL:         summary.SingboxURL,
	})
}

func (r *Runner) getClientAPISubscription(c *fiber.Ctx) error {
	if r.metrics != nil {
		r.metrics.IncSubscriptionRequest()
	}
	client, ok := currentClient(c)
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Authorization required")
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "client", &client.ID, "api_subscription", "client", &client.ID, nil)
	return c.Status(fiber.StatusOK).JSON(clientAPISubscriptionResponse{
		SubscriptionURL: summary.SubscriptionURL,
		Formats: map[string]string{
			"raw":     summary.RawSubscriptionURL,
			"base64":  summary.Base64URL,
			"clash":   summary.ClashURL,
			"singbox": summary.SingboxURL,
		},
		Active: summary.StatusText == "active",
	})
}

func (r *Runner) getClientAPIConfigs(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Authorization required")
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	items := make([]clientAPIConfigItem, 0, len(summary.Configs))
	for i, cfg := range summary.Configs {
		items = append(items, clientAPIConfigItem{
			ID:            int64(i + 1),
			PanelName:     cfg.PanelName,
			InboundRemark: cfg.InboundRemark,
			Protocol:      cfg.Protocol,
			Enabled:       cfg.Enabled,
			Config:        cfg.CopyValue,
		})
	}
	_ = r.logAudit(c.UserContext(), "client", &client.ID, "api_configs", "client", &client.ID, map[string]any{"configs": len(items)})
	return c.Status(fiber.StatusOK).JSON(clientAPIConfigsResponse{Configs: items})
}

func (r *Runner) getClientAPIUsage(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Authorization required")
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	_ = r.logAudit(c.UserContext(), "client", &client.ID, "api_usage", "client", &client.ID, map[string]any{"total_bytes": summary.TotalBytes})
	return c.Status(fiber.StatusOK).JSON(clientAPIUsageResponse{
		UploadBytes:           summary.UploadBytes,
		DownloadBytes:         summary.DownloadBytes,
		TotalBytes:            summary.TotalBytes,
		TrafficLimitBytes:     client.TrafficLimitBytes,
		RemainingTrafficBytes: summary.RemainingBytes,
	})
}

func (r *Runner) getClientAPIStatus(c *fiber.Ctx) error {
	client, ok := currentClient(c)
	if !ok {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Authorization required")
	}
	summary, err := r.loadClientSummary(c.UserContext(), client)
	if err != nil {
		return err
	}
	remainingSeconds, remainingDays := int64(0), int64(0)
	_ = r.logAudit(c.UserContext(), "client", &client.ID, "api_status", "client", &client.ID, map[string]any{"status": summary.StatusText})
	isExpired := summary.StatusText == "expired"
	if client.ExpiryTime != nil {
		remaining := time.Until(*client.ExpiryTime)
		if remaining > 0 {
			remainingSeconds = int64(remaining.Seconds())
			remainingDays = int64(remaining.Hours() / 24)
		}
	}
	return c.Status(fiber.StatusOK).JSON(clientAPIStatusResponse{
		Status:                summary.StatusText,
		IsActive:              summary.StatusText == "active",
		IsExpired:             isExpired,
		ExpiryTime:            summary.ExpiryText,
		RemainingSeconds:      remainingSeconds,
		RemainingDays:         remainingDays,
		TrafficLimitBytes:     client.TrafficLimitBytes,
		UsedTrafficBytes:      summary.TotalBytes,
		RemainingTrafficBytes: summary.RemainingBytes,
	})
}

func (r *Runner) requireClientAPIJWT(c *fiber.Ctx) error {
	client, err := r.loadClientFromBearer(c)
	if err != nil {
		return apiError(c, fiber.StatusUnauthorized, "unauthorized", "Authorization required")
	}
	c.Locals("client", client)
	return c.Next()
}

func (r *Runner) loadClientFromBearer(c *fiber.Ctx) (*models.Client, error) {
	authorization := strings.TrimSpace(c.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(authorization), "bearer ") {
		return nil, errors.New("missing bearer token")
	}
	token := strings.TrimSpace(authorization[7:])
	claims, err := security.ValidateAccessToken(token, r.cfg.HUBSecretKey)
	if err != nil {
		return nil, err
	}
	clientID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return nil, err
	}
	client, err := r.loadClientByID(c.UserContext(), clientID)
	if err != nil || client == nil || client.Status == "disabled" || client.Status == "deleted" {
		return nil, errors.New("client not found")
	}
	return client, nil
}

func (r *Runner) issueClientTokens(ctx context.Context, client *models.Client, userAgent, ipAddress string) (string, string, error) {
	accessToken, err := security.SignAccessToken(strconv.FormatInt(client.ID, 10), 15*time.Minute, r.cfg.HUBSecretKey)
	if err != nil {
		return "", "", err
	}
	rawRefresh, err := security.GenerateRefreshToken()
	if err != nil {
		return "", "", err
	}
	refresh := &models.RefreshToken{
		ClientID:  client.ID,
		TokenHash: security.HashRefreshToken(rawRefresh),
		UserAgent: userAgent,
		IPAddress: ipAddress,
		ExpiresAt: time.Now().UTC().Add(30 * 24 * time.Hour),
	}
	if err := r.refreshes.Create(ctx, refresh); err != nil {
		return "", "", err
	}
	return accessToken, rawRefresh, nil
}

func apiError(c *fiber.Ctx, status int, code, message string) error {
	envelope := apiErrorEnvelope{}
	envelope.Error.Code = code
	envelope.Error.Message = message
	envelope.Error.RequestID = services.RequestIDFromContext(c.UserContext())
	if envelope.Error.RequestID != "" {
		c.Set("X-Request-ID", envelope.Error.RequestID)
	}
	return c.Status(status).JSON(envelope)
}
