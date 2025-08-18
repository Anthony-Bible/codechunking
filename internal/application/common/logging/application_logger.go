package logging

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codechunking/internal/adapter/inbound/api/middleware"

	"github.com/google/uuid"
)

const (
	// Sampling constants.
	SamplingBase = 100

	// Log level constants.
	DebugLevel = 0
	InfoLevel  = 1
	WarnLevel  = 2
	ErrorLevel = 3

	// Memory monitoring constants.
	MemoryMonitorInterval = 30 // seconds
	BytesToMB             = 1024

	// Performance calculation constants.
	LatencyAverageDiv = 2
)

// ApplicationLogger defines the interface for structured application logging.
type ApplicationLogger interface {
	Debug(ctx context.Context, message string, fields Fields)
	Info(ctx context.Context, message string, fields Fields)
	Warn(ctx context.Context, message string, fields Fields)
	Error(ctx context.Context, message string, fields Fields)
	ErrorWithError(ctx context.Context, err error, message string, fields Fields)
	LogPerformance(ctx context.Context, operation string, duration time.Duration, fields Fields)
	WithComponent(component string) ApplicationLogger

	// Extended NATS operations
	LogNATSConnectionEvent(ctx context.Context, event NATSConnectionEvent)
	LogNATSPublishEvent(ctx context.Context, event NATSPublishEvent)
	LogNATSConsumeEvent(ctx context.Context, event NATSConsumeEvent)
	LogJetStreamEvent(ctx context.Context, event JetStreamEvent)
	LogNATSPerformanceMetrics(ctx context.Context, metrics NATSPerformanceMetrics)

	// Extended metrics operations
	LogMetric(ctx context.Context, metric PrometheusMetric)
	LogApplicationMetrics(ctx context.Context, metrics ApplicationMetrics)
	LogHealthMetrics(ctx context.Context, health HealthMetrics)
	LogCustomMetrics(ctx context.Context, custom CustomMetrics)
	FlushAggregatedMetrics(ctx context.Context)
}

// Fields represents structured logging fields.
type Fields map[string]interface{}

// Config represents logger configuration.
type Config struct {
	Level            string
	Format           string // json, text
	Output           string // stdout, stderr, file, buffer (for testing)
	EnableColors     bool
	TimestampFormat  string
	EnableStackTrace bool
	FilePath         string
	MaxFileSize      int64
	MaxBackups       int
	MaxAge           int
	MetricsConfig    MetricsConfig
	// Performance settings
	EnableObjectPooling bool
	EnableAsyncLogging  bool
	AsyncBufferSize     int
	EnableSampling      bool
	SamplingRate        float64
	MaxMemoryMB         int64
	MaxEntriesPerSecond int
}

// Performance optimization structures.
type logEntryPool struct {
	pool sync.Pool
}

type asyncLogEvent struct {
	entry LogEntry
	ts    time.Time
}

type performanceMetrics struct {
	totalEntries      int64
	totalBytesWritten int64
	averageLatency    int64 // nanoseconds
	errorsCount       int64
	droppedEntries    int64
	memoryUsage       int64
}

// Global object pool for log entries.
var (
	globalLogEntryPool = &logEntryPool{ //nolint:gochecknoglobals // Performance-critical object pool for logging infrastructure
		pool: sync.Pool{
			New: func() interface{} {
				return &LogEntry{
					Metadata: make(map[string]interface{}),
					Context:  make(map[string]interface{}),
				}
			},
		},
	}
	globalPerformanceMetrics performanceMetrics //nolint:gochecknoglobals // Performance-critical metrics for logging infrastructure
)

// Constants for repeated strings.
const (
	logOutputBuffer = "buffer"
	logFormatJSON   = "json"
)

// applicationLoggerImpl implements ApplicationLogger and extended interfaces.
type applicationLoggerImpl struct {
	config    Config
	component string
	buffer    *bytes.Buffer // For testing
	logger    *slog.Logger
	// Performance optimization fields
	asyncChannel chan asyncLogEvent
	metrics      *performanceMetrics
	sampler      *logSampler
	shutdown     chan struct{}
	wg           sync.WaitGroup
}

