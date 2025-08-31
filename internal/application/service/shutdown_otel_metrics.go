package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// Metric names following OpenTelemetry semantic conventions.
const (
	ShutdownOperationsCounterName             = "shutdown_operations_total"
	ShutdownComponentsGaugeName               = "shutdown_components_total"
	ShutdownStartTimestampGaugeName           = "shutdown_start_timestamp"
	ComponentShutdownDurationHistogramName    = "component_shutdown_duration_seconds"
	ComponentShutdownCounterName              = "component_shutdown_total"
	ShutdownPhaseDurationHistogramName        = "shutdown_phase_duration_seconds"
	ShutdownPhaseTransitionsCounterName       = "shutdown_phase_transitions_total"
	ShutdownTotalDurationHistogramName        = "shutdown_total_duration_seconds"
	ShutdownCompletionCounterName             = "shutdown_completion_total"
	ShutdownComponentFailureRateGaugeName     = "shutdown_component_failure_rate"
	ShutdownTimeoutCounterName                = "shutdown_timeout_total"
	ResourceCleanupDurationHistogramName      = "resource_cleanup_duration_seconds"
	ResourceCleanupCounterName                = "resource_cleanup_total"
	WorkerShutdownDurationHistogramName       = "worker_shutdown_duration_seconds"
	WorkerActiveJobsGaugeName                 = "worker_active_jobs_at_shutdown"
	WorkerCompletedJobsGaugeName              = "worker_completed_jobs_at_shutdown"
	DatabaseShutdownDurationHistogramName     = "database_shutdown_duration_seconds"
	DatabaseActiveConnectionsGaugeName        = "database_active_connections_at_shutdown"
	DatabasePendingTransactionsGaugeName      = "database_pending_transactions_at_shutdown"
	NATSShutdownDurationHistogramName         = "nats_shutdown_duration_seconds"
	NATSPendingMessagesGaugeName              = "nats_pending_messages_at_shutdown"
	ComponentHealthCheckDurationHistogramName = "component_health_check_duration_seconds"
	ComponentHealthStatusGaugeName            = "component_health_status"
)

// Common attribute keys for consistent labeling.
const (
	AttrShutdownPhase         = "phase"
	AttrShutdownComponent     = "component"
	AttrShutdownStatus        = "status"
	AttrShutdownFromPhase     = "from"
	AttrShutdownToPhase       = "to"
	AttrShutdownResourceType  = "type"
	AttrShutdownWorkerID      = "worker_id"
	AttrShutdownDatabase      = "database"
	AttrShutdownConsumer      = "consumer"
	AttrShutdownInstanceID    = "instance_id"
	AttrShutdownCorrelationID = "correlation_id"
)

// Operation result constants for consistent status reporting.
const (
	OperationResultFailure = "failure"
)

// getComponentShutdownBuckets returns bucket boundaries for component shutdown latencies.
// Optimized for component shutdown operations (10ms to 30s range).
func getComponentShutdownBuckets() []float64 {
	return []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0}
}

// getPhaseTransitionBuckets returns bucket boundaries for phase transition latencies.
// Optimized for fast transitions (1ms to 10s range).
func getPhaseTransitionBuckets() []float64 {
	return []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.5, 1.0, 5.0, 10.0}
}

// getTotalShutdownBuckets returns bucket boundaries for total shutdown latencies.
// Optimized for longer operations (1s to 5min range).
func getTotalShutdownBuckets() []float64 {
	return []float64{1.0, 5.0, 10.0, 30.0, 60.0, 120.0, 300.0}
}

// getResourceCleanupBuckets returns bucket boundaries for resource cleanup latencies.
// Optimized for cleanup operations (100ms to 2min range).
func getResourceCleanupBuckets() []float64 {
	return []float64{0.1, 0.5, 1.0, 5.0, 10.0, 30.0, 60.0, 120.0}
}

// getWorkerShutdownBuckets returns bucket boundaries for worker shutdown latencies.
// Optimized for worker shutdown operations (100ms to 30s range).
func getWorkerShutdownBuckets() []float64 {
	return []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0}
}

// getDatabaseShutdownBuckets returns bucket boundaries for database shutdown latencies.
// Optimized for database shutdown operations (500ms to 1min range).
func getDatabaseShutdownBuckets() []float64 {
	return []float64{0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0}
}

