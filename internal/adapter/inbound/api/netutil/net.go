// Package netutil provides network utility functions for the API layer.
package netutil

import (
	"net"
	"net/http"
	"strings"
)

// ClientIP extracts the client IP address from an HTTP request.
// It checks headers in order of precedence:
// 1. X-Forwarded-For (first valid IP)
// 2. X-Real-IP
// 3. RemoteAddr (with port stripped).
func getIPFromXForwardedFor(header string) string {
	if header == "" {
		return ""
	}

	ips := strings.Split(header, ",")
	for _, ip := range ips {
		ip = strings.TrimSpace(ip)
		if ip != "" && net.ParseIP(ip) != nil {
			return ip
		}
	}
	return ""
}

func getIPFromXRealIP(header string) string {
	if header == "" {
		return ""
	}

	ip := strings.TrimSpace(header)
	if ip != "" && net.ParseIP(ip) != nil {
		return ip
	}
	return ""
}

func extractIPFromIPv6WithPort(remoteAddr string) string {
	if !strings.HasPrefix(remoteAddr, "[") {
		return ""
	}

	idx := strings.LastIndex(remoteAddr, "]")
	if idx == -1 {
		return ""
	}

	return remoteAddr[1:idx]
}

func extractIPFromIPv4WithPort(remoteAddr string) string {
	idx := strings.Index(remoteAddr, ":")
	if idx == -1 {
		return ""
	}

	ip := remoteAddr[:idx]
	if ip == "" {
		return ""
	}

	if net.ParseIP(ip) != nil && strings.Count(ip, ".") == 3 {
		return ip
	}
	return ""
}

func extractIPFromRemoteAddr(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}

	if ipv6IP := extractIPFromIPv6WithPort(remoteAddr); ipv6IP != "" {
		return ipv6IP
	}

	if ipv4IP := extractIPFromIPv4WithPort(remoteAddr); ipv4IP != "" {
		return ipv4IP
	}

	// If no valid IP was extracted and the remoteAddr is malformed (e.g., ":8080"), return empty string
	if strings.HasPrefix(remoteAddr, ":") {
		return ""
	}

	return remoteAddr
}

func ClientIP(r *http.Request) string {
	if r == nil {
		panic("request cannot be nil")
	}

	if ip := getIPFromXForwardedFor(r.Header.Get("X-Forwarded-For")); ip != "" {
		return ip
	}

	if ip := getIPFromXRealIP(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}

	return extractIPFromRemoteAddr(r.RemoteAddr)
}
