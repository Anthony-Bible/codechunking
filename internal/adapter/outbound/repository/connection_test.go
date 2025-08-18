package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDatabaseConnection_NewConnection tests database connection establishment.
func TestDatabaseConnection_NewConnection(t *testing.T) {
	tests := []struct {
		name        string
		config      DatabaseConfig
		expectError bool
	}{
		{
			name: "Valid connection string should establish connection",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "codechunking",
				Username: "dev",
				Password: "dev",
				Schema:   "public",
			},
			expectError: false,
		},
		{
			name: "Invalid host should fail connection",
			config: DatabaseConfig{
				Host:     "invalid-host",
				Port:     5432,
				Database: "codechunking",
				Username: "dev",
				Password: "dev",
				Schema:   "public",
			},
			expectError: true,
		},
		{
			name: "Invalid port should fail connection",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     9999,
				Database: "codechunking_test",
				Username: "postgres",
				Password: "password",
				Schema:   "codechunking",
			},
			expectError: true,
		},
		{
			name: "Invalid credentials should fail connection",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "codechunking_test",
				Username: "invalid_user",
				Password: "invalid_password",
				Schema:   "codechunking",
			},
			expectError: true,
		},
		{
			name: "Empty database name should fail connection",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "",
				Username: "postgres",
				Password: "password",
				Schema:   "codechunking",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn, err := NewDatabaseConnection(tt.config)

			if tt.expectError {
				validateConnectionErrorCase(t, conn, err)
				return
			}
			validateConnectionSuccessCase(t, conn, err)
		})
	}
}

// TestDatabaseConnection_ConfigValidation tests configuration validation.
func TestDatabaseConnection_ConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      DatabaseConfig
		expectError bool
		errorMsg    string
	}{
		{
			name: "Missing host should fail",
			config: DatabaseConfig{
				Port:     5432,
				Database: "test",
				Username: "user",
				Password: "pass",
				Schema:   "schema",
			},
			expectError: true,
			errorMsg:    "host is required",
		},
		{
			name: "Invalid port should fail",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     0,
				Database: "test",
				Username: "user",
				Password: "pass",
				Schema:   "schema",
			},
			expectError: true,
			errorMsg:    "port must be between 1 and 65535",
		},
		{
			name: "Missing database should fail",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Username: "user",
				Password: "pass",
				Schema:   "schema",
			},
			expectError: true,
			errorMsg:    "database is required",
		},
		{
			name: "Missing username should fail",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "test",
				Password: "pass",
				Schema:   "schema",
			},
			expectError: true,
			errorMsg:    "username is required",
		},
		{
			name: "Missing schema should fail",
			config: DatabaseConfig{
				Host:     "localhost",
				Port:     5432,
				Database: "test",
				Username: "user",
				Password: "pass",
			},
			expectError: true,
			errorMsg:    "schema is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()

			if tt.expectError {
				if err == nil {
					t.Error("Expected validation error but got none")
				}
				if err.Error() != tt.errorMsg {
					t.Errorf("Expected error message '%s', got '%s'", tt.errorMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("Expected no validation error but got: %v", err)
			}
		})
	}
}

// TestConnectionPool_Configuration tests connection pool configuration.
func TestConnectionPool_Configuration(t *testing.T) {
	config := DatabaseConfig{
		Host:            "localhost",
		Port:            5432,
		Database:        "codechunking",
		Username:        "dev",
		Password:        "dev",
		Schema:          "public",
		MaxConnections:  10,
		MinConnections:  2,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
	}

	// This test will fail because NewDatabaseConnection doesn't exist yet
	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Verify pool configuration
	stats := pool.Stat()

	if stats.MaxConns() != int32(config.MaxConnections) {
		t.Errorf("Expected max connections %d, got %d", config.MaxConnections, stats.MaxConns())
	}

	if stats.TotalConns() < 0 {
		t.Errorf("Expected min connections %d, got total %d", config.MinConnections, stats.TotalConns())
	}
}

