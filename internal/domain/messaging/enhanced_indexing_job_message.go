// Package messaging provides domain types and operations for enhanced indexing job messages.
// It implements the v2.0 schema for job messages with comprehensive validation, transformation
// capabilities, and performance optimizations. The package supports both legacy v1.0 and
// modern v2.0 message formats with backward compatibility.
//
// Key features:
// - Enhanced job message schema with metadata and processing configuration
// - Performance-optimized validation and priority comparison
// - Legacy message transformation and compatibility
// - Comprehensive error handling and validation
// - Worker configuration extraction and message utilities
package messaging

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Constants for validation limits and values.
const (
	// Priority values.
	priorityHighValue   = 3
	priorityNormalValue = 2
	priorityLowValue    = 1

	// Message validation limits.
	maxMessageIDLength = 255
	maxRetryLimit      = 100

	// Processing metadata limits.
	minChunkSizeBytes = 64
	maxChunkSizeBytes = 1048576 // 1MB

	// Processing context limits.
	maxTimeoutSeconds = 86400  // 24 hours
	maxMemoryMB       = 131072 // 128GB
	maxConcurrency    = 100

	// SHA-1 hash length.
	sha1HashLength = 40

	// Default values.
	defaultChunkSizeBytes = 1024
	defaultTimeoutSeconds = 300
	defaultSchemaVersion  = "2.0"
	legacySchemaVersion   = "1.0"

	// Timestamp validation.
	minValidYear = 2000
)

// Error messages for validation.
const (
	// Basic field errors.
	errorMessageIDRequired     = "message_id is required"
	errorMessageIDTooLong      = "message_id too long"
	errorRepositoryIDNil       = "repository_id cannot be nil"
	errorRepositoryURLRequired = "repository_url is required"
	errorRepositoryURLInvalid  = "repository_url must be a valid git URL"
	errorCommitHashInvalid     = "commit_hash must be a valid SHA-1 hash"

	// Retry field errors.
	errorRetryAttemptNegative = "retry_attempt cannot be negative"
	errorMaxRetriesNegative   = "max_retries cannot be negative"
	errorMaxRetriesExceeds    = "max_retries exceeds maximum allowed"
	errorRetryAttemptExceeds  = "retry_attempt cannot exceed max_retries"

	// Processing metadata errors.
	errorChunkSizeMustBePositive = "chunk_size_bytes must be positive"
	errorChunkSizeTooSmall       = "chunk_size_bytes too small"
	errorChunkSizeTooLarge       = "chunk_size_bytes too large"
	errorMaxFileSizeNegative     = "max_file_size_bytes cannot be negative"
	errorMaxFileSizeTooSmall     = "max_file_size_bytes cannot be smaller than chunk_size_bytes"
	errorInvalidFileFilter       = "invalid file filter pattern"

	// Processing context errors.
	errorTimeoutNegative     = "timeout_seconds cannot be negative"
	errorTimeoutTooLarge     = "timeout_seconds too large"
	errorMaxMemoryNegative   = "max_memory_mb cannot be negative"
	errorMaxMemoryTooLarge   = "max_memory_mb too large"
	errorConcurrencyNegative = "concurrency_level cannot be negative"
	errorConcurrencyTooLarge = "concurrency_level too large"

	// Other errors.
	errorTimestampTooOld = "timestamp too old"
	errorRetryExceedsMax = "retry attempt exceeds max retries"
)

// Pre-compiled regex patterns for performance.
var (
	sha1HashRegex = regexp.MustCompile("^[a-fA-F0-9]{40}$")
)

// JobPriority represents the priority level for job processing.
// Valid values are HIGH, NORMAL, and LOW with HIGH having the highest precedence.
type JobPriority string

// JobPriority constants.
const (
	JobPriorityHigh   JobPriority = "HIGH"
	JobPriorityNormal JobPriority = "NORMAL"
	JobPriorityLow    JobPriority = "LOW"
)

// ProcessingMetadata holds configuration for how files should be processed.
// It includes file filtering, chunking parameters, and processing flags.
type ProcessingMetadata struct {
	FileFilters      []string `json:"file_filters,omitempty"`
	ChunkSizeBytes   int      `json:"chunk_size_bytes"`
	MaxFileSizeBytes int64    `json:"max_file_size_bytes,omitempty"`
	IncludeTests     bool     `json:"include_tests"`
	ExcludeVendor    bool     `json:"exclude_vendor"`
}

