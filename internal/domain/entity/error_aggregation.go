package entity

import (
	"codechunking/internal/domain/valueobject"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ErrorAggregation represents an aggregation of related errors within a time window with thread-safe operations.
type ErrorAggregation struct {
	// Immutable fields (set once at creation)
	pattern        string
	windowDuration time.Duration

	// Mutable fields (protected by mutex for thread-safety)
	mu              sync.RWMutex
	windowStart     time.Time
	windowEnd       time.Time
	highestSeverity *valueobject.ErrorSeverity
	firstOccurrence time.Time
	lastOccurrence  time.Time

	// Performance optimization: atomic counter to avoid mutex for reads
	count int64
}

// Production configuration constants for error aggregation.
const (
	minWindowDuration     = 30 * time.Second // Minimum aggregation window
	maxWindowDuration     = 24 * time.Hour   // Maximum aggregation window
	defaultWindowDuration = 5 * time.Minute  // Default aggregation window
	maxAggregationCount   = 10000            // Maximum errors per aggregation to prevent memory issues
	patternMatchTolerance = time.Second      // Tolerance for timestamp matching
)

// NewErrorAggregation creates a new error aggregation with comprehensive validation.
func NewErrorAggregation(pattern string, windowDuration time.Duration) (*ErrorAggregation, error) {
	if len(pattern) == 0 {
		return nil, errors.New("error_aggregation: pattern cannot be empty")
	}
	if len(pattern) > 500 {
		return nil, errors.New("error_aggregation: pattern cannot exceed 500 characters")
	}
	if windowDuration <= 0 {
		return nil, errors.New("error_aggregation: window duration must be positive")
	}
	if windowDuration < minWindowDuration {
		return nil, errors.New("window duration must be at least 30 seconds")
	}
	if windowDuration > maxWindowDuration {
		return nil, errors.New("window duration cannot exceed 24 hours")
	}

	now := time.Now()
	return &ErrorAggregation{
		pattern:        pattern,
		windowDuration: windowDuration,
		windowStart:    now,
		windowEnd:      now.Add(windowDuration),
		count:          0,
	}, nil
}

// NewErrorAggregationWithDefaults creates a new error aggregation with default window duration.
func NewErrorAggregationWithDefaults(pattern string) (*ErrorAggregation, error) {
	return NewErrorAggregation(pattern, defaultWindowDuration)
}

// Pattern returns the error pattern for this aggregation.
func (ea *ErrorAggregation) Pattern() string {
	return ea.pattern
}

// WindowDuration returns the duration of the aggregation window.
func (ea *ErrorAggregation) WindowDuration() time.Duration {
	return ea.windowDuration
}

// WindowStart returns the start time of the aggregation window with thread-safe access.
func (ea *ErrorAggregation) WindowStart() time.Time {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.windowStart
}

// WindowEnd returns the end time of the aggregation window with thread-safe access.
func (ea *ErrorAggregation) WindowEnd() time.Time {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.windowEnd
}

// Count returns the number of errors in this aggregation with atomic read.
func (ea *ErrorAggregation) Count() int {
	return int(atomic.LoadInt64(&ea.count))
}

// IsEmpty returns true if no errors have been added to this aggregation.
func (ea *ErrorAggregation) IsEmpty() bool {
	return atomic.LoadInt64(&ea.count) == 0
}

// HighestSeverity returns the highest severity of errors in this aggregation with thread-safe access.
func (ea *ErrorAggregation) HighestSeverity() *valueobject.ErrorSeverity {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.highestSeverity
}

// FirstOccurrence returns the timestamp of the first error in this aggregation with thread-safe access.
func (ea *ErrorAggregation) FirstOccurrence() time.Time {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.firstOccurrence
}

// LastOccurrence returns the timestamp of the last error in this aggregation with thread-safe access.
func (ea *ErrorAggregation) LastOccurrence() time.Time {
	ea.mu.RLock()
	defer ea.mu.RUnlock()
	return ea.lastOccurrence
}

// AddError adds an error to this aggregation with thread-safe validation and limits.
func (ea *ErrorAggregation) AddError(classifiedError *ClassifiedError) error {
	if classifiedError == nil {
		return errors.New("error_aggregation: classified error cannot be nil")
	}

	// Check count limit before acquiring lock for better performance
	currentCount := atomic.LoadInt64(&ea.count)
	if currentCount >= maxAggregationCount {
		return fmt.Errorf("error_aggregation: maximum aggregation count of %d exceeded", maxAggregationCount)
	}

	// Pre-validate pattern match for efficiency
	if !ea.matchesPattern(classifiedError.ErrorPattern()) {
		return fmt.Errorf("error pattern does not match aggregation pattern: got %s, expected %s",
			classifiedError.ErrorPattern(), ea.pattern)
	}

	// Acquire write lock for updates
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Double-check count under lock to prevent race conditions
	if ea.count >= maxAggregationCount {
		return fmt.Errorf("error_aggregation: maximum aggregation count of %d exceeded", maxAggregationCount)
	}

	// Validate time window with tolerance
	errorTimestamp := classifiedError.Timestamp()
	if errorTimestamp.Before(ea.windowStart.Add(-patternMatchTolerance)) ||
		errorTimestamp.After(ea.windowEnd.Add(patternMatchTolerance)) {
		return fmt.Errorf("error_aggregation: error timestamp %v is outside aggregation window [%v, %v]",
			errorTimestamp, ea.windowStart, ea.windowEnd)
	}

	// Update aggregation atomically
	newCount := atomic.AddInt64(&ea.count, 1)

	// Set first/last occurrence and severity
	if newCount == 1 {
		// First error in aggregation
		ea.firstOccurrence = errorTimestamp
		ea.lastOccurrence = errorTimestamp
		ea.highestSeverity = classifiedError.Severity()
	} else {
		// Update occurrence timestamps
		if errorTimestamp.Before(ea.firstOccurrence) {
			ea.firstOccurrence = errorTimestamp
		}
		if errorTimestamp.After(ea.lastOccurrence) {
			ea.lastOccurrence = errorTimestamp
		}

		// Update highest severity if needed
		if ea.highestSeverity == nil || classifiedError.Severity().IsHigherPriority(ea.highestSeverity) {
			ea.highestSeverity = classifiedError.Severity()
		}
	}

	return nil
}

// matchesPattern efficiently checks if an error pattern matches this aggregation's pattern.
func (ea *ErrorAggregation) matchesPattern(errorPattern string) bool {
	// For aggregation, we match by error code only (ignore severity in pattern)
	// Use efficient string operations instead of splits
	patternPrefix := ea.getPatternPrefix()
	errorPatternPrefix := ea.getPatternPrefixFromString(errorPattern)

	return patternPrefix == errorPatternPrefix
}

// getPatternPrefix extracts the error code part from the aggregation pattern.
func (ea *ErrorAggregation) getPatternPrefix() string {
	if colonIndex := strings.Index(ea.pattern, ":"); colonIndex > 0 {
		return ea.pattern[:colonIndex]
	}
	return ea.pattern
}

// getPatternPrefixFromString extracts the error code part from an error pattern string.
func (ea *ErrorAggregation) getPatternPrefixFromString(pattern string) string {
	if colonIndex := strings.Index(pattern, ":"); colonIndex > 0 {
		return pattern[:colonIndex]
	}
	return pattern
}

// IsWindowExpired returns true if the aggregation window has expired with thread-safe access.
func (ea *ErrorAggregation) IsWindowExpired() bool {
	ea.mu.RLock()
	windowEnd := ea.windowEnd
	ea.mu.RUnlock()
	return time.Now().After(windowEnd)
}

// ResetWindow resets the aggregation window and clears all errors with thread-safe operations.
func (ea *ErrorAggregation) ResetWindow() error {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	// Reset all mutable fields
	atomic.StoreInt64(&ea.count, 0)
	ea.highestSeverity = nil
	ea.firstOccurrence = time.Time{}
	ea.lastOccurrence = time.Time{}

	// Reset time window
	now := time.Now()
	ea.windowStart = now
	ea.windowEnd = now.Add(ea.windowDuration)

	return nil
}

// ExtendWindow extends the aggregation window by the specified duration with validation and thread-safety.
func (ea *ErrorAggregation) ExtendWindow(extension time.Duration) error {
	if extension <= 0 {
		return errors.New("error_aggregation: extension duration must be positive")
	}
	if extension > maxWindowDuration {
		return fmt.Errorf(
			"error_aggregation: extension duration %v cannot exceed maximum %v",
			extension,
			maxWindowDuration,
		)
	}

	ea.mu.Lock()
	defer ea.mu.Unlock()

	ea.windowDuration = extension
	ea.windowEnd = ea.windowStart.Add(extension)
	return nil
}

// SetWindowEndForTesting sets the window end time (for testing purposes only).
func (ea *ErrorAggregation) SetWindowEndForTesting(endTime time.Time) {
	ea.windowEnd = endTime
}

// ExceedsCountThreshold returns true if the error count exceeds the threshold.
func (ea *ErrorAggregation) ExceedsCountThreshold(threshold int) bool {
	return atomic.LoadInt64(&ea.count) >= int64(threshold)
}

// ExceedsRateThreshold returns true if the error rate exceeds the threshold.
func (ea *ErrorAggregation) ExceedsRateThreshold(threshold float64) bool {
	if ea.windowDuration == 0 {
		return false
	}
	count := atomic.LoadInt64(&ea.count)
	rate := float64(count) / ea.windowDuration.Seconds()
	return rate > threshold
}

// MarshalJSON implements json.Marshaler.
func (ea *ErrorAggregation) MarshalJSON() ([]byte, error) {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	return json.Marshal(map[string]interface{}{
		"pattern":         ea.pattern,
		"window_duration": ea.windowDuration,
		"window_start":    ea.windowStart,
		"window_end":      ea.windowEnd,
		"count":           atomic.LoadInt64(&ea.count),
		"highest_severity": func() string {
			if ea.highestSeverity == nil {
				return ""
			}
			return ea.highestSeverity.String()
		}(),
		"first_occurrence": ea.firstOccurrence,
		"last_occurrence":  ea.lastOccurrence,
	})
}

// UnmarshalJSON implements json.Unmarshaler with reduced cognitive complexity.
func (ea *ErrorAggregation) UnmarshalJSON(data []byte) error {
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	ea.parseBasicAggregationFields(raw)

	if err := ea.parseDurationField(raw); err != nil {
		return err
	}

	if err := ea.parseAggregationTimestamps(raw); err != nil {
		return err
	}

	if err := ea.parseAggregationSeverity(raw); err != nil {
		return err
	}

	return nil
}

// parseBasicAggregationFields extracts basic fields from raw JSON data.
func (ea *ErrorAggregation) parseBasicAggregationFields(raw map[string]interface{}) {
	ea.pattern = getStringFromMap(raw, "pattern")
	atomic.StoreInt64(&ea.count, int64(getIntFromMap(raw, "count")))
}

// parseDurationField parses the window duration field which can be string or number.
func (ea *ErrorAggregation) parseDurationField(raw map[string]interface{}) error {
	durationValue, ok := raw["window_duration"]
	if !ok {
		return nil
	}

	if durationStr, ok := durationValue.(string); ok {
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("invalid window duration string: %w", err)
		}
		ea.windowDuration = duration
		return nil
	}

	if durationSeconds, ok := durationValue.(float64); ok {
		ea.windowDuration = time.Duration(durationSeconds) * time.Second
		return nil
	}

	return nil // Ignore invalid duration types
}