// getNATSShutdownBuckets returns bucket boundaries for NATS shutdown latencies.
// Optimized for NATS shutdown operations (100ms to 1min range).
func getNATSShutdownBuckets() []float64 {
	return []float64{0.1, 0.5, 1.0, 2.0, 5.0, 10.0, 30.0, 60.0}
}

// getHealthCheckBuckets returns bucket boundaries for health check latencies.
// Optimized for health check operations (1ms to 5s range).
func getHealthCheckBuckets() []float64 {
	return []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1.0, 5.0}
}

// ShutdownMetricsCollector provides OpenTelemetry-based metrics collection for shutdown operations.
type ShutdownMetricsCollector struct {
	// Gauges for current state measurements
	shutdownComponentsGauge           metric.Int64Gauge
	shutdownStartTimestampGauge       metric.Float64Gauge
	shutdownComponentFailureRateGauge metric.Float64Gauge
	workerActiveJobsGauge             metric.Int64Gauge
	workerCompletedJobsGauge          metric.Int64Gauge
	databaseActiveConnectionsGauge    metric.Int64Gauge
	databasePendingTransactionsGauge  metric.Int64Gauge
	natsPendingMessagesGauge          metric.Int64Gauge
	componentHealthStatusGauge        metric.Int64Gauge

	// Histograms for timing measurements
	componentShutdownDuration    metric.Float64Histogram
	shutdownPhaseDuration        metric.Float64Histogram
	shutdownTotalDuration        metric.Float64Histogram
	resourceCleanupDuration      metric.Float64Histogram
	workerShutdownDuration       metric.Float64Histogram
	databaseShutdownDuration     metric.Float64Histogram
	natsShutdownDuration         metric.Float64Histogram
	componentHealthCheckDuration metric.Float64Histogram

	// Counters for event counting
	shutdownOperationsCounter       metric.Int64Counter
	componentShutdownCounter        metric.Int64Counter
	shutdownPhaseTransitionsCounter metric.Int64Counter
	shutdownCompletionCounter       metric.Int64Counter
	shutdownTimeoutCounter          metric.Int64Counter
	resourceCleanupCounter          metric.Int64Counter

	// Instance identification for consistent labeling
	instanceID string

	// Cached attributes for performance optimization
	baseInstanceAttr     attribute.KeyValue
	commonStatusAttrs    map[string][]attribute.KeyValue // Pre-built status combinations
	commonComponentAttrs map[string][]attribute.KeyValue // Pre-built component combinations
	commonResourceAttrs  map[string][]attribute.KeyValue // Pre-built resource type combinations

	// Enabled flag
	enabled bool
}

// instrumentCreator provides helpers for creating metric instruments with consistent patterns.
type shutdownInstrumentCreator struct {
	meter metric.Meter
}

// newShutdownInstrumentCreator creates a new instrument creator helper.
func newShutdownInstrumentCreator(meter metric.Meter) *shutdownInstrumentCreator {
	return &shutdownInstrumentCreator{meter: meter}
}

// createFloat64Gauge creates a Float64Gauge instrument with the specified configuration.
func (ic *shutdownInstrumentCreator) createFloat64Gauge(name, description, unit string) (metric.Float64Gauge, error) {
	return ic.meter.Float64Gauge(name, metric.WithDescription(description), metric.WithUnit(unit))
}

// createInt64Gauge creates an Int64Gauge instrument with the specified configuration.
func (ic *shutdownInstrumentCreator) createInt64Gauge(name, description string) (metric.Int64Gauge, error) {
	return ic.meter.Int64Gauge(name, metric.WithDescription(description), metric.WithUnit("1"))
}

// createFloat64Histogram creates a Float64Histogram instrument with explicit bucket boundaries.
func (ic *shutdownInstrumentCreator) createFloat64Histogram(
	name, description string,
	buckets []float64,
) (metric.Float64Histogram, error) {
	return ic.meter.Float64Histogram(
		name,
		metric.WithDescription(description),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(buckets...),
	)
}

// createInt64Counter creates an Int64Counter instrument with the specified configuration.
func (ic *shutdownInstrumentCreator) createInt64Counter(name, description string) (metric.Int64Counter, error) {
	return ic.meter.Int64Counter(name, metric.WithDescription(description), metric.WithUnit("1"))
}

