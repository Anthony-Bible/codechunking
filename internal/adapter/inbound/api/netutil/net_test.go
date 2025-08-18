package netutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestClientIP_XForwardedForHeader tests extraction from X-Forwarded-For header.
func TestClientIP_XForwardedForHeader(t *testing.T) {
	tests := []struct {
		name          string
		xForwardedFor string
		xRealIP       string
		remoteAddr    string
		expectedIP    string
		description   string
	}{
		{
			name:          "single_ipv4_in_x_forwarded_for",
			xForwardedFor: "192.168.1.100",
			xRealIP:       "10.0.0.1",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should return first IP from X-Forwarded-For header",
		},
		{
			name:          "multiple_ipv4_in_x_forwarded_for",
			xForwardedFor: "203.0.113.195,70.41.3.18,150.172.238.178",
			xRealIP:       "10.0.0.1",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "203.0.113.195",
			description:   "should return first IP when multiple IPs are comma-separated",
		},
		{
			name:          "x_forwarded_for_with_whitespace",
			xForwardedFor: "  192.168.1.100  ,  10.0.0.1  ",
			xRealIP:       "172.16.0.1",
			remoteAddr:    "198.51.100.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should trim whitespace from first IP in X-Forwarded-For",
		},
		{
			name:          "single_ipv6_in_x_forwarded_for",
			xForwardedFor: "2001:db8::1",
			xRealIP:       "192.168.1.1",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "2001:db8::1",
			description:   "should handle IPv6 addresses in X-Forwarded-For",
		},
		{
			name:          "multiple_ipv6_in_x_forwarded_for",
			xForwardedFor: "2001:db8::1,2001:db8::2,2001:db8::3",
			xRealIP:       "192.168.1.1",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "2001:db8::1",
			description:   "should return first IPv6 from comma-separated list",
		},
		{
			name:          "mixed_ipv4_ipv6_in_x_forwarded_for",
			xForwardedFor: "192.168.1.100,2001:db8::1",
			xRealIP:       "10.0.0.1",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should return first IP regardless of IPv4/IPv6 mix",
		},
		{
			name:          "empty_x_forwarded_for",
			xForwardedFor: "",
			xRealIP:       "192.168.1.100",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should fall back to X-Real-IP when X-Forwarded-For is empty",
		},
		{
			name:          "whitespace_only_x_forwarded_for",
			xForwardedFor: "   ",
			xRealIP:       "192.168.1.100",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should fall back to X-Real-IP when X-Forwarded-For contains only whitespace",
		},
		{
			name:          "malformed_x_forwarded_for_commas_only",
			xForwardedFor: ",,,",
			xRealIP:       "192.168.1.100",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should fall back to X-Real-IP when X-Forwarded-For contains only commas",
		},
		{
			name:          "x_forwarded_for_with_empty_segments",
			xForwardedFor: ",192.168.1.100,,10.0.0.1,",
			xRealIP:       "172.16.0.1",
			remoteAddr:    "198.51.100.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should find first valid IP even with empty segments",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			result := ClientIP(req)

			if result != tt.expectedIP {
				t.Errorf("ClientIP() = %q, want %q\nDescription: %s", result, tt.expectedIP, tt.description)
			}
		})
	}
}

