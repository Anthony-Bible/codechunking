package outbound

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockErrorClassifier is a mock implementation for testing.
type MockErrorClassifier struct {
	mock.Mock
}

func (m *MockErrorClassifier) ClassifyError(
	ctx context.Context,
	err error,
	component string,
) (*entity.ClassifiedError, error) {
	args := m.Called(ctx, err, component)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ClassifiedError), args.Error(1)
}

func (m *MockErrorClassifier) GetErrorPattern(errorCode string, severity *valueobject.ErrorSeverity) string {
	args := m.Called(errorCode, severity)
	return args.String(0)
}

func (m *MockErrorClassifier) IsRetriableError(err error) bool {
	args := m.Called(err)
	return args.Bool(0)
}

// Test ErrorClassifier interface requirements.
func TestErrorClassifier_Interface(t *testing.T) {
	t.Run("should classify standard Go error to classified error", func(t *testing.T) {
		mockClassifier := new(MockErrorClassifier)
		ctx := context.Background()
		originalError := assert.AnError
		component := "database-service"

		// Set up expectation
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		expectedClassifiedError, _ := entity.NewClassifiedError(
			ctx,
			originalError,
			severity,
			"database_connection_failure",
			"Failed to connect to database",
			nil,
		)
		mockClassifier.On("ClassifyError", ctx, originalError, component).Return(expectedClassifiedError, nil)

		// Test the interface
		classifiedError, err := mockClassifier.ClassifyError(ctx, originalError, component)

		require.NoError(t, err)
		assert.NotNil(t, classifiedError)
		assert.Equal(t, expectedClassifiedError.ID(), classifiedError.ID())
		assert.Equal(t, "database_connection_failure", classifiedError.ErrorCode())
		mockClassifier.AssertExpectations(t)
	})

	t.Run("should handle classification errors gracefully", func(t *testing.T) {
		mockClassifier := new(MockErrorClassifier)
		ctx := context.Background()
		originalError := assert.AnError
		component := "invalid-component"

		// Set up expectation for error
		mockClassifier.On("ClassifyError", ctx, originalError, component).Return(nil, assert.AnError)

		classifiedError, err := mockClassifier.ClassifyError(ctx, originalError, component)

		require.Error(t, err)
		assert.Nil(t, classifiedError)
		mockClassifier.AssertExpectations(t)
	})

	t.Run("should generate consistent error patterns", func(t *testing.T) {
		mockClassifier := new(MockErrorClassifier)
		errorCode := "database_connection_failure"
		severity, _ := valueobject.NewErrorSeverity("ERROR")
		expectedPattern := "database_connection_failure:ERROR"

		mockClassifier.On("GetErrorPattern", errorCode, severity).Return(expectedPattern)

		pattern := mockClassifier.GetErrorPattern(errorCode, severity)

		assert.Equal(t, expectedPattern, pattern)
		mockClassifier.AssertExpectations(t)
	})

	t.Run("should identify retriable errors correctly", func(t *testing.T) {
		mockClassifier := new(MockErrorClassifier)

		// Retriable error (network timeout)
		mockClassifier.On("IsRetriableError", mock.MatchedBy(func(err error) bool {
			return err.Error() == "network timeout"
		})).Return(true)

		// Non-retriable error (validation error)
		mockClassifier.On("IsRetriableError", mock.MatchedBy(func(err error) bool {
			return err.Error() == "invalid input"
		})).Return(false)

		retriableErr := &testError{msg: "network timeout"}
		nonRetriableErr := &testError{msg: "invalid input"}

		assert.True(t, mockClassifier.IsRetriableError(retriableErr))
		assert.False(t, mockClassifier.IsRetriableError(nonRetriableErr))
		mockClassifier.AssertExpectations(t)
	})
}

// MockErrorAggregator is a mock implementation for testing.
type MockErrorAggregator struct {
	mock.Mock
}