// NewShutdownMetricsCollector creates a new OpenTelemetry metrics collector for shutdown operations
// using the default global meter provider.
func NewShutdownMetricsCollector(enabled bool) *ShutdownMetricsCollector {
	if !enabled {
		collector := &ShutdownMetricsCollector{enabled: false}
		collector.initializeCachedAttributes() // Initialize maps even for disabled collector
		return collector
	}

	instanceID := fmt.Sprintf("shutdown-metrics-%d", time.Now().UnixNano())
	collector, _ := NewShutdownMetricsCollectorWithProvider(instanceID, otel.GetMeterProvider())
	collector.enabled = enabled
	return collector
}

// NewShutdownMetricsCollectorWithProvider creates a new OpenTelemetry metrics collector for shutdown operations
// with a specific meter provider.
func NewShutdownMetricsCollectorWithProvider(
	instanceID string,
	provider metric.MeterProvider,
) (*ShutdownMetricsCollector, error) {
	if instanceID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}

	meter := provider.Meter("codechunking/service", metric.WithInstrumentationVersion("1.0.0"))
	creator := newShutdownInstrumentCreator(meter)

	// Create all metric instruments using helper functions
	gauges, err := createShutdownGauges(creator)
	if err != nil {
		return nil, err
	}

	histograms, err := createShutdownHistograms(creator)
	if err != nil {
		return nil, err
	}

	counters, err := createShutdownCounters(creator)
	if err != nil {
		return nil, err
	}

	// Build collector with all instruments
	collector := buildShutdownCollector(instanceID, gauges, histograms, counters)

	// Initialize cached attribute combinations for optimal performance
	collector.initializeCachedAttributes()

	return collector, nil
}

// initializeCachedAttributes pre-builds common attribute combinations for optimal performance.
func (s *ShutdownMetricsCollector) initializeCachedAttributes() {
	// Initialize maps for cached attributes
	s.commonStatusAttrs = make(map[string][]attribute.KeyValue)
	s.commonComponentAttrs = make(map[string][]attribute.KeyValue)
	s.commonResourceAttrs = make(map[string][]attribute.KeyValue)

	// Pre-build common status combinations
	s.commonStatusAttrs[OperationResultSuccess] = []attribute.KeyValue{
		attribute.String(AttrShutdownStatus, OperationResultSuccess),
		s.baseInstanceAttr,
	}
	s.commonStatusAttrs[OperationResultFailure] = []attribute.KeyValue{
		attribute.String(AttrShutdownStatus, OperationResultFailure),
		s.baseInstanceAttr,
	}

	// Pre-build common component type combinations (based on common shutdown components)
	commonComponents := []string{"database", "nats", "worker", "api", "health"}
	for _, component := range commonComponents {
		s.commonComponentAttrs[component+"_success"] = []attribute.KeyValue{
			attribute.String(AttrShutdownComponent, component),
			attribute.String(AttrShutdownStatus, OperationResultSuccess),
			s.baseInstanceAttr,
		}
		s.commonComponentAttrs[component+"_failure"] = []attribute.KeyValue{
			attribute.String(AttrShutdownComponent, component),
			attribute.String(AttrShutdownStatus, OperationResultFailure),
			s.baseInstanceAttr,
		}
	}

	// Pre-build common resource type combinations
	commonResources := []string{"database", "messaging", "filesystem", "network"}
	for _, resource := range commonResources {
		s.commonResourceAttrs[resource+"_success"] = []attribute.KeyValue{
			attribute.String(AttrShutdownResourceType, resource),
			attribute.String(AttrShutdownStatus, OperationResultSuccess),
			s.baseInstanceAttr,
		}
		s.commonResourceAttrs[resource+"_failure"] = []attribute.KeyValue{
			attribute.String(AttrShutdownResourceType, resource),
			attribute.String(AttrShutdownStatus, OperationResultFailure),
			s.baseInstanceAttr,
		}
	}
}

// buildShutdownStartAttrs builds optimized attribute slice for shutdown start metrics.
func (s *ShutdownMetricsCollector) buildShutdownStartAttrs(phase ShutdownPhase) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrShutdownPhase, string(phase)),
		s.baseInstanceAttr,
	}
}

// buildComponentShutdownAttrs builds optimized attribute slice for component shutdown metrics.
// Uses cached attributes for common component/status combinations for better performance.
func (s *ShutdownMetricsCollector) buildComponentShutdownAttrs(component string, success bool) []attribute.KeyValue {
	status := "_success"
	if !success {
		status = "_failure"
	}

	// Try to use cached attributes for common components
	if cached, exists := s.commonComponentAttrs[component+status]; exists {
		return cached
	}

	// Fallback to dynamic attribute building for uncommon components
	statusValue := OperationResultSuccess
	if !success {
		statusValue = OperationResultFailure
	}
	return []attribute.KeyValue{
		attribute.String(AttrShutdownComponent, component),
		attribute.String(AttrShutdownStatus, statusValue),
		s.baseInstanceAttr,
	}
}

