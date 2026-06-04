package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"

	"github.com/AmooVPM/hub/internal/config"
)

func TestSecurityHeadersApplied(t *testing.T) {
	r := &Runner{cfg: &config.Config{AppName: "hub", MaxUploadSizeMB: 10}}
	app := r.buildServer()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	for header, expected := range map[string]string{
		"X-Frame-Options":        "DENY",
		"X-Content-Type-Options": "nosniff",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
		"Permissions-Policy":     "geolocation=(), microphone=(), camera=()",
		"Content-Security-Policy": "default-src 'self'",
	} {
		if got := resp.Header.Get(header); got != expected {
			if header != "Content-Security-Policy" || !strings.HasPrefix(got, expected) {
				t.Fatalf("expected %s=%q, got %q", header, expected, got)
			}
		}
	}
}

func TestCSRFMiddlewareProtectsFormPosts(t *testing.T) {
	r := &Runner{cfg: &config.Config{AppName: "hub"}}
	app := fiber.New()
	app.Use(r.csrfMiddleware())
	app.Get("/form", func(c *fiber.Ctx) error {
		return c.Type("html").SendString(renderPage("Form", `<form method="post" action="/submit"><button type="submit">Submit</button></form>`))
	})
	app.Post("/submit", func(c *fiber.Ctx) error {
		return c.SendStatus(http.StatusOK)
	})

	getResp, err := app.Test(httptest.NewRequest(http.MethodGet, "/form", nil))
	if err != nil {
		t.Fatalf("get form: %v", err)
	}
	if !strings.Contains(getResp.Header.Get("Set-Cookie"), csrfCookieName+"=") {
		t.Fatalf("expected csrf cookie, got %q", getResp.Header.Get("Set-Cookie"))
	}
	body, err := io.ReadAll(getResp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if !strings.Contains(string(body), csrfFormField) || !strings.Contains(string(body), csrfCookieName) {
		t.Fatalf("expected csrf script in body, got %s", string(body))
	}
	token := cookieValue(getResp.Header.Get("Set-Cookie"), csrfCookieName)
	if token == "" {
		t.Fatal("expected csrf token cookie")
	}
	cookieHeader := csrfCookieName + "=" + token

	blockedReq := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(url.Values{}.Encode()))
	blockedReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	blockedReq.Header.Set("Cookie", cookieHeader)
	blockedResp, err := app.Test(blockedReq)
	if err != nil {
		t.Fatalf("blocked post: %v", err)
	}
	if blockedResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected forbidden, got %d", blockedResp.StatusCode)
	}

	okReq := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader(url.Values{csrfFormField: {token}}.Encode()))
	okReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	okReq.Header.Set("Cookie", cookieHeader)
	okResp, err := app.Test(okReq)
	if err != nil {
		t.Fatalf("allowed post: %v", err)
	}
	if okResp.StatusCode != http.StatusOK {
		t.Fatalf("expected ok, got %d", okResp.StatusCode)
	}
}

func TestValidatePanelBaseURL(t *testing.T) {
	if err := validatePanelBaseURL("https://panel.example.com/"); err != nil {
		t.Fatalf("expected valid url: %v", err)
	}
	if got := normalizeBaseURL(" https://panel.example.com/ "); got != "https://panel.example.com" {
		t.Fatalf("unexpected normalized url: %q", got)
	}
	for _, raw := range []string{"ftp://panel.example.com", "not-a-url", "https://"} {
		if err := validatePanelBaseURL(raw); err == nil {
			t.Fatalf("expected validation error for %q", raw)
		}
	}
	if err := validatePanelBaseURLPolicy("https://127.0.0.1:2053", true, false); err == nil {
		t.Fatal("expected private address to be rejected in strict mode")
	}
}

func TestRenderPageUsesLocalAssets(t *testing.T) {
	page := renderPage("Test", "<main>ok</main>")
	for _, expected := range []string{"/static/vendor/bootstrap/bootstrap.min.css?v=", "/static/vendor/bootstrap/bootstrap.bundle.min.js?v=", "prefers-color-scheme", "global-loading-indicator"} {
		if !strings.Contains(page, expected) {
			t.Fatalf("expected %q in page", expected)
		}
	}
}

func TestStaticAssetURLAddsVersion(t *testing.T) {
	for _, rel := range []string{"vendor/bootstrap/bootstrap.min.css", "vendor/htmx/htmx.min.js"} {
		url := staticAssetURL(rel)
		if !strings.HasPrefix(url, "/static/"+rel) {
			t.Fatalf("unexpected asset url: %q", url)
		}
		if !regexp.MustCompile(`\?v=\d+$`).MatchString(url) {
			t.Fatalf("expected version query in %q", url)
		}
	}
}

func TestStaticAssetsHaveLongCacheHeaders(t *testing.T) {
	r, _ := newHandlerTestRunner(t)
	app := r.buildServer()
	req := httptest.NewRequest(http.MethodGet, "/static/vendor/bootstrap/bootstrap.min.css", nil)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("test request: %v", err)
	}
	if got := resp.Header.Get("Cache-Control"); !strings.Contains(got, "max-age=31536000") || !strings.Contains(got, "immutable") {
		t.Fatalf("expected immutable cache headers, got %q", got)
	}
}

func TestOpenAPIDocsDoesNotReferenceCDN(t *testing.T) {
	page := renderOpenAPIDocsPage("hub")
	for _, forbidden := range []string{"unpkg.com/swagger-ui-dist", "cdn.jsdelivr.net", "SwaggerUIBundle"} {
		if strings.Contains(page, forbidden) {
			t.Fatalf("did not expect %q in docs page", forbidden)
		}
	}
}

func cookieValue(setCookie, name string) string {
	for _, part := range strings.Split(setCookie, ";") {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, name+"=") {
			return strings.TrimPrefix(part, name+"=")
		}
	}
	return ""
}