// TestConnectionPool_Concurrent tests concurrent connection usage.
func TestConnectionPool_Concurrent(t *testing.T) {
	config := DatabaseConfig{
		Host:           "localhost",
		Port:           5432,
		Database:       "codechunking",
		Username:       "dev",
		Password:       "dev",
		Schema:         "public",
		MaxConnections: 5,
		MinConnections: 2,
	}

	// This test will fail because NewDatabaseConnection doesn't exist yet
	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Test concurrent connections
	const numGoroutines = 10
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	errCh := make(chan error, numGoroutines)
	done := make(chan bool, numGoroutines)

	for i := range numGoroutines {
		go func(_ int) {
			defer func() { done <- true }()

			conn, err := pool.Acquire(ctx)
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Release()

			// Simulate some work
			time.Sleep(100 * time.Millisecond)

			if err := conn.Ping(ctx); err != nil {
				errCh <- err
				return
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		select {
		case err := <-errCh:
			t.Errorf("Goroutine failed: %v", err)
		case <-done:
			// Success
		case <-time.After(10 * time.Second):
			t.Fatal("Test timeout")
		}
	}
}

// TestConnectionPool_Stats tests connection pool statistics.
func TestConnectionPool_Stats(t *testing.T) {
	config := DatabaseConfig{
		Host:           "localhost",
		Port:           5432,
		Database:       "codechunking",
		Username:       "dev",
		Password:       "dev",
		Schema:         "public",
		MaxConnections: 5,
		MinConnections: 2,
	}

	// This test will fail because NewDatabaseConnection doesn't exist yet
	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Get initial stats
	initialStats := pool.Stat()
	if initialStats.TotalConns() < int32(config.MinConnections) {
		t.Errorf("Expected at least %d initial connections, got %d",
			config.MinConnections, initialStats.TotalConns())
	}

	// Acquire some connections
	var conns []*pgxpool.Conn
	for range 3 {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			t.Fatalf("Failed to acquire connection: %v", err)
		}
		conns = append(conns, conn)
	}

	// Check stats after acquiring connections
	activeStats := pool.Stat()
	if activeStats.AcquiredConns() != 3 {
		t.Errorf("Expected 3 acquired connections, got %d", activeStats.AcquiredConns())
	}

	// Release connections
	for _, conn := range conns {
		conn.Release()
	}

	// Check stats after releasing connections
	releasedStats := pool.Stat()
	if releasedStats.AcquiredConns() != 0 {
		t.Errorf("Expected 0 acquired connections after release, got %d", releasedStats.AcquiredConns())
	}
}

// TestConnectionPool_HealthCheck tests connection pool health monitoring.
func TestConnectionPool_HealthCheck(t *testing.T) {
	config := DatabaseConfig{
		Host:           "localhost",
		Port:           5432,
		Database:       "codechunking",
		Username:       "dev",
		Password:       "dev",
		Schema:         "public",
		MaxConnections: 5,
		MinConnections: 2,
	}

	// This test will fail because NewDatabaseConnection doesn't exist yet
	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Test health check function
	healthChecker := NewDatabaseHealthChecker(pool)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	healthy := healthChecker.IsHealthy(ctx)
	if !healthy {
		t.Error("Expected database to be healthy")
	}

	// Test health check with metrics
	metrics := healthChecker.GetMetrics(ctx)
	if metrics == nil {
		t.Error("Expected health metrics but got nil")
		return
	}

	if metrics.TotalConnections == 0 {
		t.Error("Expected total connections > 0")
	}

	if metrics.ActiveConnections < 0 {
		t.Error("Expected active connections >= 0")
	}
}

// =============================================================================
// TTL-Based Caching Tests for DatabaseHealthChecker.GetMetrics
// These tests define the expected caching behavior and will FAIL with the current implementation
// =============================================================================

// TestDatabaseHealthChecker_CacheBasicBehavior tests basic cache hit and miss scenarios.
func TestDatabaseHealthChecker_CacheBasicBehavior(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}

	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Test cache configuration
	cacheConfig := HealthCheckCacheConfig{
		TTL:     3 * time.Second,
		Enabled: true,
	}

	// This constructor should accept cache configuration (will fail - doesn't exist yet)
	healthChecker := NewDatabaseHealthCheckerWithCache(pool, cacheConfig)
	ctx := context.Background()

	// First call - should cache the metrics
	metrics1 := healthChecker.GetMetrics(ctx)
	if metrics1 == nil {
		t.Fatal("Expected metrics but got nil")
	}

	// Second call immediately after - should return cached metrics
	metrics2 := healthChecker.GetMetrics(ctx)
	if metrics2 == nil {
		t.Fatal("Expected cached metrics but got nil")
	}

	// Cached metrics should be identical (same ResponseTime, same values)
	if metrics1.ResponseTime != metrics2.ResponseTime {
		t.Errorf("Expected cached metrics to have identical ResponseTime: %v != %v",
			metrics1.ResponseTime, metrics2.ResponseTime)
	}

	if metrics1.TotalConnections != metrics2.TotalConnections {
		t.Errorf("Expected cached metrics to have identical TotalConnections: %d != %d",
			metrics1.TotalConnections, metrics2.TotalConnections)
	}

	if metrics1.ActiveConnections != metrics2.ActiveConnections {
		t.Errorf("Expected cached metrics to have identical ActiveConnections: %d != %d",
			metrics1.ActiveConnections, metrics2.ActiveConnections)
	}

	if metrics1.IdleConnections != metrics2.IdleConnections {
		t.Errorf("Expected cached metrics to have identical IdleConnections: %d != %d",
			metrics1.IdleConnections, metrics2.IdleConnections)
	}
}

