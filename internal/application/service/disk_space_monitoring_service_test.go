package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

// DiskSpaceMonitoringService interface for monitoring disk usage and cleanup.
type DiskSpaceMonitoringService interface {
	// Disk usage monitoring
	GetCurrentDiskUsage(ctx context.Context, path string) (*DiskUsageInfo, error)
	MonitorDiskUsage(ctx context.Context, paths []string, interval time.Duration) (<-chan DiskUsageUpdate, error)
	SetDiskThreshold(path string, threshold DiskThreshold) error

	// Alerts and notifications
	GetDiskAlerts(ctx context.Context, paths []string) ([]*DiskAlert, error)
	ClearDiskAlert(ctx context.Context, alertID string) error

	// Analytics and reporting
	GetDiskUsageReport(ctx context.Context, path string, period time.Duration) (*DiskUsageReport, error)
	PredictDiskUsage(ctx context.Context, path string, queueSize int, avgRepoSize int64) (*DiskUsagePrediction, error)

	// Health checks
	CheckDiskHealth(ctx context.Context, paths []string) (*DiskHealthStatus, error)
}

// ============================================================================
// FAILING TESTS FOR DISK SPACE MONITORING
// ============================================================================

// ============================================================================
// COMPREHENSIVE FAILING TESTS FOR CheckDiskHealth REFACTORING
// ============================================================================

// TestCheckDiskHealth_ComprehensiveRefactoringValidation provides comprehensive
// failing tests for the CheckDiskHealth function to verify behavior remains
// unchanged during refactoring from 105 lines to under 100 lines.
func TestCheckDiskHealth_ComprehensiveRefactoringValidation(t *testing.T) {
	t.Run("error_cases", func(t *testing.T) {
		testCheckDiskHealthErrorCases(t)
	})

	t.Run("path_processing_logic", func(t *testing.T) {
		testCheckDiskHealthPathProcessing(t)
	})

	t.Run("health_status_determination", func(t *testing.T) {
		testCheckDiskHealthStatusDetermination(t)
	})

	t.Run("overall_health_calculation", func(t *testing.T) {
		testCheckDiskHealthOverallCalculation(t)
	})

	t.Run("metrics_recording", func(t *testing.T) {
		testCheckDiskHealthMetricsRecording(t)
	})

	t.Run("complete_response_structure", func(t *testing.T) {
		testCheckDiskHealthCompleteResponse(t)
	})
}

// testCheckDiskHealthErrorCases verifies error cases for empty paths and invalid paths.
func testCheckDiskHealthErrorCases(t *testing.T) {
	tests := []struct {
		name                    string
		paths                   []string
		expectedError           string
		expectedOperationResult string
		expectedMetricsCalls    int // Expected calls to RecordDiskOperation
	}{
		{
			name:                    "empty_paths_array_should_return_error",
			paths:                   []string{},
			expectedError:           "no paths specified",
			expectedOperationResult: "error",
			expectedMetricsCalls:    1, // One call for the error
		},
		{
			name:                    "nil_paths_array_should_return_error",
			paths:                   nil,
			expectedError:           "no paths specified",
			expectedOperationResult: "error",
			expectedMetricsCalls:    1,
		},
		{
			name:                    "path_with_invalid_substring_should_return_error",
			paths:                   []string{"/tmp/cache", "/tmp/invalid/path"},
			expectedError:           "invalid path: /tmp/invalid/path",
			expectedOperationResult: "error",
			expectedMetricsCalls:    1,
		},
		{
			name:                    "single_invalid_path_should_return_error",
			paths:                   []string{"/invalid/only"},
			expectedError:           "invalid path: /invalid/only",
			expectedOperationResult: "error",
			expectedMetricsCalls:    1,
		},
		{
			name:                    "multiple_invalid_paths_should_return_first_error",
			paths:                   []string{"/invalid/first", "/invalid/second"},
			expectedError:           "invalid path: /invalid/first",
			expectedOperationResult: "error",
			expectedMetricsCalls:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because CheckDiskHealth function doesn't exist yet
			// or doesn't behave as expected for refactoring validation
			t.Errorf(
				"EXPECTED FAILURE: CheckDiskHealth error case '%s' not properly validated - paths: %v, expected error: %s",
				tt.name,
				tt.paths,
				tt.expectedError,
			)
		})
	}
}

// testCheckDiskHealthPathProcessing verifies path processing and GetCurrentDiskUsage calls.
func testCheckDiskHealthPathProcessing(t *testing.T) {
	tests := []struct {
		name                     string
		paths                    []string
		expectedGetUsageCalls    int
		expectedPathHealthsCount int
		simulateGetUsageError    bool
		getUsageErrorPath        string
		expectedFailedPathHealth bool
	}{
		{
			name:                     "single_valid_path_should_call_get_usage_once",
			paths:                    []string{"/tmp/cache1"},
			expectedGetUsageCalls:    1,
			expectedPathHealthsCount: 1,
			simulateGetUsageError:    false,
		},
		{
			name:                     "multiple_valid_paths_should_call_get_usage_for_each",
			paths:                    []string{"/tmp/cache1", "/tmp/cache2", "/tmp/cache3"},
			expectedGetUsageCalls:    3,
			expectedPathHealthsCount: 3,
			simulateGetUsageError:    false,
		},
		{
			name:                     "path_with_get_usage_error_should_create_failed_health",
			paths:                    []string{"/tmp/cache1", "/tmp/error_path"},
			expectedGetUsageCalls:    2,
			expectedPathHealthsCount: 2,
			simulateGetUsageError:    true,
			getUsageErrorPath:        "/tmp/error_path",
			expectedFailedPathHealth: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because we need to verify GetCurrentDiskUsage
			// is called the correct number of times during path processing
			t.Errorf("EXPECTED FAILURE: Path processing validation not implemented - paths: %v, expected calls: %d",
				tt.paths, tt.expectedGetUsageCalls)
		})
	}
}

