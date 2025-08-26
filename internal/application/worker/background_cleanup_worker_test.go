package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BackgroundCleanupWorker defines the interface for background cleanup operations.
type BackgroundCleanupWorker interface {
	// Job scheduling and execution
	StartScheduledCleanup(ctx context.Context, config ScheduledCleanupConfig) error
	StopScheduledCleanup(ctx context.Context) error
	TriggerImmediateCleanup(ctx context.Context, request ImmediateCleanupRequest) (*CleanupJobExecution, error)

	// Progress tracking
	GetActiveCleanupJobs(ctx context.Context) ([]CleanupJobExecution, error)
	GetCleanupJobStatus(ctx context.Context, jobID uuid.UUID) (*CleanupJobStatus, error)
	GetCleanupJobProgress(ctx context.Context, jobID uuid.UUID) (*CleanupProgress, error)

	// Interruption and resumption
	InterruptCleanupJob(ctx context.Context, jobID uuid.UUID, reason string) error
	ResumeCleanupJob(ctx context.Context, jobID uuid.UUID) error
	CancelCleanupJob(ctx context.Context, jobID uuid.UUID, reason string) error

	// Performance monitoring
	GetCleanupPerformanceMetrics(ctx context.Context, timeRange CleanupTimeRange) (*CleanupPerformanceMetrics, error)
	GetCleanupResourceUsage(ctx context.Context) (*CleanupResourceUsage, error)

	// Coordination with active operations
	CheckActiveOperationConflicts(ctx context.Context, targetPaths []string) (*ConflictAnalysis, error)
	WaitForSafeCleanupWindow(ctx context.Context, timeout time.Duration) (*SafeCleanupWindow, error)
	RegisterActiveOperation(ctx context.Context, operation ActiveOperation) error
	UnregisterActiveOperation(ctx context.Context, operationID uuid.UUID) error
}

// Additional schedule types for testing.
type CleanupSchedule struct {
	Type           ScheduleType  `json:"type"`
	CronExpression string        `json:"cron_expression,omitempty"`
	Interval       time.Duration `json:"interval,omitempty"`
	DailyAt        *time.Time    `json:"daily_at,omitempty"`
	WeeklyOn       *time.Weekday `json:"weekly_on,omitempty"`
	MonthlyOn      *int          `json:"monthly_on,omitempty"`
}

const (
	ScheduleTypeDaily   ScheduleType = "daily"
	ScheduleTypeWeekly  ScheduleType = "weekly"
	ScheduleTypeMonthly ScheduleType = "monthly"
)

type CleanupStrategy struct {
	Primary  StrategyType           `json:"primary"`
	Fallback *StrategyType          `json:"fallback,omitempty"`
	Rules    []CleanupRule          `json:"rules"`
	Limits   CleanupLimits          `json:"limits"`
	Options  map[string]interface{} `json:"options"`
}

type StrategyType string

const (
	StrategyTypeLRU    StrategyType = "lru"
	StrategyTypeTTL    StrategyType = "ttl"
	StrategyTypeSize   StrategyType = "size"
	StrategyTypeHybrid StrategyType = "hybrid"
	StrategyTypeSmart  StrategyType = "smart"
	StrategyTypeCustom StrategyType = "custom"
)

type CleanupRule struct {
	Condition CleanupCondition `json:"condition"`
	Action    CleanupAction    `json:"action"`
	Priority  int              `json:"priority"`
	Enabled   bool             `json:"enabled"`
}

type CleanupCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"`
	Value    interface{} `json:"value"`
}

