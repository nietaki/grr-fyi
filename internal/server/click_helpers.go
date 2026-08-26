package server

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/labstack/echo/v5"
)

func extractIP(c *echo.Context) string {
	if xff := c.Request().Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	if xri := c.Request().Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	ip := c.RealIP()
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		return ip[:idx]
	}
	return ip
}

func hashIP(ip string) string {
	if ip == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(ip))
	return hex.EncodeToString(hash[:])[:16]
}

func extractReferrer(c *echo.Context) string {
	if ref := c.Request().Header.Get("Referer"); ref != "" {
		return ref
	}
	return c.Request().Header.Get("Origin")
}
