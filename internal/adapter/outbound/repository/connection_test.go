package repository

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestDatabaseConnection_NewConnection tests database connection establishment
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
				if err == nil {
					t.Error("Expected error but got none")
				}
				if conn != nil {
					t.Error("Expected nil connection on error")
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error but got: %v", err)
				}
				if conn == nil {
					t.Error("Expected valid connection but got nil")
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
		})
	}
}

// TestDatabaseConnection_ConfigValidation tests configuration validation
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
			} else {
				if err != nil {
					t.Errorf("Expected no validation error but got: %v", err)
				}
			}
		})
	}
}

// TestConnectionPool_Configuration tests connection pool configuration
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

// TestConnectionPool_Concurrent tests concurrent connection usage
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

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
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
	for i := 0; i < numGoroutines; i++ {
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

// TestConnectionPool_Stats tests connection pool statistics
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
	for i := 0; i < 3; i++ {
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

// TestConnectionPool_HealthCheck tests connection pool health monitoring
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
	}

	if metrics.TotalConnections == 0 {
		t.Error("Expected total connections > 0")
	}

	if metrics.ActiveConnections < 0 {
		t.Error("Expected active connections >= 0")
	}
}