// TestDatabaseHealthChecker_CacheTTLExpiration tests that cache expires after TTL.
func TestDatabaseHealthChecker_CacheTTLExpiration(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}

	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	// Short TTL for testing
	cacheConfig := HealthCheckCacheConfig{
		TTL:     100 * time.Millisecond,
		Enabled: true,
	}

	// This constructor should accept cache configuration (will fail - doesn't exist yet)
	healthChecker := NewDatabaseHealthCheckerWithCache(pool, cacheConfig)
	ctx := context.Background()

	// First call - caches metrics
	metrics1 := healthChecker.GetMetrics(ctx)
	if metrics1 == nil {
		t.Fatal("Expected metrics but got nil")
	}

	// Second call within TTL - should return cached metrics
	time.Sleep(50 * time.Millisecond) // Half the TTL
	metrics2 := healthChecker.GetMetrics(ctx)

	// Should be cached (identical ResponseTime)
	if metrics1.ResponseTime != metrics2.ResponseTime {
		t.Errorf("Expected cached metrics within TTL to have identical ResponseTime")
	}

	// Third call after TTL expiry - should fetch fresh metrics
	time.Sleep(100 * time.Millisecond) // Wait for TTL to expire
	metrics3 := healthChecker.GetMetrics(ctx)

	// Should be fresh (potentially different ResponseTime)
	// We can't guarantee different values, but the behavior should be to fetch fresh data
	if metrics3 == nil {
		t.Fatal("Expected fresh metrics after TTL expiry but got nil")
	}

	// The implementation should have called pool.Ping() again for fresh metrics
	// This is indirectly tested by ensuring the caching mechanism respects TTL
}

