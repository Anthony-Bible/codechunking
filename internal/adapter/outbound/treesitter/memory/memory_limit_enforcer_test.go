package memory

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryLimitEnforcer_NewEnforcer_ConfigurableLimits(t *testing.T) {
	t.Parallel()

	defaultLimits := MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024,  // 256MB
		TotalProcessLimit: 1024 * 1024 * 1024, // 1GB
		AlertThreshold:    0.8,
		CheckInterval:     time.Second,
	}

	enforcer := NewEnforcer(defaultLimits)
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	// Verify default limits are set correctly
	assert.Equal(t, int64(256*1024*1024), enforcer.limits.PerOperationLimit, "PerOperationLimit should be 256MB")
	assert.Equal(t, int64(1024*1024*1024), enforcer.limits.TotalProcessLimit, "TotalProcessLimit should be 1GB")
	assert.Equal(t, 0.8, enforcer.limits.AlertThreshold, "AlertThreshold should be 0.8")
	assert.Equal(t, time.Second, enforcer.limits.CheckInterval, "CheckInterval should be 1 second")

	// Test with custom limits
	customLimits := MemoryLimits{
		PerOperationLimit: 512 * 1024 * 1024,      // 512MB
		TotalProcessLimit: 2 * 1024 * 1024 * 1024, // 2GB
		AlertThreshold:    0.9,
		CheckInterval:     500 * time.Millisecond,
	}

	enforcer = NewEnforcer(customLimits)
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer with custom limits")

	assert.Equal(t, int64(512*1024*1024), enforcer.limits.PerOperationLimit, "PerOperationLimit should be 512MB")
	assert.Equal(t, int64(2*1024*1024*1024), enforcer.limits.TotalProcessLimit, "TotalProcessLimit should be 2GB")
	assert.Equal(t, 0.9, enforcer.limits.AlertThreshold, "AlertThreshold should be 0.9")
	assert.Equal(t, 500*time.Millisecond, enforcer.limits.CheckInterval, "CheckInterval should be 500ms")
}

func TestMemoryLimitEnforcer_MonitorMemory_DuringParsing(t *testing.T) {
	t.Parallel()

	enforcer := NewEnforcer(MemoryLimits{})
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx := context.Background()
	operationID := "test-operation-1"

	err := enforcer.StartMonitoring(ctx, operationID)
	require.NoError(t, err, "StartMonitoring should not return an error for valid context and operation ID")

	// Check that monitoring has started
	usage, err := enforcer.CheckMemoryUsage()
	require.NoError(t, err, "CheckMemoryUsage should not return an error after starting monitoring")
	assert.Equal(t, operationID, usage.OperationID, "OperationID should match the one provided to StartMonitoring")
	assert.NotZero(t, usage.Current, "Current memory usage should not be zero")
	assert.NotZero(t, usage.Peak, "Peak memory usage should not be zero")
	assert.WithinDuration(t, time.Now(), usage.Timestamp, time.Second, "Timestamp should be within one second of now")

	// Test with cancelled context
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err = enforcer.StartMonitoring(cancelledCtx, "test-operation-2")
	assert.Error(t, err, "StartMonitoring should return an error when context is already cancelled")
	assert.Contains(t, err.Error(), "context cancelled", "Error message should indicate context cancellation")
}

func TestMemoryLimitEnforcer_CheckLimit_WithinBounds(t *testing.T) {
	t.Parallel()

	limits := MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024,  // 256MB
		TotalProcessLimit: 1024 * 1024 * 1024, // 1GB
	}

	enforcer := NewEnforcer(limits)
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx := context.Background()
	operationID := "within-bounds-operation"

	err := enforcer.StartMonitoring(ctx, operationID)
	require.NoError(t, err, "StartMonitoring should not return an error")

	// Simulate memory usage within limits
	enforcer.simulateMemoryUsage(100 * 1024 * 1024) // 100MB

	usage, err := enforcer.CheckMemoryUsage()
	require.NoError(t, err, "CheckMemoryUsage should not return an error when within limits")
	assert.Less(t, usage.Current, limits.PerOperationLimit, "Current usage should be less than per operation limit")
	assert.Equal(t, operationID, usage.OperationID, "OperationID should match")

	// No violation should be detected
	violation := enforcer.detectViolation(usage)
	assert.Equal(t, MemoryViolationType(0), violation.ViolationType, "ViolationType should be zero when within limits")
}

func TestMemoryLimitEnforcer_CheckLimit_ExceedsLimit(t *testing.T) {
	t.Parallel()

	limits := MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024,  // 256MB
		TotalProcessLimit: 1024 * 1024 * 1024, // 1GB
	}

	enforcer := NewEnforcer(limits)
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx := context.Background()
	operationID := "exceeds-limit-operation"

	err := enforcer.StartMonitoring(ctx, operationID)
	require.NoError(t, err, "StartMonitoring should not return an error")

	// Simulate memory usage that exceeds the limit
	enforcer.simulateMemoryUsage(300 * 1024 * 1024) // 300MB

	usage, err := enforcer.CheckMemoryUsage()
	require.NoError(t, err, "CheckMemoryUsage should not return an error")

	// Violation should be detected
	violation := enforcer.detectViolation(usage)
	assert.Equal(
		t,
		MemoryViolationHard,
		violation.ViolationType,
		"ViolationType should be MemoryViolationHard when exceeding limit",
	)
	assert.Equal(t, operationID, violation.OperationID, "OperationID should match")
	assert.Equal(t, int64(300*1024*1024), violation.CurrentUsage, "CurrentUsage should match simulated usage")
	assert.Equal(t, limits.PerOperationLimit, violation.Limit, "Limit should match the configured per operation limit")
}

