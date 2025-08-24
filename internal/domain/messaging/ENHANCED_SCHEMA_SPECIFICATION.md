# Enhanced Indexing Job Message Schema Specification

## Overview

This document defines the complete enhanced message schema for Phase 3.1 NATS/JetStream Worker Setup based on the comprehensive failing tests. This specification serves as the definitive guide for implementing the enhanced message types during the GREEN phase of TDD.

## Core Types Required

### 1. EnhancedIndexingJobMessage

The main message structure with all enhanced fields:

```go
type EnhancedIndexingJobMessage struct {
    // Core message identification
    MessageID     string    `json:"message_id"`
    CorrelationID string    `json:"correlation_id"`
    SchemaVersion string    `json:"schema_version"`
    Timestamp     time.Time `json:"timestamp"`

    // Repository information
    RepositoryID  uuid.UUID `json:"repository_id"`
    RepositoryURL string    `json:"repository_url"`
    BranchName    string    `json:"branch_name,omitempty"`
    CommitHash    string    `json:"commit_hash,omitempty"`

    // Job control
    Priority     JobPriority `json:"priority"`
    RetryAttempt int         `json:"retry_attempt"`
    MaxRetries   int         `json:"max_retries"`

    // Processing configuration
    ProcessingMetadata ProcessingMetadata `json:"processing_metadata"`
    ProcessingContext  ProcessingContext  `json:"processing_context"`
}
```

### 2. JobPriority Value Object

Priority levels for queue management:

```go
type JobPriority string

const (
    JobPriorityHigh   JobPriority = "HIGH"
    JobPriorityNormal JobPriority = "NORMAL"
    JobPriorityLow    JobPriority = "LOW"
)
```

### 3. ProcessingMetadata

File processing configuration:

```go
type ProcessingMetadata struct {
    FileFilters      []string `json:"file_filters,omitempty"`
    ChunkSizeBytes   int      `json:"chunk_size_bytes"`
    MaxFileSizeBytes int64    `json:"max_file_size_bytes"`
    IncludeTests     bool     `json:"include_tests"`
    ExcludeVendor    bool     `json:"exclude_vendor"`
}
```

### 4. ProcessingContext

Worker execution context:

```go
type ProcessingContext struct {
    TimeoutSeconds     int  `json:"timeout_seconds"`
    MaxMemoryMB        int  `json:"max_memory_mb"`
    ConcurrencyLevel   int  `json:"concurrency_level"`
    EnableDeepAnalysis bool `json:"enable_deep_analysis"`
}
```

### 5. BasicIndexingJobMessage

Legacy message format for backward compatibility:

```go
type BasicIndexingJobMessage struct {
    RepositoryID  uuid.UUID `json:"repository_id"`
    RepositoryURL string    `json:"repository_url"`
    Timestamp     time.Time `json:"timestamp"`
    MessageID     string    `json:"message_id"`
}
```

### 6. TransformOptions

Options for transforming basic to enhanced messages:

```go
type TransformOptions struct {
    Priority         JobPriority
    MaxRetries       int
    BranchName       string
    CommitHash       string
    ChunkSizeBytes   int
    TimeoutSeconds   int
    MaxMemoryMB      int
    ConcurrencyLevel int
}
```

## Required Methods

### JobPriority Methods

```go
func NewJobPriority(priority string) (JobPriority, error)
func (p JobPriority) IsHigherThan(other JobPriority) bool
```

### EnhancedIndexingJobMessage Methods

```go
func (m EnhancedIndexingJobMessage) Validate() error
```

### ProcessingMetadata Methods

```go
func (m ProcessingMetadata) Validate() error
```

### ProcessingContext Methods

```go
func (c ProcessingContext) Validate() error
```

## Required Functions

### Core Message Operations

```go
func TransformFromBasic(basic BasicIndexingJobMessage, options TransformOptions) (EnhancedIndexingJobMessage, error)
func GenerateCorrelationID() string
func GenerateUniqueMessageID() string
func ParseLegacyMessage(jsonData string) (EnhancedIndexingJobMessage, error)
```

### Retry Management

```go
func CreateRetryMessage(original EnhancedIndexingJobMessage, retryAttempt int) (EnhancedIndexingJobMessage, error)
```

### Message Processing