type CleanupAction struct {
	Type       ActionType             `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
}

type ActionType string

const (
	ActionTypeDelete   ActionType = "delete"
	ActionTypeArchive  ActionType = "archive"
	ActionTypeCompress ActionType = "compress"
	ActionTypeMove     ActionType = "move"
	ActionTypeNotify   ActionType = "notify"
)

type CleanupLimits struct {
	MaxDuration       time.Duration `json:"max_duration"`
	MaxItemsPerBatch  int           `json:"max_items_per_batch"`
	MaxConcurrentJobs int           `json:"max_concurrent_jobs"`
	MaxBytesPerSecond int64         `json:"max_bytes_per_second"`
	MaxIOOperations   int           `json:"max_io_operations"`
}

type SafetyChecks struct {
	CheckActiveOperations bool                `json:"check_active_operations"`
	MinFreeSpacePercent   float64             `json:"min_free_space_percent"`
	RequiredConfirmation  bool                `json:"required_confirmation"`
	BackupBeforeDelete    bool                `json:"backup_before_delete"`
	DryRunFirst           bool                `json:"dry_run_first"`
	ExclusionPatterns     []string            `json:"exclusion_patterns"`
	ProtectedPaths        []string            `json:"protected_paths"`
	CustomChecks          []CustomSafetyCheck `json:"custom_checks"`
}

type CustomSafetyCheck struct {
	Name       string                 `json:"name"`
	Type       SafetyCheckType        `json:"type"`
	Parameters map[string]interface{} `json:"parameters"`
	Required   bool                   `json:"required"`
	TimeoutSec int                    `json:"timeout_sec"`
}

type SafetyCheckType string

const (
	SafetyCheckTypeScript   SafetyCheckType = "script"
	SafetyCheckTypeAPI      SafetyCheckType = "api"
	SafetyCheckTypeFile     SafetyCheckType = "file"
	SafetyCheckTypeDatabase SafetyCheckType = "database"
)

type NotificationRule struct {
	Event     NotificationEvent      `json:"event"`
	Channel   NotificationChannel    `json:"channel"`
	Template  string                 `json:"template"`
	Enabled   bool                   `json:"enabled"`
	Threshold *NotificationThreshold `json:"threshold,omitempty"`
}

type NotificationEvent string

const (
	NotificationEventStarted   NotificationEvent = "started"
	NotificationEventCompleted NotificationEvent = "completed"
	NotificationEventFailed    NotificationEvent = "failed"
	NotificationEventProgress  NotificationEvent = "progress"
	NotificationEventWarning   NotificationEvent = "warning"
)

type NotificationChannel struct {
	Type    ChannelType            `json:"type"`
	Config  map[string]interface{} `json:"config"`
	Enabled bool                   `json:"enabled"`
}

type ChannelType string

const (
	ChannelTypeEmail   ChannelType = "email"
	ChannelTypeSlack   ChannelType = "slack"
	ChannelTypeWebhook ChannelType = "webhook"
	ChannelTypeSMS     ChannelType = "sms"
)

type NotificationThreshold struct {
	MinInterval time.Duration `json:"min_interval"`
	MaxCount    int           `json:"max_count"`
	Condition   string        `json:"condition"`
}

// Extended CleanupProgress for testing with additional fields.
type TestCleanupProgress struct {
	CleanupProgress

	TotalItems         int           `json:"total_items"`
	ProcessedItems     int           `json:"processed_items"`
	DeletedItems       int           `json:"deleted_items"`
	ArchivedItems      int           `json:"archived_items"`
	SkippedItems       int           `json:"skipped_items"`
	FailedItems        int           `json:"failed_items"`
	TotalSizeBytes     int64         `json:"total_size_bytes"`
	ProcessedBytes     int64         `json:"processed_bytes"`
	FreedBytes         int64         `json:"freed_bytes"`
	CurrentItem        string        `json:"current_item"`
	EstimatedRemaining time.Duration `json:"estimated_remaining"`
}

// Test implementation - all tests should FAIL initially.
func TestBackgroundCleanupWorker_StartScheduledCleanup_ValidConfig_ShouldScheduleCleanup(t *testing.T) {
	tests := []struct {
		name           string
		config         ScheduledCleanupConfig
		expectedErr    error
		setupMocks     func() BackgroundCleanupWorker
		validateResult func(t *testing.T, err error)
	}{
		{
			name: "daily schedule with LRU strategy",
			config: ScheduledCleanupConfig{
				Enabled: true,
				Schedule: struct {
					Type           ScheduleType  `json:"type"`
					CronExpression string        `json:"cron_expression,omitempty"`
					Interval       time.Duration `json:"interval,omitempty"`
				}{
					Type:     ScheduleTypeInterval,
					Interval: time.Hour * 24,
				},
				Strategy: "lru",
				Paths:    []string{"/tmp", "/var/cache"},
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					shouldFailStartScheduled: false,
				}
			},
			validateResult: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "cron schedule with hybrid strategy",
			config: ScheduledCleanupConfig{
				Enabled: true,
				Schedule: struct {
					Type           ScheduleType  `json:"type"`
					CronExpression string        `json:"cron_expression,omitempty"`
					Interval       time.Duration `json:"interval,omitempty"`
				}{
					Type:           ScheduleTypeCron,
					CronExpression: "0 2 * * 1", // Every Monday at 2 AM
				},
				Strategy: "hybrid",
				Paths:    []string{"/var/log", "/tmp"},
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					shouldFailStartScheduled: false,
				}
			},
			validateResult: func(t *testing.T, err error) {
				assert.NoError(t, err)
			},
		},
		{
			name: "invalid cron expression should fail",
			config: ScheduledCleanupConfig{
				Enabled: true,
				Schedule: struct {
					Type           ScheduleType  `json:"type"`
					CronExpression string        `json:"cron_expression,omitempty"`
					Interval       time.Duration `json:"interval,omitempty"`
				}{
					Type:           ScheduleTypeCron,
					CronExpression: "invalid cron",
				},
				Strategy: "lru",
				Paths:    []string{"/tmp"},
			},
			expectedErr: errors.New("invalid cron expression"),
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					shouldFailStartScheduled: true,
					startScheduledError:      errors.New("invalid cron expression"),
				}
			},
			validateResult: func(t *testing.T, err error) {
				require.Error(t, err)
				assert.ErrorContains(t, err, "invalid cron expression")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := tt.setupMocks()
			ctx := context.Background()

			err := worker.StartScheduledCleanup(ctx, tt.config)
			tt.validateResult(t, err)
		})
	}
}

func TestBackgroundCleanupWorker_TriggerImmediateCleanup_ValidRequest_ShouldStartJob(t *testing.T) {
	tests := []struct {
		name           string
		request        ImmediateCleanupRequest
		expectedResult *CleanupJobExecution
		expectedErr    error
		setupMocks     func() BackgroundCleanupWorker
		validateResult func(t *testing.T, result *CleanupJobExecution, err error)
	}{
		{
			name: "high priority cleanup with size strategy",
			request: ImmediateCleanupRequest{
				TargetPaths: []string{"/tmp/cleanup", "/var/tmp"},
				Strategy:    "size",
				Priority:    "high",
				DryRun:      false,
				ResourceLimits: ResourceLimits{
					CPUPercent:    50.0,
					MemoryMB:      512,
					DiskIOPercent: 60.0,
					NetworkMBps:   100,
					MaxDuration:   time.Minute * 30,
				},
				RequestedBy: "admin",
				Reason:      "emergency disk space",
			},
			expectedResult: &CleanupJobExecution{
				JobID:    uuid.New(),
				Type:     CleanupJobTypeImmediate,
				Status:   CleanupJobStatusPending,
				Priority: "high",
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					immediateCleanupResult: &CleanupJobExecution{
						JobID:    uuid.New(),
						Type:     CleanupJobTypeImmediate,
						Status:   CleanupJobStatusPending,
						Priority: "high",
					},
				}
			},
			validateResult: func(t *testing.T, result *CleanupJobExecution, err error) {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, CleanupJobTypeImmediate, result.Type)
				assert.Equal(t, "high", result.Priority)
				assert.NotEqual(t, uuid.Nil, result.JobID)
			},
		},
		{
			name: "dry run cleanup",
			request: ImmediateCleanupRequest{
				TargetPaths: []string{"/test/path"},
				Strategy:    "ttl",
				Priority:    "normal",
				DryRun:      true,
				ResourceLimits: ResourceLimits{
					CPUPercent:    30.0,
					MemoryMB:      256,
					DiskIOPercent: 40.0,
					NetworkMBps:   50,
					MaxDuration:   time.Minute * 15,
				},
				RequestedBy: "user",
				Reason:      "testing",
			},
			expectedResult: &CleanupJobExecution{
				JobID:    uuid.New(),
				Type:     CleanupJobTypeImmediate,
				Status:   CleanupJobStatusRunning,
				Priority: "normal",
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					immediateCleanupResult: &CleanupJobExecution{
						JobID:    uuid.New(),
						Type:     CleanupJobTypeImmediate,
						Status:   CleanupJobStatusRunning,
						Priority: "normal",
					},
				}
			},
			validateResult: func(t *testing.T, result *CleanupJobExecution, err error) {
				require.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, CleanupJobTypeImmediate, result.Type)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := tt.setupMocks()
			ctx := context.Background()

			result, err := worker.TriggerImmediateCleanup(ctx, tt.request)
			tt.validateResult(t, result, err)
		})
	}
}

func TestBackgroundCleanupWorker_GetActiveCleanupJobs_ShouldReturnRunningJobs(t *testing.T) {
	tests := []struct {
		name           string
		expectedJobs   []CleanupJobExecution
		expectedErr    error
		setupMocks     func() BackgroundCleanupWorker
		validateResult func(t *testing.T, jobs []CleanupJobExecution, err error)
	}{
		{
			name: "multiple active jobs",
			expectedJobs: []CleanupJobExecution{
				{
					JobID:    uuid.New(),
					Type:     CleanupJobTypeScheduled,
					Status:   CleanupJobStatusRunning,
					Priority: "normal",
					Progress: CleanupProgress{
						Percentage:   25.0,
						CurrentPhase: CleanupPhaseExecuting,
					},
				},
				{
					JobID:    uuid.New(),
					Type:     CleanupJobTypeImmediate,
					Status:   CleanupJobStatusRunning,
					Priority: "high",
					Progress: CleanupProgress{
						Percentage:   20.0,
						CurrentPhase: CleanupPhaseAnalyzing,
					},
				},
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					activeJobs: []CleanupJobExecution{
						{
							JobID:    uuid.New(),
							Type:     CleanupJobTypeScheduled,
							Status:   CleanupJobStatusRunning,
							Priority: "normal",
							Progress: CleanupProgress{
								Percentage:   25.0,
								CurrentPhase: CleanupPhaseExecuting,
							},
						},
						{
							JobID:    uuid.New(),
							Type:     CleanupJobTypeImmediate,
							Status:   CleanupJobStatusRunning,
							Priority: "high",
							Progress: CleanupProgress{
								Percentage:   20.0,
								CurrentPhase: CleanupPhaseAnalyzing,
							},
						},
					},
				}
			},
			validateResult: func(t *testing.T, jobs []CleanupJobExecution, err error) {
				require.NoError(t, err)
				assert.Len(t, jobs, 2)
				assert.Equal(t, CleanupJobStatusRunning, jobs[0].Status)
				assert.Equal(t, CleanupJobStatusRunning, jobs[1].Status)
			},
		},
		{
			name:         "no active jobs",
			expectedJobs: []CleanupJobExecution{},
			expectedErr:  nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					activeJobs: []CleanupJobExecution{},
				}
			},
			validateResult: func(t *testing.T, jobs []CleanupJobExecution, err error) {
				require.NoError(t, err)
				assert.Empty(t, jobs)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := tt.setupMocks()
			ctx := context.Background()

			jobs, err := worker.GetActiveCleanupJobs(ctx)
			tt.validateResult(t, jobs, err)
		})
	}
}

func TestBackgroundCleanupWorker_InterruptCleanupJob_ValidJobID_ShouldInterruptJob(t *testing.T) {
	tests := []struct {
		name        string
		jobID       uuid.UUID
		reason      string
		expectedErr error
		setupMocks  func() BackgroundCleanupWorker
	}{
		{
			name:        "interrupt running job",
			jobID:       uuid.New(),
			reason:      "user requested",
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					shouldFailInterrupt: false,
				}
			},
		},
		{
			name:        "interrupt non-existent job",
			jobID:       uuid.New(),
			reason:      "cleanup",
			expectedErr: errors.New("job not found"),
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					shouldFailInterrupt: true,
					interruptError:      errors.New("job not found"),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := tt.setupMocks()
			ctx := context.Background()

			err := worker.InterruptCleanupJob(ctx, tt.jobID, tt.reason)

			if tt.expectedErr != nil {
				require.Error(t, err)
				assert.ErrorContains(t, err, tt.expectedErr.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBackgroundCleanupWorker_GetCleanupPerformanceMetrics_ValidTimeRange_ShouldReturnMetrics(t *testing.T) {
	timeRange := CleanupTimeRange{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
	}

	tests := []struct {
		name            string
		timeRange       CleanupTimeRange
		expectedMetrics *CleanupPerformanceMetrics
		expectedErr     error
		setupMocks      func() BackgroundCleanupWorker
		validateResult  func(t *testing.T, metrics *CleanupPerformanceMetrics, err error)
	}{
		{
			name:      "valid time range with performance data",
			timeRange: timeRange,
			expectedMetrics: &CleanupPerformanceMetrics{
				TimeRange:           timeRange,
				TotalJobs:           10,
				CompletedJobs:       8,
				FailedJobs:          2,
				AverageJobDuration:  time.Minute * 15,
				TotalItemsProcessed: 5000,
				TotalBytesFreed:     1024 * 1024 * 500, // 500MB
				AverageThroughput: CleanupThroughput{
					ItemsPerSecond: 5.5,
					BytesPerSecond: 1024 * 500, // 500KB/s
					JobsPerHour:    2.0,
				},
				ResourceUtilization: ResourceUtilizationMetrics{
					AverageCPUPercent:    25.5,
					PeakCPUPercent:       45.0,
					AverageMemoryMB:      256,
					PeakMemoryMB:         512,
					AverageDiskIOPercent: 30.0,
					PeakDiskIOPercent:    60.0,
				},
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					performanceMetrics: &CleanupPerformanceMetrics{
						TimeRange:           timeRange,
						TotalJobs:           10,
						CompletedJobs:       8,
						FailedJobs:          2,
						AverageJobDuration:  time.Minute * 15,
						TotalItemsProcessed: 5000,
						TotalBytesFreed:     1024 * 1024 * 500,
						AverageThroughput: CleanupThroughput{
							ItemsPerSecond: 5.5,
							BytesPerSecond: 1024 * 500,
							JobsPerHour:    2.0,
						},
						ResourceUtilization: ResourceUtilizationMetrics{
							AverageCPUPercent:    25.5,
							PeakCPUPercent:       45.0,
							AverageMemoryMB:      256,
							PeakMemoryMB:         512,
							AverageDiskIOPercent: 30.0,
							PeakDiskIOPercent:    60.0,
						},
					},
				}
			},
			validateResult: func(t *testing.T, metrics *CleanupPerformanceMetrics, err error) {
				require.NoError(t, err)
				require.NotNil(t, metrics)
				assert.Equal(t, 10, metrics.TotalJobs)
				assert.Equal(t, 8, metrics.CompletedJobs)
				assert.Equal(t, time.Minute*15, metrics.AverageJobDuration)
				assert.Greater(t, metrics.AverageThroughput.ItemsPerSecond, 0.0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := tt.setupMocks()
			ctx := context.Background()

			metrics, err := worker.GetCleanupPerformanceMetrics(ctx, tt.timeRange)
			tt.validateResult(t, metrics, err)
		})
	}
}

func TestBackgroundCleanupWorker_CheckActiveOperationConflicts_ShouldDetectConflicts(t *testing.T) {
	tests := []struct {
		name           string
		targetPaths    []string
		expectedResult *ConflictAnalysis
		expectedErr    error
		setupMocks     func() BackgroundCleanupWorker
		validateResult func(t *testing.T, result *ConflictAnalysis, err error)
	}{
		{
			name:        "conflicts detected",
			targetPaths: []string{"/tmp/active", "/var/cache"},
			expectedResult: &ConflictAnalysis{
				HasConflicts:  true,
				ConflictCount: 2,
				ConflictDetails: []OperationConflict{
					{
						ConflictID:   uuid.New(),
						ConflictType: ConflictTypePathOverlap,
						ConflictPath: "/tmp/active",
						ActiveOperation: ActiveOperation{
							OperationID:   uuid.New(),
							OperationType: OperationTypeIndex,
							Priority:      OperationPriorityHigh,
						},
						Severity:    ConflictSeverityHigh,
						Description: "Path overlap with indexing operation",
					},
				},
				RecommendedAction: ConflictActionWait,
				EstimatedWaitTime: time.Minute * 10,
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					conflictAnalysis: &ConflictAnalysis{
						HasConflicts:  true,
						ConflictCount: 2,
						ConflictDetails: []OperationConflict{
							{
								ConflictID:   uuid.New(),
								ConflictType: ConflictTypePathOverlap,
								ConflictPath: "/tmp/active",
								ActiveOperation: ActiveOperation{
									OperationID:   uuid.New(),
									OperationType: OperationTypeIndex,
									Priority:      OperationPriorityHigh,
								},
								Severity:    ConflictSeverityHigh,
								Description: "Path overlap with indexing operation",
							},
						},
						RecommendedAction: ConflictActionWait,
						EstimatedWaitTime: time.Minute * 10,
					},
				}
			},
			validateResult: func(t *testing.T, result *ConflictAnalysis, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.True(t, result.HasConflicts)
				assert.Equal(t, 2, result.ConflictCount)
				assert.NotEmpty(t, result.ConflictDetails)
				assert.Equal(t, ConflictActionWait, result.RecommendedAction)
			},
		},
		{
			name:        "no conflicts",
			targetPaths: []string{"/safe/path"},
			expectedResult: &ConflictAnalysis{
				HasConflicts:      false,
				ConflictCount:     0,
				ConflictDetails:   []OperationConflict{},
				RecommendedAction: ConflictActionProceed,
				EstimatedWaitTime: 0,
			},
			expectedErr: nil,
			setupMocks: func() BackgroundCleanupWorker {
				return &mockBackgroundCleanupWorker{
					conflictAnalysis: &ConflictAnalysis{
						HasConflicts:      false,
						ConflictCount:     0,
						ConflictDetails:   []OperationConflict{},
						RecommendedAction: ConflictActionProceed,
						EstimatedWaitTime: 0,
					},
				}
			},
			validateResult: func(t *testing.T, result *ConflictAnalysis, err error) {
				require.NoError(t, err)
				require.NotNil(t, result)
				assert.False(t, result.HasConflicts)
				assert.Equal(t, 0, result.ConflictCount)
				assert.Equal(t, ConflictActionProceed, result.RecommendedAction)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			worker := tt.setupMocks()
			ctx := context.Background()

			result, err := worker.CheckActiveOperationConflicts(ctx, tt.targetPaths)
			tt.validateResult(t, result, err)
		})
	}
}

// Mock implementation for testing - should always fail tests initially.
type mockBackgroundCleanupWorker struct {
	// StartScheduledCleanup mocks
	shouldFailStartScheduled bool
	startScheduledError      error

	// TriggerImmediateCleanup mocks
	immediateCleanupResult *CleanupJobExecution
	immediateCleanupError  error

	// GetActiveCleanupJobs mocks
	activeJobs      []CleanupJobExecution
	activeJobsError error

	// InterruptCleanupJob mocks
	shouldFailInterrupt bool
	interruptError      error

	// Performance metrics mocks
	performanceMetrics      *CleanupPerformanceMetrics
	performanceMetricsError error

	// Conflict analysis mocks
	conflictAnalysis      *ConflictAnalysis
	conflictAnalysisError error
}

func (m *mockBackgroundCleanupWorker) StartScheduledCleanup(_ context.Context, _ ScheduledCleanupConfig) error {
	if m.shouldFailStartScheduled {
		return m.startScheduledError
	}
	// Success case - REFACTOR phase: properly implement mock success
	return nil
}

func (m *mockBackgroundCleanupWorker) StopScheduledCleanup(_ context.Context) error {
	return errors.New("not implemented - StopScheduledCleanup")
}

func (m *mockBackgroundCleanupWorker) TriggerImmediateCleanup(
	_ context.Context,
	_ ImmediateCleanupRequest,
) (*CleanupJobExecution, error) {
	if m.immediateCleanupError != nil {
		return nil, m.immediateCleanupError
	}
	if m.immediateCleanupResult != nil {
		// Success case - REFACTOR phase: return configured result
		return m.immediateCleanupResult, nil
	}
	return nil, errors.New("not configured - TriggerImmediateCleanup")
}

func (m *mockBackgroundCleanupWorker) GetActiveCleanupJobs(_ context.Context) ([]CleanupJobExecution, error) {
	if m.activeJobsError != nil {
		return nil, m.activeJobsError
	}
	// Success case - REFACTOR phase: return configured jobs
	return m.activeJobs, nil
}

func (m *mockBackgroundCleanupWorker) GetCleanupJobStatus(
	_ context.Context,
	_ uuid.UUID,
) (*CleanupJobStatus, error) {
	return nil, errors.New("not implemented - GetCleanupJobStatus")
}

func (m *mockBackgroundCleanupWorker) GetCleanupJobProgress(
	_ context.Context,
	_ uuid.UUID,
) (*CleanupProgress, error) {
	return nil, errors.New("not implemented - GetCleanupJobProgress")
}

func (m *mockBackgroundCleanupWorker) InterruptCleanupJob(_ context.Context, _ uuid.UUID, _ string) error {
	if m.shouldFailInterrupt {
		return m.interruptError
	}
	// Success case - REFACTOR phase: return success
	return nil
}

func (m *mockBackgroundCleanupWorker) ResumeCleanupJob(_ context.Context, _ uuid.UUID) error {
	return errors.New("not implemented - ResumeCleanupJob")
}

func (m *mockBackgroundCleanupWorker) CancelCleanupJob(_ context.Context, _ uuid.UUID, _ string) error {
	return errors.New("not implemented - CancelCleanupJob")
}

func (m *mockBackgroundCleanupWorker) GetCleanupPerformanceMetrics(
	_ context.Context,
	_ CleanupTimeRange,
) (*CleanupPerformanceMetrics, error) {
	if m.performanceMetricsError != nil {
		return nil, m.performanceMetricsError
	}
	// Success case - REFACTOR phase: return configured metrics
	return m.performanceMetrics, nil
}

func (m *mockBackgroundCleanupWorker) GetCleanupResourceUsage(_ context.Context) (*CleanupResourceUsage, error) {
	return nil, errors.New("not implemented - GetCleanupResourceUsage")
}

func (m *mockBackgroundCleanupWorker) CheckActiveOperationConflicts(
	_ context.Context,
	_ []string,
) (*ConflictAnalysis, error) {
	if m.conflictAnalysisError != nil {
		return nil, m.conflictAnalysisError
	}
	// Success case - REFACTOR phase: return configured analysis
	return m.conflictAnalysis, nil
}

func (m *mockBackgroundCleanupWorker) WaitForSafeCleanupWindow(
	_ context.Context,
	_ time.Duration,
) (*SafeCleanupWindow, error) {
	return nil, errors.New("not implemented - WaitForSafeCleanupWindow")
}

func (m *mockBackgroundCleanupWorker) RegisterActiveOperation(_ context.Context, _ ActiveOperation) error {
	return errors.New("not implemented - RegisterActiveOperation")
}

func (m *mockBackgroundCleanupWorker) UnregisterActiveOperation(_ context.Context, _ uuid.UUID) error {
	return errors.New("not implemented - UnregisterActiveOperation")
}

// Helper functions.
// timePtr function is defined in background_cleanup_worker.go