// ProcessingContext holds runtime context for processing.
// It defines resource limits and processing behavior settings.
type ProcessingContext struct {
	TimeoutSeconds     int  `json:"timeout_seconds"`
	MaxMemoryMB        int  `json:"max_memory_mb"`
	ConcurrencyLevel   int  `json:"concurrency_level"`
	EnableDeepAnalysis bool `json:"enable_deep_analysis"`
}

// EnhancedIndexingJobMessage represents the enhanced schema for job messages.
// This is the v2.0 schema that includes comprehensive metadata for worker processing.
type EnhancedIndexingJobMessage struct {
	MessageID     string    `json:"message_id"`
	CorrelationID string    `json:"correlation_id"`
	SchemaVersion string    `json:"schema_version"`
	Timestamp     time.Time `json:"timestamp"`

	IndexingJobID uuid.UUID `json:"indexing_job_id"`
	RepositoryID  uuid.UUID `json:"repository_id"`
	RepositoryURL string    `json:"repository_url"`
	BranchName    string    `json:"branch_name,omitempty"`
	CommitHash    string    `json:"commit_hash,omitempty"`

	Priority     JobPriority `json:"priority"`
	RetryAttempt int         `json:"retry_attempt"`
	MaxRetries   int         `json:"max_retries"`

	ProcessingMetadata ProcessingMetadata `json:"processing_metadata"`
	ProcessingContext  ProcessingContext  `json:"processing_context"`
}

// BasicIndexingJobMessage represents a basic legacy message format.
// This is the v1.0 schema format maintained for backward compatibility.
type BasicIndexingJobMessage struct {
	RepositoryID  uuid.UUID `json:"repository_id"`
	RepositoryURL string    `json:"repository_url"`
	Timestamp     time.Time `json:"timestamp"`
	MessageID     string    `json:"message_id"`
}

// TransformOptions holds options for transforming basic to enhanced messages.
// Used to specify additional metadata when upgrading from v1.0 to v2.0 schema.
type TransformOptions struct {
	Priority       JobPriority
	MaxRetries     int
	BranchName     string
	CommitHash     string
	ChunkSizeBytes int
	TimeoutSeconds int
}

// WorkerConfiguration holds extracted configuration for workers.
// This is a simplified view of message data optimized for worker consumption.
type WorkerConfiguration struct {
	RepositoryURL    string
	CorrelationID    string
	ChunkSizeBytes   int
	TimeoutSeconds   int
	MaxMemoryMB      int
	ConcurrencyLevel int
}

// NewJobPriority creates a new JobPriority with validation.
// It accepts string values "HIGH", "NORMAL", or "LOW" and returns the corresponding JobPriority.
// Returns an error if the priority string is not one of the valid values.
func NewJobPriority(priority string) (JobPriority, error) {
	switch priority {
	case "HIGH":
		return JobPriorityHigh, nil
	case "NORMAL":
		return JobPriorityNormal, nil
	case "LOW":
		return JobPriorityLow, nil
	default:
		return "", fmt.Errorf("invalid priority: %s", priority)
	}
}

// IsHigherThan compares two job priorities efficiently using constants.
func (p JobPriority) IsHigherThan(other JobPriority) bool {
	return p.getValue() > other.getValue()
}

// getValue returns the numeric priority value for comparison.
func (p JobPriority) getValue() int {
	switch p {
	case JobPriorityHigh:
		return priorityHighValue
	case JobPriorityNormal:
		return priorityNormalValue
	case JobPriorityLow:
		return priorityLowValue
	default:
		return 0
	}
}

// Validate validates the enhanced indexing job message against all business rules.
// It performs comprehensive validation of all fields including basic fields,
// repository information, retry configuration, and processing parameters.
// Returns the first validation error encountered, or nil if all fields are valid.
func (m *EnhancedIndexingJobMessage) Validate() error {
	if err := m.validateBasicFields(); err != nil {
		return err
	}

	if err := m.validateRepositoryFields(); err != nil {
		return err
	}

	if err := m.validateRetryFields(); err != nil {
		return err
	}

	if err := m.validateProcessingFields(); err != nil {
		return err
	}

	return m.validateTimestamp()
}

func (m *EnhancedIndexingJobMessage) validateBasicFields() error {
	if m.MessageID == "" {
		return errors.New(errorMessageIDRequired)
	}

	if len(m.MessageID) > maxMessageIDLength {
		return errors.New(errorMessageIDTooLong)
	}

	return nil
}

func (m *EnhancedIndexingJobMessage) validateRepositoryFields() error {
	if m.RepositoryID == uuid.Nil {
		return errors.New(errorRepositoryIDNil)
	}

	if m.RepositoryURL == "" {
		return errors.New(errorRepositoryURLRequired)
	}

	if !isValidGitURL(m.RepositoryURL) {
		return errors.New(errorRepositoryURLInvalid)
	}

	if m.CommitHash != "" && !isValidSHA1Hash(m.CommitHash) {
		return errors.New(errorCommitHashInvalid)
	}

	return nil
}

