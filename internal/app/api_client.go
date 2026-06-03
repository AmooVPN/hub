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
)

type apiErrorEnvelope struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
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
	Client            models.ClientDTO `json:"client"`
	Status            string           `json:"status"`
	ExpiryTime        string           `json:"expiry_time,omitempty"`
	RemainingTime     string           `json:"remaining_time,omitempty"`
	TrafficLimitBytes int64            `json:"traffic_limit_bytes"`
	UploadBytes       int64            `json:"upload_bytes"`
	DownloadBytes     int64            `json:"download_bytes"`
	TotalBytes        int64            `json:"total_bytes"`
	RemainingBytes    int64            `json:"remaining_bytes"`
	ActiveConfigs     int              `json:"active_configs"`
	SubscriptionURL   string           `json:"subscription_url"`
	RawSubscriptionURL string          `json:"raw_subscription_url"`
	Base64URL         string           `json:"base64_subscription_url"`
	ClashURL          string           `json:"clash_url"`
	SingboxURL        string           `json:"singbox_url"`
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
	key := "api:client:" + c.IP() + ":" + req.Username
	if r.loginLocks.blocked(key) {
		return apiError(c, fiber.StatusTooManyRequests, "rate_limited", "Too many attempts. Try again later.")
	}

	client, err := r.clients.FindByUsername(c.UserContext(), req.Username)
	if err != nil || client == nil || client.Status == "disabled" || security.ComparePassword(req.Password, client.PasswordHash) != nil {
		r.loginLocks.fail(key)
		return apiError(c, fiber.StatusUnauthorized, "invalid_credentials", "Invalid username or password")
	}
	r.loginLocks.success(key)

	accessToken, refreshToken, err := r.issueClientTokens(c.UserContext(), client, c.Get("User-Agent"), c.IP())
	if err != nil {
		return err
	}
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
	if err != nil || client == nil || client.Status == "disabled" {
		return apiError(c, fiber.StatusUnauthorized, "invalid_refresh_token", "Invalid refresh token")
	}
	if err := r.refreshes.RevokeByHash(ctx, refreshHash); err != nil {
		return err
	}
	accessToken, refreshToken, err := r.issueClientTokens(ctx, client, c.Get("User-Agent"), c.IP())
	if err != nil {
		return err
	}
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
	_ = r.logAudit(ctx, "client", nil, "api_logout", "refresh_token", nil, map[string]any{"ip": c.IP()})
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
	if err != nil || client == nil || client.Status == "disabled" {
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
	return c.Status(status).JSON(apiErrorEnvelope{Error: struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}{Code: code, Message: message}})
}
