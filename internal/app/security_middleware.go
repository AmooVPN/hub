package app

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v2"
)

const (
	csrfCookieName = "hub_csrf"
	csrfFormField  = "csrf_token"
)

func (r *Runner) securityHeadersMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		c.Set("Content-Security-Policy", "default-src 'self'; base-uri 'self'; frame-ancestors 'none'; img-src 'self' data:; font-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'")
		return c.Next()
	}
}

func (r *Runner) csrfMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		secure := r != nil && r.cfg != nil && r.cfg.IsProduction()
		if isSafeHTTPMethod(c.Method()) {
			ensureCSRFCookie(c, secure)
			return c.Next()
		}
		if strings.HasPrefix(c.Path(), "/api/") || isJSONRequest(c) {
			return c.Next()
		}
		cookieToken := strings.TrimSpace(c.Cookies(csrfCookieName))
		if cookieToken == "" {
			return forbiddenCSRF(c)
		}
		submitted := strings.TrimSpace(c.Get("X-CSRF-Token"))
		if submitted == "" {
			submitted = strings.TrimSpace(c.FormValue(csrfFormField))
		}
		if submitted == "" || subtle.ConstantTimeCompare([]byte(cookieToken), []byte(submitted)) != 1 {
			return forbiddenCSRF(c)
		}
		return c.Next()
	}
}

func ensureCSRFCookie(c *fiber.Ctx, secure bool) {
	if strings.TrimSpace(c.Cookies(csrfCookieName)) != "" {
		return
	}
	c.Cookie(&fiber.Cookie{
		Name:     csrfCookieName,
		Value:    newCSRFToken(),
		Path:     "/",
		HTTPOnly: false,
		Secure:   secure,
		SameSite: "Lax",
	})
}

func forbiddenCSRF(c *fiber.Ctx) error {
	return c.Status(fiber.StatusForbidden).Type("html").SendString(renderErrorPage("Forbidden", "Invalid form submission.", ""))
}

func isSafeHTTPMethod(method string) bool {
	switch method {
	case fiber.MethodGet, fiber.MethodHead, fiber.MethodOptions, fiber.MethodTrace:
		return true
	default:
		return false
	}
}

func isJSONRequest(c *fiber.Ctx) bool {
	contentType := strings.ToLower(c.Get("Content-Type"))
	return strings.Contains(contentType, "application/json") || strings.Contains(contentType, "application/ld+json")
}

func newCSRFToken() string {
	var buf [32]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return newRequestID()
	}
	return hex.EncodeToString(buf[:])
}