// testCheckDiskHealthStatusDetermination verifies health status logic based on path names.
func testCheckDiskHealthStatusDetermination(t *testing.T) {
	tests := []struct {
		name                 string
		paths                []string
		expectedPathHealths  []DiskHealthLevel
		expectedIssues       [][]string
		expectedActiveAlerts int
	}{
		{
			name:                 "normal_paths_should_have_good_health",
			paths:                []string{"/tmp/cache1", "/tmp/cache2"},
			expectedPathHealths:  []DiskHealthLevel{DiskHealthGood, DiskHealthGood},
			expectedIssues:       [][]string{{}, {}}, // Empty issues for good health
			expectedActiveAlerts: 0,
		},
		{
			name:                 "warning_path_should_have_warning_health",
			paths:                []string{"/tmp/warning_path"},
			expectedPathHealths:  []DiskHealthLevel{DiskHealthWarning},
			expectedIssues:       [][]string{{"high_disk_usage"}},
			expectedActiveAlerts: 1,
		},
		{
			name:                 "critical_path_should_have_critical_health",
			paths:                []string{"/tmp/critical_path"},
			expectedPathHealths:  []DiskHealthLevel{DiskHealthCritical},
			expectedIssues:       [][]string{{"critical_disk_usage", "performance_degraded"}},
			expectedActiveAlerts: 2, // Critical paths add 2 alerts
		},
		{
			name:                 "mixed_paths_should_have_corresponding_health",
			paths:                []string{"/tmp/normal", "/tmp/warning_high", "/tmp/critical_low"},
			expectedPathHealths:  []DiskHealthLevel{DiskHealthGood, DiskHealthWarning, DiskHealthCritical},
			expectedIssues:       [][]string{{}, {"high_disk_usage"}, {"critical_disk_usage", "performance_degraded"}},
			expectedActiveAlerts: 3, // 0 + 1 + 2 = 3
		},
		{
			name:                 "multiple_warning_paths_should_accumulate_alerts",
			paths:                []string{"/tmp/warning_1", "/tmp/warning_2", "/tmp/warning_3"},
			expectedPathHealths:  []DiskHealthLevel{DiskHealthWarning, DiskHealthWarning, DiskHealthWarning},
			expectedIssues:       [][]string{{"high_disk_usage"}, {"high_disk_usage"}, {"high_disk_usage"}},
			expectedActiveAlerts: 3, // 1 + 1 + 1 = 3
		},
		{
			name:                "multiple_critical_paths_should_accumulate_alerts",
			paths:               []string{"/tmp/critical_1", "/tmp/critical_2"},
			expectedPathHealths: []DiskHealthLevel{DiskHealthCritical, DiskHealthCritical},
			expectedIssues: [][]string{
				{"critical_disk_usage", "performance_degraded"},
				{"critical_disk_usage", "performance_degraded"},
			},
			expectedActiveAlerts: 4, // 2 + 2 = 4
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because we need to verify health status determination logic
			t.Errorf("EXPECTED FAILURE: Health status determination not validated - paths: %v, expected alerts: %d",
				tt.paths, tt.expectedActiveAlerts)
		})
	}
}

// testCheckDiskHealthOverallCalculation verifies overall health calculation logic.
func testCheckDiskHealthOverallCalculation(t *testing.T) {
	tests := []struct {
		name                  string
		paths                 []string
		expectedOverallHealth DiskHealthLevel
		expectedHealthString  string // For metrics
		simulateGetUsageError bool
		getUsageErrorPath     string
	}{
		{
			name:                  "all_good_paths_should_result_in_good_overall",
			paths:                 []string{"/tmp/cache1", "/tmp/cache2"},
			expectedOverallHealth: DiskHealthGood,
			expectedHealthString:  "good",
		},
		{
			name:                  "good_and_warning_should_result_in_warning_overall",
			paths:                 []string{"/tmp/cache1", "/tmp/warning_cache"},
			expectedOverallHealth: DiskHealthWarning,
			expectedHealthString:  "warning",
		},
		{
			name:                  "warning_and_critical_should_result_in_critical_overall",
			paths:                 []string{"/tmp/warning_cache", "/tmp/critical_cache"},
			expectedOverallHealth: DiskHealthCritical,
			expectedHealthString:  "critical",
		},
		{
			name:                  "any_failed_path_should_result_in_failed_overall",
			paths:                 []string{"/tmp/cache1", "/tmp/error_path"},
			expectedOverallHealth: DiskHealthFailed,
			expectedHealthString:  "failed",
			simulateGetUsageError: true,
			getUsageErrorPath:     "/tmp/error_path",
		},
		{
			name:                  "failed_overrides_critical",
			paths:                 []string{"/tmp/critical_cache", "/tmp/error_path"},
			expectedOverallHealth: DiskHealthFailed,
			expectedHealthString:  "failed",
			simulateGetUsageError: true,
			getUsageErrorPath:     "/tmp/error_path",
		},
		{
			name:                  "critical_overrides_warning",
			paths:                 []string{"/tmp/warning_cache", "/tmp/critical_cache"},
			expectedOverallHealth: DiskHealthCritical,
			expectedHealthString:  "critical",
		},
		{
			name:                  "warning_overrides_good",
			paths:                 []string{"/tmp/cache1", "/tmp/warning_cache"},
			expectedOverallHealth: DiskHealthWarning,
			expectedHealthString:  "warning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because overall health calculation logic needs validation
			t.Errorf("EXPECTED FAILURE: Overall health calculation not validated - paths: %v, expected: %v (%s)",
				tt.paths, tt.expectedOverallHealth, tt.expectedHealthString)
		})
	}
}

// testCheckDiskHealthMetricsRecording verifies metrics recording calls.
func testCheckDiskHealthMetricsRecording(t *testing.T) {
	tests := []struct {
		name                         string
		paths                        []string
		expectedOperationResult      string
		expectedHealthString         string
		expectedPathsChecked         int
		expectedActiveAlerts         int
		expectedRecordOperationCalls int // Calls to RecordDiskOperation
		expectedRecordHealthCalls    int // Calls to RecordDiskHealthCheck
		simulateError                bool
	}{
		{
			name:                         "successful_check_should_record_success_metrics",
			paths:                        []string{"/tmp/cache1", "/tmp/cache2"},
			expectedOperationResult:      "success",
			expectedHealthString:         "good",
			expectedPathsChecked:         2,
			expectedActiveAlerts:         0,
			expectedRecordOperationCalls: 1, // One success call
			expectedRecordHealthCalls:    1, // One health check call
			simulateError:                false,
		},
		{
			name:                         "error_case_should_record_error_metrics",
			paths:                        []string{},
			expectedOperationResult:      "error",
			expectedHealthString:         "", // No health check call for errors
			expectedPathsChecked:         0,
			expectedActiveAlerts:         0,
			expectedRecordOperationCalls: 1, // One error call
			expectedRecordHealthCalls:    0, // No health check call for errors
			simulateError:                true,
		},
		{
			name:                         "warning_paths_should_record_correct_alert_count",
			paths:                        []string{"/tmp/warning_1", "/tmp/warning_2"},
			expectedOperationResult:      "success",
			expectedHealthString:         "warning",
			expectedPathsChecked:         2,
			expectedActiveAlerts:         2, // 1 + 1
			expectedRecordOperationCalls: 1,
			expectedRecordHealthCalls:    1,
			simulateError:                false,
		},
		{
			name:                         "critical_paths_should_record_correct_alert_count",
			paths:                        []string{"/tmp/critical_1", "/tmp/critical_2"},
			expectedOperationResult:      "success",
			expectedHealthString:         "critical",
			expectedPathsChecked:         2,
			expectedActiveAlerts:         4, // 2 + 2
			expectedRecordOperationCalls: 1,
			expectedRecordHealthCalls:    1,
			simulateError:                false,
		},
		{
			name:                         "mixed_paths_should_record_total_alert_count",
			paths:                        []string{"/tmp/normal", "/tmp/warning_path", "/tmp/critical_path"},
			expectedOperationResult:      "success",
			expectedHealthString:         "critical", // Critical overrides warning
			expectedPathsChecked:         3,
			expectedActiveAlerts:         3, // 0 + 1 + 2
			expectedRecordOperationCalls: 1,
			expectedRecordHealthCalls:    1,
			simulateError:                false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because metrics recording validation is not implemented
			t.Errorf(
				"EXPECTED FAILURE: Metrics recording not validated - operation calls: %d, health calls: %d, alerts: %d",
				tt.expectedRecordOperationCalls,
				tt.expectedRecordHealthCalls,
				tt.expectedActiveAlerts,
			)
		})
	}
}