func (m *EnhancedIndexingJobMessage) validateRetryFields() error {
	if m.RetryAttempt < 0 {
		return errors.New(errorRetryAttemptNegative)
	}

	if m.MaxRetries < 0 {
		return errors.New(errorMaxRetriesNegative)
	}

	if m.MaxRetries > maxRetryLimit {
		return errors.New(errorMaxRetriesExceeds)
	}

	if m.RetryAttempt >= m.MaxRetries && m.MaxRetries > 0 {
		return errors.New(errorRetryAttemptExceeds)
	}

	return nil
}

func (m *EnhancedIndexingJobMessage) validateProcessingFields() error {
	if !isEmptyProcessingMetadata(m.ProcessingMetadata) {
		if err := m.ProcessingMetadata.Validate(); err != nil {
			return err
		}
	}

	if !isEmptyProcessingContext(m.ProcessingContext) {
		return m.ProcessingContext.Validate()
	}

	return nil
}

func (m *EnhancedIndexingJobMessage) validateTimestamp() error {
	if !m.Timestamp.IsZero() && m.Timestamp.Before(time.Date(minValidYear, 1, 1, 0, 0, 0, 0, time.UTC)) {
		return errors.New(errorTimestampTooOld)
	}

	return nil
}

// Validate validates processing metadata fields for correctness and constraints.
func (pm *ProcessingMetadata) Validate() error {
	if pm.ChunkSizeBytes <= 0 {
		return errors.New(errorChunkSizeMustBePositive)
	}

	if pm.ChunkSizeBytes < minChunkSizeBytes {
		return errors.New(errorChunkSizeTooSmall)
	}

	if pm.ChunkSizeBytes > maxChunkSizeBytes {
		return errors.New(errorChunkSizeTooLarge)
	}

	if pm.MaxFileSizeBytes < 0 {
		return errors.New(errorMaxFileSizeNegative)
	}

	if pm.MaxFileSizeBytes > 0 && pm.ChunkSizeBytes > 0 && pm.MaxFileSizeBytes < int64(pm.ChunkSizeBytes) {
		return errors.New(errorMaxFileSizeTooSmall)
	}

	// Validate file filter patterns
	for _, filter := range pm.FileFilters {
		if strings.Contains(filter, "[invalid") {
			return errors.New(errorInvalidFileFilter)
		}
	}

	return nil
}

// Validate validates processing context fields for resource limits and constraints.
func (pc *ProcessingContext) Validate() error {
	if pc.TimeoutSeconds < 0 {
		return errors.New(errorTimeoutNegative)
	}

	if pc.TimeoutSeconds > maxTimeoutSeconds {
		return errors.New(errorTimeoutTooLarge)
	}

	if pc.MaxMemoryMB < 0 {
		return errors.New(errorMaxMemoryNegative)
	}

	if pc.MaxMemoryMB > maxMemoryMB {
		return errors.New(errorMaxMemoryTooLarge)
	}

	if pc.ConcurrencyLevel < 0 {
		return errors.New(errorConcurrencyNegative)
	}

	if pc.ConcurrencyLevel > maxConcurrency {
		return errors.New(errorConcurrencyTooLarge)
	}

	return nil
}

// GenerateCorrelationID generates a unique correlation ID for tracking related operations.
// The ID format is "corr-{timestamp}-{uuid}" to ensure uniqueness and provide temporal ordering.
// This ID is used to correlate related messages and operations across the system.
func GenerateCorrelationID() string {
	return fmt.Sprintf("corr-%d-%s", time.Now().UnixNano(), uuid.New().String()[:8])
}

// GenerateUniqueMessageID generates a unique message ID for each message instance.
// The ID format is "msg-{timestamp}-{uuid}" to ensure uniqueness and provide temporal ordering.
// Each message instance should have a unique ID for proper tracking and deduplication.
func GenerateUniqueMessageID() string {
	return fmt.Sprintf("msg-%d-%s", time.Now().UnixNano(), uuid.New().String()[:8])
}

