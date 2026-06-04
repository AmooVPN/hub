package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/AmooVPN/hub/internal/models"
)

func TestSignWebhookPayload(t *testing.T) {
	timestamp := time.Unix(1700000000, 0).UTC()
	sig1 := SignWebhookPayload("secret", models.WebhookEventBackupExported, "delivery-1", timestamp, []byte(`{"ok":true}`))
	sig2 := SignWebhookPayload("secret", models.WebhookEventBackupExported, "delivery-1", timestamp, []byte(`{"ok":true}`))
	if sig1 == "" || sig1 != sig2 {
		t.Fatalf("unexpected signature behavior: %q %q", sig1, sig2)
	}
}

func TestWebhookEventValidation(t *testing.T) {
	if !IsSupportedWebhookEvent(models.WebhookEventPanelOffline) {
		t.Fatal("expected supported webhook event")
	}
	if IsSupportedWebhookEvent("nope") {
		t.Fatal("did not expect unsupported webhook event")
	}
}

func TestWebhookServicePublish(t *testing.T) {
	reqCh := make(chan *http.Request, 1)
	service := NewWebhookService(
		&stubWebhookRepository{webhooks: []models.Webhook{{ID: 7, URL: "https://example.test/webhook", Secret: "secret", Active: true, Events: []string{models.WebhookEventPanelOffline}}}},
		&stubWebhookDeliveryRepository{},
	)
	service.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		reqCh <- req
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})
	service.now = func() time.Time { return time.Unix(1700000000, 0).UTC() }

	deliveries, err := service.Publish(context.Background(), models.WebhookEventPanelOffline, map[string]any{"ok": true})
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}
	if deliveries[0].Status != string(WebhookDeliverySuccess) {
		t.Fatalf("unexpected delivery status %q", deliveries[0].Status)
	}
	select {
	case req := <-reqCh:
		if got := req.Header.Get("X-Ahub-Event"); got != models.WebhookEventPanelOffline {
			t.Fatalf("unexpected event header %q", got)
		}
		if got := req.Header.Get("X-Ahub-Delivery"); got != "1" {
			t.Fatalf("unexpected delivery header %q", got)
		}
		if got := req.Header.Get("X-Ahub-Signature"); !strings.HasPrefix(got, "sha256=") {
			t.Fatalf("unexpected signature header %q", got)
		}
	default:
		t.Fatal("expected outbound webhook request")
	}
}

func TestWebhookServiceRetryDueDeliveries(t *testing.T) {
	reqCh := make(chan *http.Request, 1)
	now := time.Unix(1700000000, 0).UTC()
	repo := &stubWebhookDeliveryRepository{
		deliveries: []models.WebhookDelivery{{ID: 1, WebhookID: 7, EventType: models.WebhookEventPanelOffline, PayloadJSON: `{"event":"panel.offline"}`, Status: string(WebhookDeliveryFailed), Attempts: 1, NextRetryAt: &now}},
	}
	service := NewWebhookService(
		&stubWebhookRepository{webhooks: []models.Webhook{{ID: 7, URL: "https://example.test/webhook", Secret: "secret", Active: true, Events: []string{models.WebhookEventPanelOffline}}}},
		repo,
	)
	service.client = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		reqCh <- req
		return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: http.Header{}}, nil
	})
	service.now = func() time.Time { return now }

	count, err := service.RetryDueDeliveries(context.Background(), 10)
	if err != nil {
		t.Fatalf("retry failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 retried delivery, got %d", count)
	}
	if repo.deliveries[0].Status != string(WebhookDeliverySuccess) {
		t.Fatalf("unexpected delivery status %q", repo.deliveries[0].Status)
	}
	select {
	case req := <-reqCh:
		if got := req.Header.Get("X-Ahub-Delivery"); got != "1" {
			t.Fatalf("unexpected delivery header %q", got)
		}
	default:
		t.Fatal("expected retry request")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) Do(req *http.Request) (*http.Response, error) { return f(req) }

type stubWebhookRepository struct{ webhooks []models.Webhook }

func (s *stubWebhookRepository) Create(context.Context, *models.Webhook) error                { return nil }
func (s *stubWebhookRepository) Update(context.Context, *models.Webhook) error                { return nil }
func (s *stubWebhookRepository) Delete(context.Context, int64) error                          { return nil }
func (s *stubWebhookRepository) FindByID(_ context.Context, id int64) (*models.Webhook, error) {
	for _, webhook := range s.webhooks {
		if webhook.ID == id {
			copy := webhook
			return &copy, nil
		}
	}
	return nil, nil
}
func (s *stubWebhookRepository) List(context.Context) ([]models.Webhook, error)               { return append([]models.Webhook(nil), s.webhooks...), nil }
func (s *stubWebhookRepository) ListActiveByEvent(_ context.Context, event string) ([]models.Webhook, error) {
	items := make([]models.Webhook, 0)
	for _, webhook := range s.webhooks {
		if !webhook.Active {
			continue
		}
		for _, candidate := range webhook.Events {
			if candidate == event {
				items = append(items, webhook)
				break
			}
		}
	}
	return items, nil
}

type stubWebhookDeliveryRepository struct {
	nextID    int64
	deliveries []models.WebhookDelivery
}

func (s *stubWebhookDeliveryRepository) Create(_ context.Context, delivery *models.WebhookDelivery) error {
	s.nextID++
	delivery.ID = s.nextID
	s.deliveries = append(s.deliveries, *delivery)
	return nil
}
func (s *stubWebhookDeliveryRepository) Update(_ context.Context, delivery *models.WebhookDelivery) error {
	for i := range s.deliveries {
		if s.deliveries[i].ID == delivery.ID {
			s.deliveries[i] = *delivery
			return nil
		}
	}
	return nil
}
func (s *stubWebhookDeliveryRepository) FindByID(context.Context, int64) (*models.WebhookDelivery, error) {
	return nil, nil
}
func (s *stubWebhookDeliveryRepository) ListByWebhook(context.Context, int64) ([]models.WebhookDelivery, error) {
	return append([]models.WebhookDelivery(nil), s.deliveries...), nil
}
func (s *stubWebhookDeliveryRepository) ListDueForRetry(_ context.Context, _ time.Time, limit int) ([]models.WebhookDelivery, error) {
	items := make([]models.WebhookDelivery, 0)
	for _, delivery := range s.deliveries {
		if delivery.NextRetryAt != nil {
			items = append(items, delivery)
		}
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}
