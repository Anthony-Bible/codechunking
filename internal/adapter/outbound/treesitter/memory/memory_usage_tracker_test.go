package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestMemoryUsageTracker_NewTracker_OTELIntegration(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")

	config := TrackerConfig{
		OTELMeter:         noopMeter,
		ReportingInterval: 30 * time.Second,
		PressureThreshold: 0.8,
		EnableGCTracking:  true,
		MetricLabels:      []attribute.KeyValue{attribute.String("test", "value")},
	}

	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)
	require.NotNil(t, tracker)
	assert.Equal(t, noopMeter, tracker.meter)
	assert.Equal(t, config, tracker.config)
	assert.NotNil(t, tracker.memoryUsageHist)
	assert.NotNil(t, tracker.activeMemoryGauge)
	assert.NotNil(t, tracker.memoryPressureGauge)
	assert.NotNil(t, tracker.gcEventsCounter)
	assert.NotNil(t, tracker.memoryLeakCounter)
	assert.NotNil(t, tracker.alertsCounter)
}

func TestMemoryUsageTracker_RecordMemoryUsage_Histogram(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{OTELMeter: noopMeter}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	err = tracker.RecordMemoryUsage(ctx, "parse-operation-1", 1024*1024)
	require.NoError(t, err)
}

func TestMemoryUsageTracker_TrackActiveMemory_Gauge(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{OTELMeter: noopMeter}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	err = tracker.TrackActiveMemory(ctx, "chunk-operation-1", 2048*1024)
	require.NoError(t, err)
}

func TestMemoryUsageTracker_MonitorMemoryPressure_Alerts(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{
		OTELMeter:         noopMeter,
		PressureThreshold: 0.75,
	}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	pressureLevel, err := tracker.MonitorMemoryPressure(ctx)
	require.NoError(t, err)
	assert.Equal(t, MemoryPressureLow, pressureLevel)
	assert.True(t, pressureLevel >= MemoryPressureLow && pressureLevel <= MemoryPressureCritical,
		"Pressure level should be within valid range")
}

func TestMemoryUsageTracker_CorrelateMemoryWithFileSize_Metrics(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{OTELMeter: noopMeter}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Simulate memory usage with file processing
	fileSizes := []int64{1024, 1024 * 1024, 1024 * 1024 * 10}
	memoryUsages := []int64{1024 * 10, 1024 * 1024 * 2, 1024 * 1024 * 50}

	for i := range fileSizes {
		operationID := "file-process-" + string(rune(i))
		err = tracker.RecordMemoryUsage(ctx, operationID, memoryUsages[i])
		require.NoError(t, err)
	}

	breakdown, err := tracker.ReportMemoryBreakdown()
	require.NoError(t, err)
	assert.NotEmpty(t, breakdown.ByFileSize)
}

func TestMemoryUsageTracker_TrackMemoryCleanup_GCMetrics(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{
		OTELMeter:        noopMeter,
		EnableGCTracking: true,
	}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Simulate garbage collection event
	err = tracker.TrackMemoryCleanup(ctx, "gc-operation-1", 1024*1024*100)
	require.NoError(t, err)
}

func TestMemoryUsageTracker_ReportMemoryBreakdown_ByOperation(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{OTELMeter: noopMeter}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Record memory usage for different operations
	operations := []string{"parse", "chunk", "validate", "transform"}
	usages := []int64{1024 * 1024, 2 * 1024 * 1024, 512 * 1024, 4 * 1024 * 1024}

	for i, operation := range operations {
		err = tracker.RecordMemoryUsage(ctx, operation+"-op-1", usages[i])
		require.NoError(t, err)
		err = tracker.TrackActiveMemory(ctx, operation+"-op-1", usages[i])
		require.NoError(t, err)
	}

	breakdown, err := tracker.ReportMemoryBreakdown()
	require.NoError(t, err)
	assert.NotNil(t, breakdown.ByOperation)
	assert.NotNil(t, breakdown.TotalActive)
	assert.NotNil(t, breakdown.PeakUsage)
	assert.NotNil(t, breakdown.Timestamp)

	// Verify breakdown contains expected operations
	for _, operation := range operations {
		_, exists := breakdown.ByOperation[operation]
		assert.True(t, exists, "Memory breakdown should include operation type: %s", operation)
	}
}

func TestMemoryUsageTracker_HandleMemorySpikes_AlertGeneration(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{
		OTELMeter:         noopMeter,
		PressureThreshold: 0.7,
	}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Simulate memory spike
	spikeUsage := int64(1024 * 1024 * 1024) // 1GB
	err = tracker.RecordMemoryUsage(ctx, "spike-operation", spikeUsage)
	require.NoError(t, err)

	// Check if alert was generated
	pressureLevel, err := tracker.MonitorMemoryPressure(ctx)
	require.NoError(t, err)
	assert.Equal(t, MemoryPressureCritical, pressureLevel)
}

func TestMemoryUsageTracker_ConfigurableReporting_Intervals(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{
		OTELMeter:         noopMeter,
		ReportingInterval: 5 * time.Second,
	}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Start periodic reporting
	err = tracker.StartPeriodicReporting(ctx, config.ReportingInterval)
	require.NoError(t, err)

	// Stop reporting
	err = tracker.StopPeriodicReporting()
	require.NoError(t, err)
}

func TestMemoryUsageTracker_MemoryLeakDetection_LongRunning(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{
		OTELMeter:           noopMeter,
		EnableLeakDetection: true,
		ReportingInterval:   1 * time.Second,
	}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Simulate potential memory leak over time
	for i := range 10 {
		err = tracker.TrackActiveMemory(ctx, "leak-operation", int64(1024*1024*(i+1)))
		require.NoError(t, err)
		time.Sleep(100 * time.Millisecond)
	}
}

func TestMemoryUsageTracker_IntegrateWithEnforcer_ViolationReporting(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{OTELMeter: noopMeter}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Create memory alert for enforcer violation
	alert := MemoryAlert{
		OperationID:  "enforce-violation-op",
		AlertType:    MemoryAlertLimitExceeded,
		CurrentUsage: 1024 * 1024 * 1024 * 2, // 2GB
		Threshold:    1024 * 1024 * 1024,     // 1GB
		Timestamp:    time.Now(),
		Metadata: map[string]interface{}{
			"policy": "memory-limit-policy",
			"limit":  "1GB",
		},
	}

	err = tracker.GenerateMemoryAlert(ctx, alert)
	require.NoError(t, err)
}

func TestMemoryUsageTracker_ExportMetrics_OTELFormat(t *testing.T) {
	ctx := context.Background()
	noopMeter := noop.NewMeterProvider().Meter("test")
	config := TrackerConfig{OTELMeter: noopMeter}
	tracker, err := NewMemoryUsageTracker(ctx, config)
	require.NoError(t, err)

	// Record various metrics
	err = tracker.RecordMemoryUsage(ctx, "export-test-op", 1024*1024*5)
	require.NoError(t, err)
	err = tracker.TrackActiveMemory(ctx, "export-test-op", 1024*1024*3)
	require.NoError(t, err)
}