// parseAggregationTimestamps parses all timestamp fields from raw JSON data.
func (ea *ErrorAggregation) parseAggregationTimestamps(raw map[string]interface{}) error {
	timestamps := []struct {
		key    string
		target *time.Time
	}{
		{"window_start", &ea.windowStart},
		{"window_end", &ea.windowEnd},
		{"first_occurrence", &ea.firstOccurrence},
		{"last_occurrence", &ea.lastOccurrence},
	}

	for _, ts := range timestamps {
		if timestampStr := getStringFromMap(raw, ts.key); timestampStr != "" {
			parsedTime, err := time.Parse(time.RFC3339, timestampStr)
			if err != nil {
				return fmt.Errorf("invalid %s timestamp: %w", ts.key, err)
			}
			*ts.target = parsedTime
		}
	}

	return nil
}

// parseAggregationSeverity parses the severity field from raw JSON data.
func (ea *ErrorAggregation) parseAggregationSeverity(raw map[string]interface{}) error {
	severityStr := getStringFromMap(raw, "highest_severity")
	if severityStr == "" {
		return nil
	}

	severity, err := valueobject.NewErrorSeverity(severityStr)
	if err != nil {
		return fmt.Errorf("invalid severity: %w", err)
	}

	ea.highestSeverity = severity
	return nil
}