// buildPhaseTransitionAttrs builds optimized attribute slice for phase transition metrics.
func (s *ShutdownMetricsCollector) buildPhaseTransitionAttrs(fromPhase, toPhase ShutdownPhase) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrShutdownFromPhase, string(fromPhase)),
		attribute.String(AttrShutdownToPhase, string(toPhase)),
		s.baseInstanceAttr,
	}
}

// buildResourceCleanupAttrs builds optimized attribute slice for resource cleanup metrics.
// Uses cached attributes for common resource type/status combinations for better performance.
func (s *ShutdownMetricsCollector) buildResourceCleanupAttrs(
	resourceType ResourceType,
	success bool,
) []attribute.KeyValue {
	status := "_success"
	if !success {
		status = "_failure"
	}

	// Try to use cached attributes for common resource types
	if cached, exists := s.commonResourceAttrs[string(resourceType)+status]; exists {
		return cached
	}

	// Fallback to dynamic attribute building for uncommon resource types
	statusValue := OperationResultSuccess
	if !success {
		statusValue = OperationResultFailure
	}
	return []attribute.KeyValue{
		attribute.String(AttrShutdownResourceType, string(resourceType)),
		attribute.String(AttrShutdownStatus, statusValue),
		s.baseInstanceAttr,
	}
}

// buildWorkerShutdownAttrs builds optimized attribute slice for worker shutdown metrics.
func (s *ShutdownMetricsCollector) buildWorkerShutdownAttrs(workerID string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrShutdownWorkerID, workerID),
		s.baseInstanceAttr,
	}
}

// buildDatabaseShutdownAttrs builds optimized attribute slice for database shutdown metrics.
func (s *ShutdownMetricsCollector) buildDatabaseShutdownAttrs(database string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrShutdownDatabase, database),
		s.baseInstanceAttr,
	}
}

// buildNATSShutdownAttrs builds optimized attribute slice for NATS shutdown metrics.
func (s *ShutdownMetricsCollector) buildNATSShutdownAttrs(consumer string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String(AttrShutdownConsumer, consumer),
		s.baseInstanceAttr,
	}
}

// buildHealthCheckAttrs builds optimized attribute slice for health check metrics.
func (s *ShutdownMetricsCollector) buildHealthCheckAttrs(component string, healthy bool) []attribute.KeyValue {
	status := "healthy"
	if !healthy {
		status = "unhealthy"
	}
	return []attribute.KeyValue{
		attribute.String(AttrShutdownComponent, component),
		attribute.String(AttrShutdownStatus, status),
		s.baseInstanceAttr,
	}
}

// RecordShutdownStart records the start of a graceful shutdown operation.
func (s *ShutdownMetricsCollector) RecordShutdownStart(ctx context.Context, phase ShutdownPhase, componentCount int) {
	if !s.enabled {
		return
	}

	attributes := s.buildShutdownStartAttrs(phase)
	s.shutdownOperationsCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
	s.shutdownComponentsGauge.Record(ctx, int64(componentCount), metric.WithAttributes(s.baseInstanceAttr))
	s.shutdownStartTimestampGauge.Record(ctx, float64(time.Now().Unix()), metric.WithAttributes(s.baseInstanceAttr))
}

