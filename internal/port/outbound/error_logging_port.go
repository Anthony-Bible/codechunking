package outbound

import (
	"codechunking/internal/domain/entity"
	"codechunking/internal/domain/valueobject"
	"context"
)

// ErrorClassifier interface for classifying errors.
type ErrorClassifier interface {
	// ClassifyError converts a standard Go error into a classified error with context
	ClassifyError(ctx context.Context, err error, component string) (*entity.ClassifiedError, error)

	// GetErrorPattern returns the pattern for an error code and severity combination
	GetErrorPattern(errorCode string, severity *valueobject.ErrorSeverity) string

	// IsRetriableError determines if an error is retriable
	IsRetriableError(err error) bool
}

// ErrorAggregator interface for aggregating related errors.
type ErrorAggregator interface {
	// AggregateError adds an error to an existing aggregation or creates a new one
	AggregateError(ctx context.Context, classifiedError *entity.ClassifiedError) (*entity.ErrorAggregation, error)

	// GetActiveAggregations returns all active error aggregations
	GetActiveAggregations(ctx context.Context) ([]*entity.ErrorAggregation, error)

	// GetAggregationByPattern finds an aggregation by its pattern
	GetAggregationByPattern(ctx context.Context, pattern string) (*entity.ErrorAggregation, error)

	// DetectCascadeFailures determines if a set of aggregations represents a cascade failure
	DetectCascadeFailures(ctx context.Context, aggregations []*entity.ErrorAggregation) (bool, error)

	// GroupSimilarAggregations groups similar aggregations based on a similarity threshold
	GroupSimilarAggregations(
		ctx context.Context,
		aggregations []*entity.ErrorAggregation,
		threshold float64,
	) ([][]*entity.ErrorAggregation, error)
}

// AlertingService interface for sending alerts.
type AlertingService interface {
	// SendRealTimeAlert sends an immediate alert
	SendRealTimeAlert(ctx context.Context, alert *entity.Alert) error

	// SendBatchAlert sends a batch alert
	SendBatchAlert(ctx context.Context, alert *entity.Alert) error

	// SendCascadeAlert sends a cascade failure alert
	SendCascadeAlert(ctx context.Context, alert *entity.Alert) error

	// CheckDeduplication checks if an alert should be deduplicated
	CheckDeduplication(ctx context.Context, alert *entity.Alert) (bool, error)

	// RetryFailedAlert retries a failed alert delivery
	RetryFailedAlert(ctx context.Context, alertID string) error

	// GetAlertStatus returns the current status of an alert
	GetAlertStatus(ctx context.Context, alertID string) (string, error)

	// EscalateAlert escalates an alert to the next level
	EscalateAlert(ctx context.Context, alertID string) error
}