// TestClientIP_XRealIPHeader tests extraction from X-Real-IP header.
func TestClientIP_XRealIPHeader(t *testing.T) {
	tests := []struct {
		name        string
		xRealIP     string
		remoteAddr  string
		expectedIP  string
		description string
	}{
		{
			name:        "valid_ipv4_x_real_ip",
			xRealIP:     "203.0.113.195",
			remoteAddr:  "192.168.1.100:8080",
			expectedIP:  "203.0.113.195",
			description: "should return X-Real-IP when no X-Forwarded-For is present",
		},
		{
			name:        "valid_ipv6_x_real_ip",
			xRealIP:     "2001:db8::1",
			remoteAddr:  "192.168.1.100:8080",
			expectedIP:  "2001:db8::1",
			description: "should handle IPv6 address in X-Real-IP",
		},
		{
			name:        "x_real_ip_with_whitespace",
			xRealIP:     "  192.168.1.100  ",
			remoteAddr:  "172.16.0.1:8080",
			expectedIP:  "192.168.1.100",
			description: "should trim whitespace from X-Real-IP",
		},
		{
			name:        "empty_x_real_ip",
			xRealIP:     "",
			remoteAddr:  "192.168.1.100:8080",
			expectedIP:  "192.168.1.100",
			description: "should fall back to RemoteAddr when X-Real-IP is empty",
		},
		{
			name:        "whitespace_only_x_real_ip",
			xRealIP:     "   ",
			remoteAddr:  "192.168.1.100:8080",
			expectedIP:  "192.168.1.100",
			description: "should fall back to RemoteAddr when X-Real-IP contains only whitespace",
		},
		{
			name:        "invalid_x_real_ip",
			xRealIP:     "not-an-ip",
			remoteAddr:  "192.168.1.100:8080",
			expectedIP:  "192.168.1.100",
			description: "should fall back to RemoteAddr when X-Real-IP is invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			result := ClientIP(req)

			if result != tt.expectedIP {
				t.Errorf("ClientIP() = %q, want %q\nDescription: %s", result, tt.expectedIP, tt.description)
			}
		})
	}
}

// TestClientIP_RemoteAddrFallback tests fallback to RemoteAddr.
func TestClientIP_RemoteAddrFallback(t *testing.T) {
	tests := []struct {
		name        string
		remoteAddr  string
		expectedIP  string
		description string
	}{
		{
			name:        "ipv4_without_port",
			remoteAddr:  "192.168.1.100",
			expectedIP:  "192.168.1.100",
			description: "should return IPv4 address when no port is present",
		},
		{
			name:        "ipv4_with_port",
			remoteAddr:  "192.168.1.100:8080",
			expectedIP:  "192.168.1.100",
			description: "should strip port from IPv4 address",
		},
		{
			name:        "ipv4_with_standard_http_port",
			remoteAddr:  "203.0.113.195:80",
			expectedIP:  "203.0.113.195",
			description: "should strip standard HTTP port from IPv4 address",
		},
		{
			name:        "ipv4_with_https_port",
			remoteAddr:  "203.0.113.195:443",
			expectedIP:  "203.0.113.195",
			description: "should strip HTTPS port from IPv4 address",
		},
		{
			name:        "ipv6_without_port",
			remoteAddr:  "2001:db8::1",
			expectedIP:  "2001:db8::1",
			description: "should return IPv6 address when no port is present",
		},
		{
			name:        "ipv6_with_port_bracketed",
			remoteAddr:  "[2001:db8::1]:8080",
			expectedIP:  "2001:db8::1",
			description: "should strip port from bracketed IPv6 address",
		},
		{
			name:        "ipv6_with_standard_http_port",
			remoteAddr:  "[2001:db8::1]:80",
			expectedIP:  "2001:db8::1",
			description: "should strip standard HTTP port from IPv6 address",
		},
		{
			name:        "ipv6_with_https_port",
			remoteAddr:  "[2001:db8::1]:443",
			expectedIP:  "2001:db8::1",
			description: "should strip HTTPS port from IPv6 address",
		},
		{
			name:        "ipv6_full_address",
			remoteAddr:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			expectedIP:  "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			description: "should handle full IPv6 address without port",
		},
		{
			name:        "ipv6_compressed_address",
			remoteAddr:  "2001:db8:85a3::8a2e:370:7334",
			expectedIP:  "2001:db8:85a3::8a2e:370:7334",
			description: "should handle compressed IPv6 address without port",
		},
		{
			name:        "localhost_ipv4",
			remoteAddr:  "127.0.0.1:3000",
			expectedIP:  "127.0.0.1",
			description: "should handle localhost IPv4 with port",
		},
		{
			name:        "localhost_ipv6",
			remoteAddr:  "[::1]:3000",
			expectedIP:  "::1",
			description: "should handle localhost IPv6 with port",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr

			result := ClientIP(req)

			if result != tt.expectedIP {
				t.Errorf("ClientIP() = %q, want %q\nDescription: %s", result, tt.expectedIP, tt.description)
			}
		})
	}
}