```go
func SortMessagesByPriority(messages []EnhancedIndexingJobMessage) []EnhancedIndexingJobMessage
func GroupMessagesByCorrelationID(messages []EnhancedIndexingJobMessage) map[string][]EnhancedIndexingJobMessage
func ExtractWorkerConfiguration(message EnhancedIndexingJobMessage) (WorkerConfiguration, error)
```

### Schema Compatibility

```go
func IsSchemaVersionCompatible(messageVersion string, supportedVersions []string) bool
```

### Message Size Management

```go
func CalculateMessageSize(message EnhancedIndexingJobMessage) (int, error)
func ValidateMessageSize(message EnhancedIndexingJobMessage, maxSizeBytes int) error
```

## Validation Rules

### EnhancedIndexingJobMessage Validation

1. **Required Fields:**
   - `MessageID` must not be empty
   - `CorrelationID` must not be empty  
   - `RepositoryID` must not be nil UUID
   - `RepositoryURL` must be valid git URL ending in .git
   - `Priority` must be valid enum value

2. **Field Constraints:**
   - `MessageID` length must be ≤ 255 characters
   - `CommitHash` must be valid SHA-1 hash (40 characters) if provided
   - `RetryAttempt` must be ≥ 0 and < `MaxRetries`
   - `MaxRetries` must be ≥ 0 and ≤ reasonable limit (e.g., 10)
   - `Timestamp` must not be extremely old (e.g., pre-1970)

3. **Branch Name Rules:**
   - Empty branch name defaults to "main"
   - Special characters are allowed in branch names
   - No leading/trailing whitespace

### JobPriority Validation

1. Must be exact match: "HIGH", "NORMAL", or "LOW"
2. Case-sensitive validation
3. No leading/trailing whitespace
4. No special characters or numbers

### ProcessingMetadata Validation

1. **ChunkSizeBytes:**
   - Must be > 0
   - Must be ≥ 64 bytes (reasonable minimum)
   - Must be ≤ 10MB (reasonable maximum)

2. **MaxFileSizeBytes:**
   - Must be ≥ 0 (0 means unlimited)
   - Must be ≥ `ChunkSizeBytes` if > 0
   - Must be ≤ reasonable limit (e.g., 1GB)

3. **FileFilters:**
   - Can be empty/nil (means process all files)
   - Must be valid regex patterns if provided

### ProcessingContext Validation

1. **TimeoutSeconds:**
   - Must be ≥ 0 (0 means no timeout)
   - Must be ≤ reasonable limit (e.g., 1 hour)

2. **MaxMemoryMB:**
   - Must be ≥ 0 (0 means unlimited)
   - Must be ≤ reasonable limit (e.g., 16GB)

3. **ConcurrencyLevel:**
   - Must be ≥ 0 (0 defaults to 1)
   - Must be ≤ reasonable limit (e.g., 100)

## Priority Ordering

Priority ordering for queue processing (highest to lowest):
1. `JobPriorityHigh`
2. `JobPriorityNormal`  
3. `JobPriorityLow`

## Schema Versioning

- Current schema version: "2.0"
- Backward compatibility required for version "1.0"
- Version compatibility uses semantic versioning rules
- Patch version differences are compatible
- Major version differences are incompatible

## JSON Serialization

- All fields use snake_case in JSON
- Optional fields use `omitempty` tag
- Proper UUID serialization/deserialization
- RFC3339 timestamp format
- Handle Unicode characters correctly
- Proper escaping of special JSON characters

## Message Size Constraints

- Recommend maximum message size: 4KB
- Track and validate message size before sending
- Large file filter lists may exceed size limits
- Provide clear error messages for oversized messages

## Correlation ID Format

- Must be unique for each processing pipeline
- Used for distributed tracing across multiple messages
- Format: implementation-defined but must be non-empty
- Length: reasonable length (e.g., 10-100 characters)
- No spaces allowed

## Error Handling

- All validation methods return descriptive errors
- Multiple validation errors should be combined
- Clear field-specific error messages
- Graceful handling of malformed input

## Integration Requirements

- Compatible with NATS JetStream message publishing
- Support for message queue priority ordering  
- Enable distributed tracing through correlation IDs
- Provide complete worker configuration extraction
- Support retry mechanisms with attempt tracking
- Backward compatibility with existing basic messages

## File Locations

Based on hexagonal architecture patterns:

- Domain types: `internal/domain/messaging/`
- Value objects follow existing patterns in `internal/domain/valueobject/`
- Maintain files under 500 lines for readability
- Use existing UUID, time, and JSON patterns from codebase