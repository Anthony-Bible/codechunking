// Package messaging provides tests for NATS messaging configuration
// and related inbound adapter functionality.
package messaging

import (
	"codechunking/internal/config"
	"testing"
	"time"
)

// TestNATSConfigType tests that we can create a NATSConfig type.
func TestNATSConfigType(t *testing.T) {
	natsConfig := config.NATSConfig{
		URL:           "nats://localhost:4222",
		MaxReconnects: 10,
		ReconnectWait: 2 * time.Second,
	}

	if natsConfig.URL != "nats://localhost:4222" {
		t.Errorf("Expected URL to be 'nats://localhost:4222', got %s", natsConfig.URL)
	}
}
