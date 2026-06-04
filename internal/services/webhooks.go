package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/AmooVPM/hub/internal/models"
	"github.com/AmooVPM/hub/internal/repositories"
)

type WebhookEnvelope struct {
	Event     string    `json:"event"`
	Delivery  string    `json:"delivery"`
	Timestamp time.Time `json:"timestamp"`
	Payload   any       `json:"payload"`
}

type WebhookDeliveryStatus string

const (
	WebhookDeliveryQueued   WebhookDeliveryStatus = "queued"
	WebhookDeliveryRunning  WebhookDeliveryStatus = "running"
	WebhookDeliverySuccess  WebhookDeliveryStatus = "success"
	WebhookDeliveryFailed   WebhookDeliveryStatus = "failed"
	WebhookDeliveryRetrying WebhookDeliveryStatus = "retrying"
)

type WebhookHTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type WebhookService struct {
	webhooks   repositories.WebhookRepository
	deliveries repositories.WebhookDeliveryRepository
	client     WebhookHTTPClient
	now        func() time.Time
}

const defaultWebhookRetryDelay = 5 * time.Minute

func NewWebhookService(webhooks repositories.WebhookRepository, deliveries repositories.WebhookDeliveryRepository) *WebhookService {
	return &WebhookService{webhooks: webhooks, deliveries: deliveries, client: http.DefaultClient, now: time.Now}
}

func SignWebhookPayload(secret, event, delivery string, timestamp time.Time, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(event))
	mac.Write([]byte("."))
	mac.Write([]byte(delivery))
	mac.Write([]byte("."))
	mac.Write([]byte(fmt.Sprintf("%d.", timestamp.UTC().Unix())))
	mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func NewWebhookEnvelope(event, delivery string, payload any) WebhookEnvelope {
	return WebhookEnvelope{Event: event, Delivery: delivery, Timestamp: time.Now().UTC(), Payload: payload}
}

func IsSupportedWebhookEvent(event string) bool { return models.IsValidWebhookEvent(event) }

func (s *WebhookService) Publish(ctx context.Context, event string, payload any) ([]models.WebhookDelivery, error) {
	if s == nil || s.webhooks == nil || s.deliveries == nil {
		return nil, errors.New("webhook service is not configured")
	}
	if !models.IsValidWebhookEvent(event) {
		return nil, fmt.Errorf("invalid webhook event %q", event)
	}
	webhooks, err := s.webhooks.ListActiveByEvent(ctx, event)
	if err != nil {
		return nil, err
	}
	deliveries := make([]models.WebhookDelivery, 0, len(webhooks))
	for i := range webhooks {
		delivery, err := s.DeliverToWebhook(ctx, webhooks[i], event, payload)
		if err != nil {
			return nil, err
		}
		deliveries = append(deliveries, delivery)
	}
	return deliveries, nil
}