// TestClientIP_HeaderPrecedence tests the precedence order of headers.
func TestClientIP_HeaderPrecedence(t *testing.T) {
	tests := []struct {
		name          string
		xForwardedFor string
		xRealIP       string
		remoteAddr    string
		expectedIP    string
		description   string
	}{
		{
			name:          "x_forwarded_for_takes_precedence_over_x_real_ip",
			xForwardedFor: "203.0.113.195",
			xRealIP:       "192.168.1.100",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "203.0.113.195",
			description:   "X-Forwarded-For should take precedence over X-Real-IP",
		},
		{
			name:          "x_forwarded_for_takes_precedence_over_remote_addr",
			xForwardedFor: "203.0.113.195",
			xRealIP:       "",
			remoteAddr:    "192.168.1.100:8080",
			expectedIP:    "203.0.113.195",
			description:   "X-Forwarded-For should take precedence over RemoteAddr",
		},
		{
			name:          "x_real_ip_takes_precedence_over_remote_addr",
			xForwardedFor: "",
			xRealIP:       "203.0.113.195",
			remoteAddr:    "192.168.1.100:8080",
			expectedIP:    "203.0.113.195",
			description:   "X-Real-IP should take precedence over RemoteAddr",
		},
		{
			name:          "all_headers_present_prefer_x_forwarded_for",
			xForwardedFor: "203.0.113.195,70.41.3.18",
			xRealIP:       "192.168.1.100",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "203.0.113.195",
			description:   "should prefer first IP from X-Forwarded-For when all headers are present",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			result := ClientIP(req)

			if result != tt.expectedIP {
				t.Errorf("ClientIP() = %q, want %q\nDescription: %s", result, tt.expectedIP, tt.description)
			}
		})
	}
}

// TestClientIP_EdgeCases tests edge cases and error conditions.
func TestClientIP_EdgeCases(t *testing.T) {
	tests := []struct {
		name          string
		xForwardedFor string
		xRealIP       string
		remoteAddr    string
		expectedIP    string
		description   string
	}{
		{
			name:          "all_headers_empty",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    "192.168.1.100:8080",
			expectedIP:    "192.168.1.100",
			description:   "should fall back to RemoteAddr when all headers are empty",
		},
		{
			name:          "all_headers_whitespace",
			xForwardedFor: "   ",
			xRealIP:       "   ",
			remoteAddr:    "192.168.1.100:8080",
			expectedIP:    "192.168.1.100",
			description:   "should fall back to RemoteAddr when all headers contain only whitespace",
		},
		{
			name:          "empty_remote_addr",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    "",
			expectedIP:    "",
			description:   "should return empty string when all sources are empty",
		},
		{
			name:          "malformed_remote_addr_multiple_colons",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    "192.168.1.100:8080:extra",
			expectedIP:    "192.168.1.100",
			description:   "should handle malformed RemoteAddr with multiple colons gracefully",
		},
		{
			name:          "x_forwarded_for_invalid_ip_format",
			xForwardedFor: "not.an.ip.address,192.168.1.100",
			xRealIP:       "172.16.0.1",
			remoteAddr:    "198.51.100.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should skip invalid IP and use next valid IP in X-Forwarded-For",
		},
		{
			name:          "x_forwarded_for_all_invalid",
			xForwardedFor: "not.an.ip,also.not.ip,still.invalid",
			xRealIP:       "192.168.1.100",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "192.168.1.100",
			description:   "should fall back to X-Real-IP when all IPs in X-Forwarded-For are invalid",
		},
		{
			name:          "port_without_ip",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    ":8080",
			expectedIP:    "",
			description:   "should handle malformed RemoteAddr with only port",
		},
		{
			name:          "ipv6_without_brackets_with_port_ambiguous",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    "2001:db8::1:8080",
			expectedIP:    "2001:db8::1:8080",
			description:   "should treat unbracketed IPv6 with colon as complete address (ambiguous case)",
		},
		{
			name:          "very_long_x_forwarded_for_chain",
			xForwardedFor: "203.0.113.1,203.0.113.2,203.0.113.3,203.0.113.4,203.0.113.5,203.0.113.6,203.0.113.7,203.0.113.8,203.0.113.9,203.0.113.10",
			xRealIP:       "192.168.1.1",
			remoteAddr:    "172.16.0.1:8080",
			expectedIP:    "203.0.113.1",
			description:   "should handle very long proxy chain and return first IP",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			result := ClientIP(req)

			if result != tt.expectedIP {
				t.Errorf("ClientIP() = %q, want %q\nDescription: %s", result, tt.expectedIP, tt.description)
			}
		})
	}
}