func TestMemoryLimitEnforcer_HandleViolation_GracefulDegradation(t *testing.T) {
	t.Parallel()

	limits := MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024, // 256MB
	}

	enforcer := NewEnforcer(limits)
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx := context.Background()
	operationID := "graceful-degradation-operation"

	err := enforcer.StartMonitoring(ctx, operationID)
	require.NoError(t, err, "StartMonitoring should not return an error")

	violation := MemoryViolation{
		OperationID:   operationID,
		CurrentUsage:  300 * 1024 * 1024, // 300MB
		Limit:         limits.PerOperationLimit,
		ViolationType: MemoryViolationHard,
		Timestamp:     time.Now(),
	}

	err = enforcer.HandleMemoryViolation(ctx, violation)
	require.Error(t, err, "HandleMemoryViolation should return an error for hard violation")
	assert.Contains(t, err.Error(), "memory limit exceeded", "Error message should indicate memory limit exceeded")

	// Verify that the operation was stopped
	usage, err := enforcer.CheckMemoryUsage()
	assert.Error(t, err, "CheckMemoryUsage should return an error after handling violation")
	assert.Contains(t, err.Error(), "not monitoring", "Error message should indicate not monitoring")
	assert.Equal(t, MemoryUsage{}, usage, "Usage should be empty after stopping monitoring")
}

func TestMemoryLimitEnforcer_HandleViolation_ErrorReporting(t *testing.T) {
	t.Parallel()

	limits := MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024, // 256MB
	}

	enforcer := NewEnforcer(limits)
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx := context.Background()
	operationID := "error-reporting-operation"

	err := enforcer.StartMonitoring(ctx, operationID)
	require.NoError(t, err, "StartMonitoring should not return an error")

	violation := MemoryViolation{
		OperationID:   operationID,
		CurrentUsage:  300 * 1024 * 1024, // 300MB
		Limit:         limits.PerOperationLimit,
		ViolationType: MemoryViolationCritical,
		Timestamp:     time.Now(),
	}

	err = enforcer.HandleMemoryViolation(ctx, violation)
	require.Error(t, err, "HandleMemoryViolation should return an error for critical violation")
	assert.ErrorIs(t, err, ErrMemoryLimitExceeded, "Error should be ErrMemoryLimitExceeded")
	assert.Contains(t, err.Error(), "critical memory violation", "Error message should indicate critical violation")

	// Verify that detailed violation information is included in the error
	assert.Contains(t, err.Error(), operationID, "Error message should contain operation ID")
	assert.Contains(t, err.Error(), "300MB", "Error message should contain current usage")
	assert.Contains(t, err.Error(), "256MB", "Error message should contain limit")
}

func TestMemoryLimitEnforcer_ContextCancellation_CleanupMemory(t *testing.T) {
	t.Parallel()

	enforcer := NewEnforcer(MemoryLimits{})
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx, cancel := context.WithCancel(context.Background())
	operationID := "cancellation-operation"

	err := enforcer.StartMonitoring(ctx, operationID)
	require.NoError(t, err, "StartMonitoring should not return an error")

	// Cancel the context
	cancel()

	// Give time for cleanup to happen
	time.Sleep(100 * time.Millisecond)

	// Verify monitoring has stopped
	usage, err := enforcer.CheckMemoryUsage()
	assert.Error(t, err, "CheckMemoryUsage should return an error after context cancellation")
	assert.Contains(t, err.Error(), "not monitoring", "Error message should indicate not monitoring")
	assert.Equal(t, MemoryUsage{}, usage, "Usage should be empty after cancellation")

	// Verify memory is cleaned up
	stats := enforcer.GetMemoryStats()
	assert.Zero(t, stats.CurrentUsage, "CurrentUsage should be zero after cleanup")
	assert.Zero(t, stats.PeakUsage, "PeakUsage should be zero after cleanup")
}

