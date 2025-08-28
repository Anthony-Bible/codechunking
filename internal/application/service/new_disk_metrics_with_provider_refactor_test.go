package service

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/embedded"
	"go.opentelemetry.io/otel/metric/noop"
)

// mockMeterProvider for testing instrument creation failure scenarios.
type mockMeterProvider struct {
	embedded.MeterProvider

	meter *mockMeter
}

func (m *mockMeterProvider) Meter(_ string, _ ...metric.MeterOption) metric.Meter {
	return m.meter
}

// mockMeter allows us to control instrument creation behavior for testing.
type mockMeter struct {
	embedded.Meter

	failOnFloat64Gauge     bool
	failOnInt64Gauge       bool
	failOnFloat64Histogram bool
	failOnInt64Counter     bool
	callLog                []string
}

func (m *mockMeter) Int64Counter(name string, _ ...metric.Int64CounterOption) (metric.Int64Counter, error) {
	m.callLog = append(m.callLog, "Int64Counter:"+name)
	if m.failOnInt64Counter {
		return nil, errors.New("failed to create int64 counter")
	}
	return noop.Int64Counter{}, nil
}

func (m *mockMeter) Int64UpDownCounter(
	_ string,
	_ ...metric.Int64UpDownCounterOption,
) (metric.Int64UpDownCounter, error) {
	return noop.Int64UpDownCounter{}, nil
}

func (m *mockMeter) Int64Histogram(_ string, _ ...metric.Int64HistogramOption) (metric.Int64Histogram, error) {
	return noop.Int64Histogram{}, nil
}

func (m *mockMeter) Int64Gauge(name string, _ ...metric.Int64GaugeOption) (metric.Int64Gauge, error) {
	m.callLog = append(m.callLog, "Int64Gauge:"+name)
	if m.failOnInt64Gauge {
		return nil, errors.New("failed to create int64 gauge")
	}
	return noop.Int64Gauge{}, nil
}

func (m *mockMeter) Int64ObservableCounter(
	_ string,
	_ ...metric.Int64ObservableCounterOption,
) (metric.Int64ObservableCounter, error) {
	return noop.Int64ObservableCounter{}, nil
}

func (m *mockMeter) Int64ObservableUpDownCounter(
	_ string,
	_ ...metric.Int64ObservableUpDownCounterOption,
) (metric.Int64ObservableUpDownCounter, error) {
	return noop.Int64ObservableUpDownCounter{}, nil
}

func (m *mockMeter) Int64ObservableGauge(
	_ string,
	_ ...metric.Int64ObservableGaugeOption,
) (metric.Int64ObservableGauge, error) {
	return noop.Int64ObservableGauge{}, nil
}

func (m *mockMeter) Float64Counter(_ string, _ ...metric.Float64CounterOption) (metric.Float64Counter, error) {
	return noop.Float64Counter{}, nil
}

func (m *mockMeter) Float64UpDownCounter(
	_ string,
	_ ...metric.Float64UpDownCounterOption,
) (metric.Float64UpDownCounter, error) {
	return noop.Float64UpDownCounter{}, nil
}

func (m *mockMeter) Float64Histogram(
	name string,
	_ ...metric.Float64HistogramOption,
) (metric.Float64Histogram, error) {
	m.callLog = append(m.callLog, "Float64Histogram:"+name)
	if m.failOnFloat64Histogram {
		return nil, errors.New("failed to create float64 histogram")
	}
	return noop.Float64Histogram{}, nil
}

func (m *mockMeter) Float64Gauge(name string, _ ...metric.Float64GaugeOption) (metric.Float64Gauge, error) {
	m.callLog = append(m.callLog, "Float64Gauge:"+name)
	if m.failOnFloat64Gauge {
		return nil, errors.New("failed to create float64 gauge")
	}
	return noop.Float64Gauge{}, nil
}

