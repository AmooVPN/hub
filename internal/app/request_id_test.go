package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/config"
)

func TestRequestIDMiddlewareAndAPIErrorIncludeRequestID(t *testing.T) {
	r := &Runner{cfg: &config.Config{AppName: "hub"}}
	app := fiber.New(fiber.Config{ErrorHandler: r.errorHandler})
	app.Use(r.requestIDMiddleware())
	app.Get("/api/test", func(c *fiber.Ctx) error {
		return apiError(c, fiber.StatusBadRequest, "bad_request", "bad request")
	})
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("X-Request-ID", "test-request-id")
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if got := resp.Header.Get("X-Request-ID"); got != "test-request-id" {
		t.Fatalf("expected request id header, got %q", got)
	}
	var payload struct {
		Error struct {
			Code      string `json:"code"`
			Message   string `json:"message"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload.Error.RequestID != "test-request-id" || payload.Error.Code != "bad_request" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestRenderErrorPageIncludesRequestID(t *testing.T) {
	page := renderErrorPage("Bad Request", "Something went wrong.", "req-123")
	for _, expected := range []string{"Bad Request", "Request ID: req-123"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}