func (s *WebhookService) RetryDueDeliveries(ctx context.Context, limit int) (int, error) {
	if s == nil || s.webhooks == nil || s.deliveries == nil {
		return 0, errors.New("webhook service is not configured")
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	due, err := s.deliveries.ListDueForRetry(ctx, now().UTC(), limit)
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range due {
		delivery := due[i]
		webhook, err := s.webhooks.FindByID(ctx, delivery.WebhookID)
		if err != nil {
			continue
		}
		if err := s.retryDelivery(ctx, webhook, &delivery); err != nil {
			continue
		}
		count++
	}
	return count, nil
}

func (s *WebhookService) ListWebhooks(ctx context.Context) ([]models.Webhook, error) {
	if s == nil || s.webhooks == nil {
		return nil, errors.New("webhook service is not configured")
	}
	return s.webhooks.List(ctx)
}

func (s *WebhookService) GetWebhook(ctx context.Context, id int64) (*models.Webhook, error) {
	if s == nil || s.webhooks == nil {
		return nil, errors.New("webhook service is not configured")
	}
	return s.webhooks.FindByID(ctx, id)
}

func (s *WebhookService) CreateWebhook(ctx context.Context, webhook *models.Webhook) error {
	if s == nil || s.webhooks == nil {
		return errors.New("webhook service is not configured")
	}
	return s.webhooks.Create(ctx, webhook)
}

func (s *WebhookService) UpdateWebhook(ctx context.Context, webhook *models.Webhook) error {
	if s == nil || s.webhooks == nil {
		return errors.New("webhook service is not configured")
	}
	return s.webhooks.Update(ctx, webhook)
}

func (s *WebhookService) DeleteWebhook(ctx context.Context, id int64) error {
	if s == nil || s.webhooks == nil {
		return errors.New("webhook service is not configured")
	}
	return s.webhooks.Delete(ctx, id)
}

func (s *WebhookService) ListDeliveries(ctx context.Context, webhookID int64) ([]models.WebhookDelivery, error) {
	if s == nil || s.deliveries == nil {
		return nil, errors.New("webhook service is not configured")
	}
	return s.deliveries.ListByWebhook(ctx, webhookID)
}

func (s *WebhookService) GetDelivery(ctx context.Context, id int64) (*models.WebhookDelivery, error) {
	if s == nil || s.deliveries == nil {
		return nil, errors.New("webhook service is not configured")
	}
	return s.deliveries.FindByID(ctx, id)
}

func (s *WebhookService) DeliverToWebhook(ctx context.Context, webhook models.Webhook, event string, payload any) (models.WebhookDelivery, error) {
	if s == nil || s.deliveries == nil {
		return models.WebhookDelivery{}, errors.New("webhook service is not configured")
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	delivery := models.WebhookDelivery{
		WebhookID: webhook.ID,
		EventType: event,
		Status:    string(WebhookDeliveryQueued),
		Attempts:  0,
		CreatedAt: now().UTC(),
	}
	if err := s.deliveries.Create(ctx, &delivery); err != nil {
		return models.WebhookDelivery{}, err
	}
	timestamp := now().UTC()
	envelope := WebhookEnvelope{Event: event, Delivery: fmt.Sprintf("%d", delivery.ID), Timestamp: timestamp, Payload: payload}
	body, err := json.Marshal(envelope)
	if err != nil {
		return s.failDelivery(ctx, delivery, timestamp.Add(defaultWebhookRetryDelay), err.Error())
	}
	delivery.PayloadJSON = string(body)
	delivery.Status = string(WebhookDeliveryRunning)
	delivery.Attempts = 1
	if err := s.deliveries.Update(ctx, &delivery); err != nil {
		return models.WebhookDelivery{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(body))
	if err != nil {
		return s.failDelivery(ctx, delivery, timestamp.Add(defaultWebhookRetryDelay), err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ahub-Event", event)
	req.Header.Set("X-Ahub-Delivery", fmt.Sprintf("%d", delivery.ID))
	req.Header.Set("X-Ahub-Timestamp", fmt.Sprintf("%d", timestamp.Unix()))
	req.Header.Set("X-Ahub-Signature", SignWebhookPayload(webhook.Secret, event, fmt.Sprintf("%d", delivery.ID), timestamp, body))
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return s.failDelivery(ctx, delivery, timestamp.Add(defaultWebhookRetryDelay), err.Error())
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	delivery.ResponseBody = string(responseBody)
	statusCode := resp.StatusCode
	delivery.ResponseStatus = &statusCode
	if statusCode >= 200 && statusCode < 300 {
		delivery.Status = string(WebhookDeliverySuccess)
		done := now().UTC()
		delivery.DeliveredAt = &done
		delivery.ErrorMessage = ""
		delivery.NextRetryAt = nil
	} else {
		message := fmt.Sprintf("unexpected webhook response %d", statusCode)
		if len(responseBody) > 0 {
			message = message + ": " + string(responseBody)
		}
		return s.failDelivery(ctx, delivery, timestamp.Add(defaultWebhookRetryDelay), message)
	}
	if err := s.deliveries.Update(ctx, &delivery); err != nil {
		return models.WebhookDelivery{}, err
	}
	return delivery, nil
}

func (s *WebhookService) retryDelivery(ctx context.Context, webhook *models.Webhook, delivery *models.WebhookDelivery) error {
	if webhook == nil || delivery == nil {
		return errors.New("webhook delivery retry is nil")
	}
	now := s.now
	if now == nil {
		now = time.Now
	}
	body := []byte(delivery.PayloadJSON)
	timestamp := now().UTC()
	delivery.Status = string(WebhookDeliveryRetrying)
	delivery.Attempts++
	delivery.ErrorMessage = ""
	if err := s.deliveries.Update(ctx, delivery); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(body))
	if err != nil {
		return s.markRetryFailure(ctx, delivery, timestamp.Add(defaultWebhookRetryDelay), err.Error())
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Ahub-Event", delivery.EventType)
	req.Header.Set("X-Ahub-Delivery", fmt.Sprintf("%d", delivery.ID))
	req.Header.Set("X-Ahub-Timestamp", fmt.Sprintf("%d", timestamp.Unix()))
	req.Header.Set("X-Ahub-Signature", SignWebhookPayload(webhook.Secret, delivery.EventType, fmt.Sprintf("%d", delivery.ID), timestamp, body))
	if requestID := RequestIDFromContext(ctx); requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return s.markRetryFailure(ctx, delivery, timestamp.Add(defaultWebhookRetryDelay), err.Error())
	}
	defer resp.Body.Close()
	responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	delivery.ResponseBody = string(responseBody)
	statusCode := resp.StatusCode
	delivery.ResponseStatus = &statusCode
	if statusCode >= 200 && statusCode < 300 {
		delivery.Status = string(WebhookDeliverySuccess)
		done := now().UTC()
		delivery.DeliveredAt = &done
		delivery.ErrorMessage = ""
		delivery.NextRetryAt = nil
	} else {
		message := fmt.Sprintf("unexpected webhook response %d", statusCode)
		if len(responseBody) > 0 {
			message += ": " + string(responseBody)
		}
		return s.markRetryFailure(ctx, delivery, timestamp.Add(defaultWebhookRetryDelay), message)
	}
	return s.deliveries.Update(ctx, delivery)
}

func (s *WebhookService) markRetryFailure(ctx context.Context, delivery *models.WebhookDelivery, nextRetryAt time.Time, message string) error {
	delivery.Status = string(WebhookDeliveryFailed)
	delivery.ErrorMessage = message
	delivery.NextRetryAt = &nextRetryAt
	return s.deliveries.Update(ctx, delivery)
}

func (s *WebhookService) failDelivery(ctx context.Context, delivery models.WebhookDelivery, nextRetryAt time.Time, message string) (models.WebhookDelivery, error) {
	delivery.Status = string(WebhookDeliveryFailed)
	delivery.ErrorMessage = message
	delivery.NextRetryAt = &nextRetryAt
	if delivery.Attempts <= 0 {
		delivery.Attempts = 1
	}
	if err := s.deliveries.Update(ctx, &delivery); err != nil {
		return models.WebhookDelivery{}, err
	}
	return delivery, nil
}