// Helper function.
func getIntFromMap(m map[string]interface{}, key string) int {
	if val, ok := m[key]; ok {
		if intVal, ok := val.(int); ok {
			return intVal
		}
		if floatVal, ok := val.(float64); ok {
			return int(floatVal)
		}
	}
	return 0
}

// CalculateSimilarity calculates the similarity score between two aggregations.
func (ea *ErrorAggregation) CalculateSimilarity(other *ErrorAggregation) float64 {
	// Simple similarity based on pattern matching
	pattern1 := strings.ToLower(ea.pattern)
	pattern2 := strings.ToLower(other.pattern)

	// Extract error codes (before the colon)
	parts1 := strings.Split(pattern1, ":")
	parts2 := strings.Split(pattern2, ":")

	if len(parts1) < 1 || len(parts2) < 1 {
		return 0.0
	}

	errorCode1 := parts1[0]
	errorCode2 := parts2[0]

	// Calculate word-based similarity
	words1 := strings.Split(errorCode1, "_")
	words2 := strings.Split(errorCode2, "_")

	commonWords := 0
	for _, w1 := range words1 {
		for _, w2 := range words2 {
			if w1 == w2 {
				commonWords++
				break
			}
		}
	}

	// Use Jaccard similarity: |intersection| / |union|
	union := len(words1) + len(words2) - commonWords
	if union == 0 {
		return 0.0
	}

	return float64(commonWords) / float64(union)
}