// RecordComponentShutdown records the completion of an individual component shutdown.
func (s *ShutdownMetricsCollector) RecordComponentShutdown(
	ctx context.Context,
	component string,
	duration time.Duration,
	success bool,
) {
	if !s.enabled {
		return
	}

	attributes := s.buildComponentShutdownAttrs(component, success)
	s.componentShutdownDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	s.componentShutdownCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordShutdownPhaseTransition records transitions between shutdown phases.
func (s *ShutdownMetricsCollector) RecordShutdownPhaseTransition(
	ctx context.Context,
	fromPhase, toPhase ShutdownPhase,
	duration time.Duration,
) {
	if !s.enabled {
		return
	}

	attributes := s.buildPhaseTransitionAttrs(fromPhase, toPhase)
	s.shutdownPhaseDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	s.shutdownPhaseTransitionsCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordShutdownComplete records the completion of the entire shutdown process.
func (s *ShutdownMetricsCollector) RecordShutdownComplete(
	ctx context.Context,
	duration time.Duration,
	metrics ShutdownMetrics,
) {
	if !s.enabled {
		return
	}

	// Record total duration
	s.shutdownTotalDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(s.baseInstanceAttr))

	// Record completion counter with status
	status := "success"
	if metrics.TimeoutCount > 0 {
		status = "timeout"
		s.shutdownTimeoutCounter.Add(ctx, metrics.TimeoutCount, metric.WithAttributes(s.baseInstanceAttr))
	}
	if metrics.ForceShutdowns > 0 {
		status = "force"
	}

	completionAttrs := []attribute.KeyValue{
		attribute.String(AttrShutdownStatus, status),
		s.baseInstanceAttr,
	}
	s.shutdownCompletionCounter.Add(ctx, 1, metric.WithAttributes(completionAttrs...))

	// Record component failure rate
	s.shutdownComponentFailureRateGauge.Record(
		ctx,
		metrics.ComponentFailureRate,
		metric.WithAttributes(s.baseInstanceAttr),
	)
}

// RecordResourceCleanup records resource cleanup metrics.
func (s *ShutdownMetricsCollector) RecordResourceCleanup(
	ctx context.Context,
	resourceType ResourceType,
	duration time.Duration,
	success bool,
) {
	if !s.enabled {
		return
	}

	attributes := s.buildResourceCleanupAttrs(resourceType, success)
	s.resourceCleanupDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	s.resourceCleanupCounter.Add(ctx, 1, metric.WithAttributes(attributes...))
}

// RecordWorkerShutdown records worker shutdown metrics.
func (s *ShutdownMetricsCollector) RecordWorkerShutdown(
	ctx context.Context,
	workerID string,
	activeJobs int,
	completedJobs int64,
	duration time.Duration,
) {
	if !s.enabled {
		return
	}

	attributes := s.buildWorkerShutdownAttrs(workerID)
	s.workerShutdownDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	s.workerActiveJobsGauge.Record(ctx, int64(activeJobs), metric.WithAttributes(attributes...))
	s.workerCompletedJobsGauge.Record(ctx, completedJobs, metric.WithAttributes(attributes...))
}

// RecordDatabaseShutdown records database shutdown metrics.
func (s *ShutdownMetricsCollector) RecordDatabaseShutdown(
	ctx context.Context,
	database string,
	activeConnections int,
	pendingTransactions int,
	duration time.Duration,
) {
	if !s.enabled {
		return
	}

	attributes := s.buildDatabaseShutdownAttrs(database)
	s.databaseShutdownDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	s.databaseActiveConnectionsGauge.Record(ctx, int64(activeConnections), metric.WithAttributes(attributes...))
	s.databasePendingTransactionsGauge.Record(ctx, int64(pendingTransactions), metric.WithAttributes(attributes...))
}

// RecordNATSShutdown records NATS shutdown metrics.
func (s *ShutdownMetricsCollector) RecordNATSShutdown(
	ctx context.Context,
	consumer string,
	pendingMessages int64,
	duration time.Duration,
) {
	if !s.enabled {
		return
	}

	attributes := s.buildNATSShutdownAttrs(consumer)
	s.natsShutdownDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))
	s.natsPendingMessagesGauge.Record(ctx, pendingMessages, metric.WithAttributes(attributes...))
}

// RecordHealthCheck records health check results for shutdown components.
func (s *ShutdownMetricsCollector) RecordHealthCheck(
	ctx context.Context,
	component string,
	healthy bool,
	duration time.Duration,
) {
	if !s.enabled {
		return
	}

	attributes := s.buildHealthCheckAttrs(component, healthy)
	s.componentHealthCheckDuration.Record(ctx, duration.Seconds(), metric.WithAttributes(attributes...))

	// Record health status as gauge (1 for healthy, 0 for unhealthy)
	healthValue := int64(0)
	if healthy {
		healthValue = 1
	}
	s.componentHealthStatusGauge.Record(ctx, healthValue, metric.WithAttributes(attributes...))
}

// SetEnabled enables or disables metrics collection.
func (s *ShutdownMetricsCollector) SetEnabled(enabled bool) {
	s.enabled = enabled
}

// IsEnabled returns whether metrics collection is enabled.
func (s *ShutdownMetricsCollector) IsEnabled() bool {
	return s.enabled
}

