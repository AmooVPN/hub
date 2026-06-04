package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestInitialSetupFlow(t *testing.T) {
	r, _ := newEmptyHandlerTestRunner(t)
	app := r.buildServer()

	rootResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("get root: %v", err)
	}
	if rootResp.StatusCode != http.StatusFound || rootResp.Header.Get("Location") != "/setup" {
		t.Fatalf("expected redirect to setup, got %d %q", rootResp.StatusCode, rootResp.Header.Get("Location"))
	}

	getResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/setup", nil))
	if err != nil {
		t.Fatalf("get setup: %v", err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected setup page, got %d", getResp.StatusCode)
	}
	body, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read setup page: %v", err)
	}
	if !strings.Contains(string(body), "Initial setup") {
		t.Fatal("expected setup page content")
	}
	csrf := cookieValue(getResp.Header.Get("Set-Cookie"), csrfCookieName)
	if csrf == "" {
		t.Fatal("expected csrf cookie")
	}

	form := url.Values{
		csrfFormField:    {csrf},
		"username":       {"admin"},
		"password":       {"change-me-now-123"},
		"confirm_password": {"change-me-now-123"},
	}
	postReq := httptest.NewRequest(http.MethodPost, "/setup", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.Header.Set("Cookie", csrfCookieName+"="+csrf)
	postResp, err := app.Test(postReq)
	if err != nil {
		t.Fatalf("post setup: %v", err)
	}
	if postResp.StatusCode != http.StatusFound || postResp.Header.Get("Location") != "/admin" {
		t.Fatalf("expected redirect to admin, got %d %q", postResp.StatusCode, postResp.Header.Get("Location"))
	}

	count, err := r.admins.Count(context.Background())
	if err != nil {
		t.Fatalf("count admins: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one admin, got %d", count)
	}
	if cookieValue(postResp.Header.Get("Set-Cookie"), r.cfg.SessionCookieName) == "" {
		t.Fatal("expected admin session cookie after setup")
	}
}

func TestSetupRedirectsAwayWhenAlreadyInitialized(t *testing.T) {
	r, _ := newHandlerTestRunner(t)
	app := r.buildServer()

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/setup", nil))
	if err != nil {
		t.Fatalf("get setup: %v", err)
	}
	if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != "/admin/login" {
		t.Fatalf("expected redirect to login, got %d %q", resp.StatusCode, resp.Header.Get("Location"))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty redirect body, got %q", string(body))
	}
}