// testCheckDiskHealthCompleteResponse verifies complete DiskHealthStatus structure.
func testCheckDiskHealthCompleteResponse(t *testing.T) {
	tests := []struct {
		name                      string
		paths                     []string
		expectedOverallHealth     DiskHealthLevel
		expectedPathCount         int
		expectedActiveAlerts      int
		expectedRecentErrorsCount int
		expectedPerformanceNotNil bool
		expectedLastCheckedRecent bool     // Within last few seconds
		expectedPathHealthFields  []string // Fields to verify in PathHealthStatus
		expectedDiskErrorFields   []string // Fields to verify in DiskError for failed paths
		simulateGetUsageError     bool
		getUsageErrorPath         string
	}{
		{
			name:                      "successful_response_should_have_complete_structure",
			paths:                     []string{"/tmp/cache1", "/tmp/warning_cache"},
			expectedOverallHealth:     DiskHealthWarning,
			expectedPathCount:         2,
			expectedActiveAlerts:      1,
			expectedRecentErrorsCount: 0, // Empty slice but not nil
			expectedPerformanceNotNil: true,
			expectedLastCheckedRecent: true,
			expectedPathHealthFields:  []string{"Path", "Health", "Issues", "IOLatency", "ErrorRate"},
			simulateGetUsageError:     false,
		},
		{
			name:                      "failed_path_should_create_disk_error",
			paths:                     []string{"/tmp/cache1", "/tmp/error_path"},
			expectedOverallHealth:     DiskHealthFailed,
			expectedPathCount:         2,
			expectedActiveAlerts:      0, // Failed paths don't increment alerts directly
			expectedRecentErrorsCount: 0, // Still empty - errors are in PathHealthStatus
			expectedPerformanceNotNil: true,
			expectedLastCheckedRecent: true,
			expectedPathHealthFields:  []string{"Path", "Health", "Issues", "LastError", "IOLatency", "ErrorRate"},
			expectedDiskErrorFields:   []string{"ID", "Type", "Message", "Path", "Timestamp", "Severity"},
			simulateGetUsageError:     true,
			getUsageErrorPath:         "/tmp/error_path",
		},
		{
			name:                      "performance_metrics_should_have_expected_values",
			paths:                     []string{"/tmp/cache1"},
			expectedOverallHealth:     DiskHealthGood,
			expectedPathCount:         1,
			expectedActiveAlerts:      0,
			expectedRecentErrorsCount: 0,
			expectedPerformanceNotNil: true,
			expectedLastCheckedRecent: true,
			expectedPathHealthFields:  []string{"Path", "Health", "Issues", "IOLatency", "ErrorRate"},
			simulateGetUsageError:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because complete response structure validation is not implemented
			t.Errorf(
				"EXPECTED FAILURE: Complete response structure not validated - paths: %v, expected structure fields: %v",
				tt.paths,
				tt.expectedPathHealthFields,
			)
		})
	}
}

// Additional failing test for PathHealthStatus field validation during refactoring.
func TestCheckDiskHealth_PathHealthStatusFieldValidation(t *testing.T) {
	tests := []struct {
		name                  string
		path                  string
		expectedHealth        DiskHealthLevel
		expectedIssues        []string
		expectedIOLatency     float64
		expectedErrorRate     float64
		expectedLastErrorNil  bool
		simulateGetUsageError bool
	}{
		{
			name:                  "good_path_should_have_correct_fields",
			path:                  "/tmp/normal_cache",
			expectedHealth:        DiskHealthGood,
			expectedIssues:        []string{}, // Empty but not nil
			expectedIOLatency:     2.0,        // From mock GetCurrentDiskUsage
			expectedErrorRate:     0.0,
			expectedLastErrorNil:  true,
			simulateGetUsageError: false,
		},
		{
			name:                  "warning_path_should_have_correct_fields",
			path:                  "/tmp/warning_cache",
			expectedHealth:        DiskHealthWarning,
			expectedIssues:        []string{"high_disk_usage"},
			expectedIOLatency:     2.0,
			expectedErrorRate:     0.0,
			expectedLastErrorNil:  true,
			simulateGetUsageError: false,
		},
		{
			name:                  "critical_path_should_have_correct_fields",
			path:                  "/tmp/critical_cache",
			expectedHealth:        DiskHealthCritical,
			expectedIssues:        []string{"critical_disk_usage", "performance_degraded"},
			expectedIOLatency:     2.0,
			expectedErrorRate:     0.0,
			expectedLastErrorNil:  true,
			simulateGetUsageError: false,
		},
		{
			name:                  "failed_path_should_have_error_fields",
			path:                  "/tmp/error_cache",
			expectedHealth:        DiskHealthFailed,
			expectedIssues:        []string{"cannot_access_path"},
			expectedIOLatency:     -1, // Error case
			expectedErrorRate:     1.0,
			expectedLastErrorNil:  false,
			simulateGetUsageError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because PathHealthStatus field validation is not implemented
			t.Errorf(
				"EXPECTED FAILURE: PathHealthStatus fields not validated for path '%s' - expected health: %v, issues: %v",
				tt.path,
				tt.expectedHealth,
				tt.expectedIssues,
			)
		})
	}
}

