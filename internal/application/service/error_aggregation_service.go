package service

import (
	"codechunking/internal/application/common/logging"
	"codechunking/internal/domain/entity"
	"context"
)

// ErrorAggregationService handles error aggregation concerns.
type ErrorAggregationService struct {
	logger  logging.ApplicationLogger
	metrics MetricsRecorder
}

// NewErrorAggregationServiceWithMetrics creates a new ErrorAggregationService with metrics.
func NewErrorAggregationServiceWithMetrics(
	logger logging.ApplicationLogger,
	metrics MetricsRecorder,
) *ErrorAggregationService {
	return &ErrorAggregationService{
		logger:  logger,
		metrics: metrics,
	}
}

// RecordAggregationMetrics records metrics for error aggregations.
func (s *ErrorAggregationService) RecordAggregationMetrics(
	ctx context.Context,
	aggregation *entity.ErrorAggregation,
) error {
	if s.metrics != nil {
		s.metrics.RecordAggregationMetrics(ctx, aggregation)
	}
	return nil
}

// RecordErrorRate records error rate metrics.
func (s *ErrorAggregationService) RecordErrorRate(ctx context.Context, pattern string, rate float64) error {
	if s.metrics != nil {
		s.metrics.RecordErrorRate(ctx, pattern, rate)
	}
	return nil
}
