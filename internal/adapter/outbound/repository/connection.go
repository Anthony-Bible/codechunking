package repository

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseConfig represents database connection configuration.
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

// Validate validates the database configuration.
func (c DatabaseConfig) Validate() error {
	if c.Host == "" {
		return errors.New("host is required")
	}
	if c.Port <= 0 || c.Port > 65535 {
		return errors.New("port must be between 1 and 65535")
	}
	if c.Database == "" {
		return errors.New("database is required")
	}
	if c.Username == "" {
		return errors.New("username is required")
	}
	if c.Schema == "" {
		return errors.New("schema is required")
	}
	return nil
}

// NewDatabaseConnection creates a new database connection pool.
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

	if pingErr := pool.Ping(ctx); pingErr != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", pingErr)
	}

	return pool, nil
}

// Default cache configuration constants.
const (
	DefaultCacheTTL     = 5 * time.Second
	DefaultCacheEnabled = false
)

// HealthCheckCacheConfig represents configuration for health check metrics caching.
type HealthCheckCacheConfig struct {
	TTL     time.Duration
	Enabled bool
}

// IsValid returns true if the cache configuration is valid for caching.
func (c HealthCheckCacheConfig) IsValid() bool {
	return c.Enabled && c.TTL > 0
}

// metricsCache handles caching of health metrics with TTL support.
type metricsCache struct {
	data      *HealthMetrics
	timestamp time.Time
	mutex     sync.RWMutex
	config    HealthCheckCacheConfig
}

// newMetricsCache creates a new metrics cache with the given configuration.
func newMetricsCache(config HealthCheckCacheConfig) *metricsCache {
	return &metricsCache{
		config: config,
	}
}

// get retrieves cached metrics or fetches fresh ones using the provided fetcher function.
func (c *metricsCache) get(ctx context.Context, fetcher func(context.Context) *HealthMetrics) *HealthMetrics {
	// If caching is disabled, always fetch fresh
	if !c.config.IsValid() {
		return fetcher(ctx)
	}

	// Check cache with read lock first
	c.mutex.RLock()
	if c.data != nil && !c.isExpired() {
		cached := c.data
		c.mutex.RUnlock()
		return cached
	}
	c.mutex.RUnlock()

	// Cache miss or expired, fetch fresh with write lock
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Double-check pattern in case another goroutine updated cache
	if c.data != nil && !c.isExpired() {
		return c.data
	}

	// Fetch fresh metrics
	metrics := fetcher(ctx)
	c.data = metrics
	c.timestamp = time.Now()
	return metrics
}

// isExpired returns true if the cached data has expired.
func (c *metricsCache) isExpired() bool {
	return time.Since(c.timestamp) >= c.config.TTL
}

// metricsCollector handles collection of database health metrics.
type metricsCollector struct {
	pool *pgxpool.Pool
}

// newMetricsCollector creates a new metrics collector for the given pool.
func newMetricsCollector(pool *pgxpool.Pool) *metricsCollector {
	return &metricsCollector{
		pool: pool,
	}
}

// isPoolValid returns true if the pool is valid for operations.
func (c *metricsCollector) isPoolValid() bool {
	return c.pool != nil
}

// collect gathers fresh health metrics from the database.
func (c *metricsCollector) collect(ctx context.Context) *HealthMetrics {
	if !c.isPoolValid() {
		return nil
	}

	start := time.Now()

	// Test response time with a ping (ignore error for metrics collection)
	_ = c.pool.Ping(ctx)
	responseTime := time.Since(start)

	stats := c.pool.Stat()

	return &HealthMetrics{
		TotalConnections:  stats.TotalConns(),
		ActiveConnections: stats.AcquiredConns(),
		IdleConnections:   stats.IdleConns(),
		ResponseTime:      responseTime,
	}
}

// HealthCheckerOption defines functional options for DatabaseHealthChecker.
type HealthCheckerOption func(*DatabaseHealthChecker)

// WithCache enables caching with the specified configuration.
func WithCache(config HealthCheckCacheConfig) HealthCheckerOption {
	return func(hc *DatabaseHealthChecker) {
		hc.cache = newMetricsCache(config)
	}
}

// DatabaseHealthChecker checks database health with optional caching.
type DatabaseHealthChecker struct {
	collector *metricsCollector
	cache     *metricsCache
}

// NewDatabaseHealthChecker creates a new health checker with optional caching.
func NewDatabaseHealthChecker(pool *pgxpool.Pool, opts ...HealthCheckerOption) *DatabaseHealthChecker {
	hc := &DatabaseHealthChecker{
		collector: newMetricsCollector(pool),
		cache:     newMetricsCache(HealthCheckCacheConfig{TTL: DefaultCacheTTL, Enabled: DefaultCacheEnabled}),
	}

	// Apply options
	for _, opt := range opts {
		opt(hc)
	}

	return hc
}

// NewDatabaseHealthCheckerWithCache creates a new health checker with caching support
// Deprecated: Use NewDatabaseHealthChecker with WithCache option instead.
func NewDatabaseHealthCheckerWithCache(pool *pgxpool.Pool, config HealthCheckCacheConfig) *DatabaseHealthChecker {
	return NewDatabaseHealthChecker(pool, WithCache(config))
}

// IsHealthy checks if the database is healthy.
func (h *DatabaseHealthChecker) IsHealthy(ctx context.Context) bool {
	if !h.collector.isPoolValid() {
		return false
	}

	if err := h.collector.pool.Ping(ctx); err != nil {
		return false
	}

	return true
}

// HealthMetrics represents database health metrics.
type HealthMetrics struct {
	TotalConnections  int32
	ActiveConnections int32
	IdleConnections   int32
	ResponseTime      time.Duration
}

// GetMetrics returns database health metrics with optional caching.
func (h *DatabaseHealthChecker) GetMetrics(ctx context.Context) *HealthMetrics {
	return h.cache.get(ctx, h.collector.collect)
}
