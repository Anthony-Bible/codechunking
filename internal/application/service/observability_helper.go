package service

import (
	"codechunking/internal/application/common/slogger"
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

// observabilityHelper provides common observability patterns for service methods.
type observabilityHelper struct {
	tracer                trace.Tracer
	detectionCounter      metric.Int64Counter
	detectionDuration     metric.Float64Histogram
	detectionErrorCounter metric.Int64Counter
}

// detectionOperation represents a detection operation with observability.
type detectionOperation struct {
	ctx        context.Context
	span       trace.Span
	method     string
	startTime  time.Time
	helper     *observabilityHelper
	attributes []attribute.KeyValue
}

// startOperation begins a new detection operation with tracing and metrics.
// The caller MUST call either finishWithSuccess or finishWithError to end the span.
func (oh *observabilityHelper) startOperation(
	ctx context.Context,
	operationName, method string,
	attributes ...attribute.KeyValue,
) (*detectionOperation, func()) {
	ctx, span := oh.tracer.Start(ctx, operationName)

	allAttributes := append([]attribute.KeyValue{
		attribute.String("method", method),
	}, attributes...)

	span.SetAttributes(allAttributes...)

	op := &detectionOperation{
		ctx:        ctx,
		span:       span,
		method:     method,
		startTime:  time.Now(),
		helper:     oh,
		attributes: allAttributes,
	}

	// Return cleanup function to satisfy spancheck
	cleanup := func() {
		span.End()
	}

	return op, cleanup
}

// finishWithSuccess completes the operation successfully with metrics and logging.
func (op *detectionOperation) finishWithSuccess(logMessage string, logFields slogger.Fields) {
	defer op.span.End()

	// Record metrics
	duration := time.Since(op.startTime).Seconds()
	op.helper.detectionDuration.Record(op.ctx, duration, metric.WithAttributes(op.attributes...))
	op.helper.detectionCounter.Add(op.ctx, 1, metric.WithAttributes(op.attributes...))

	// Log success
	slogger.Info(op.ctx, logMessage, logFields)
}

// finishWithError completes the operation with error handling, metrics, and logging.
func (op *detectionOperation) finishWithError(err error, logMessage string, logFields slogger.Fields) {
	defer op.span.End()

	// Record error metrics
	duration := time.Since(op.startTime).Seconds()
	op.helper.detectionDuration.Record(op.ctx, duration, metric.WithAttributes(op.attributes...))

	errorAttrs := append([]attribute.KeyValue(nil), op.attributes...)
	errorAttrs = append(errorAttrs, attribute.String("error", err.Error()))
	op.helper.detectionErrorCounter.Add(op.ctx, 1, metric.WithAttributes(errorAttrs...))

	// Add error fields to log
	if logFields == nil {
		logFields = slogger.Fields{}
	}
	logFields["error"] = err.Error()

	// Log error
	slogger.Error(op.ctx, logMessage, logFields)

	// Set span status
	op.span.RecordError(err)
}

// addAttributes adds additional attributes to the span and operation.
func (op *detectionOperation) addAttributes(attrs ...attribute.KeyValue) {
	op.span.SetAttributes(attrs...)
	op.attributes = append(op.attributes, attrs...)
}

// getContext returns the operation context.
func (op *detectionOperation) getContext() context.Context {
	return op.ctx
}