// Additional failing test for DiskError field validation during refactoring.
func TestCheckDiskHealth_DiskErrorFieldValidation(t *testing.T) {
	tests := []struct {
		name                    string
		path                    string
		expectedErrorType       string
		expectedErrorSeverity   string
		expectedErrorIDNotEmpty bool
		expectedErrorTimeRecent bool
	}{
		{
			name:                    "access_error_should_create_proper_disk_error",
			path:                    "/tmp/access_error",
			expectedErrorType:       "access_error",
			expectedErrorSeverity:   "high",
			expectedErrorIDNotEmpty: true,
			expectedErrorTimeRecent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because DiskError field validation is not implemented
			t.Errorf("EXPECTED FAILURE: DiskError fields not validated for path '%s' - expected type: %s, severity: %s",
				tt.path, tt.expectedErrorType, tt.expectedErrorSeverity)
		})
	}
}

func TestDiskSpaceMonitoringService_GetCurrentDiskUsage(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		expectedUsage *DiskUsageInfo
		expectError   bool
		errorType     string
	}{
		{
			name: "get current disk usage for repository cache directory",
			path: "/tmp/codechunking-cache",
			expectedUsage: &DiskUsageInfo{
				Path:            "/tmp/codechunking-cache",
				TotalSpaceBytes: 100 * 1024 * 1024 * 1024, // 100GB
				UsedSpaceBytes:  25 * 1024 * 1024 * 1024,  // 25GB
				AvailableBytes:  75 * 1024 * 1024 * 1024,  // 75GB
				UsagePercentage: 25.0,
				CacheUsageBytes: 20 * 1024 * 1024 * 1024, // 20GB
				RepositoryCount: 150,
				IOPSCurrent:     500,
				ReadLatencyMs:   2.5,
				WriteLatencyMs:  3.2,
			},
			expectError: false,
		},
		{
			name:          "get disk usage for non-existent path",
			path:          "/non/existent/path",
			expectedUsage: nil,
			expectError:   true,
			errorType:     "path_not_found",
		},
		{
			name:          "get disk usage for path with permission denied",
			path:          "/root/restricted",
			expectedUsage: nil,
			expectError:   true,
			errorType:     "permission_denied",
		},
		{
			name:          "get disk usage for path on failed disk",
			path:          "/mnt/failed-disk",
			expectedUsage: nil,
			expectError:   true,
			errorType:     "disk_io_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testGetCurrentDiskUsage(t, tt)
		})
	}
}

func testGetCurrentDiskUsage(t *testing.T, tt struct {
	name          string
	path          string
	expectedUsage *DiskUsageInfo
	expectError   bool
	errorType     string
},
) {
	// Create concrete implementation for GREEN phase
	service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
	ctx := context.Background()

	usage, err := service.GetCurrentDiskUsage(ctx, tt.path)

	if tt.expectError {
		if err == nil {
			t.Errorf("Expected error of type %s, got nil", tt.errorType)
		}
		// Test should validate error type when implemented
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	validateDiskUsageResult(t, usage, tt.expectedUsage)
}

func validateDiskUsageResult(t *testing.T, usage, expected *DiskUsageInfo) {
	if usage == nil {
		t.Fatal("Expected disk usage info, got nil")
	}

	if usage.Path != expected.Path {
		t.Errorf("Expected path %s, got %s", expected.Path, usage.Path)
	}

	if usage.UsagePercentage != expected.UsagePercentage {
		t.Errorf("Expected usage %f%%, got %f%%", expected.UsagePercentage, usage.UsagePercentage)
	}

	// Validate that LastUpdated is recent (within last minute)
	if time.Since(usage.LastUpdated) > time.Minute {
		t.Errorf("LastUpdated should be recent, got %v", usage.LastUpdated)
	}
}

func TestDiskSpaceMonitoringService_MonitorDiskUsage(t *testing.T) {
	tests := []struct {
		name            string
		paths           []string
		interval        time.Duration
		expectedUpdates int
		monitorDuration time.Duration
		expectError     bool
	}{
		{
			name:            "monitor single cache directory with 1-second intervals",
			paths:           []string{"/tmp/codechunking-cache"},
			interval:        1 * time.Second,
			expectedUpdates: 3,
			monitorDuration: 3 * time.Second,
			expectError:     false,
		},
		{
			name:            "monitor multiple paths simultaneously",
			paths:           []string{"/tmp/cache1", "/tmp/cache2", "/var/cache"},
			interval:        500 * time.Millisecond,
			expectedUpdates: 6, // 2 updates per path
			monitorDuration: 1 * time.Second,
			expectError:     false,
		},
		{
			name:            "monitor with very short interval should rate-limit",
			paths:           []string{"/tmp/codechunking-cache"},
			interval:        10 * time.Millisecond, // Too frequent
			expectedUpdates: 1,                     // Should be rate-limited
			monitorDuration: 100 * time.Millisecond,
			expectError:     false,
		},
		{
			name:        "monitor invalid paths should return error",
			paths:       []string{"/invalid/path"},
			interval:    1 * time.Second,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCase := diskMonitorTestCase{
				paths:           tt.paths,
				interval:        tt.interval,
				expectedUpdates: tt.expectedUpdates,
				monitorDuration: tt.monitorDuration,
				expectError:     tt.expectError,
			}
			testMonitorDiskUsage(t, testCase)
		})
	}
}

type diskMonitorTestCase struct {
	paths           []string
	interval        time.Duration
	expectedUpdates int
	monitorDuration time.Duration
	expectError     bool
}

func testMonitorDiskUsage(t *testing.T, tt diskMonitorTestCase) {
	// Create concrete implementation for GREEN phase
	service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
	ctx, cancel := context.WithTimeout(context.Background(), tt.monitorDuration)
	defer cancel()

	updatesChan, err := service.MonitorDiskUsage(ctx, tt.paths, tt.interval)

	if tt.expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if updatesChan == nil {
		t.Fatal("Expected updates channel, got nil")
	}

	updates := collectDiskUsageUpdates(ctx, updatesChan)
	validateDiskUsageUpdates(t, updates, tt.expectedUpdates, tt.paths)
}

func collectDiskUsageUpdates(ctx context.Context, updatesChan <-chan DiskUsageUpdate) []DiskUsageUpdate {
	var updates []DiskUsageUpdate
	done := make(chan struct{})

	go func() {
		for update := range updatesChan {
			updates = append(updates, update)
		}
		close(done)
	}()

	<-ctx.Done()
	<-done
	return updates
}

func validateDiskUsageUpdates(t *testing.T, updates []DiskUsageUpdate, expectedCount int, paths []string) {
	if len(updates) < expectedCount {
		t.Errorf("Expected at least %d updates, got %d", expectedCount, len(updates))
	}

	for _, update := range updates {
		if update.Usage == nil {
			t.Error("Update should contain disk usage info")
		}
		if update.Timestamp.IsZero() {
			t.Error("Update should have timestamp")
		}
		if !containsString(paths, update.Path) {
			t.Errorf("Update path %s not in monitored paths", update.Path)
		}
	}
}

