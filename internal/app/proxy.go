package app

import (
	"net"
	"strings"

	"github.com/gofiber/fiber/v2"
)

func requestClientIP(c *fiber.Ctx, trustProxy bool) string {
	if c == nil {
		return ""
	}
	if trustProxy {
		if forwarded := strings.TrimSpace(c.Get("X-Forwarded-For")); forwarded != "" {
			parts := strings.Split(forwarded, ",")
			if len(parts) > 0 {
				candidate := strings.TrimSpace(parts[0])
				if candidate != "" {
					return candidate
				}
			}
		}
		if realIP := strings.TrimSpace(c.Get("X-Real-IP")); realIP != "" {
			return realIP
		}
	}
	ip := strings.TrimSpace(c.IP())
	if host, _, err := net.SplitHostPort(ip); err == nil && host != "" {
		return host
	}
	return ip
}

func requestForwardedProto(c *fiber.Ctx, trustProxy bool) string {
	if c == nil {
		return ""
	}
	if trustProxy {
		if proto := strings.TrimSpace(c.Get("X-Forwarded-Proto")); proto != "" {
			return proto
		}
	}
	return strings.TrimSpace(c.Protocol())
}

func requestForwardedHost(c *fiber.Ctx, trustProxy bool) string {
	if c == nil {
		return ""
	}
	if trustProxy {
		if host := strings.TrimSpace(c.Get("X-Forwarded-Host")); host != "" {
			return host
		}
	}
	return strings.TrimSpace(c.Hostname())
}
