package slogger

import (
	"codechunking/internal/application/common/logging"
	"context"
	"sync"
)

// Fields is an alias for logging.Fields for convenience.
type Fields = logging.Fields

// LoggerManager manages logger instances with proper encapsulation.
type LoggerManager struct {
	mu          sync.RWMutex
	logger      logging.ApplicationLogger
	initialized bool
}

var (
	defaultManagerInstance *LoggerManager //nolint:gochecknoglobals // Required for singleton logging infrastructure
	defaultManagerOnce     sync.Once      //nolint:gochecknoglobals // Required for thread-safe singleton initialization
)

// getDefaultManager returns the singleton logger manager instance.
func getDefaultManager() *LoggerManager {
	defaultManagerOnce.Do(func() {
		defaultManagerInstance = &LoggerManager{}
	})
	return defaultManagerInstance
}

// initLogger initializes the logger using double-checked locking.
func (lm *LoggerManager) initLogger() {
	lm.mu.RLock()
	if lm.initialized {
		lm.mu.RUnlock()
		return
	}
	lm.mu.RUnlock()

	lm.mu.Lock()
	defer lm.mu.Unlock()
	if lm.initialized {
		return
	}

	config := logging.Config{
		Level:            "INFO",
		Format:           "json",
		Output:           "stdout",
		EnableColors:     false,
		TimestampFormat:  "",
		EnableStackTrace: false,
	}

	logger, err := logging.NewApplicationLogger(config)
	if err != nil {
		panic("Failed to initialize logger: " + err.Error())
	}
	lm.logger = logger
	lm.initialized = true
}

// getLogger returns the logger instance, initializing it if necessary.
func (lm *LoggerManager) getLogger() logging.ApplicationLogger {
	lm.initLogger()
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	return lm.logger
}

// SetLogger allows setting a custom logger (useful for testing).
// Passing nil resets the manager so it will auto-initialize on next use.
func (lm *LoggerManager) SetLogger(logger logging.ApplicationLogger) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.logger = logger
	lm.initialized = logger != nil
}

// getLogger returns the default logger instance.
func getLogger() logging.ApplicationLogger {
	return getDefaultManager().getLogger()
}

// SetGlobalLogger allows setting a custom global logger (useful for testing).
func SetGlobalLogger(logger logging.ApplicationLogger) {
	getDefaultManager().SetLogger(logger)
}

// Context-aware logging functions (preferred)

// Debug logs a debug message with context.
func Debug(ctx context.Context, msg string, fields Fields) {
	getLogger().Debug(ctx, msg, fields)
}

// Info logs an info message with context.
func Info(ctx context.Context, msg string, fields Fields) {
	getLogger().Info(ctx, msg, fields)
}

// Warn logs a warning message with context.
func Warn(ctx context.Context, msg string, fields Fields) {
	getLogger().Warn(ctx, msg, fields)
}

// Error logs an error message with context.
func Error(ctx context.Context, msg string, fields Fields) {
	getLogger().Error(ctx, msg, fields)
}

// ErrorWithError logs an error message with an error object and context.
func ErrorWithError(ctx context.Context, err error, msg string, fields Fields) {
	getLogger().ErrorWithError(ctx, err, msg, fields)
}

// No-context fallback functions (for easy migration from global slog)

// DebugNoCtx logs a debug message without context (uses background context).
func DebugNoCtx(msg string, fields Fields) {
	getLogger().Debug(context.Background(), msg, fields)
}

// InfoNoCtx logs an info message without context (uses background context).
func InfoNoCtx(msg string, fields Fields) {
	getLogger().Info(context.Background(), msg, fields)
}

// WarnNoCtx logs a warning message without context (uses background context).
func WarnNoCtx(msg string, fields Fields) {
	getLogger().Warn(context.Background(), msg, fields)
}

// ErrorNoCtx logs an error message without context (uses background context).
func ErrorNoCtx(msg string, fields Fields) {
	getLogger().Error(context.Background(), msg, fields)
}

// ErrorWithErrorNoCtx logs an error message with an error object without context.
func ErrorWithErrorNoCtx(err error, msg string, fields Fields) {
	getLogger().ErrorWithError(context.Background(), err, msg, fields)
}

// Helper functions for creating Fields

// Field creates a single-field Fields map.
func Field(key string, value interface{}) Fields {
	return Fields{key: value}
}

// Fields2 creates a Fields map with two key-value pairs.
func Fields2(k1 string, v1 interface{}, k2 string, v2 interface{}) Fields {
	return Fields{k1: v1, k2: v2}
}

// Fields3 creates a Fields map with three key-value pairs.
func Fields3(k1 string, v1 interface{}, k2 string, v2 interface{}, k3 string, v3 interface{}) Fields {
	return Fields{k1: v1, k2: v2, k3: v3}
}

// WithComponent returns a logger with a specific component name.
func WithComponent(component string) logging.ApplicationLogger {
	return getLogger().WithComponent(component)
}