// TestDatabaseHealthChecker_CacheConcurrentAccess tests thread safety.
func TestDatabaseHealthChecker_CacheConcurrentAccess(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}

	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	cacheConfig := HealthCheckCacheConfig{
		TTL:     2 * time.Second,
		Enabled: true,
	}

	// This constructor should accept cache configuration (will fail - doesn't exist yet)
	healthChecker := NewDatabaseHealthCheckerWithCache(pool, cacheConfig)
	ctx := context.Background()

	const numGoroutines = 50
	const callsPerGoroutine = 10

	var wg sync.WaitGroup
	metricsChan := make(chan *HealthMetrics, numGoroutines*callsPerGoroutine)
	errorChan := make(chan error, numGoroutines*callsPerGoroutine)

	// Launch concurrent goroutines calling GetMetrics
	for i := range numGoroutines {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := range callsPerGoroutine {
				metrics := healthChecker.GetMetrics(ctx)
				if metrics == nil {
					errorChan <- fmt.Errorf("goroutine %d call %d: got nil metrics", routineID, j)
					continue
				}
				metricsChan <- metrics
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(metricsChan)
	close(errorChan)

	// Check for errors
	for err := range errorChan {
		t.Error(err)
	}

	// Collect all metrics
	var allMetrics []*HealthMetrics
	for metrics := range metricsChan {
		allMetrics = append(allMetrics, metrics)
	}

	if len(allMetrics) != numGoroutines*callsPerGoroutine {
		t.Errorf("Expected %d metrics, got %d", numGoroutines*callsPerGoroutine, len(allMetrics))
	}

	// Verify that most calls returned cached metrics (should have similar ResponseTime values)
	// This test primarily ensures no race conditions occur
}

// TestDatabaseHealthChecker_CacheConfiguration tests different cache configurations.
func TestDatabaseHealthChecker_CacheConfiguration(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}

	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	tests := []struct {
		name        string
		cacheConfig HealthCheckCacheConfig
		expectCache bool
	}{
		{
			name: "Cache enabled with 2 second TTL",
			cacheConfig: HealthCheckCacheConfig{
				TTL:     2 * time.Second,
				Enabled: true,
			},
			expectCache: true,
		},
		{
			name: "Cache enabled with 5 second TTL",
			cacheConfig: HealthCheckCacheConfig{
				TTL:     5 * time.Second,
				Enabled: true,
			},
			expectCache: true,
		},
		{
			name: "Cache disabled",
			cacheConfig: HealthCheckCacheConfig{
				TTL:     2 * time.Second,
				Enabled: false,
			},
			expectCache: false,
		},
		{
			name: "Zero TTL (should disable caching)",
			cacheConfig: HealthCheckCacheConfig{
				TTL:     0,
				Enabled: true,
			},
			expectCache: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This constructor should accept cache configuration (will fail - doesn't exist yet)
			healthChecker := NewDatabaseHealthCheckerWithCache(pool, tt.cacheConfig)
			ctx := context.Background()

			// Get metrics twice
			metrics1 := healthChecker.GetMetrics(ctx)
			metrics2 := healthChecker.GetMetrics(ctx)

			if metrics1 == nil || metrics2 == nil {
				t.Fatal("Expected metrics but got nil")
			}

			if tt.expectCache {
				// Should be cached - identical ResponseTime
				if metrics1.ResponseTime != metrics2.ResponseTime {
					t.Errorf("Expected cached metrics to have identical ResponseTime when caching enabled")
				}
			}
			// Note: When caching is disabled, we can't guarantee different ResponseTime values
			// The test primarily validates that the configuration is respected
		})
	}
}

// TestDatabaseHealthChecker_CachePerformance tests that caching improves performance.
func TestDatabaseHealthChecker_CachePerformance(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}

	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	ctx := context.Background()

	// Test without caching (current behavior)
	nonCachedChecker := NewDatabaseHealthChecker(pool)

	// Time multiple calls without caching
	nonCachedStart := time.Now()
	for range 10 {
		metrics := nonCachedChecker.GetMetrics(ctx)
		if metrics == nil {
			t.Fatal("Expected metrics but got nil")
		}
	}
	nonCachedDuration := time.Since(nonCachedStart)

	// Test with caching
	cacheConfig := HealthCheckCacheConfig{
		TTL:     5 * time.Second,
		Enabled: true,
	}

	// This constructor should accept cache configuration (will fail - doesn't exist yet)
	cachedChecker := NewDatabaseHealthCheckerWithCache(pool, cacheConfig)

	// Time multiple calls with caching (first call populates cache, rest use cache)
	cachedStart := time.Now()
	for range 10 {
		metrics := cachedChecker.GetMetrics(ctx)
		if metrics == nil {
			t.Fatal("Expected cached metrics but got nil")
		}
	}
	cachedDuration := time.Since(cachedStart)

	// Cached calls should be significantly faster
	// First call will be similar speed, but subsequent 9 calls should be much faster
	if cachedDuration >= nonCachedDuration {
		t.Errorf("Expected cached calls to be faster. Non-cached: %v, Cached: %v",
			nonCachedDuration, cachedDuration)
	}

	// Log the performance difference for information
	t.Logf("Performance improvement: Non-cached: %v, Cached: %v (%.2fx faster)",
		nonCachedDuration, cachedDuration, float64(nonCachedDuration)/float64(cachedDuration))
}

// TestDatabaseHealthChecker_CacheWithNilPool tests cache behavior with nil pool.
func TestDatabaseHealthChecker_CacheWithNilPool(t *testing.T) {
	cacheConfig := HealthCheckCacheConfig{
		TTL:     2 * time.Second,
		Enabled: true,
	}

	// This constructor should accept cache configuration (will fail - doesn't exist yet)
	healthChecker := NewDatabaseHealthCheckerWithCache(nil, cacheConfig)
	ctx := context.Background()

	// Should handle nil pool gracefully
	metrics := healthChecker.GetMetrics(ctx)
	if metrics != nil {
		t.Error("Expected nil metrics for nil pool but got metrics")
	}
}