// LogEntry represents the structure of log entries.
type LogEntry struct {
	Timestamp     string                 `json:"timestamp"`
	Level         string                 `json:"level"`
	Message       string                 `json:"message"`
	CorrelationID string                 `json:"correlation_id"`
	Component     string                 `json:"component"`
	Operation     string                 `json:"operation,omitempty"`
	Duration      string                 `json:"duration,omitempty"`
	Error         string                 `json:"error,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Context       map[string]interface{} `json:"context,omitempty"`
}

// Context keys for correlation ID management.
type contextKey string

const (
	CorrelationIDKey contextKey = "correlation_id"
	RequestIDKey     contextKey = "request_id"
	UserContextKey   contextKey = "user_context"
)

// UserContext holds user-specific context information.
type UserContext struct {
	UserID    string
	ClientIP  string
	UserAgent string
}

// logSampler implements intelligent log sampling.
type logSampler struct {
	mutex         sync.RWMutex
	rate          float64
	counter       int64
	lastReset     time.Time
	entriesPerSec int
	maxPerSec     int
}

// newLogSampler creates a new log sampler.
func newLogSampler(rate float64, maxPerSec int) *logSampler {
	return &logSampler{
		rate:      rate,
		lastReset: time.Now(),
		maxPerSec: maxPerSec,
	}
}

// shouldSample determines if a log entry should be sampled.
func (s *logSampler) shouldSample() bool {
	if s.rate >= 1.0 && s.maxPerSec <= 0 {
		return true
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	now := time.Now()
	// Reset counter every second
	if now.Sub(s.lastReset) > time.Second {
		s.entriesPerSec = 0
		s.lastReset = now
	}

	// Check rate limit
	if s.maxPerSec > 0 && s.entriesPerSec >= s.maxPerSec {
		atomic.AddInt64(&globalPerformanceMetrics.droppedEntries, 1)
		return false
	}

	// Sample based on rate
	atomic.AddInt64(&s.counter, 1)
	if s.rate < 1.0 {
		if float64(s.counter%SamplingBase)/float64(SamplingBase) > s.rate {
			atomic.AddInt64(&globalPerformanceMetrics.droppedEntries, 1)
			return false
		}
	}

	s.entriesPerSec++
	return true
}

// NewApplicationLogger creates a new application logger with performance optimizations.
func NewApplicationLogger(config Config) (ApplicationLogger, error) {
	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	// Set default performance values if not specified
	if config.AsyncBufferSize == 0 {
		config.AsyncBufferSize = 1000
	}
	if config.MaxEntriesPerSecond == 0 {
		config.MaxEntriesPerSecond = 10000 // Default rate limit
	}
	if config.SamplingRate == 0 {
		config.SamplingRate = 1.0 // No sampling by default
	}

	logger := &applicationLoggerImpl{
		config:   config,
		metrics:  &performanceMetrics{},
		shutdown: make(chan struct{}),
	}

	// Initialize sampler if sampling is enabled
	if config.EnableSampling {
		logger.sampler = newLogSampler(config.SamplingRate, config.MaxEntriesPerSecond)
	}

	// Set up output destination
	switch config.Output {
	case logOutputBuffer:
		logger.buffer = &bytes.Buffer{}
		logger.logger = slog.New(slog.NewTextHandler(logger.buffer, &slog.HandlerOptions{}))
	case "stderr":
		logger.logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{}))
	case "stdout":
		fallthrough
	default:
		logger.logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{}))
	}

	// Initialize async logging if enabled, but not for buffer output (testing)
	if config.EnableAsyncLogging && config.Output != logOutputBuffer {
		logger.asyncChannel = make(chan asyncLogEvent, config.AsyncBufferSize)
		logger.startAsyncProcessor()
	}

	// Start memory monitoring
	if config.MaxMemoryMB > 0 {
		logger.startMemoryMonitor()
	}

	return logger, nil
}

// validateConfig validates logger configuration.
func validateConfig(config Config) error {
	validLevels := []string{"DEBUG", "INFO", "WARN", "ERROR"}
	levelValid := false
	for _, level := range validLevels {
		if strings.ToUpper(config.Level) == level {
			levelValid = true
			break
		}
	}
	if !levelValid {
		return fmt.Errorf("invalid log level: %s", config.Level)
	}

	validFormats := []string{logFormatJSON, "text"}
	formatValid := false
	for _, format := range validFormats {
		if config.Format == format {
			formatValid = true
			break
		}
	}
	if !formatValid {
		return fmt.Errorf("invalid log format: %s", config.Format)
	}

	validOutputs := []string{"stdout", "stderr", "file", logOutputBuffer}
	outputValid := false
	for _, output := range validOutputs {
		if config.Output == output {
			outputValid = true
			break
		}
	}
	if !outputValid {
		return fmt.Errorf("invalid log output: %s", config.Output)
	}

	return nil
}

// shouldLog determines if a message should be logged based on level.
func (l *applicationLoggerImpl) shouldLog(level string) bool {
	levels := map[string]int{
		"DEBUG": DebugLevel,
		"INFO":  InfoLevel,
		"WARN":  WarnLevel,
		"ERROR": ErrorLevel,
	}

	configLevel := levels[strings.ToUpper(l.config.Level)]
	messageLevel := levels[level]

	return messageLevel >= configLevel
}

// Debug logs debug messages.
func (l *applicationLoggerImpl) Debug(ctx context.Context, message string, fields Fields) {
	if l.shouldLog("DEBUG") {
		l.logEntry(ctx, "DEBUG", message, "", fields)
	}
}

// Info logs info messages.
func (l *applicationLoggerImpl) Info(ctx context.Context, message string, fields Fields) {
	if l.shouldLog("INFO") {
		l.logEntry(ctx, "INFO", message, "", fields)
	}
}

// Warn logs warning messages.
func (l *applicationLoggerImpl) Warn(ctx context.Context, message string, fields Fields) {
	if l.shouldLog("WARN") {
		l.logEntry(ctx, "WARN", message, "", fields)
	}
}

// Error logs error messages.
func (l *applicationLoggerImpl) Error(ctx context.Context, message string, fields Fields) {
	if l.shouldLog("ERROR") {
		l.logEntry(ctx, "ERROR", message, "", fields)
	}
}

// ErrorWithError logs error messages with an error object.
func (l *applicationLoggerImpl) ErrorWithError(ctx context.Context, err error, message string, fields Fields) {
	if l.shouldLog("ERROR") {
		l.logEntry(ctx, "ERROR", message, err.Error(), fields)
	}
}

// LogPerformance logs performance metrics.
func (l *applicationLoggerImpl) LogPerformance(
	ctx context.Context,
	operation string,
	duration time.Duration,
	fields Fields,
) {
	if l.shouldLog("INFO") {
		if fields == nil {
			fields = make(Fields)
		}
		fields["operation"] = operation
		fields["duration"] = duration.String()
		l.logEntry(ctx, "INFO", fmt.Sprintf("Performance metrics for %s", operation), "", fields)
	}
}

// startAsyncProcessor starts the asynchronous log processing goroutine.
func (l *applicationLoggerImpl) startAsyncProcessor() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		for {
			select {
			case event := <-l.asyncChannel:
				l.writeLogEntry(&event.entry)
			case <-l.shutdown:
				// Drain remaining events
				for {
					select {
					case event := <-l.asyncChannel:
						l.writeLogEntry(&event.entry)
					default:
						return
					}
				}
			}
		}
	}()
}

// startMemoryMonitor starts monitoring memory usage.
func (l *applicationLoggerImpl) startMemoryMonitor() {
	l.wg.Add(1)
	go func() {
		defer l.wg.Done()
		ticker := time.NewTicker(MemoryMonitorInterval * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				var m runtime.MemStats
				runtime.ReadMemStats(&m)
				currentMB := int64(m.Alloc / BytesToMB / BytesToMB)
				atomic.StoreInt64(&globalPerformanceMetrics.memoryUsage, currentMB)

				// Log warning if memory usage is high
				if l.config.MaxMemoryMB > 0 && currentMB > l.config.MaxMemoryMB {
					l.Warn(context.Background(), "High memory usage detected", Fields{
						"current_mb": currentMB,
						"max_mb":     l.config.MaxMemoryMB,
						"memory_stats": map[string]interface{}{
							"alloc_mb":       m.Alloc / BytesToMB / BytesToMB,
							"total_alloc_mb": m.TotalAlloc / BytesToMB / BytesToMB,
							"sys_mb":         m.Sys / BytesToMB / BytesToMB,
							"num_gc":         m.NumGC,
						},
					})
				}
			case <-l.shutdown:
				return
			}
		}
	}()
}

// Shutdown gracefully shuts down the logger.
func (l *applicationLoggerImpl) Shutdown(ctx context.Context) error {
	close(l.shutdown)
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// WithComponent creates a new logger instance with a specific component.
func (l *applicationLoggerImpl) WithComponent(component string) ApplicationLogger {
	return &applicationLoggerImpl{
		config:       l.config,
		component:    component,
		buffer:       l.buffer,
		logger:       l.logger,
		asyncChannel: l.asyncChannel,
		metrics:      l.metrics,
		sampler:      l.sampler,
		shutdown:     l.shutdown,
	}
}

// logEntry creates and logs a structured log entry with performance optimizations.
func (l *applicationLoggerImpl) createBufferLogEntry(
	ctx context.Context,
	level, message, errorStr, correlationID string,
	fields Fields,
) *LogEntry {
	component := l.component
	if component == "" {
		component = "default"
	}

	entry := &LogEntry{
		Timestamp:     time.Now().UTC().Format(time.RFC3339),
		Level:         level,
		Message:       message,
		CorrelationID: correlationID,
		Component:     component,
		Error:         errorStr,
		Metadata:      make(map[string]interface{}),
		Context:       make(map[string]interface{}),
	}

	l.addFieldsToEntry(entry, fields)
	l.addContextToEntry(entry, ctx)
	return entry
}

func (l *applicationLoggerImpl) addFieldsToEntry(entry *LogEntry, fields Fields) {
	for key, value := range fields {
		if key == "operation" {
			if operation, ok := value.(string); ok {
				entry.Operation = operation
			}
		}
		entry.Metadata[key] = value
	}
}

func (l *applicationLoggerImpl) addContextToEntry(entry *LogEntry, ctx context.Context) {
	if requestID := getRequestIDFromContext(ctx); requestID != "" {
		entry.Context["request_id"] = requestID
	}

	if userCtx := getUserContextFromContext(ctx); userCtx != nil {
		entry.Context["user_id"] = userCtx.UserID
		entry.Context["client_ip"] = userCtx.ClientIP
		entry.Context["user_agent"] = userCtx.UserAgent
	}
}

func (l *applicationLoggerImpl) getLogEntryFromPool() *LogEntry {
	if l.config.EnableObjectPooling {
		return globalLogEntryPool.getLogEntry()
	}
	return &LogEntry{
		Metadata: make(map[string]interface{}),
		Context:  make(map[string]interface{}),
	}
}

func (l *applicationLoggerImpl) logEntry(ctx context.Context, level, message, errorStr string, fields Fields) {
	start := time.Now()

	if l.sampler != nil && !l.sampler.shouldSample() {
		return
	}

	correlationID := getOrGenerateCorrelationID(ctx)

	if l.config.Output == logOutputBuffer {
		entry := l.createBufferLogEntry(ctx, level, message, errorStr, correlationID, fields)
		l.writeLogEntry(entry)
		return
	}

	// Get log entry from pool if object pooling is enabled
	var entry *LogEntry
	if l.config.EnableObjectPooling {
		entry = globalLogEntryPool.getLogEntry()
		defer globalLogEntryPool.putLogEntry(entry)
	} else {
		entry = &LogEntry{
			Metadata: make(map[string]interface{}),
			Context:  make(map[string]interface{}),
		}
	}

	// Build log entry efficiently
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	entry.Level = level
	entry.Message = message
	entry.CorrelationID = correlationID
	entry.Component = l.component
	entry.Error = errorStr

	// Add fields to metadata (reuse map if from pool)
	// Also extract special fields to set in the log entry structure
	for key, value := range fields {
		if key == "operation" {
			if operation, ok := value.(string); ok {
				entry.Operation = operation
			}
		}
		entry.Metadata[key] = value
	}

	// Add context information efficiently
	if requestID := getRequestIDFromContext(ctx); requestID != "" {
		entry.Context["request_id"] = requestID
	}

	if userCtx := getUserContextFromContext(ctx); userCtx != nil {
		entry.Context["user_id"] = userCtx.UserID
		entry.Context["client_ip"] = userCtx.ClientIP
		entry.Context["user_agent"] = userCtx.UserAgent
	}

	// Process log entry asynchronously if enabled
	if l.config.EnableAsyncLogging && l.asyncChannel != nil {
		select {
		case l.asyncChannel <- asyncLogEvent{entry: *entry, ts: time.Now()}:
			// Successfully queued for async processing
		default:
			// Channel full, log synchronously
			atomic.AddInt64(&globalPerformanceMetrics.droppedEntries, 1)
			l.writeLogEntry(entry)
		}
	} else {
		l.writeLogEntry(entry)
	}

	// Record performance metrics
	duration := time.Since(start)
	recordLogPerformance(duration, 0) // Will be updated in writeLogEntry
}

// writeLogEntry handles the actual writing of log entries.
func (l *applicationLoggerImpl) writeLogEntry(entry *LogEntry) {
	var bytesWritten int

	// Special handling for buffer output (testing) - write directly to buffer
	if l.config.Output == logOutputBuffer && l.buffer != nil {
		bytesWritten = l.writeToBuffer(entry)
	} else {
		bytesWritten = l.writeToSlog(entry)
	}

	// Update metrics
	atomic.AddInt64(&globalPerformanceMetrics.totalBytesWritten, int64(bytesWritten))
}

// getOrGenerateCorrelationID gets correlation ID from context or generates a new one.
func getOrGenerateCorrelationID(ctx context.Context) string {
	if correlationID := getCorrelationIDFromContext(ctx); correlationID != "" {
		return correlationID
	}
	return uuid.New().String()
}

// Context management functions.
func WithCorrelationID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, CorrelationIDKey, id)
}

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, RequestIDKey, id)
}

func WithUserContext(ctx context.Context, userCtx UserContext) context.Context {
	return context.WithValue(ctx, UserContextKey, userCtx)
}

func getCorrelationIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	// First check our own context key
	if id, ok := ctx.Value(CorrelationIDKey).(string); ok {
		return id
	}
	// Use the middleware's function to get correlation ID from its context
	// This ensures we use the correct context key type
	if middlewareID := middleware.GetCorrelationIDFromContext(ctx); middlewareID != "" {
		return middlewareID
	}
	return ""
}

func getRequestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if id, ok := ctx.Value(RequestIDKey).(string); ok {
		return id
	}
	return ""
}

func getUserContextFromContext(ctx context.Context) *UserContext {
	if ctx == nil {
		return nil
	}
	if userCtx, ok := ctx.Value(UserContextKey).(UserContext); ok {
		return &userCtx
	}
	return nil
}

// Extended interface implementations for NATS logging.
func (l *applicationLoggerImpl) LogNATSConnectionEvent(ctx context.Context, event NATSConnectionEvent) {
	natsLogger := NewNATSApplicationLogger(l)
	natsLogger.LogNATSConnectionEvent(ctx, event)
}

func (l *applicationLoggerImpl) LogNATSPublishEvent(ctx context.Context, event NATSPublishEvent) {
	natsLogger := NewNATSApplicationLogger(l)
	natsLogger.LogNATSPublishEvent(ctx, event)
}

func (l *applicationLoggerImpl) LogNATSConsumeEvent(ctx context.Context, event NATSConsumeEvent) {
	natsLogger := NewNATSApplicationLogger(l)
	natsLogger.LogNATSConsumeEvent(ctx, event)
}

func (l *applicationLoggerImpl) LogJetStreamEvent(ctx context.Context, event JetStreamEvent) {
	natsLogger := NewNATSApplicationLogger(l)
	natsLogger.LogJetStreamEvent(ctx, event)
}

func (l *applicationLoggerImpl) LogNATSPerformanceMetrics(ctx context.Context, metrics NATSPerformanceMetrics) {
	natsLogger := NewNATSApplicationLogger(l)
	natsLogger.LogNATSPerformanceMetrics(ctx, metrics)
}

// Extended interface implementations for metrics logging.
func (l *applicationLoggerImpl) LogMetric(ctx context.Context, metric PrometheusMetric) {
	metricsLogger := NewMetricsApplicationLogger(l, l.config.MetricsConfig)
	metricsLogger.LogMetric(ctx, metric)
}

func (l *applicationLoggerImpl) LogApplicationMetrics(ctx context.Context, metrics ApplicationMetrics) {
	metricsLogger := NewMetricsApplicationLogger(l, l.config.MetricsConfig)
	metricsLogger.LogApplicationMetrics(ctx, metrics)
}

func (l *applicationLoggerImpl) LogHealthMetrics(ctx context.Context, health HealthMetrics) {
	metricsLogger := NewMetricsApplicationLogger(l, l.config.MetricsConfig)
	metricsLogger.LogHealthMetrics(ctx, health)
}

func (l *applicationLoggerImpl) LogCustomMetrics(ctx context.Context, custom CustomMetrics) {
	metricsLogger := NewMetricsApplicationLogger(l, l.config.MetricsConfig)
	metricsLogger.LogCustomMetrics(ctx, custom)
}

func (l *applicationLoggerImpl) FlushAggregatedMetrics(ctx context.Context) {
	metricsLogger := NewMetricsApplicationLogger(l, l.config.MetricsConfig)
	metricsLogger.FlushAggregatedMetrics(ctx)
}

// Test helper function for buffer output
// Object pool functions for performance optimization.
func (p *logEntryPool) getLogEntry() *LogEntry {
	entry := p.pool.Get().(*LogEntry)
	// Reset fields
	entry.Timestamp = ""
	entry.Level = ""
	entry.Message = ""
	entry.CorrelationID = ""
	entry.Component = ""
	entry.Operation = ""
	entry.Duration = ""
	entry.Error = ""
	// Clear maps without reallocating
	for k := range entry.Metadata {
		delete(entry.Metadata, k)
	}
	for k := range entry.Context {
		delete(entry.Context, k)
	}
	return entry
}

func (p *logEntryPool) putLogEntry(entry *LogEntry) {
	p.pool.Put(entry)
}

// Performance monitoring functions.
func recordLogPerformance(duration time.Duration, bytesWritten int) {
	atomic.AddInt64(&globalPerformanceMetrics.totalEntries, 1)
	atomic.AddInt64(&globalPerformanceMetrics.totalBytesWritten, int64(bytesWritten))
	// Simple moving average for latency
	currentLatency := atomic.LoadInt64(&globalPerformanceMetrics.averageLatency)
	newLatency := (currentLatency + duration.Nanoseconds()) / LatencyAverageDiv
	atomic.StoreInt64(&globalPerformanceMetrics.averageLatency, newLatency)
}

func getLoggerOutput(logger interface{}) string {
	if appLogger, ok := logger.(*applicationLoggerImpl); ok && appLogger.buffer != nil {
		output := strings.TrimSpace(appLogger.buffer.String())

		// If we have multiple lines (multiple JSON objects), return the last one
		if output != "" {
			lines := strings.Split(output, "\n")
			for i := len(lines) - 1; i >= 0; i-- {
				if strings.TrimSpace(lines[i]) != "" {
					return strings.TrimSpace(lines[i])
				}
			}
		}
		return output
	}
	return ""
}

// getLoggerOutputByOperation finds a log entry with specific operation.
func parseLogEntryLine(line string) (*LogEntry, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil, errors.New("empty line")
	}

	var entry LogEntry
	err := json.Unmarshal([]byte(line), &entry)
	return &entry, err
}

func entryMatchesOperation(entry *LogEntry, operation string) bool {
	if entry.Operation == operation {
		return true
	}

	if op, exists := entry.Metadata["operation"]; exists && op == operation {
		return true
	}

	return false
}

func searchLogLinesForOperation(lines []string, operation string) string {
	for _, line := range lines {
		entry, err := parseLogEntryLine(line)
		if err != nil {
			continue
		}

		if entryMatchesOperation(entry, operation) {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func getLoggerOutputByOperation(logger interface{}, operation string) string {
	appLogger, ok := logger.(*applicationLoggerImpl)
	if !ok || appLogger.buffer == nil {
		return ""
	}

	output := strings.TrimSpace(appLogger.buffer.String())
	if output == "" {
		return ""
	}

	lines := strings.Split(output, "\n")
	return searchLogLinesForOperation(lines, operation)
}

// GetLoggerPerformanceMetrics returns current performance metrics.
func GetLoggerPerformanceMetrics() map[string]interface{} {
	return map[string]interface{}{
		"total_entries":       atomic.LoadInt64(&globalPerformanceMetrics.totalEntries),
		"total_bytes_written": atomic.LoadInt64(&globalPerformanceMetrics.totalBytesWritten),
		"average_latency_ns":  atomic.LoadInt64(&globalPerformanceMetrics.averageLatency),
		"errors_count":        atomic.LoadInt64(&globalPerformanceMetrics.errorsCount),
		"dropped_entries":     atomic.LoadInt64(&globalPerformanceMetrics.droppedEntries),
		"memory_usage_mb":     atomic.LoadInt64(&globalPerformanceMetrics.memoryUsage),
	}
}

// writeToBuffer writes log entry to buffer for testing purposes.
func (l *applicationLoggerImpl) writeToBuffer(entry *LogEntry) int {
	if l.config.Format == logFormatJSON {
		jsonData, err := json.Marshal(entry)
		if err != nil {
			atomic.AddInt64(&globalPerformanceMetrics.errorsCount, 1)
			return 0
		}
		l.buffer.Write(jsonData)
		l.buffer.WriteString("\n")
		return len(jsonData)
	}

	logLine := fmt.Sprintf("[%s] %s %s: %s", entry.Timestamp, entry.Level, entry.Component, entry.Message)
	l.buffer.WriteString(logLine)
	l.buffer.WriteString("\n")
	return len(logLine)
}

// writeToSlog writes log entry using slog for production output.
func (l *applicationLoggerImpl) writeToSlog(entry *LogEntry) int {
	if l.config.Format == logFormatJSON {
		jsonData, err := json.Marshal(entry)
		if err != nil {
			atomic.AddInt64(&globalPerformanceMetrics.errorsCount, 1)
			return 0
		}
		l.logger.Info(string(jsonData))
		return len(jsonData)
	}

	logLine := fmt.Sprintf("[%s] %s %s: %s", entry.Timestamp, entry.Level, entry.Component, entry.Message)
	l.logger.Info(logLine)
	return len(logLine)
}
