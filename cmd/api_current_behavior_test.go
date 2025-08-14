package cmd

import (
	"testing"

	"codechunking/internal/config"

	"github.com/stretchr/testify/assert"
)

// TestServiceFactory_shouldEnableDefaultMiddleware_CurrentBehavior tests the current
// implementation that always returns true, demonstrating the need for refactoring
func TestServiceFactory_shouldEnableDefaultMiddleware_CurrentBehavior(t *testing.T) {
	// Create minimal config
	cfg := &config.Config{
		API: config.APIConfig{
			Host: "localhost",
			Port: "8080",
		},
	}

	factory := NewServiceFactory(cfg)

	// Current implementation always returns true (regardless of any configuration)
	result := factory.shouldEnableDefaultMiddleware()

	// This demonstrates the current hardcoded behavior that needs to be made configurable
	assert.True(t, result, "Current implementation always returns true - this needs to be configurable")
}
