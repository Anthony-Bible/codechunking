package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseConfig represents database connection configuration
type DatabaseConfig struct {
	Host            string
	Port            int
	Database        string
	Username        string
	Password        string
	Schema          string
	MaxConnections  int
	MinConnections  int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
	SSLMode         string
}

// Validate validates the database configuration
func (c DatabaseConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if c.Database == "" {
		return fmt.Errorf("database is required")
	}
	if c.Username == "" {
		return fmt.Errorf("username is required")
	}
	if c.Schema == "" {
		return fmt.Errorf("schema is required")
	}
	return nil
}

// NewDatabaseConnection creates a new database connection pool
func NewDatabaseConnection(config DatabaseConfig) (*pgxpool.Pool, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	// Build connection string
	sslMode := config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	connString := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=%s search_path=%s",
		config.Host, config.Port, config.Database, config.Username, config.Password, sslMode, config.Schema,
	)

	// Parse connection string and create pool config
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Configure connection pool
	if config.MaxConnections > 0 {
		poolConfig.MaxConns = int32(config.MaxConnections)
	} else {
		poolConfig.MaxConns = 10 // default
	}

	if config.MinConnections >= 0 {
		poolConfig.MinConns = int32(config.MinConnections)
	} else {
		poolConfig.MinConns = 0 // default
	}

	if config.ConnMaxLifetime > 0 {
		poolConfig.MaxConnLifetime = config.ConnMaxLifetime
	}

	if config.ConnMaxIdleTime > 0 {
		poolConfig.MaxConnIdleTime = config.ConnMaxIdleTime
	}

	// Create the pool
	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return pool, nil
}

// DatabaseHealthChecker checks database health
type DatabaseHealthChecker struct {
	pool *pgxpool.Pool
}

// NewDatabaseHealthChecker creates a new health checker
func NewDatabaseHealthChecker(pool *pgxpool.Pool) *DatabaseHealthChecker {
	return &DatabaseHealthChecker{
		pool: pool,
	}
}

// IsHealthy checks if the database is healthy
func (h *DatabaseHealthChecker) IsHealthy(ctx context.Context) bool {
	if h.pool == nil {
		return false
	}

	if err := h.pool.Ping(ctx); err != nil {
		return false
	}

	return true
}

// HealthMetrics represents database health metrics
type HealthMetrics struct {
	TotalConnections  int32
	ActiveConnections int32
	IdleConnections   int32
	ResponseTime      time.Duration
}

// GetMetrics returns database health metrics
func (h *DatabaseHealthChecker) GetMetrics(ctx context.Context) *HealthMetrics {
	if h.pool == nil {
		return nil
	}

	start := time.Now()

	// Test response time with a ping
	_ = h.pool.Ping(ctx)
	responseTime := time.Since(start)

	stats := h.pool.Stat()

	return &HealthMetrics{
		TotalConnections:  stats.TotalConns(),
		ActiveConnections: stats.AcquiredConns(),
		IdleConnections:   stats.IdleConns(),
		ResponseTime:      responseTime,
	}
}
