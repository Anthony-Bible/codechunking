// Package util provides utility functions for the API layer.
package util

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP extracts the client IP address from an HTTP request.
// It checks headers in order of precedence:
// 1. X-Forwarded-For (first valid IP)
// 2. X-Real-IP
// 3. RemoteAddr (with port stripped)
func ClientIP(r *http.Request) string {
	if r == nil {
		panic("request cannot be nil")
	}

	// 1. Check X-Forwarded-For header first
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Split by comma and check each IP
		ips := strings.Split(xff, ",")
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip != "" && net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	// 2. Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		ip := strings.TrimSpace(xri)
		if ip != "" && net.ParseIP(ip) != nil {
			return ip
		}
	}

	// 3. Fall back to RemoteAddr
	if r.RemoteAddr == "" {
		return ""
	}

	// Handle IPv6 with brackets and port: [2001:db8::1]:8080 -> 2001:db8::1
	if strings.HasPrefix(r.RemoteAddr, "[") {
		if idx := strings.LastIndex(r.RemoteAddr, "]"); idx != -1 {
			if idx+1 < len(r.RemoteAddr) && r.RemoteAddr[idx+1] == ':' {
				// Has port after bracket
				return r.RemoteAddr[1:idx]
			}
			// No port after bracket (malformed, but handle gracefully)
			return r.RemoteAddr[1:idx]
		}
	}

	// Handle IPv4 with port: 192.168.1.100:8080 -> 192.168.1.100
	// Also handle edge cases like ":8080" and "192.168.1.100:8080:extra"
	if idx := strings.Index(r.RemoteAddr, ":"); idx != -1 {
		ip := r.RemoteAddr[:idx]

		// Handle case where there's only a port (":8080")
		if ip == "" {
			return ""
		}

		// Check if it's a valid IPv4 address
		if net.ParseIP(ip) != nil && strings.Count(ip, ".") == 3 {
			// It's an IPv4 address, so we can strip everything after the first colon
			return ip
		}
	}

	// Return as-is (IPv6 without brackets or no port)
	return r.RemoteAddr
}