func TestMemoryLimitEnforcer_AdaptiveLimits_BasedOnFileSize(t *testing.T) {
	t.Parallel()

	enforcer := NewEnforcer(MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024, // 256MB
	})
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	// Test small file (should use default limit)
	smallFileLimit := enforcer.GetAdaptiveLimit(10 * 1024) // 10KB
	assert.Equal(t, int64(256*1024*1024), smallFileLimit, "Small file should use default limit of 256MB")

	// Test medium file (should use increased limit)
	mediumFileLimit := enforcer.GetAdaptiveLimit(1024 * 1024) // 1MB
	assert.Greater(t, mediumFileLimit, int64(256*1024*1024), "Medium file should use increased limit")
	assert.Less(t, mediumFileLimit, int64(512*1024*1024), "Medium file limit should be less than 512MB")

	// Test large file (should use much higher limit)
	largeFileLimit := enforcer.GetAdaptiveLimit(100 * 1024 * 1024) // 100MB
	assert.Greater(t, largeFileLimit, int64(512*1024*1024), "Large file should use higher limit")
	assert.LessOrEqual(
		t,
		largeFileLimit,
		enforcer.limits.TotalProcessLimit,
		"Large file limit should not exceed total process limit",
	)

	// Test with zero file size
	zeroFileLimit := enforcer.GetAdaptiveLimit(0)
	assert.Equal(t, int64(256*1024*1024), zeroFileLimit, "Zero file size should use default limit")

	// Test with negative file size
	negativeFileLimit := enforcer.GetAdaptiveLimit(-1)
	assert.Equal(t, int64(256*1024*1024), negativeFileLimit, "Negative file size should use default limit")
}

func TestMemoryLimitEnforcer_MemoryPressure_AlertGeneration(t *testing.T) {
	t.Parallel()

	limits := MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024, // 256MB
		AlertThreshold:    0.8,
	}

	enforcer := NewEnforcer(limits)
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx := context.Background()
	operationID := "alert-generation-operation"

	err := enforcer.StartMonitoring(ctx, operationID)
	require.NoError(t, err, "StartMonitoring should not return an error")

	// Simulate memory usage at 75% (below alert threshold)
	enforcer.simulateMemoryUsage(int64(float64(limits.PerOperationLimit) * 0.75))

	usage, err := enforcer.CheckMemoryUsage()
	require.NoError(t, err, "CheckMemoryUsage should not return an error")
	alert := enforcer.generateAlert(usage)
	assert.False(t, alert.Metadata["is_under_pressure"].(bool), "Should not be under memory pressure at 75% usage")

	// Simulate memory usage at 85% (above alert threshold)
	enforcer.simulateMemoryUsage(int64(float64(limits.PerOperationLimit) * 0.85))

	usage, err = enforcer.CheckMemoryUsage()
	require.NoError(t, err, "CheckMemoryUsage should not return an error")
	alert = enforcer.generateAlert(usage)
	assert.True(t, alert.Metadata["is_under_pressure"].(bool), "Should be under memory pressure at 85% usage")
	assert.Equal(t, operationID, alert.OperationID, "Alert OperationID should match")
	assert.Equal(
		t,
		float64(0.85),
		alert.Metadata["usage_percentage"].(float64),
		"Alert UsagePercentage should match simulated usage",
	)
	assert.WithinDuration(
		t,
		time.Now(),
		alert.Timestamp,
		time.Second,
		"Alert Timestamp should be within one second of now",
	)
}

func TestMemoryLimitEnforcer_MemoryTracking_ConcurrentOperations(t *testing.T) {
	t.Parallel()

	enforcer := NewEnforcer(MemoryLimits{
		PerOperationLimit: 256 * 1024 * 1024, // 256MB
	})
	require.NotNil(t, enforcer, "NewEnforcer should return a non-nil enforcer")

	ctx := context.Background()
	operationIDs := []string{"concurrent-op-1", "concurrent-op-2", "concurrent-op-3"}

	// Start multiple concurrent operations
	for _, opID := range operationIDs {
		err := enforcer.StartMonitoring(ctx, opID)
		require.NoError(t, err, "StartMonitoring should not return an error for operation %s", opID)
	}

	// Check memory usage for each operation
	for _, opID := range operationIDs {
		usage, err := enforcer.CheckMemoryUsageByOperation(opID)
		require.NoError(t, err, "CheckMemoryUsageByOperation should not return an error for operation %s", opID)
		assert.Equal(t, opID, usage.OperationID, "Usage OperationID should match for operation %s", opID)
		assert.NotZero(t, usage.Current, "Current memory usage should not be zero for operation %s", opID)
	}

	// Verify total memory stats account for all operations
	stats := enforcer.GetMemoryStats()
	assert.Equal(
		t,
		len(operationIDs),
		stats.ActiveOperations,
		"ActiveOperations count should match number of started operations",
	)
	assert.NotZero(t, stats.TotalUsage, "TotalUsage should not be zero with active operations")
	assert.NotZero(t, stats.PeakUsage, "PeakUsage should not be zero with active operations")

	// Stop one operation
	err := enforcer.StopMonitoring(operationIDs[0])
	require.NoError(t, err, "StopMonitoring should not return an error for operation %s", operationIDs[0])

	// Verify that operation is no longer tracked
	_, err = enforcer.CheckMemoryUsageByOperation(operationIDs[0])
	assert.Error(t, err, "CheckMemoryUsageByOperation should return an error for stopped operation")
	assert.Contains(t, err.Error(), "not found", "Error message should indicate operation not found")

	// Verify stats are updated
	stats = enforcer.GetMemoryStats()
	assert.Equal(
		t,
		len(operationIDs)-1,
		stats.ActiveOperations,
		"ActiveOperations count should decrease after stopping operation",
	)
}