// GetMetricLabels creates standardized metric labels for shutdown operations.
func (s *ShutdownMetricsCollector) GetMetricLabels(
	operation string,
	component string,
	phase ShutdownPhase,
) map[string]string {
	labels := map[string]string{
		"operation": operation,
		"component": component,
	}

	if phase != "" {
		labels["phase"] = string(phase)
	}

	return labels
}

// FormatDuration formats duration for metrics in a consistent way.
func (s *ShutdownMetricsCollector) FormatDuration(duration time.Duration) float64 {
	return duration.Seconds()
}

// GetHealthMetrics returns health-related metrics for shutdown components.
func (s *ShutdownMetricsCollector) GetHealthMetrics(_ context.Context) map[string]interface{} {
	if !s.enabled {
		return nil
	}

	return map[string]interface{}{
		"metrics_collector_enabled": s.enabled,
		"metrics_collector_status":  "healthy",
		"last_updated":              time.Now().UTC().Format(time.RFC3339),
	}
}

// CreateMetricsSummary creates a summary of all shutdown metrics for reporting.
func (s *ShutdownMetricsCollector) CreateMetricsSummary(
	_ context.Context,
	metrics ShutdownMetrics,
) map[string]interface{} {
	if !s.enabled {
		return nil
	}

	return map[string]interface{}{
		"total_shutdowns":        metrics.TotalShutdowns,
		"successful_shutdowns":   metrics.SuccessfulShutdowns,
		"failed_shutdowns":       metrics.FailedShutdowns,
		"force_shutdowns":        metrics.ForceShutdowns,
		"average_shutdown_time":  metrics.AverageShutdownTime.Milliseconds(),
		"last_shutdown_duration": metrics.LastShutdownDuration.Milliseconds(),
		"timeout_count":          metrics.TimeoutCount,
		"component_failure_rate": fmt.Sprintf("%.2f", metrics.ComponentFailureRate*100),
		"signal_counts":          metrics.SignalCounts,
		"component_count":        len(metrics.ComponentMetrics),
	}
}

// shutdownGauges holds all gauge metrics for the collector.
type shutdownGauges struct {
	shutdownComponentsGauge           metric.Int64Gauge
	shutdownStartTimestampGauge       metric.Float64Gauge
	shutdownComponentFailureRateGauge metric.Float64Gauge
	workerActiveJobsGauge             metric.Int64Gauge
	workerCompletedJobsGauge          metric.Int64Gauge
	databaseActiveConnectionsGauge    metric.Int64Gauge
	databasePendingTransactionsGauge  metric.Int64Gauge
	natsPendingMessagesGauge          metric.Int64Gauge
	componentHealthStatusGauge        metric.Int64Gauge
}

// shutdownHistograms holds all histogram metrics for the collector.
type shutdownHistograms struct {
	componentShutdownDuration    metric.Float64Histogram
	shutdownPhaseDuration        metric.Float64Histogram
	shutdownTotalDuration        metric.Float64Histogram
	resourceCleanupDuration      metric.Float64Histogram
	workerShutdownDuration       metric.Float64Histogram
	databaseShutdownDuration     metric.Float64Histogram
	natsShutdownDuration         metric.Float64Histogram
	componentHealthCheckDuration metric.Float64Histogram
}

// shutdownCounters holds all counter metrics for the collector.
type shutdownCounters struct {
	shutdownOperationsCounter       metric.Int64Counter
	componentShutdownCounter        metric.Int64Counter
	shutdownPhaseTransitionsCounter metric.Int64Counter
	shutdownCompletionCounter       metric.Int64Counter
	shutdownTimeoutCounter          metric.Int64Counter
	resourceCleanupCounter          metric.Int64Counter
}