func (m *mockMeter) Float64ObservableCounter(
	_ string,
	_ ...metric.Float64ObservableCounterOption,
) (metric.Float64ObservableCounter, error) {
	return noop.Float64ObservableCounter{}, nil
}

func (m *mockMeter) Float64ObservableUpDownCounter(
	_ string,
	_ ...metric.Float64ObservableUpDownCounterOption,
) (metric.Float64ObservableUpDownCounter, error) {
	return noop.Float64ObservableUpDownCounter{}, nil
}

func (m *mockMeter) Float64ObservableGauge(
	_ string,
	_ ...metric.Float64ObservableGaugeOption,
) (metric.Float64ObservableGauge, error) {
	return noop.Float64ObservableGauge{}, nil
}

func (m *mockMeter) RegisterCallback(
	_ metric.Callback,
	_ ...metric.Observable,
) (metric.Registration, error) {
	return noop.Registration{}, nil
}

// TestNewDiskMetricsWithProvider_EmptyInstanceID tests that empty instance ID returns error.
func TestNewDiskMetricsWithProvider_EmptyInstanceID(t *testing.T) {
	provider := noop.NewMeterProvider()

	metrics, err := NewDiskMetricsWithProvider("", provider)

	require.Error(t, err)
	assert.Equal(t, "instance ID cannot be empty", err.Error())
	assert.Nil(t, metrics)
}

// TestNewDiskMetricsWithProvider_AllInstrumentsCreated tests that all required instruments are created.
func TestNewDiskMetricsWithProvider_AllInstrumentsCreated(t *testing.T) {
	provider := noop.NewMeterProvider()

	metrics, err := NewDiskMetricsWithProvider("test-instance", provider)

	require.NoError(t, err)
	require.NotNil(t, metrics)

	// Verify all struct fields are populated (not nil)
	assert.NotNil(t, metrics.diskUsageGauge, "diskUsageGauge should be populated")
	assert.NotNil(t, metrics.diskHealthStatusGauge, "diskHealthStatusGauge should be populated")
	assert.NotNil(t, metrics.diskOperationDuration, "diskOperationDuration should be populated")
	assert.NotNil(t, metrics.diskHealthCheckDuration, "diskHealthCheckDuration should be populated")
	assert.NotNil(t, metrics.cleanupOperationDuration, "cleanupOperationDuration should be populated")
	assert.NotNil(t, metrics.retentionPolicyDuration, "retentionPolicyDuration should be populated")
	assert.NotNil(t, metrics.diskOperationCounter, "diskOperationCounter should be populated")
	assert.NotNil(t, metrics.diskAlertCounter, "diskAlertCounter should be populated")
	assert.NotNil(t, metrics.cleanupItemsCounter, "cleanupItemsCounter should be populated")
	assert.NotNil(t, metrics.cleanupBytesFreedCounter, "cleanupBytesFreedCounter should be populated")
	assert.NotNil(t, metrics.retentionPolicyCounter, "retentionPolicyCounter should be populated")
}

// TestNewDiskMetricsWithProvider_InstanceIDAndAttrSet tests instanceID and baseInstanceAttr assignment.
func TestNewDiskMetricsWithProvider_InstanceIDAndAttrSet(t *testing.T) {
	instanceID := "test-service-instance-123"
	provider := noop.NewMeterProvider()

	metrics, err := NewDiskMetricsWithProvider(instanceID, provider)

	require.NoError(t, err)
	require.NotNil(t, metrics)

	assert.Equal(t, instanceID, metrics.instanceID)

	expectedAttr := attribute.String(AttrInstanceID, instanceID)
	assert.Equal(t, expectedAttr, metrics.baseInstanceAttr)
}

// TestNewDiskMetricsWithProvider_Float64GaugeCreationFailure tests error when Float64Gauge creation fails.
func TestNewDiskMetricsWithProvider_Float64GaugeCreationFailure(t *testing.T) {
	mockMeter := &mockMeter{failOnFloat64Gauge: true}
	provider := &mockMeterProvider{meter: mockMeter}

	metrics, err := NewDiskMetricsWithProvider("test-instance", provider)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create float64 gauge")
	assert.Nil(t, metrics)
}