func TestDiskSpaceMonitoringService_SetDiskThreshold(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		threshold   DiskThreshold
		expectError bool
	}{
		{
			name: "set valid disk threshold for cache directory",
			path: "/tmp/codechunking-cache",
			threshold: DiskThreshold{
				WarningPercentage:  75.0,
				CriticalPercentage: 90.0,
				MaxUsageBytes:      80 * 1024 * 1024 * 1024, // 80GB
				MinFreeBytes:       10 * 1024 * 1024 * 1024, // 10GB
			},
			expectError: false,
		},
		{
			name: "set threshold with invalid percentages should error",
			path: "/tmp/codechunking-cache",
			threshold: DiskThreshold{
				WarningPercentage:  95.0, // Warning higher than critical
				CriticalPercentage: 80.0,
			},
			expectError: true,
		},
		{
			name: "set threshold with negative values should error",
			path: "/tmp/codechunking-cache",
			threshold: DiskThreshold{
				WarningPercentage:  -10.0,
				CriticalPercentage: 90.0,
			},
			expectError: true,
		},
		{
			name: "set threshold for non-existent path should error",
			path: "/non/existent/path",
			threshold: DiskThreshold{
				WarningPercentage:  75.0,
				CriticalPercentage: 90.0,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create concrete implementation for GREEN phase
			service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())

			err := service.SetDiskThreshold(tt.path, tt.threshold)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestDiskSpaceMonitoringService_GetDiskAlerts(t *testing.T) {
	tests := []struct {
		name            string
		paths           []string
		setupThresholds func(DiskSpaceMonitoringService)
		expectedAlerts  int
		expectError     bool
	}{
		{
			name:  "get alerts for paths with no issues",
			paths: []string{"/tmp/normal-cache"},
			setupThresholds: func(service DiskSpaceMonitoringService) {
				service.SetDiskThreshold("/tmp/normal-cache", DiskThreshold{
					WarningPercentage:  75.0,
					CriticalPercentage: 90.0,
				})
			},
			expectedAlerts: 0,
			expectError:    false,
		},
		{
			name:  "get alerts for paths exceeding warning threshold",
			paths: []string{"/tmp/warning-cache"},
			setupThresholds: func(service DiskSpaceMonitoringService) {
				service.SetDiskThreshold("/tmp/warning-cache", DiskThreshold{
					WarningPercentage:  50.0, // Low threshold to trigger warning
					CriticalPercentage: 90.0,
				})
			},
			expectedAlerts: 1,
			expectError:    false,
		},
		{
			name:  "get alerts for paths exceeding critical threshold",
			paths: []string{"/tmp/critical-cache"},
			setupThresholds: func(service DiskSpaceMonitoringService) {
				service.SetDiskThreshold("/tmp/critical-cache", DiskThreshold{
					WarningPercentage:  50.0,
					CriticalPercentage: 60.0, // Low threshold to trigger critical
				})
			},
			expectedAlerts: 1,
			expectError:    false,
		},
		{
			name:           "get alerts for invalid paths should error",
			paths:          []string{"/invalid/path"},
			expectedAlerts: 0,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCase := diskAlertsTestCase{
				paths:           tt.paths,
				setupThresholds: tt.setupThresholds,
				expectedAlerts:  tt.expectedAlerts,
				expectError:     tt.expectError,
			}
			testGetDiskAlerts(t, testCase)
		})
	}
}

type diskAlertsTestCase struct {
	paths           []string
	setupThresholds func(DiskSpaceMonitoringService)
	expectedAlerts  int
	expectError     bool
}

func testGetDiskAlerts(t *testing.T, tt diskAlertsTestCase) {
	// Create concrete implementation for GREEN phase
	service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
	ctx := context.Background()

	if tt.setupThresholds != nil {
		tt.setupThresholds(service)
	}

	alerts, err := service.GetDiskAlerts(ctx, tt.paths)

	if tt.expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(alerts) != tt.expectedAlerts {
		t.Errorf("Expected %d alerts, got %d", tt.expectedAlerts, len(alerts))
	}

	validateAlertStructures(t, alerts)
}

func validateAlertStructures(t *testing.T, alerts []*DiskAlert) {
	for _, alert := range alerts {
		if alert.ID == "" {
			t.Error("Alert should have ID")
		}
		if alert.CurrentUsage == nil {
			t.Error("Alert should contain current usage info")
		}
		if alert.CreatedAt.IsZero() {
			t.Error("Alert should have creation time")
		}
		if len(alert.Actions) == 0 {
			t.Error("Alert should suggest recommended actions")
		}
	}
}

func TestDiskSpaceMonitoringService_PredictDiskUsage(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		queueSize          int
		avgRepoSize        int64
		expectedPrediction *DiskUsagePrediction
		expectError        bool
	}{
		{
			name:        "predict usage with moderate queue and average repo size",
			path:        "/tmp/codechunking-cache",
			queueSize:   100,
			avgRepoSize: 50 * 1024 * 1024, // 50MB
			expectedPrediction: &DiskUsagePrediction{
				Path:                 "/tmp/codechunking-cache",
				PredictedUsage:       30 * 1024 * 1024 * 1024, // 30GB
				TimeToFull:           72 * time.Hour,
				ConfidenceLevel:      0.85,
				RecommendedCleanupGB: 5,
				PredictionModel:      "linear_regression",
				Factors:              []string{"queue_size", "avg_repo_size", "historical_growth"},
			},
			expectError: false,
		},
		{
			name:        "predict usage with large queue should recommend more cleanup",
			path:        "/tmp/codechunking-cache",
			queueSize:   1000,
			avgRepoSize: 100 * 1024 * 1024, // 100MB
			expectedPrediction: &DiskUsagePrediction{
				RecommendedCleanupGB: 20,
				TimeToFull:           12 * time.Hour,
				ConfidenceLevel:      0.95,
			},
			expectError: false,
		},
		{
			name:        "predict usage with zero queue size",
			path:        "/tmp/codechunking-cache",
			queueSize:   0,
			avgRepoSize: 50 * 1024 * 1024,
			expectedPrediction: &DiskUsagePrediction{
				RecommendedCleanupGB: 0,
				TimeToFull:           0, // No growth expected
			},
			expectError: false,
		},
		{
			name:        "predict usage for invalid path should error",
			path:        "/invalid/path",
			queueSize:   10,
			avgRepoSize: 50 * 1024 * 1024,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCase := diskPredictionTestCase{
				path:               tt.path,
				queueSize:          tt.queueSize,
				avgRepoSize:        tt.avgRepoSize,
				expectedPrediction: tt.expectedPrediction,
				expectError:        tt.expectError,
			}
			testPredictDiskUsage(t, testCase)
		})
	}
}