// TestClientIP_NilRequest tests behavior with nil request.
func TestClientIP_NilRequest(t *testing.T) {
	t.Run("nil_request", func(t *testing.T) {
		// This test should panic or handle nil gracefully
		// The expected behavior needs to be defined based on requirements
		defer func() {
			if r := recover(); r == nil {
				t.Error("Expected panic when passing nil request, but didn't panic")
			}
		}()

		ClientIP(nil)
	})
}

// TestClientIP_RealWorldScenarios tests realistic scenarios.
func TestClientIP_RealWorldScenarios(t *testing.T) {
	tests := []struct {
		name          string
		xForwardedFor string
		xRealIP       string
		remoteAddr    string
		expectedIP    string
		description   string
		scenario      string
	}{
		{
			name:          "cloudflare_proxy",
			xForwardedFor: "203.0.113.195",
			xRealIP:       "",
			remoteAddr:    "108.162.192.1:443",
			expectedIP:    "203.0.113.195",
			description:   "should extract client IP from Cloudflare proxy setup",
			scenario:      "Client -> Cloudflare -> Server",
		},
		{
			name:          "aws_load_balancer",
			xForwardedFor: "203.0.113.195,172.31.1.1",
			xRealIP:       "",
			remoteAddr:    "10.0.1.100:8080",
			expectedIP:    "203.0.113.195",
			description:   "should extract client IP from AWS ALB proxy chain",
			scenario:      "Client -> ALB -> Instance",
		},
		{
			name:          "nginx_reverse_proxy",
			xForwardedFor: "",
			xRealIP:       "203.0.113.195",
			remoteAddr:    "127.0.0.1:3000",
			expectedIP:    "203.0.113.195",
			description:   "should extract client IP from nginx reverse proxy",
			scenario:      "Client -> nginx -> App",
		},
		{
			name:          "multiple_proxy_layers",
			xForwardedFor: "203.0.113.195,70.41.3.18,150.172.238.178",
			xRealIP:       "192.168.1.1",
			remoteAddr:    "127.0.0.1:8080",
			expectedIP:    "203.0.113.195",
			description:   "should extract original client IP through multiple proxy layers",
			scenario:      "Client -> CDN -> Load Balancer -> nginx -> App",
		},
		{
			name:          "direct_connection_no_proxy",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    "203.0.113.195:54321",
			expectedIP:    "203.0.113.195",
			description:   "should extract IP from direct connection without proxy",
			scenario:      "Client -> App (direct)",
		},
		{
			name:          "development_localhost",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    "127.0.0.1:3000",
			expectedIP:    "127.0.0.1",
			description:   "should handle localhost development environment",
			scenario:      "Local development",
		},
		{
			name:          "docker_container_networking",
			xForwardedFor: "",
			xRealIP:       "",
			remoteAddr:    "172.17.0.1:8080",
			expectedIP:    "172.17.0.1",
			description:   "should handle Docker container networking",
			scenario:      "Docker container environment",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.xForwardedFor != "" {
				req.Header.Set("X-Forwarded-For", tt.xForwardedFor)
			}
			if tt.xRealIP != "" {
				req.Header.Set("X-Real-IP", tt.xRealIP)
			}
			req.RemoteAddr = tt.remoteAddr

			result := ClientIP(req)

			if result != tt.expectedIP {
				t.Errorf(
					"ClientIP() = %q, want %q\nDescription: %s\nScenario: %s",
					result,
					tt.expectedIP,
					tt.description,
					tt.scenario,
				)
			}
		})
	}
}