// createShutdownGauges creates all gauge instruments for shutdown metrics.
func createShutdownGauges(creator *shutdownInstrumentCreator) (*shutdownGauges, error) {
	shutdownComponentsGauge, err := creator.createInt64Gauge(
		ShutdownComponentsGaugeName,
		"Total number of components registered for shutdown",
	)
	if err != nil {
		return nil, err
	}

	shutdownStartTimestampGauge, err := creator.createFloat64Gauge(
		ShutdownStartTimestampGaugeName,
		"Timestamp when shutdown process started",
		"s",
	)
	if err != nil {
		return nil, err
	}

	shutdownComponentFailureRateGauge, err := creator.createFloat64Gauge(
		ShutdownComponentFailureRateGaugeName,
		"Rate of component failures during shutdown",
		"1",
	)
	if err != nil {
		return nil, err
	}

	workerActiveJobsGauge, err := creator.createInt64Gauge(
		WorkerActiveJobsGaugeName,
		"Number of active jobs at worker shutdown",
	)
	if err != nil {
		return nil, err
	}

	workerCompletedJobsGauge, err := creator.createInt64Gauge(
		WorkerCompletedJobsGaugeName,
		"Number of completed jobs at worker shutdown",
	)
	if err != nil {
		return nil, err
	}

	databaseActiveConnectionsGauge, err := creator.createInt64Gauge(
		DatabaseActiveConnectionsGaugeName,
		"Number of active database connections at shutdown",
	)
	if err != nil {
		return nil, err
	}

	databasePendingTransactionsGauge, err := creator.createInt64Gauge(
		DatabasePendingTransactionsGaugeName,
		"Number of pending database transactions at shutdown",
	)
	if err != nil {
		return nil, err
	}

	natsPendingMessagesGauge, err := creator.createInt64Gauge(
		NATSPendingMessagesGaugeName,
		"Number of pending NATS messages at shutdown",
	)
	if err != nil {
		return nil, err
	}

	componentHealthStatusGauge, err := creator.createInt64Gauge(
		ComponentHealthStatusGaugeName,
		"Health status of components (1=healthy, 0=unhealthy)",
	)
	if err != nil {
		return nil, err
	}

	return &shutdownGauges{
		shutdownComponentsGauge:           shutdownComponentsGauge,
		shutdownStartTimestampGauge:       shutdownStartTimestampGauge,
		shutdownComponentFailureRateGauge: shutdownComponentFailureRateGauge,
		workerActiveJobsGauge:             workerActiveJobsGauge,
		workerCompletedJobsGauge:          workerCompletedJobsGauge,
		databaseActiveConnectionsGauge:    databaseActiveConnectionsGauge,
		databasePendingTransactionsGauge:  databasePendingTransactionsGauge,
		natsPendingMessagesGauge:          natsPendingMessagesGauge,
		componentHealthStatusGauge:        componentHealthStatusGauge,
	}, nil
}

// createShutdownHistograms creates all histogram instruments for shutdown metrics.
func createShutdownHistograms(creator *shutdownInstrumentCreator) (*shutdownHistograms, error) {
	componentShutdownDuration, err := creator.createFloat64Histogram(
		ComponentShutdownDurationHistogramName,
		"Duration of individual component shutdown operations",
		getComponentShutdownBuckets(),
	)
	if err != nil {
		return nil, err
	}

	shutdownPhaseDuration, err := creator.createFloat64Histogram(
		ShutdownPhaseDurationHistogramName,
		"Duration of shutdown phase transitions",
		getPhaseTransitionBuckets(),
	)
	if err != nil {
		return nil, err
	}

	shutdownTotalDuration, err := creator.createFloat64Histogram(
		ShutdownTotalDurationHistogramName,
		"Total duration of complete shutdown process",
		getTotalShutdownBuckets(),
	)
	if err != nil {
		return nil, err
	}

	resourceCleanupDuration, err := creator.createFloat64Histogram(
		ResourceCleanupDurationHistogramName,
		"Duration of resource cleanup operations",
		getResourceCleanupBuckets(),
	)
	if err != nil {
		return nil, err
	}

	workerShutdownDuration, err := creator.createFloat64Histogram(
		WorkerShutdownDurationHistogramName,
		"Duration of worker shutdown operations",
		getWorkerShutdownBuckets(),
	)
	if err != nil {
		return nil, err
	}

	databaseShutdownDuration, err := creator.createFloat64Histogram(
		DatabaseShutdownDurationHistogramName,
		"Duration of database shutdown operations",
		getDatabaseShutdownBuckets(),
	)
	if err != nil {
		return nil, err
	}

	natsShutdownDuration, err := creator.createFloat64Histogram(
		NATSShutdownDurationHistogramName,
		"Duration of NATS shutdown operations",
		getNATSShutdownBuckets(),
	)
	if err != nil {
		return nil, err
	}

	componentHealthCheckDuration, err := creator.createFloat64Histogram(
		ComponentHealthCheckDurationHistogramName,
		"Duration of component health check operations",
		getHealthCheckBuckets(),
	)
	if err != nil {
		return nil, err
	}

	return &shutdownHistograms{
		componentShutdownDuration:    componentShutdownDuration,
		shutdownPhaseDuration:        shutdownPhaseDuration,
		shutdownTotalDuration:        shutdownTotalDuration,
		resourceCleanupDuration:      resourceCleanupDuration,
		workerShutdownDuration:       workerShutdownDuration,
		databaseShutdownDuration:     databaseShutdownDuration,
		natsShutdownDuration:         natsShutdownDuration,
		componentHealthCheckDuration: componentHealthCheckDuration,
	}, nil
}