func (m *MockErrorAggregator) AggregateError(
	ctx context.Context,
	classifiedError *entity.ClassifiedError,
) (*entity.ErrorAggregation, error) {
	args := m.Called(ctx, classifiedError)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ErrorAggregation), args.Error(1)
}

func (m *MockErrorAggregator) GetActiveAggregations(ctx context.Context) ([]*entity.ErrorAggregation, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*entity.ErrorAggregation), args.Error(1)
}

func (m *MockErrorAggregator) GetAggregationByPattern(
	ctx context.Context,
	pattern string,
) (*entity.ErrorAggregation, error) {
	args := m.Called(ctx, pattern)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.ErrorAggregation), args.Error(1)
}

func (m *MockErrorAggregator) DetectCascadeFailures(
	ctx context.Context,
	aggregations []*entity.ErrorAggregation,
) (bool, error) {
	args := m.Called(ctx, aggregations)
	return args.Bool(0), args.Error(1)
}

func (m *MockErrorAggregator) GroupSimilarAggregations(
	ctx context.Context,
	aggregations []*entity.ErrorAggregation,
	threshold float64,
) ([][]*entity.ErrorAggregation, error) {
	args := m.Called(ctx, aggregations, threshold)
	return args.Get(0).([][]*entity.ErrorAggregation), args.Error(1)
}

// Test ErrorAggregator interface requirements.
func TestErrorAggregator_Interface(t *testing.T) {
	t.Run("should aggregate classified error into existing or new aggregation", func(t *testing.T) {
		mockAggregator := new(MockErrorAggregator)
		ctx := context.Background()

		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := entity.NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"database_connection_failure",
			"Database connection failed",
			nil,
		)

		expectedAggregation, _ := entity.NewErrorAggregation(
			classifiedError.ErrorPattern(),
			time.Minute*5,
		)
		expectedAggregation.AddError(classifiedError)

		mockAggregator.On("AggregateError", ctx, classifiedError).Return(expectedAggregation, nil)

		aggregation, err := mockAggregator.AggregateError(ctx, classifiedError)

		require.NoError(t, err)
		assert.NotNil(t, aggregation)
		assert.Equal(t, classifiedError.ErrorPattern(), aggregation.Pattern())
		assert.Equal(t, 1, aggregation.Count())
		mockAggregator.AssertExpectations(t)
	})

	t.Run("should retrieve active aggregations", func(t *testing.T) {
		mockAggregator := new(MockErrorAggregator)
		ctx := context.Background()

		// Create sample aggregations
		aggregations := make([]*entity.ErrorAggregation, 2)
		aggregations[0], _ = entity.NewErrorAggregation("error1:ERROR", time.Minute*5)
		aggregations[1], _ = entity.NewErrorAggregation("error2:CRITICAL", time.Minute*5)

		mockAggregator.On("GetActiveAggregations", ctx).Return(aggregations, nil)

		result, err := mockAggregator.GetActiveAggregations(ctx)

		require.NoError(t, err)
		assert.Len(t, result, 2)
		assert.Equal(t, "error1:ERROR", result[0].Pattern())
		assert.Equal(t, "error2:CRITICAL", result[1].Pattern())
		mockAggregator.AssertExpectations(t)
	})

	t.Run("should find aggregation by pattern", func(t *testing.T) {
		mockAggregator := new(MockErrorAggregator)
		ctx := context.Background()
		pattern := "database_connection_failure:ERROR"

		expectedAggregation, _ := entity.NewErrorAggregation(pattern, time.Minute*5)
		mockAggregator.On("GetAggregationByPattern", ctx, pattern).Return(expectedAggregation, nil)

		aggregation, err := mockAggregator.GetAggregationByPattern(ctx, pattern)

		require.NoError(t, err)
		assert.NotNil(t, aggregation)
		assert.Equal(t, pattern, aggregation.Pattern())
		mockAggregator.AssertExpectations(t)
	})

	t.Run("should detect cascade failures", func(t *testing.T) {
		mockAggregator := new(MockErrorAggregator)
		ctx := context.Background()

		// Create aggregations representing cascade failure
		aggregations := make([]*entity.ErrorAggregation, 3)
		aggregations[0], _ = entity.NewErrorAggregation("database_failure:CRITICAL", time.Minute*5)
		aggregations[1], _ = entity.NewErrorAggregation("api_failure:ERROR", time.Minute*5)
		aggregations[2], _ = entity.NewErrorAggregation("worker_failure:ERROR", time.Minute*5)

		mockAggregator.On("DetectCascadeFailures", ctx, aggregations).Return(true, nil)

		isCascade, err := mockAggregator.DetectCascadeFailures(ctx, aggregations)

		require.NoError(t, err)
		assert.True(t, isCascade)
		mockAggregator.AssertExpectations(t)
	})

	t.Run("should group similar aggregations", func(t *testing.T) {
		mockAggregator := new(MockErrorAggregator)
		ctx := context.Background()
		threshold := 0.7

		aggregations := make([]*entity.ErrorAggregation, 4)
		aggregations[0], _ = entity.NewErrorAggregation("database_connection_failure:ERROR", time.Minute*5)
		aggregations[1], _ = entity.NewErrorAggregation("database_timeout_failure:ERROR", time.Minute*5)
		aggregations[2], _ = entity.NewErrorAggregation("database_query_failure:ERROR", time.Minute*5)
		aggregations[3], _ = entity.NewErrorAggregation("git_clone_failure:ERROR", time.Minute*5)

		// Expected groups: [database-related group of 3, git-related group of 1]
		expectedGroups := [][]*entity.ErrorAggregation{
			{aggregations[0], aggregations[1], aggregations[2]}, // Database group
			{aggregations[3]}, // Git group
		}

		mockAggregator.On("GroupSimilarAggregations", ctx, aggregations, threshold).Return(expectedGroups, nil)

		groups, err := mockAggregator.GroupSimilarAggregations(ctx, aggregations, threshold)

		require.NoError(t, err)
		assert.Len(t, groups, 2)
		assert.Len(t, groups[0], 3) // Database group
		assert.Len(t, groups[1], 1) // Git group
		mockAggregator.AssertExpectations(t)
	})
}