// TestNewDiskMetricsWithProvider_Int64GaugeCreationFailure tests error when Int64Gauge creation fails.
func TestNewDiskMetricsWithProvider_Int64GaugeCreationFailure(t *testing.T) {
	mockMeter := &mockMeter{failOnInt64Gauge: true}
	provider := &mockMeterProvider{meter: mockMeter}

	metrics, err := NewDiskMetricsWithProvider("test-instance", provider)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create int64 gauge")
	assert.Nil(t, metrics)
}

// TestNewDiskMetricsWithProvider_HistogramCreationFailure tests error when histogram creation fails.
func TestNewDiskMetricsWithProvider_HistogramCreationFailure(t *testing.T) {
	mockMeter := &mockMeter{failOnFloat64Histogram: true}
	provider := &mockMeterProvider{meter: mockMeter}

	metrics, err := NewDiskMetricsWithProvider("test-instance", provider)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create float64 histogram")
	assert.Nil(t, metrics)
}

// TestNewDiskMetricsWithProvider_CounterCreationFailure tests error when counter creation fails.
func TestNewDiskMetricsWithProvider_CounterCreationFailure(t *testing.T) {
	mockMeter := &mockMeter{failOnInt64Counter: true}
	provider := &mockMeterProvider{meter: mockMeter}

	metrics, err := NewDiskMetricsWithProvider("test-instance", provider)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create int64 counter")
	assert.Nil(t, metrics)
}

// TestNewDiskMetricsWithProvider_InstrumentCreationOrder tests that instruments are created in expected order.
func TestNewDiskMetricsWithProvider_InstrumentCreationOrder(t *testing.T) {
	mockMeter := &mockMeter{callLog: []string{}}
	provider := &mockMeterProvider{meter: mockMeter}

	_, err := NewDiskMetricsWithProvider("test-instance", provider)

	require.NoError(t, err)
	require.NotEmpty(t, mockMeter.callLog)

	// Verify that all expected instruments were created
	expectedInstruments := []string{
		"Float64Gauge:" + DiskUsageGaugeName,
		"Int64Gauge:" + DiskHealthStatusGaugeName,
		"Float64Histogram:" + DiskOperationDurationHistogramName,
		"Float64Histogram:" + DiskHealthCheckDurationHistogramName,
		"Float64Histogram:" + CleanupOperationDurationHistogramName,
		"Float64Histogram:" + RetentionPolicyDurationHistogramName,
		"Int64Counter:" + DiskOperationCounterName,
		"Int64Counter:" + DiskAlertCounterName,
		"Int64Counter:" + CleanupItemsCounterName,
		"Int64Counter:" + CleanupBytesFreedCounterName,
		"Int64Counter:" + RetentionPolicyCounterName,
	}

	// Verify all instruments were created
	for _, expected := range expectedInstruments {
		assert.Contains(t, mockMeter.callLog, expected, "Expected instrument %s to be created", expected)
	}

	// Verify expected number of each instrument type
	gaugeCount := 0
	histogramCount := 0
	counterCount := 0

	for _, call := range mockMeter.callLog {
		switch {
		case hasPrefix(call, "Float64Gauge:"), hasPrefix(call, "Int64Gauge:"):
			gaugeCount++
		case hasPrefix(call, "Float64Histogram:"):
			histogramCount++
		case hasPrefix(call, "Int64Counter:"):
			counterCount++
		}
	}

	assert.Equal(t, 2, gaugeCount, "Expected 2 gauges")
	assert.Equal(t, 4, histogramCount, "Expected 4 histograms")
	assert.Equal(t, 5, counterCount, "Expected 5 counters")
}

// Helper function to check string prefix.
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