type diskPredictionTestCase struct {
	path               string
	queueSize          int
	avgRepoSize        int64
	expectedPrediction *DiskUsagePrediction
	expectError        bool
}

func testPredictDiskUsage(t *testing.T, tt diskPredictionTestCase) {
	// Create concrete implementation for GREEN phase
	service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
	ctx := context.Background()

	prediction, err := service.PredictDiskUsage(ctx, tt.path, tt.queueSize, tt.avgRepoSize)

	if tt.expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if prediction == nil {
		t.Fatal("Expected prediction, got nil")
	}

	validateBasicPredictionFields(t, prediction, tt.path)
	validatePredictionExpectations(t, prediction, tt.expectedPrediction)
}

func validateBasicPredictionFields(t *testing.T, prediction *DiskUsagePrediction, expectedPath string) {
	if prediction.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, prediction.Path)
	}

	if prediction.ConfidenceLevel < 0 || prediction.ConfidenceLevel > 1 {
		t.Errorf("Confidence level should be between 0 and 1, got %f", prediction.ConfidenceLevel)
	}

	if prediction.PredictionModel == "" {
		t.Error("Prediction should specify model used")
	}

	if len(prediction.Factors) == 0 {
		t.Error("Prediction should list factors considered")
	}
}

func validatePredictionExpectations(t *testing.T, prediction, expected *DiskUsagePrediction) {
	if expected == nil {
		return
	}

	if expected.RecommendedCleanupGB > 0 {
		if prediction.RecommendedCleanupGB < expected.RecommendedCleanupGB {
			t.Errorf("Expected at least %d GB cleanup, got %d GB",
				expected.RecommendedCleanupGB, prediction.RecommendedCleanupGB)
		}
	}

	if expected.ConfidenceLevel > 0 {
		if prediction.ConfidenceLevel < expected.ConfidenceLevel {
			t.Errorf("Expected confidence at least %f, got %f",
				expected.ConfidenceLevel, prediction.ConfidenceLevel)
		}
	}
}

func TestDiskSpaceMonitoringService_GetDiskUsageReport(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		period         time.Duration
		expectedReport *DiskUsageReport
		expectError    bool
	}{
		{
			name:   "get 24-hour disk usage report",
			path:   "/tmp/codechunking-cache",
			period: 24 * time.Hour,
			expectedReport: &DiskUsageReport{
				Path:               "/tmp/codechunking-cache",
				Period:             24 * time.Hour,
				AverageUsage:       65.5,
				PeakUsage:          82.3,
				GrowthRate:         2.1, // 2.1% growth per day
				RecommendedActions: []string{"cleanup_old_repositories", "enable_compression"},
				EfficiencyScore:    0.78,
			},
			expectError: false,
		},
		{
			name:   "get 7-day disk usage report with trend analysis",
			path:   "/tmp/codechunking-cache",
			period: 7 * 24 * time.Hour,
			expectedReport: &DiskUsageReport{
				GrowthRate:         1.8, // Lower growth rate over longer period
				EfficiencyScore:    0.72,
				RecommendedActions: []string{"implement_lru_cleanup", "increase_cache_compression"},
			},
			expectError: false,
		},
		{
			name:        "get report for invalid path should error",
			path:        "/invalid/path",
			period:      24 * time.Hour,
			expectError: true,
		},
		{
			name:        "get report with zero period should error",
			path:        "/tmp/codechunking-cache",
			period:      0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCase := diskUsageReportTestCase{
				path:           tt.path,
				period:         tt.period,
				expectedReport: tt.expectedReport,
				expectError:    tt.expectError,
			}
			testGetDiskUsageReport(t, testCase)
		})
	}
}

type diskUsageReportTestCase struct {
	path           string
	period         time.Duration
	expectedReport *DiskUsageReport
	expectError    bool
}

func testGetDiskUsageReport(t *testing.T, tt diskUsageReportTestCase) {
	// Create concrete implementation for GREEN phase
	service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
	ctx := context.Background()

	report, err := service.GetDiskUsageReport(ctx, tt.path, tt.period)

	if tt.expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if report == nil {
		t.Fatal("Expected report, got nil")
	}

	validateBasicReportFields(t, report, tt.path, tt.period)
	validateReportContents(t, report)
	validateReportExpectations(t, report, tt.expectedReport)
}

func validateBasicReportFields(
	t *testing.T,
	report *DiskUsageReport,
	expectedPath string,
	expectedPeriod time.Duration,
) {
	if report.Path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, report.Path)
	}

	if report.Period != expectedPeriod {
		t.Errorf("Expected period %v, got %v", expectedPeriod, report.Period)
	}

	if report.AverageUsage < 0 || report.AverageUsage > 100 {
		t.Errorf("Average usage should be percentage 0-100, got %f", report.AverageUsage)
	}

	if report.EfficiencyScore < 0 || report.EfficiencyScore > 1 {
		t.Errorf("Efficiency score should be 0-1, got %f", report.EfficiencyScore)
	}
}

func validateReportContents(t *testing.T, report *DiskUsageReport) {
	if len(report.UsageHistory) == 0 {
		t.Error("Report should contain usage history")
	}

	if len(report.TopRepositories) == 0 {
		t.Error("Report should list top repositories by usage")
	}

	if len(report.RecommendedActions) == 0 {
		t.Error("Report should provide recommended actions")
	}
}

func validateReportExpectations(t *testing.T, report *DiskUsageReport, expected *DiskUsageReport) {
	if expected == nil {
		return
	}

	if expected.GrowthRate > 0 && report.GrowthRate <= 0 {
		t.Error("Report should show positive growth rate for active system")
	}
}

func TestDiskSpaceMonitoringService_CheckDiskHealth(t *testing.T) {
	tests := []struct {
		name           string
		paths          []string
		expectedHealth DiskHealthLevel
		expectedAlerts int
		expectError    bool
	}{
		{
			name:           "check health of normal paths",
			paths:          []string{"/tmp/cache1", "/tmp/cache2"},
			expectedHealth: DiskHealthGood,
			expectedAlerts: 0,
			expectError:    false,
		},
		{
			name:           "check health with warning conditions",
			paths:          []string{"/tmp/warning-cache"},
			expectedHealth: DiskHealthWarning,
			expectedAlerts: 1,
			expectError:    false,
		},
		{
			name:           "check health with critical conditions",
			paths:          []string{"/tmp/critical-cache"},
			expectedHealth: DiskHealthCritical,
			expectedAlerts: 2,
			expectError:    false,
		},
		{
			name:        "check health of invalid paths should error",
			paths:       []string{"/invalid/path"},
			expectError: true,
		},
		{
			name:        "check health with empty path list should error",
			paths:       []string{},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCase := diskHealthTestCase{
				paths:          tt.paths,
				expectedHealth: tt.expectedHealth,
				expectedAlerts: tt.expectedAlerts,
				expectError:    tt.expectError,
			}
			testCheckDiskHealth(t, testCase)
		})
	}
}