// MockAlertingService is a mock implementation for testing.
type MockAlertingService struct {
	mock.Mock
}

func (m *MockAlertingService) SendRealTimeAlert(ctx context.Context, alert *entity.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func (m *MockAlertingService) SendBatchAlert(ctx context.Context, alert *entity.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func (m *MockAlertingService) SendCascadeAlert(ctx context.Context, alert *entity.Alert) error {
	args := m.Called(ctx, alert)
	return args.Error(0)
}

func (m *MockAlertingService) CheckDeduplication(ctx context.Context, alert *entity.Alert) (bool, error) {
	args := m.Called(ctx, alert)
	return args.Bool(0), args.Error(1)
}

func (m *MockAlertingService) RetryFailedAlert(ctx context.Context, alertID string) error {
	args := m.Called(ctx, alertID)
	return args.Error(0)
}

func (m *MockAlertingService) GetAlertStatus(ctx context.Context, alertID string) (string, error) {
	args := m.Called(ctx, alertID)
	return args.String(0), args.Error(1)
}

func (m *MockAlertingService) EscalateAlert(ctx context.Context, alertID string) error {
	args := m.Called(ctx, alertID)
	return args.Error(0)
}

// Test AlertingService interface requirements.
func TestAlertingService_Interface(t *testing.T) {
	t.Run("should send real-time alert successfully", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()

		// Create critical alert
		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := entity.NewClassifiedError(
			ctx,
			assert.AnError,
			severity,
			"system_failure",
			"Critical system failure",
			nil,
		)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewAlert(
			classifiedError,
			alertType,
			"Critical system failure - immediate attention required",
		)

		mockService.On("SendRealTimeAlert", ctx, alert).Return(nil)

		err := mockService.SendRealTimeAlert(ctx, alert)

		require.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("should send batch alert successfully", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()

		// Create batch alert for aggregation
		_, _ = valueobject.NewErrorSeverity("ERROR")
		aggregation, _ := entity.NewErrorAggregation("database_connection_failure:ERROR", time.Minute*10)

		alertType, _ := valueobject.NewAlertType("BATCH")
		alert, _ := entity.NewAggregationAlert(aggregation, alertType, "Multiple database connection failures detected")

		mockService.On("SendBatchAlert", ctx, alert).Return(nil)

		err := mockService.SendBatchAlert(ctx, alert)

		require.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("should send cascade failure alert successfully", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()

		// Create cascade failure alert
		aggregations := make([]*entity.ErrorAggregation, 2)
		aggregations[0], _ = entity.NewErrorAggregation("database_failure:CRITICAL", time.Minute*5)
		aggregations[1], _ = entity.NewErrorAggregation("api_failure:ERROR", time.Minute*5)

		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewCascadeFailureAlert(aggregations, alertType, "Cascade failure detected")

		mockService.On("SendCascadeAlert", ctx, alert).Return(nil)

		err := mockService.SendCascadeAlert(ctx, alert)

		require.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("should check deduplication correctly", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()

		severity, _ := valueobject.NewErrorSeverity("ERROR")
		classifiedError, _ := entity.NewClassifiedError(ctx, assert.AnError, severity, "test_error", "Test", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewAlert(classifiedError, alertType, "Test alert")

		// First call - not duplicate
		mockService.On("CheckDeduplication", ctx, alert).Return(false, nil).Once()
		isDuplicate, err := mockService.CheckDeduplication(ctx, alert)
		require.NoError(t, err)
		assert.False(t, isDuplicate)

		// Second call - is duplicate
		mockService.On("CheckDeduplication", ctx, alert).Return(true, nil).Once()
		isDuplicate, err = mockService.CheckDeduplication(ctx, alert)
		require.NoError(t, err)
		assert.True(t, isDuplicate)

		mockService.AssertExpectations(t)
	})

	t.Run("should retry failed alert", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()
		alertID := "alert-123"

		mockService.On("RetryFailedAlert", ctx, alertID).Return(nil)

		err := mockService.RetryFailedAlert(ctx, alertID)

		require.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("should get alert status", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()
		alertID := "alert-123"
		expectedStatus := "SENT"

		mockService.On("GetAlertStatus", ctx, alertID).Return(expectedStatus, nil)

		status, err := mockService.GetAlertStatus(ctx, alertID)

		require.NoError(t, err)
		assert.Equal(t, expectedStatus, status)
		mockService.AssertExpectations(t)
	})

	t.Run("should escalate alert", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()
		alertID := "critical-alert-456"

		mockService.On("EscalateAlert", ctx, alertID).Return(nil)

		err := mockService.EscalateAlert(ctx, alertID)

		require.NoError(t, err)
		mockService.AssertExpectations(t)
	})

	t.Run("should handle service errors gracefully", func(t *testing.T) {
		mockService := new(MockAlertingService)
		ctx := context.Background()

		severity, _ := valueobject.NewErrorSeverity("CRITICAL")
		classifiedError, _ := entity.NewClassifiedError(ctx, assert.AnError, severity, "system_failure", "Failure", nil)
		alertType, _ := valueobject.NewAlertType("REAL_TIME")
		alert, _ := entity.NewAlert(classifiedError, alertType, "Critical failure")

		mockService.On("SendRealTimeAlert", ctx, alert).Return(assert.AnError)

		err := mockService.SendRealTimeAlert(ctx, alert)

		require.Error(t, err)
		mockService.AssertExpectations(t)
	})
}

// Helper struct for testing.
type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

// Interface definitions that should exist in the actual port layer
// These are defined here for testing purposes in the Red phase