// GroupSimilarAggregations groups aggregations based on similarity threshold.
func GroupSimilarAggregations(aggregations []*ErrorAggregation, threshold float64) [][]*ErrorAggregation {
	var groups [][]*ErrorAggregation
	used := make(map[int]bool)

	for i, agg1 := range aggregations {
		if used[i] {
			continue
		}

		group := []*ErrorAggregation{agg1}
		used[i] = true

		for j, agg2 := range aggregations {
			if i == j || used[j] {
				continue
			}

			similarity := agg1.CalculateSimilarity(agg2)
			if similarity >= threshold {
				group = append(group, agg2)
				used[j] = true
			}
		}

		groups = append(groups, group)
	}

	return groups
}

// DetectCascadeFailure detects if multiple aggregations represent a cascade failure.
func DetectCascadeFailure(aggregations []*ErrorAggregation, cascadeWindow time.Duration) bool {
	if len(aggregations) < 2 {
		return false
	}

	// Check if aggregations occurred within a reasonable time window
	var earliest, latest time.Time
	for i, agg := range aggregations {
		if i == 0 {
			earliest = agg.FirstOccurrence()
			latest = agg.LastOccurrence()
			continue
		}

		if agg.FirstOccurrence().Before(earliest) {
			earliest = agg.FirstOccurrence()
		}
		if agg.LastOccurrence().After(latest) {
			latest = agg.LastOccurrence()
		}
	}

	// Cascade detected if multiple failures within the specified window
	return latest.Sub(earliest) <= cascadeWindow
}