type diskHealthTestCase struct {
	paths          []string
	expectedHealth DiskHealthLevel
	expectedAlerts int
	expectError    bool
}

func testCheckDiskHealth(t *testing.T, tt diskHealthTestCase) {
	// Create concrete implementation for GREEN phase
	service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
	ctx := context.Background()

	health, err := service.CheckDiskHealth(ctx, tt.paths)

	if tt.expectError {
		if err == nil {
			t.Error("Expected error, got nil")
		}
		return
	}

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if health == nil {
		t.Fatal("Expected health status, got nil")
	}

	validateHealthBasics(t, health, tt.expectedHealth, tt.expectedAlerts, len(tt.paths))
	validateHealthStructure(t, health)
	validatePathHealthEntries(t, health.Paths, tt.paths)
}

func validateHealthBasics(
	t *testing.T,
	health *DiskHealthStatus,
	expectedHealth DiskHealthLevel,
	expectedAlerts, expectedPaths int,
) {
	if health.OverallHealth != expectedHealth {
		t.Errorf("Expected health level %v, got %v", expectedHealth, health.OverallHealth)
	}

	if health.ActiveAlerts != expectedAlerts {
		t.Errorf("Expected %d active alerts, got %d", expectedAlerts, health.ActiveAlerts)
	}

	if len(health.Paths) != expectedPaths {
		t.Errorf("Expected health status for %d paths, got %d", expectedPaths, len(health.Paths))
	}
}

func validateHealthStructure(t *testing.T, health *DiskHealthStatus) {
	if health.Performance == nil {
		t.Error("Health status should include performance metrics")
	}

	if health.LastChecked.IsZero() {
		t.Error("Health status should have last checked time")
	}
}

func validatePathHealthEntries(t *testing.T, pathHealths []*PathHealthStatus, expectedPaths []string) {
	for _, pathHealth := range pathHealths {
		if pathHealth.Path == "" {
			t.Error("Path health should specify path")
		}
		if !containsString(expectedPaths, pathHealth.Path) {
			t.Errorf("Unexpected path in health status: %s", pathHealth.Path)
		}
	}
}

// ============================================================================
// FAILING TESTS FOR GOCONST VIOLATION - CACHE DIRECTORY CONSTANT
// ============================================================================
// These tests address the goconst linting violation where "/tmp/codechunking-cache"
// appears multiple times and should be made into a constant.
// Specific linting violation: goconst - string `/tmp/codechunking-cache` has 3 occurrences, make it a constant
// File: internal/application/service/disk_space_monitoring_service.go:61:13

// TestDefaultCacheDirectoryConstantExists tests that a constant exists for the default cache directory path.
// This test will FAIL initially because the constant does not exist yet.
func TestDefaultCacheDirectoryConstantExists(t *testing.T) {
	tests := []struct {
		name                  string
		expectedConstantValue string
		expectedConstantName  string
	}{
		{
			name:                  "default cache directory constant should exist with correct value",
			expectedConstantValue: "/tmp/codechunking-cache",
			expectedConstantName:  "DefaultCacheDirectory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will FAIL because DefaultCacheDirectory constant doesn't exist yet
			// The constant should be defined in disk_space_monitoring_service.go
			if DefaultCacheDirectory != tt.expectedConstantValue {
				t.Errorf("Expected DefaultCacheDirectory constant to equal %s, got %s",
					tt.expectedConstantValue, DefaultCacheDirectory)
			}

			// Verify the constant is a string type and not empty
			if DefaultCacheDirectory == "" {
				t.Error("DefaultCacheDirectory constant should not be empty")
			}

			// Verify the constant follows expected path format
			if !strings.HasPrefix(DefaultCacheDirectory, "/tmp/") {
				t.Errorf("DefaultCacheDirectory should start with /tmp/, got %s", DefaultCacheDirectory)
			}
		})
	}
}

// TestDiskSpaceMonitoringService_UsesDefaultCacheDirectoryConstant tests that the service
// uses the DefaultCacheDirectory constant instead of hardcoded strings.
// This test will FAIL initially because the implementation still uses hardcoded strings.
func TestDiskSpaceMonitoringService_UsesDefaultCacheDirectoryConstant(t *testing.T) {
	tests := []struct {
		name               string
		path               string
		useConstantPath    bool
		expectedBehavior   string
		expectedUsageBytes int64
	}{
		{
			name:               "service should recognize DefaultCacheDirectory constant path",
			path:               DefaultCacheDirectory, // This will fail compilation initially
			useConstantPath:    true,
			expectedBehavior:   "cache_directory_recognized",
			expectedUsageBytes: 25 * 1024 * 1024 * 1024, // 25GB as per existing logic
		},
		{
			name:               "service behavior should be identical for constant and hardcoded path",
			path:               DefaultCacheDirectory, // This will fail compilation initially
			useConstantPath:    true,
			expectedBehavior:   "consistent_behavior",
			expectedUsageBytes: 25 * 1024 * 1024 * 1024,
		},
		{
			name:               "hardcoded path should be eliminated from implementation",
			path:               "/tmp/codechunking-cache", // This should match the constant
			useConstantPath:    false,
			expectedBehavior:   "hardcoded_eliminated",
			expectedUsageBytes: 25 * 1024 * 1024 * 1024,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
			ctx := context.Background()

			usage, err := service.GetCurrentDiskUsage(ctx, tt.path)
			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if usage == nil {
				t.Fatal("Expected disk usage info, got nil")
			}

			// Verify the service recognizes the cache directory path correctly
			if usage.UsedSpaceBytes != tt.expectedUsageBytes {
				t.Errorf("Expected used space %d bytes, got %d bytes",
					tt.expectedUsageBytes, usage.UsedSpaceBytes)
			}

			// The path in the result should use the constant value
			if usage.Path != DefaultCacheDirectory {
				t.Errorf("Expected result path to use DefaultCacheDirectory constant %s, got %s",
					DefaultCacheDirectory, usage.Path)
			}

			// Verify specific cache directory attributes
			if usage.CacheUsageBytes == 0 {
				t.Error("Cache directory should report non-zero cache usage")
			}

			if usage.RepositoryCount == 0 {
				t.Error("Cache directory should report repository count")
			}
		})
	}
}