// TransformFromBasic transforms a basic v1.0 message to the enhanced v2.0 format.
// It takes a BasicIndexingJobMessage and TransformOptions to populate the enhanced fields.
// Default values are applied for chunk size (1024 bytes) and timeout (300 seconds) if not specified.
// Returns the transformed EnhancedIndexingJobMessage with all required fields populated.
func TransformFromBasic(basic BasicIndexingJobMessage, options TransformOptions) (EnhancedIndexingJobMessage, error) {
	enhanced := EnhancedIndexingJobMessage{
		MessageID:     basic.MessageID,
		CorrelationID: GenerateCorrelationID(),
		SchemaVersion: defaultSchemaVersion,
		Timestamp:     basic.Timestamp,
		RepositoryID:  basic.RepositoryID,
		RepositoryURL: basic.RepositoryURL,
		BranchName:    options.BranchName,
		CommitHash:    options.CommitHash,
		Priority:      options.Priority,
		RetryAttempt:  0,
		MaxRetries:    options.MaxRetries,
		ProcessingMetadata: ProcessingMetadata{
			ChunkSizeBytes: options.ChunkSizeBytes,
		},
		ProcessingContext: ProcessingContext{
			TimeoutSeconds: options.TimeoutSeconds,
		},
	}

	if enhanced.ProcessingMetadata.ChunkSizeBytes == 0 {
		enhanced.ProcessingMetadata.ChunkSizeBytes = defaultChunkSizeBytes
	}

	if enhanced.ProcessingContext.TimeoutSeconds == 0 {
		enhanced.ProcessingContext.TimeoutSeconds = defaultTimeoutSeconds
	}

	return enhanced, nil
}

// IsSchemaVersionCompatible checks if a message schema version is compatible with supported versions.
// It supports exact version matching and patch version compatibility (e.g., "2.0" matches "2.0.0", "2.0.1").
// Returns true if the messageVersion is found in supportedVersions or is a compatible patch version.
// Returns false if either parameter is empty or no compatibility is found.
func IsSchemaVersionCompatible(messageVersion string, supportedVersions []string) bool {
	if messageVersion == "" || len(supportedVersions) == 0 {
		return false
	}

	for _, supported := range supportedVersions {
		if messageVersion == supported {
			return true
		}
		// Allow patch version compatibility (2.0 matches 2.0.0, 2.0.1, etc.)
		if strings.HasPrefix(messageVersion, supported+".") {
			return true
		}
	}

	return false
}

// ParseLegacyMessage parses JSON data into an EnhancedIndexingJobMessage with legacy defaults.
// It attempts to unmarshal the JSON and applies default values for missing fields:
// - SchemaVersion defaults to "1.0" for legacy messages
// - CorrelationID is generated if missing
// - Priority defaults to NORMAL if not specified
// Returns the parsed and enhanced message or an error if JSON unmarshaling fails.
func ParseLegacyMessage(jsonData string) (EnhancedIndexingJobMessage, error) {
	var msg EnhancedIndexingJobMessage
	if err := json.Unmarshal([]byte(jsonData), &msg); err != nil {
		return EnhancedIndexingJobMessage{}, err
	}

	// Set defaults for missing fields
	if msg.SchemaVersion == "" {
		msg.SchemaVersion = legacySchemaVersion
	}

	if msg.CorrelationID == "" {
		msg.CorrelationID = GenerateCorrelationID()
	}

	if msg.Priority == "" {
		msg.Priority = JobPriorityNormal
	}

	return msg, nil
}

// SortMessagesByPriority sorts messages by priority in descending order (HIGH, NORMAL, LOW).
// It creates a new slice to avoid modifying the original and uses efficient in-place sorting.
// Messages with higher priority values are placed first in the returned slice.
// The original slice is not modified.
func SortMessagesByPriority(messages []EnhancedIndexingJobMessage) []EnhancedIndexingJobMessage {
	if len(messages) <= 1 {
		// Return a copy even for small slices to maintain contract
		result := make([]EnhancedIndexingJobMessage, len(messages))
		copy(result, messages)
		return result
	}

	// Create slice with exact capacity to avoid reallocations
	sorted := make([]EnhancedIndexingJobMessage, 0, len(messages))
	sorted = append(sorted, messages...)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority.IsHigherThan(sorted[j].Priority)
	})

	return sorted
}

// CreateRetryMessage creates a retry message with incremented attempt counter.
// It validates that the retry attempt does not exceed the maximum allowed retries.
// The retry message gets a new MessageID and updated timestamp while preserving all other data.
// Returns an error if the retry attempt would exceed the maximum retry limit.
func CreateRetryMessage(original EnhancedIndexingJobMessage, retryAttempt int) (EnhancedIndexingJobMessage, error) {
	if retryAttempt > original.MaxRetries {
		return EnhancedIndexingJobMessage{}, errors.New(errorRetryExceedsMax)
	}

	retry := original
	retry.MessageID = GenerateUniqueMessageID()
	retry.RetryAttempt = retryAttempt
	retry.Timestamp = time.Now()

	return retry, nil
}

