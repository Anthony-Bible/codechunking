package messaging

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEnhancedIndexingJobMessage_SchemaDefinition tests the complete enhanced schema structure.
func TestEnhancedIndexingJobMessage_SchemaDefinition(t *testing.T) {
	tests := []struct {
		name           string
		createMessage  func() EnhancedIndexingJobMessage
		expectedFields []string
		description    string
	}{
		{
			name: "complete_enhanced_schema_has_all_required_fields",
			createMessage: func() EnhancedIndexingJobMessage {
				return EnhancedIndexingJobMessage{
					// Core identification
					MessageID:     "msg-12345",
					CorrelationID: "corr-67890",
					SchemaVersion: "2.0",
					Timestamp:     time.Now(),

					// Repository information
					RepositoryID:  uuid.New(),
					RepositoryURL: "https://github.com/user/repo.git",
					BranchName:    "feature/new-feature",
					CommitHash:    "abc123def456",

					// Job control
					Priority:     JobPriorityHigh,
					RetryAttempt: 1,
					MaxRetries:   3,

					// Processing configuration
					ProcessingMetadata: ProcessingMetadata{
						FileFilters:      []string{"*.go", "*.js"},
						ChunkSizeBytes:   1024,
						MaxFileSizeBytes: 10485760, // 10MB
						IncludeTests:     true,
						ExcludeVendor:    true,
					},

					// Processing context
					ProcessingContext: ProcessingContext{
						TimeoutSeconds:     300,
						MaxMemoryMB:        512,
						ConcurrencyLevel:   4,
						EnableDeepAnalysis: true,
					},
				}
			},
			expectedFields: []string{
				"message_id", "correlation_id", "schema_version", "timestamp",
				"repository_id", "repository_url", "branch_name", "commit_hash",
				"priority", "retry_attempt", "max_retries",
				"processing_metadata", "processing_context",
			},
			description: "Enhanced schema should include all required fields for worker processing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test will fail until the EnhancedIndexingJobMessage type is defined
			msg := tt.createMessage()

			// Verify message can be created (will fail until struct exists)
			assert.NotNil(t, msg, "Enhanced message should be creatable")

			// Test JSON marshaling includes all expected fields
			jsonData, err := json.Marshal(msg)
			require.NoError(t, err, "Message should be JSON serializable")

			var jsonMap map[string]interface{}
			err = json.Unmarshal(jsonData, &jsonMap)
			require.NoError(t, err, "JSON should be valid")

			// Verify all expected fields are present in JSON
			for _, field := range tt.expectedFields {
				assert.Contains(t, jsonMap, field, "JSON should contain field %s", field)
			}

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestJobPriority_ValueObject tests the JobPriority value object.
func TestJobPriority_ValueObject(t *testing.T) {
	tests := []struct {
		name        string
		priority    string
		expectError bool
		description string
	}{
		{
			name:        "high_priority_is_valid",
			priority:    "HIGH",
			expectError: false,
			description: "HIGH priority should be valid for urgent processing",
		},
		{
			name:        "normal_priority_is_valid",
			priority:    "NORMAL",
			expectError: false,
			description: "NORMAL priority should be valid for standard processing",
		},
		{
			name:        "low_priority_is_valid",
			priority:    "LOW",
			expectError: false,
			description: "LOW priority should be valid for background processing",
		},
		{
			name:        "invalid_priority_returns_error",
			priority:    "URGENT",
			expectError: true,
			description: "Invalid priority values should be rejected",
		},
		{
			name:        "empty_priority_returns_error",
			priority:    "",
			expectError: true,
			description: "Empty priority should be rejected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail until JobPriority type and NewJobPriority function exist
			priority, err := NewJobPriority(tt.priority)

			if tt.expectError {
				assert.Error(t, err, "Should return error for invalid priority")
				assert.Empty(t, priority, "Should return empty priority on error")
			} else {
				assert.NoError(t, err, "Should not return error for valid priority")
				assert.Equal(t, JobPriority(tt.priority), priority, "Should return correct priority")
			}

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestJobPriority_Ordering tests priority ordering for queue management.
func TestJobPriority_Ordering(t *testing.T) {
	tests := []struct {
		name         string
		priority1    JobPriority
		priority2    JobPriority
		expectHigher bool
		expectEqual  bool
		description  string
	}{
		{
			name:         "high_priority_is_higher_than_normal",
			priority1:    JobPriorityHigh,
			priority2:    JobPriorityNormal,
			expectHigher: true,
			description:  "HIGH priority should have higher precedence than NORMAL",
		},
		{
			name:         "normal_priority_is_higher_than_low",
			priority1:    JobPriorityNormal,
			priority2:    JobPriorityLow,
			expectHigher: true,
			description:  "NORMAL priority should have higher precedence than LOW",
		},
		{
			name:         "high_priority_is_higher_than_low",
			priority1:    JobPriorityHigh,
			priority2:    JobPriorityLow,
			expectHigher: true,
			description:  "HIGH priority should have higher precedence than LOW",
		},
		{
			name:        "same_priorities_are_equal",
			priority1:   JobPriorityNormal,
			priority2:   JobPriorityNormal,
			expectEqual: true,
			description: "Same priorities should be considered equal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.expectEqual {
				// This will fail until IsHigherThan method exists
				assert.False(t, tt.priority1.IsHigherThan(tt.priority2), "Same priorities should not be higher")
				assert.False(t, tt.priority2.IsHigherThan(tt.priority1), "Same priorities should not be higher")
			} else if tt.expectHigher {
				// This will fail until IsHigherThan method exists
				assert.True(t, tt.priority1.IsHigherThan(tt.priority2), "Priority1 should be higher than priority2")
				assert.False(t, tt.priority2.IsHigherThan(tt.priority1), "Priority2 should not be higher than priority1")
			}

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestEnhancedIndexingJobMessage_Validation tests message field validation.
func TestEnhancedIndexingJobMessage_Validation(t *testing.T) {
	tests := []struct {
		name           string
		setupMessage   func() EnhancedIndexingJobMessage
		expectError    bool
		expectedErrors []string
		description    string
	}{
		{
			name: "valid_message_passes_validation",
			setupMessage: func() EnhancedIndexingJobMessage {
				return EnhancedIndexingJobMessage{
					MessageID:     "msg-valid-12345",
					CorrelationID: "corr-valid-67890",
					SchemaVersion: "2.0",
					Timestamp:     time.Now(),
					RepositoryID:  uuid.New(),
					RepositoryURL: "https://github.com/user/repo.git",
					BranchName:    "main",
					CommitHash:    "abc123def456789012345678901234567890abcd",
					Priority:      JobPriorityNormal,
					RetryAttempt:  0,
					MaxRetries:    3,
					ProcessingMetadata: ProcessingMetadata{
						ChunkSizeBytes: 1024,
					},
					ProcessingContext: ProcessingContext{
						TimeoutSeconds: 300,
					},
				}
			},
			expectError: false,
			description: "Complete valid message should pass validation",
		},
		{
			name: "missing_message_id_fails_validation",
			setupMessage: func() EnhancedIndexingJobMessage {
				return EnhancedIndexingJobMessage{
					CorrelationID: "corr-67890",
					SchemaVersion: "2.0",
					RepositoryID:  uuid.New(),
					RepositoryURL: "https://github.com/user/repo.git",
				}
			},
			expectError:    true,
			expectedErrors: []string{"message_id is required"},
			description:    "Missing MessageID should cause validation failure",
		},
		{
			name: "invalid_repository_url_fails_validation",
			setupMessage: func() EnhancedIndexingJobMessage {
				return EnhancedIndexingJobMessage{
					MessageID:     "msg-12345",
					CorrelationID: "corr-67890",
					SchemaVersion: "2.0",
					RepositoryID:  uuid.New(),
					RepositoryURL: "not-a-valid-url",
				}
			},
			expectError:    true,
			expectedErrors: []string{"repository_url must be a valid git URL"},
			description:    "Invalid repository URL should cause validation failure",
		},
		{
			name: "retry_attempt_exceeding_max_fails_validation",
			setupMessage: func() EnhancedIndexingJobMessage {
				return EnhancedIndexingJobMessage{
					MessageID:     "msg-12345",
					CorrelationID: "corr-67890",
					RepositoryID:  uuid.New(),
					RepositoryURL: "https://github.com/user/repo.git",
					RetryAttempt:  5,
					MaxRetries:    3,
				}
			},
			expectError:    true,
			expectedErrors: []string{"retry_attempt cannot exceed max_retries"},
			description:    "Retry attempt exceeding max retries should cause validation failure",
		},
		{
			name: "invalid_commit_hash_fails_validation",
			setupMessage: func() EnhancedIndexingJobMessage {
				return EnhancedIndexingJobMessage{
					MessageID:     "msg-12345",
					CorrelationID: "corr-67890",
					RepositoryID:  uuid.New(),
					RepositoryURL: "https://github.com/user/repo.git",
					CommitHash:    "not-a-valid-hash",
				}
			},
			expectError:    true,
			expectedErrors: []string{"commit_hash must be a valid SHA-1 hash"},
			description:    "Invalid commit hash should cause validation failure",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := tt.setupMessage()

			// This will fail until Validate method exists
			err := msg.Validate()

			if tt.expectError {
				assert.Error(t, err, "Should return validation error")

				// Check specific error messages if provided
				for _, expectedError := range tt.expectedErrors {
					assert.Contains(t, err.Error(), expectedError, "Error should contain expected message")
				}
			} else {
				assert.NoError(t, err, "Should not return validation error for valid message")
			}

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestEnhancedIndexingJobMessage_JSONSerialization tests JSON marshal/unmarshal.
func TestEnhancedIndexingJobMessage_JSONSerialization(t *testing.T) {
	tests := []struct {
		name        string
		message     EnhancedIndexingJobMessage
		description string
	}{
		{
			name: "complete_message_serializes_correctly",
			message: EnhancedIndexingJobMessage{
				MessageID:     "msg-12345",
				CorrelationID: "corr-67890",
				SchemaVersion: "2.0",
				Timestamp:     time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				RepositoryID:  uuid.MustParse("123e4567-e89b-12d3-a456-426614174000"),
				RepositoryURL: "https://github.com/user/repo.git",
				BranchName:    "feature/test",
				CommitHash:    "abc123def456789012345678901234567890abcd",
				Priority:      JobPriorityHigh,
				RetryAttempt:  1,
				MaxRetries:    3,
				ProcessingMetadata: ProcessingMetadata{
					FileFilters:      []string{"*.go", "*.js"},
					ChunkSizeBytes:   1024,
					MaxFileSizeBytes: 10485760,
					IncludeTests:     true,
					ExcludeVendor:    true,
				},
				ProcessingContext: ProcessingContext{
					TimeoutSeconds:     300,
					MaxMemoryMB:        512,
					ConcurrencyLevel:   4,
					EnableDeepAnalysis: true,
				},
			},
			description: "Complete message should serialize to JSON and deserialize back correctly",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test marshaling
			jsonData, err := json.Marshal(tt.message)
			require.NoError(t, err, "Message should marshal to JSON without error")
			assert.NotEmpty(t, jsonData, "JSON data should not be empty")

			// Test unmarshaling
			var unmarshaled EnhancedIndexingJobMessage
			err = json.Unmarshal(jsonData, &unmarshaled)
			require.NoError(t, err, "JSON should unmarshal without error")

			// Verify all fields are preserved
			assert.Equal(t, tt.message.MessageID, unmarshaled.MessageID, "MessageID should be preserved")
			assert.Equal(t, tt.message.CorrelationID, unmarshaled.CorrelationID, "CorrelationID should be preserved")
			assert.Equal(t, tt.message.SchemaVersion, unmarshaled.SchemaVersion, "SchemaVersion should be preserved")
			assert.Equal(t, tt.message.RepositoryID, unmarshaled.RepositoryID, "RepositoryID should be preserved")
			assert.Equal(t, tt.message.RepositoryURL, unmarshaled.RepositoryURL, "RepositoryURL should be preserved")
			assert.Equal(t, tt.message.BranchName, unmarshaled.BranchName, "BranchName should be preserved")
			assert.Equal(t, tt.message.CommitHash, unmarshaled.CommitHash, "CommitHash should be preserved")
			assert.Equal(t, tt.message.Priority, unmarshaled.Priority, "Priority should be preserved")
			assert.Equal(t, tt.message.RetryAttempt, unmarshaled.RetryAttempt, "RetryAttempt should be preserved")
			assert.Equal(t, tt.message.MaxRetries, unmarshaled.MaxRetries, "MaxRetries should be preserved")

			// Verify processing metadata
			assert.Equal(
				t,
				tt.message.ProcessingMetadata.ChunkSizeBytes,
				unmarshaled.ProcessingMetadata.ChunkSizeBytes,
				"ChunkSizeBytes should be preserved",
			)
			assert.Equal(
				t,
				tt.message.ProcessingMetadata.FileFilters,
				unmarshaled.ProcessingMetadata.FileFilters,
				"FileFilters should be preserved",
			)

			// Verify processing context
			assert.Equal(
				t,
				tt.message.ProcessingContext.TimeoutSeconds,
				unmarshaled.ProcessingContext.TimeoutSeconds,
				"TimeoutSeconds should be preserved",
			)
			assert.Equal(
				t,
				tt.message.ProcessingContext.MaxMemoryMB,
				unmarshaled.ProcessingContext.MaxMemoryMB,
				"MaxMemoryMB should be preserved",
			)

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestEnhancedIndexingJobMessage_TransformFromBasic tests transformation from basic message.
func TestEnhancedIndexingJobMessage_TransformFromBasic(t *testing.T) {
	tests := []struct {
		name             string
		basicMessage     BasicIndexingJobMessage
		options          TransformOptions
		expectedPriority JobPriority
		expectedRetries  int
		description      string
	}{
		{
			name: "basic_message_transforms_with_defaults",
			basicMessage: BasicIndexingJobMessage{
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/user/repo.git",
				Timestamp:     time.Now(),
				MessageID:     "basic-msg-123",
			},
			options: TransformOptions{
				Priority:   JobPriorityNormal,
				MaxRetries: 3,
			},
			expectedPriority: JobPriorityNormal,
			expectedRetries:  3,
			description:      "Basic message should transform to enhanced with default options",
		},
		{
			name: "basic_message_transforms_with_custom_options",
			basicMessage: BasicIndexingJobMessage{
				RepositoryID:  uuid.New(),
				RepositoryURL: "https://github.com/user/repo.git",
				Timestamp:     time.Now(),
				MessageID:     "basic-msg-456",
			},
			options: TransformOptions{
				Priority:       JobPriorityHigh,
				MaxRetries:     5,
				BranchName:     "feature/urgent",
				CommitHash:     "def456abc123789012345678901234567890abcd",
				ChunkSizeBytes: 2048,
				TimeoutSeconds: 600,
			},
			expectedPriority: JobPriorityHigh,
			expectedRetries:  5,
			description:      "Basic message should transform with custom options",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail until TransformFromBasic function exists
			enhanced, err := TransformFromBasic(tt.basicMessage, tt.options)
			require.NoError(t, err, "Should transform basic message without error")

			// Verify core fields are preserved
			assert.Equal(t, tt.basicMessage.RepositoryID, enhanced.RepositoryID, "RepositoryID should be preserved")
			assert.Equal(t, tt.basicMessage.RepositoryURL, enhanced.RepositoryURL, "RepositoryURL should be preserved")
			assert.Equal(t, tt.basicMessage.MessageID, enhanced.MessageID, "MessageID should be preserved")

			// Verify enhanced fields are set correctly
			assert.Equal(t, tt.expectedPriority, enhanced.Priority, "Priority should be set correctly")
			assert.Equal(t, tt.expectedRetries, enhanced.MaxRetries, "MaxRetries should be set correctly")
			assert.Equal(t, 0, enhanced.RetryAttempt, "RetryAttempt should start at 0")
			assert.NotEmpty(t, enhanced.CorrelationID, "CorrelationID should be generated")
			assert.Equal(t, "2.0", enhanced.SchemaVersion, "SchemaVersion should be set to 2.0")

			// Verify optional fields from options
			if tt.options.BranchName != "" {
				assert.Equal(t, tt.options.BranchName, enhanced.BranchName, "BranchName should be set from options")
			}
			if tt.options.CommitHash != "" {
				assert.Equal(t, tt.options.CommitHash, enhanced.CommitHash, "CommitHash should be set from options")
			}

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestCorrelationIDGenerator tests correlation ID generation.
func TestCorrelationIDGenerator(t *testing.T) {
	tests := []struct {
		name        string
		count       int
		description string
	}{
		{
			name:        "generates_unique_correlation_ids",
			count:       100,
			description: "Should generate unique correlation IDs for distributed tracing",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			correlationIDs := make(map[string]bool)

			for range tt.count {
				// This will fail until GenerateCorrelationID function exists
				id := GenerateCorrelationID()

				assert.NotEmpty(t, id, "Correlation ID should not be empty")
				assert.False(t, correlationIDs[id], "Correlation ID should be unique")
				correlationIDs[id] = true
			}

			assert.Len(t, correlationIDs, tt.count, "Should generate expected number of unique IDs")

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestProcessingMetadata_Validation tests processing metadata validation.
func TestProcessingMetadata_Validation(t *testing.T) {
	tests := []struct {
		name        string
		metadata    ProcessingMetadata
		expectError bool
		description string
	}{
		{
			name: "valid_metadata_passes_validation",
			metadata: ProcessingMetadata{
				FileFilters:      []string{"*.go", "*.js", "*.ts"},
				ChunkSizeBytes:   1024,
				MaxFileSizeBytes: 10485760,
				IncludeTests:     true,
				ExcludeVendor:    true,
			},
			expectError: false,
			description: "Valid processing metadata should pass validation",
		},
		{
			name: "zero_chunk_size_fails_validation",
			metadata: ProcessingMetadata{
				ChunkSizeBytes: 0,
			},
			expectError: true,
			description: "Zero chunk size should fail validation",
		},
		{
			name: "negative_max_file_size_fails_validation",
			metadata: ProcessingMetadata{
				ChunkSizeBytes:   1024,
				MaxFileSizeBytes: -1,
			},
			expectError: true,
			description: "Negative max file size should fail validation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail until Validate method exists on ProcessingMetadata
			err := tt.metadata.Validate()

			if tt.expectError {
				assert.Error(t, err, "Should return validation error")
			} else {
				assert.NoError(t, err, "Should not return validation error")
			}

			t.Logf("Test description: %s", tt.description)
		})
	}
}

// TestSchemaVersionCompatibility tests schema version compatibility.
func TestSchemaVersionCompatibility(t *testing.T) {
	tests := []struct {
		name              string
		messageVersion    string
		supportedVersions []string
		expectCompatible  bool
		description       string
	}{
		{
			name:              "version_2_0_is_compatible",
			messageVersion:    "2.0",
			supportedVersions: []string{"1.0", "2.0"},
			expectCompatible:  true,
			description:       "Version 2.0 should be compatible with supported versions",
		},
		{
			name:              "unsupported_version_is_incompatible",
			messageVersion:    "3.0",
			supportedVersions: []string{"1.0", "2.0"},
			expectCompatible:  false,
			description:       "Unsupported version should be incompatible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This will fail until IsSchemaVersionCompatible function exists
			compatible := IsSchemaVersionCompatible(tt.messageVersion, tt.supportedVersions)

			assert.Equal(t, tt.expectCompatible, compatible, "Schema compatibility should match expected")

			t.Logf("Test description: %s", tt.description)
		})
	}
}
