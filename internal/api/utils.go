package api

import (
	"net/http"
	"strings"
)

// shortID returns first 12 characters of an ID (safe for short IDs)
func shortID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// getClientIP extracts client IP from request, considering reverse proxy headers
func getClientIP(r *http.Request) string {
	// Check X-Real-IP first (set by nginx)
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Check X-Forwarded-For (can contain multiple IPs)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (original client)
		if idx := strings.Index(xff, ","); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	// Fall back to RemoteAddr
	// Remove port if present
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