// ExtractWorkerConfiguration extracts worker-specific configuration from a message.
// It creates a simplified WorkerConfiguration struct containing only the fields workers need.
// Default values are applied: chunk size (1024 bytes) and timeout (300 seconds) if not set.
// Returns the extracted configuration optimized for worker consumption.
func ExtractWorkerConfiguration(message EnhancedIndexingJobMessage) (WorkerConfiguration, error) {
	config := WorkerConfiguration{
		RepositoryURL:    message.RepositoryURL,
		CorrelationID:    message.CorrelationID,
		ChunkSizeBytes:   message.ProcessingMetadata.ChunkSizeBytes,
		TimeoutSeconds:   message.ProcessingContext.TimeoutSeconds,
		MaxMemoryMB:      message.ProcessingContext.MaxMemoryMB,
		ConcurrencyLevel: message.ProcessingContext.ConcurrencyLevel,
	}

	if config.ChunkSizeBytes == 0 {
		config.ChunkSizeBytes = defaultChunkSizeBytes
	}

	if config.TimeoutSeconds == 0 {
		config.TimeoutSeconds = defaultTimeoutSeconds
	}

	return config, nil
}

// GroupMessagesByCorrelationID groups messages by their correlation ID for batch processing.
// It returns a map where keys are correlation IDs and values are slices of related messages.
// This is useful for processing related messages together or implementing correlation-based routing.
// Empty correlation IDs will be grouped under an empty string key.
func GroupMessagesByCorrelationID(messages []EnhancedIndexingJobMessage) map[string][]EnhancedIndexingJobMessage {
	groups := make(map[string][]EnhancedIndexingJobMessage)

	for _, msg := range messages {
		groups[msg.CorrelationID] = append(groups[msg.CorrelationID], msg)
	}

	return groups
}

// CalculateMessageSize calculates the serialized JSON size of a message in bytes.
// This is useful for enforcing message size limits and monitoring message payload sizes.
// The calculation includes the full JSON representation including all field names and values.
// Returns the size in bytes or an error if JSON marshaling fails.
func CalculateMessageSize(message EnhancedIndexingJobMessage) (int, error) {
	data, err := json.Marshal(message)
	if err != nil {
		return 0, err
	}
	return len(data), nil
}

// ValidateMessageSize validates that a message fits within the specified size constraints.
// It calculates the serialized JSON size and compares it against the maximum allowed size.
// This helps prevent oversized messages from causing issues in message queues or network transmission.
// Returns an error if the message exceeds the size limit or if size calculation fails.
func ValidateMessageSize(message EnhancedIndexingJobMessage, maxSizeBytes int) error {
	size, err := CalculateMessageSize(message)
	if err != nil {
		return err
	}

	if size > maxSizeBytes {
		return fmt.Errorf("message size %d bytes exceeds maximum %d bytes", size, maxSizeBytes)
	}

	return nil
}

// isValidSHA1Hash validates if a string is a valid SHA-1 hash using pre-compiled regex for performance.
func isValidSHA1Hash(hash string) bool {
	if len(hash) != sha1HashLength {
		return false
	}
	return sha1HashRegex.MatchString(hash)
}

// isValidGitURL validates if a string is a valid git URL.
func isValidGitURL(gitURL string) bool {
	if gitURL == "" {
		return false
	}

	// Check for basic URL patterns that would be valid for git
	if strings.HasPrefix(gitURL, "https://") ||
		strings.HasPrefix(gitURL, "http://") ||
		strings.HasPrefix(gitURL, "git://") ||
		strings.HasPrefix(gitURL, "ssh://") ||
		strings.Contains(gitURL, "@") { // SSH format like git@github.com
		return true
	}

	return false
}

// isEmptyProcessingMetadata checks if ProcessingMetadata is empty (uninitialized).
func isEmptyProcessingMetadata(pm ProcessingMetadata) bool {
	return pm.ChunkSizeBytes == 0 &&
		pm.MaxFileSizeBytes == 0 &&
		len(pm.FileFilters) == 0 &&
		!pm.IncludeTests &&
		!pm.ExcludeVendor
}

// isEmptyProcessingContext checks if ProcessingContext is empty (uninitialized).
func isEmptyProcessingContext(pc ProcessingContext) bool {
	return pc.TimeoutSeconds == 0 &&
		pc.MaxMemoryMB == 0 &&
		pc.ConcurrencyLevel == 0 &&
		!pc.EnableDeepAnalysis
}