// TestDatabaseHealthChecker_CacheContextCancellation tests cache behavior with context cancellation.
func TestDatabaseHealthChecker_CacheContextCancellation(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}

	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	cacheConfig := HealthCheckCacheConfig{
		TTL:     5 * time.Second,
		Enabled: true,
	}

	// This constructor should accept cache configuration (will fail - doesn't exist yet)
	healthChecker := NewDatabaseHealthCheckerWithCache(pool, cacheConfig)

	// First call to populate cache
	ctx1 := context.Background()
	metrics1 := healthChecker.GetMetrics(ctx1)
	if metrics1 == nil {
		t.Fatal("Expected metrics but got nil")
	}

	// Second call with cancelled context - should still return cached metrics
	ctx2, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	metrics2 := healthChecker.GetMetrics(ctx2)
	if metrics2 == nil {
		t.Fatal("Expected cached metrics even with cancelled context")
	}

	// Cached metrics should be returned despite context cancellation
	if metrics1.ResponseTime != metrics2.ResponseTime {
		t.Error("Expected cached metrics to be identical even with cancelled context")
	}
}

// TestDatabaseHealthChecker_CacheMetricsFreshness tests that cached metrics represent accurate state.
func TestDatabaseHealthChecker_CacheMetricsFreshness(t *testing.T) {
	config := DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "codechunking",
		Username: "dev",
		Password: "dev",
		Schema:   "public",
	}

	pool, err := NewDatabaseConnection(config)
	if err != nil {
		t.Fatalf("Failed to create connection pool: %v", err)
	}
	defer pool.Close()

	cacheConfig := HealthCheckCacheConfig{
		TTL:     500 * time.Millisecond,
		Enabled: true,
	}

	// This constructor should accept cache configuration (will fail - doesn't exist yet)
	healthChecker := NewDatabaseHealthCheckerWithCache(pool, cacheConfig)
	ctx := context.Background()

	// Get initial metrics
	initialMetrics := healthChecker.GetMetrics(ctx)
	if initialMetrics == nil {
		t.Fatal("Expected initial metrics but got nil")
	}

	// Cached call should return same metrics
	cachedMetrics := healthChecker.GetMetrics(ctx)
	if cachedMetrics.ResponseTime != initialMetrics.ResponseTime {
		t.Error("Expected cached metrics to have identical ResponseTime")
	}

	// Wait for cache to expire
	time.Sleep(600 * time.Millisecond)

	// Fresh call should potentially have different metrics
	freshMetrics := healthChecker.GetMetrics(ctx)
	if freshMetrics == nil {
		t.Fatal("Expected fresh metrics after cache expiry but got nil")
	}

	// Fresh metrics should be valid
	if freshMetrics.TotalConnections < 0 {
		t.Error("Expected valid TotalConnections in fresh metrics")
	}
	if freshMetrics.ActiveConnections < 0 {
		t.Error("Expected valid ActiveConnections in fresh metrics")
	}
	if freshMetrics.IdleConnections < 0 {
		t.Error("Expected valid IdleConnections in fresh metrics")
	}
	if freshMetrics.ResponseTime < 0 {
		t.Error("Expected valid ResponseTime in fresh metrics")
	}
}

// Helper functions for TestNewDatabaseConnection to reduce nesting complexity

// validateConnectionErrorCase validates that an error case returns expected error state.
func validateConnectionErrorCase(t *testing.T, conn *pgxpool.Pool, err error) {
	if err == nil {
		t.Error("Expected error but got none")
	}
	if conn != nil {
		t.Error("Expected nil connection on error")
	}
}

// validateConnectionSuccessCase validates that a success case returns a working connection.
func validateConnectionSuccessCase(t *testing.T, conn *pgxpool.Pool, err error) {
	if err != nil {
		t.Errorf("Expected no error but got: %v", err)
		return
	}
	if conn == nil {
		t.Error("Expected valid connection but got nil")
		return
	}

	// Test connection is actually usable
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		t.Errorf("Connection ping failed: %v", err)
	}

	// Clean up
	conn.Close()
}