// createShutdownCounters creates all counter instruments for shutdown metrics.
func createShutdownCounters(creator *shutdownInstrumentCreator) (*shutdownCounters, error) {
	shutdownOperationsCounter, err := creator.createInt64Counter(
		ShutdownOperationsCounterName,
		"Total number of shutdown operations initiated",
	)
	if err != nil {
		return nil, err
	}

	componentShutdownCounter, err := creator.createInt64Counter(
		ComponentShutdownCounterName,
		"Total number of component shutdown attempts",
	)
	if err != nil {
		return nil, err
	}

	shutdownPhaseTransitionsCounter, err := creator.createInt64Counter(
		ShutdownPhaseTransitionsCounterName,
		"Total number of shutdown phase transitions",
	)
	if err != nil {
		return nil, err
	}

	shutdownCompletionCounter, err := creator.createInt64Counter(
		ShutdownCompletionCounterName,
		"Total number of completed shutdown operations",
	)
	if err != nil {
		return nil, err
	}

	shutdownTimeoutCounter, err := creator.createInt64Counter(
		ShutdownTimeoutCounterName,
		"Total number of shutdown timeouts",
	)
	if err != nil {
		return nil, err
	}

	resourceCleanupCounter, err := creator.createInt64Counter(
		ResourceCleanupCounterName,
		"Total number of resource cleanup attempts",
	)
	if err != nil {
		return nil, err
	}

	return &shutdownCounters{
		shutdownOperationsCounter:       shutdownOperationsCounter,
		componentShutdownCounter:        componentShutdownCounter,
		shutdownPhaseTransitionsCounter: shutdownPhaseTransitionsCounter,
		shutdownCompletionCounter:       shutdownCompletionCounter,
		shutdownTimeoutCounter:          shutdownTimeoutCounter,
		resourceCleanupCounter:          resourceCleanupCounter,
	}, nil
}

// buildShutdownCollector creates the final ShutdownMetricsCollector with all instruments.
func buildShutdownCollector(
	instanceID string,
	gauges *shutdownGauges,
	histograms *shutdownHistograms,
	counters *shutdownCounters,
) *ShutdownMetricsCollector {
	return &ShutdownMetricsCollector{
		shutdownComponentsGauge:           gauges.shutdownComponentsGauge,
		shutdownStartTimestampGauge:       gauges.shutdownStartTimestampGauge,
		shutdownComponentFailureRateGauge: gauges.shutdownComponentFailureRateGauge,
		workerActiveJobsGauge:             gauges.workerActiveJobsGauge,
		workerCompletedJobsGauge:          gauges.workerCompletedJobsGauge,
		databaseActiveConnectionsGauge:    gauges.databaseActiveConnectionsGauge,
		databasePendingTransactionsGauge:  gauges.databasePendingTransactionsGauge,
		natsPendingMessagesGauge:          gauges.natsPendingMessagesGauge,
		componentHealthStatusGauge:        gauges.componentHealthStatusGauge,
		componentShutdownDuration:         histograms.componentShutdownDuration,
		shutdownPhaseDuration:             histograms.shutdownPhaseDuration,
		shutdownTotalDuration:             histograms.shutdownTotalDuration,
		resourceCleanupDuration:           histograms.resourceCleanupDuration,
		workerShutdownDuration:            histograms.workerShutdownDuration,
		databaseShutdownDuration:          histograms.databaseShutdownDuration,
		natsShutdownDuration:              histograms.natsShutdownDuration,
		componentHealthCheckDuration:      histograms.componentHealthCheckDuration,
		shutdownOperationsCounter:         counters.shutdownOperationsCounter,
		componentShutdownCounter:          counters.componentShutdownCounter,
		shutdownPhaseTransitionsCounter:   counters.shutdownPhaseTransitionsCounter,
		shutdownCompletionCounter:         counters.shutdownCompletionCounter,
		shutdownTimeoutCounter:            counters.shutdownTimeoutCounter,
		resourceCleanupCounter:            counters.resourceCleanupCounter,
		instanceID:                        instanceID,
		baseInstanceAttr:                  attribute.String(AttrShutdownInstanceID, instanceID),
		enabled:                           true,
	}
}