// TestDefaultCacheDirectoryConstantConsistency tests that the constant is used consistently
// across different scenarios and edge cases.
// This test will FAIL initially because the constant doesn't exist and hardcoded strings are used.
func TestDefaultCacheDirectoryConstantConsistency(t *testing.T) {
	tests := []struct {
		name                    string
		inputPath               string
		expectedPathComparison  bool
		expectedConsistentUsage bool
		testScenario            string
	}{
		{
			name:                    "constant should equal expected cache directory path",
			inputPath:               "/tmp/codechunking-cache",
			expectedPathComparison:  true,
			expectedConsistentUsage: true,
			testScenario:            "path_equality",
		},
		{
			name:                    "constant should be used for path comparisons",
			inputPath:               DefaultCacheDirectory, // This will fail compilation
			expectedPathComparison:  true,
			expectedConsistentUsage: true,
			testScenario:            "constant_comparison",
		},
		{
			name:                    "different paths should not match constant",
			inputPath:               "/different/path",
			expectedPathComparison:  false,
			expectedConsistentUsage: false,
			testScenario:            "different_path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Validate path comparison using helper function to reduce complexity
			validatePathComparison(t, tt.inputPath, tt.expectedPathComparison)

			// Test service behavior consistency
			service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
			ctx := context.Background()
			usage, err := service.GetCurrentDiskUsage(ctx, tt.inputPath)

			// Use helper functions to validate behavior based on expectations
			if tt.expectedConsistentUsage {
				validateConsistentCacheUsage(t, usage, err)
			} else {
				validateNonCacheUsage(t, usage)
			}
		})
	}
}

// TestCacheDirectoryConstantRefactoring tests that the refactoring from hardcoded strings
// to constant maintains exact same behavior.
// This test will FAIL initially because the implementation hasn't been refactored yet.
func TestCacheDirectoryConstantRefactoring(t *testing.T) {
	tests := []struct {
		name                  string
		testDescription       string
		verificationScenario  string
		expectedNoRegressions bool
		expectedConstantUsage bool
	}{
		{
			name:                  "refactoring should maintain exact behavior",
			testDescription:       "behavior before and after constant introduction should be identical",
			verificationScenario:  "behavior_preservation",
			expectedNoRegressions: true,
			expectedConstantUsage: true,
		},
		{
			name:                  "constant should eliminate hardcoded string duplication",
			testDescription:       "multiple occurrences of /tmp/codechunking-cache should use constant",
			verificationScenario:  "duplication_elimination",
			expectedNoRegressions: true,
			expectedConstantUsage: true,
		},
		{
			name:                  "constant should be publicly accessible for reuse",
			testDescription:       "constant should be available for other services and tests",
			verificationScenario:  "public_accessibility",
			expectedNoRegressions: true,
			expectedConstantUsage: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail because DefaultCacheDirectory constant doesn't exist
			service := NewDefaultDiskSpaceMonitoringService(createTestMetrics())
			ctx := context.Background()

			// Test with the constant (will fail compilation initially)
			usageWithConstant, errWithConstant := service.GetCurrentDiskUsage(ctx, DefaultCacheDirectory)

			// Test with hardcoded string for comparison
			usageWithHardcoded, errWithHardcoded := service.GetCurrentDiskUsage(ctx, "/tmp/codechunking-cache")

			// Use helper functions to validate behavior - reduces nesting complexity
			if tt.expectedNoRegressions {
				validateErrorConsistency(t, errWithConstant, errWithHardcoded)
				if errWithConstant == nil && errWithHardcoded == nil {
					validateUsageConsistency(t, usageWithConstant, usageWithHardcoded)
				}
			}

			if tt.expectedConstantUsage {
				validateConstantUsageInResponse(t, usageWithConstant)
			}
		})
	}
}

// validateErrorConsistency checks that error behavior is consistent between constant and hardcoded paths.
func validateErrorConsistency(t *testing.T, errConstant, errHardcoded error) {
	if (errConstant == nil) != (errHardcoded == nil) {
		t.Error("Error behavior should be identical for constant vs hardcoded string")
	}
}

// validateUsageConsistency checks that usage values are identical between constant and hardcoded paths.
func validateUsageConsistency(t *testing.T, usageConstant, usageHardcoded *DiskUsageInfo) {
	if usageConstant == nil || usageHardcoded == nil {
		return // Handled by error validation
	}

	if usageConstant.UsedSpaceBytes != usageHardcoded.UsedSpaceBytes {
		t.Errorf("Usage bytes should be identical: constant=%d, hardcoded=%d",
			usageConstant.UsedSpaceBytes, usageHardcoded.UsedSpaceBytes)
	}

	if usageConstant.UsagePercentage != usageHardcoded.UsagePercentage {
		t.Errorf("Usage percentage should be identical: constant=%f, hardcoded=%f",
			usageConstant.UsagePercentage, usageHardcoded.UsagePercentage)
	}

	if usageConstant.RepositoryCount != usageHardcoded.RepositoryCount {
		t.Errorf("Repository count should be identical: constant=%d, hardcoded=%d",
			usageConstant.RepositoryCount, usageHardcoded.RepositoryCount)
	}
}

// validateConstantUsageInResponse checks that the response uses the constant value.
func validateConstantUsageInResponse(t *testing.T, usage *DiskUsageInfo) {
	if usage != nil && usage.Path != DefaultCacheDirectory {
		t.Errorf("Response path should use constant: expected=%s, got=%s",
			DefaultCacheDirectory, usage.Path)
	}
}

// validatePathComparison checks if the path comparison result matches expectations.
func validatePathComparison(t *testing.T, inputPath string, expectedComparison bool) {
	isPathMatch := (inputPath == DefaultCacheDirectory)
	if isPathMatch != expectedComparison {
		t.Errorf("Expected path comparison %v, got %v for path %s vs constant %s",
			expectedComparison, isPathMatch, inputPath, DefaultCacheDirectory)
	}
}

// validateConsistentCacheUsage validates that cache directory usage is consistent.
func validateConsistentCacheUsage(t *testing.T, usage *DiskUsageInfo, err error) {
	if err != nil {
		t.Errorf("Expected no error for cache directory path, got: %v", err)
		return
	}

	if usage == nil {
		t.Error("Expected usage info for cache directory path")
		return
	}

	expectedBytes := int64(25 * 1024 * 1024 * 1024)
	if usage.UsedSpaceBytes != expectedBytes {
		t.Errorf("Expected consistent cache directory usage, got %d bytes", usage.UsedSpaceBytes)
	}
}

// validateNonCacheUsage validates that non-cache paths don't have cache-specific values.
func validateNonCacheUsage(t *testing.T, usage *DiskUsageInfo) {
	cacheSpecificBytes := int64(25 * 1024 * 1024 * 1024)
	if usage != nil && usage.UsedSpaceBytes == cacheSpecificBytes {
		t.Error("Non-cache directory should not have cache-specific usage values")
	}
}

// Helper function to check if slice contains string.
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
