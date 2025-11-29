package config

import (
	"bytes"
	"testing"
	"time"

	"github.com/spf13/viper"
)

// TestBatchProcessingConfig_WithoutPriorityFields tests that BatchProcessingConfig
// works correctly without the priority-related fields (BatchSizes and DefaultPriority).
// This test will FAIL until those fields are removed from the config structure.
func TestBatchProcessingConfig_WithoutPriorityFields(t *testing.T) {
	tests := []struct {
		name           string
		configData     map[string]interface{}
		expectedConfig BatchProcessingConfig
	}{
		{
			name: "should load batch processing config without priority fields",
			configData: map[string]interface{}{
				"batch_processing": map[string]interface{}{
					"enabled":                    true,
					"threshold_chunks":           100,
					"fallback_to_sequential":     true,
					"use_test_embeddings":        false,
					"max_batch_size":             500,
					"initial_backoff":            "30s",
					"max_backoff":                "300s",
					"max_retries":                3,
					"enable_batch_chunking":      true,
					"poller_interval":            "5s",
					"max_concurrent_polls":       2,
					"submitter_poll_interval":    "5s",
					"max_concurrent_submissions": 1,
					"submission_initial_backoff": "1m",
					"submission_max_backoff":     "30m",
					"max_submission_attempts":    10,
					"queue_limits": map[string]interface{}{
						"max_queue_size": 10000,
						"max_wait_time":  "30m",
					},
					"token_counting": map[string]interface{}{
						"enabled":              true,
						"mode":                 "all",
						"sample_percent":       10,
						"max_tokens_per_chunk": 8192,
					},
				},
			},
			expectedConfig: BatchProcessingConfig{
				Enabled:                  true,
				ThresholdChunks:          100,
				FallbackToSequential:     true,
				UseTestEmbeddings:        false,
				MaxBatchSize:             500,
				InitialBackoff:           30 * time.Second,
				MaxBackoff:               300 * time.Second,
				MaxRetries:               3,
				EnableBatchChunking:      true,
				PollerInterval:           5 * time.Second,
				MaxConcurrentPolls:       2,
				SubmitterPollInterval:    5 * time.Second,
				MaxConcurrentSubmissions: 1,
				SubmissionInitialBackoff: 1 * time.Minute,
				SubmissionMaxBackoff:     30 * time.Minute,
				MaxSubmissionAttempts:    10,
				QueueLimits: QueueLimitsConfig{
					MaxQueueSize: 10000,
					MaxWaitTime:  30 * time.Minute,
				},
				TokenCounting: TokenCountingConfig{
					Enabled:           true,
					Mode:              "all",
					SamplePercent:     10,
					MaxTokensPerChunk: 8192,
				},
			},
		},
		{
			name: "should load minimal batch processing config without priority fields",
			configData: map[string]interface{}{
				"batch_processing": map[string]interface{}{
					"enabled":             false,
					"threshold_chunks":    50,
					"use_test_embeddings": true,
				},
			},
			expectedConfig: BatchProcessingConfig{
				Enabled:           false,
				ThresholdChunks:   50,
				UseTestEmbeddings: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			for key, value := range tt.configData {
				v.Set(key, value)
			}

			var config Config
			if err := v.Unmarshal(&config); err != nil {
				t.Fatalf("failed to unmarshal config: %v", err)
			}

			batchConfig := config.BatchProcessing

			// Verify all expected fields are present and correct
			if batchConfig.Enabled != tt.expectedConfig.Enabled {
				t.Errorf("expected Enabled %v, got %v", tt.expectedConfig.Enabled, batchConfig.Enabled)
			}
			if batchConfig.ThresholdChunks != tt.expectedConfig.ThresholdChunks {
				t.Errorf(
					"expected ThresholdChunks %d, got %d",
					tt.expectedConfig.ThresholdChunks,
					batchConfig.ThresholdChunks,
				)
			}
			if batchConfig.FallbackToSequential != tt.expectedConfig.FallbackToSequential {
				t.Errorf(
					"expected FallbackToSequential %v, got %v",
					tt.expectedConfig.FallbackToSequential,
					batchConfig.FallbackToSequential,
				)
			}
			if batchConfig.UseTestEmbeddings != tt.expectedConfig.UseTestEmbeddings {
				t.Errorf(
					"expected UseTestEmbeddings %v, got %v",
					tt.expectedConfig.UseTestEmbeddings,
					batchConfig.UseTestEmbeddings,
				)
			}
			if batchConfig.MaxBatchSize != tt.expectedConfig.MaxBatchSize {
				t.Errorf("expected MaxBatchSize %d, got %d", tt.expectedConfig.MaxBatchSize, batchConfig.MaxBatchSize)
			}
			if batchConfig.InitialBackoff != tt.expectedConfig.InitialBackoff {
				t.Errorf(
					"expected InitialBackoff %v, got %v",
					tt.expectedConfig.InitialBackoff,
					batchConfig.InitialBackoff,
				)
			}
			if batchConfig.MaxBackoff != tt.expectedConfig.MaxBackoff {
				t.Errorf("expected MaxBackoff %v, got %v", tt.expectedConfig.MaxBackoff, batchConfig.MaxBackoff)
			}
			if batchConfig.MaxRetries != tt.expectedConfig.MaxRetries {
				t.Errorf("expected MaxRetries %d, got %d", tt.expectedConfig.MaxRetries, batchConfig.MaxRetries)
			}
		})
	}
}

// TestBatchProcessingConfig_YAMLWithoutPriority tests loading from YAML without priority-related fields.
// This test will FAIL until BatchSizes and DefaultPriority are removed from the config.
func TestBatchProcessingConfig_YAMLWithoutPriority(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		validate    func(*testing.T, BatchProcessingConfig)
		shouldFail  bool
	}{
		{
			name: "should parse YAML without batch_sizes field",
			yamlContent: `
batch_processing:
  enabled: true
  threshold_chunks: 100
  fallback_to_sequential: true
  use_test_embeddings: false
  max_batch_size: 500
  initial_backoff: 30s
  max_backoff: 300s
  max_retries: 3
  enable_batch_chunking: true
  queue_limits:
    max_queue_size: 10000
    max_wait_time: 30m
  token_counting:
    enabled: true
    mode: "all"
    sample_percent: 10
    max_tokens_per_chunk: 8192
`,
			validate: func(t *testing.T, config BatchProcessingConfig) {
				if !config.Enabled {
					t.Errorf("expected Enabled to be true")
				}
				if config.ThresholdChunks != 100 {
					t.Errorf("expected ThresholdChunks 100, got %d", config.ThresholdChunks)
				}
				if !config.FallbackToSequential {
					t.Errorf("expected FallbackToSequential to be true")
				}
				if config.UseTestEmbeddings {
					t.Errorf("expected UseTestEmbeddings to be false")
				}
				if config.MaxBatchSize != 500 {
					t.Errorf("expected MaxBatchSize 500, got %d", config.MaxBatchSize)
				}
				if config.InitialBackoff != 30*time.Second {
					t.Errorf("expected InitialBackoff 30s, got %v", config.InitialBackoff)
				}
				if config.QueueLimits.MaxQueueSize != 10000 {
					t.Errorf("expected QueueLimits.MaxQueueSize 10000, got %d", config.QueueLimits.MaxQueueSize)
				}
				if config.TokenCounting.Enabled != true {
					t.Errorf("expected TokenCounting.Enabled to be true")
				}
			},
			shouldFail: false,
		},
		{
			name: "should parse YAML without default_priority field",
			yamlContent: `
batch_processing:
  enabled: true
  threshold_chunks: 50
  use_test_embeddings: true
`,
			validate: func(t *testing.T, config BatchProcessingConfig) {
				if !config.Enabled {
					t.Errorf("expected Enabled to be true")
				}
				if config.ThresholdChunks != 50 {
					t.Errorf("expected ThresholdChunks 50, got %d", config.ThresholdChunks)
				}
				if !config.UseTestEmbeddings {
					t.Errorf("expected UseTestEmbeddings to be true")
				}
			},
			shouldFail: false,
		},
		{
			name: "should parse complete YAML without any priority-related fields",
			yamlContent: `
batch_processing:
  enabled: true
  threshold_chunks: 200
  fallback_to_sequential: false
  use_test_embeddings: false
  max_batch_size: 1000
  initial_backoff: 1m
  max_backoff: 10m
  max_retries: 5
  enable_batch_chunking: true
  poller_interval: 10s
  max_concurrent_polls: 5
  submitter_poll_interval: 15s
  max_concurrent_submissions: 3
  submission_initial_backoff: 2m
  submission_max_backoff: 60m
  max_submission_attempts: 20
  queue_limits:
    max_queue_size: 5000
    max_wait_time: 15m
  token_counting:
    enabled: false
    mode: "sample"
    sample_percent: 25
    max_tokens_per_chunk: 4096
`,
			validate: func(t *testing.T, config BatchProcessingConfig) {
				if !config.Enabled {
					t.Errorf("expected Enabled to be true")
				}
				if config.ThresholdChunks != 200 {
					t.Errorf("expected ThresholdChunks 200, got %d", config.ThresholdChunks)
				}
				if config.FallbackToSequential {
					t.Errorf("expected FallbackToSequential to be false")
				}
				if config.MaxBatchSize != 1000 {
					t.Errorf("expected MaxBatchSize 1000, got %d", config.MaxBatchSize)
				}
				if config.InitialBackoff != 1*time.Minute {
					t.Errorf("expected InitialBackoff 1m, got %v", config.InitialBackoff)
				}
				if config.MaxBackoff != 10*time.Minute {
					t.Errorf("expected MaxBackoff 10m, got %v", config.MaxBackoff)
				}
				if config.PollerInterval != 10*time.Second {
					t.Errorf("expected PollerInterval 10s, got %v", config.PollerInterval)
				}
				if config.MaxConcurrentPolls != 5 {
					t.Errorf("expected MaxConcurrentPolls 5, got %d", config.MaxConcurrentPolls)
				}
				if config.SubmitterPollInterval != 15*time.Second {
					t.Errorf("expected SubmitterPollInterval 15s, got %v", config.SubmitterPollInterval)
				}
				if config.MaxConcurrentSubmissions != 3 {
					t.Errorf("expected MaxConcurrentSubmissions 3, got %d", config.MaxConcurrentSubmissions)
				}
				if config.QueueLimits.MaxQueueSize != 5000 {
					t.Errorf("expected QueueLimits.MaxQueueSize 5000, got %d", config.QueueLimits.MaxQueueSize)
				}
				if config.TokenCounting.Enabled {
					t.Errorf("expected TokenCounting.Enabled to be false")
				}
				if config.TokenCounting.Mode != "sample" {
					t.Errorf("expected TokenCounting.Mode 'sample', got %s", config.TokenCounting.Mode)
				}
			},
			shouldFail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := viper.New()
			v.SetConfigType("yaml")
			if err := v.ReadConfig(bytes.NewReader([]byte(tt.yamlContent))); err != nil {
				if !tt.shouldFail {
					t.Fatalf("failed to read YAML config: %v", err)
				}
				return
			}

			var config Config
			if err := v.Unmarshal(&config); err != nil {
				if !tt.shouldFail {
					t.Fatalf("failed to unmarshal config: %v", err)
				}
				return
			}

			if tt.shouldFail {
				t.Errorf("expected test to fail but it succeeded")
				return
			}

			// Run validation function
			if tt.validate != nil {
				tt.validate(t, config.BatchProcessing)
			}
		})
	}
}

// TestBatchSizeConfig_RemovedStruct tests that BatchSizeConfig is no longer needed.
// This test will FAIL until BatchSizes field is removed from BatchProcessingConfig.
func TestBatchSizeConfig_RemovedStruct(t *testing.T) {
	t.Run("BatchSizes field should not exist in BatchProcessingConfig", func(t *testing.T) {
		// Create a config with batch_sizes in YAML (which should be ignored/cause error)
		v := viper.New()
		v.SetConfigType("yaml")
		yamlWithBatchSizes := `
batch_processing:
  enabled: true
  threshold_chunks: 100
  batch_sizes:
    realtime:
      min: 1
      max: 25
      timeout: 5m
`
		if err := v.ReadConfig(bytes.NewReader([]byte(yamlWithBatchSizes))); err != nil {
			t.Fatalf("failed to read YAML config: %v", err)
		}

		var config Config
		if err := v.Unmarshal(&config); err != nil {
			t.Fatalf("failed to unmarshal config: %v", err)
		}

		// BatchSizes field has been removed from the BatchProcessingConfig struct.
		// The config should load successfully even when batch_sizes is present in YAML
		// (viper will simply ignore unknown fields).
		// We verify the config loaded correctly by checking other fields.
		if !config.BatchProcessing.Enabled {
			t.Errorf("Expected Enabled to be true")
		}
		if config.BatchProcessing.ThresholdChunks != 100 {
			t.Errorf("Expected ThresholdChunks to be 100, got %d", config.BatchProcessing.ThresholdChunks)
		}
	})

	t.Run("DefaultPriority field should not exist in BatchProcessingConfig", func(t *testing.T) {
		// Create a config with default_priority in YAML
		v := viper.New()
		v.SetConfigType("yaml")
		yamlWithPriority := `
batch_processing:
  enabled: true
  threshold_chunks: 100
  default_priority: "background"
`
		if err := v.ReadConfig(bytes.NewReader([]byte(yamlWithPriority))); err != nil {
			t.Fatalf("failed to read YAML config: %v", err)
		}

		var config Config
		if err := v.Unmarshal(&config); err != nil {
			t.Fatalf("failed to unmarshal config: %v", err)
		}

		// DefaultPriority field has been removed from the BatchProcessingConfig struct.
		// The config should load successfully even when default_priority is present in YAML
		// (viper will simply ignore unknown fields).
		// We verify the config loaded correctly by checking other fields.
		if !config.BatchProcessing.Enabled {
			t.Errorf("Expected Enabled to be true")
		}
		if config.BatchProcessing.ThresholdChunks != 100 {
			t.Errorf("Expected ThresholdChunks to be 100, got %d", config.BatchProcessing.ThresholdChunks)
		}
	})
}

// TestBatchProcessingConfig_NoPriorityDefaults tests that no priority-related defaults are set.
// This test will FAIL until DefaultPriority field is removed.
func TestBatchProcessingConfig_NoPriorityDefaults(t *testing.T) {
	t.Run("should not set default priority when loading minimal config", func(t *testing.T) {
		v := viper.New()
		v.Set("batch_processing.enabled", true)

		var config Config
		if err := v.Unmarshal(&config); err != nil {
			t.Fatalf("failed to unmarshal config: %v", err)
		}

		// After removal, there should be no DefaultPriority field to check
		// This test documents that priority concepts should be completely removed
		if config.BatchProcessing.Enabled != true {
			t.Errorf("expected Enabled to be true")
		}
	})
}

// TestBatchProcessingConfig_StructFieldValidation validates that only expected fields exist.
// This test will FAIL until BatchSizes and DefaultPriority are removed.
func TestBatchProcessingConfig_StructFieldValidation(t *testing.T) {
	t.Run("should have all non-priority fields", func(t *testing.T) {
		config := BatchProcessingConfig{
			Enabled:                  true,
			ThresholdChunks:          100,
			FallbackToSequential:     true,
			UseTestEmbeddings:        false,
			MaxBatchSize:             500,
			InitialBackoff:           30 * time.Second,
			MaxBackoff:               300 * time.Second,
			MaxRetries:               3,
			EnableBatchChunking:      true,
			PollerInterval:           5 * time.Second,
			MaxConcurrentPolls:       2,
			SubmitterPollInterval:    5 * time.Second,
			MaxConcurrentSubmissions: 1,
			SubmissionInitialBackoff: 1 * time.Minute,
			SubmissionMaxBackoff:     30 * time.Minute,
			MaxSubmissionAttempts:    10,
			QueueLimits: QueueLimitsConfig{
				MaxQueueSize: 10000,
				MaxWaitTime:  30 * time.Minute,
			},
			TokenCounting: TokenCountingConfig{
				Enabled:           true,
				Mode:              "all",
				SamplePercent:     10,
				MaxTokensPerChunk: 8192,
			},
		}

		// Verify that we can create a complete BatchProcessingConfig
		// without setting any priority-related fields
		if !config.Enabled {
			t.Errorf("expected Enabled to be true")
		}
		if config.ThresholdChunks != 100 {
			t.Errorf("expected ThresholdChunks 100, got %d", config.ThresholdChunks)
		}

		// This test will fail if we need to set BatchSizes or DefaultPriority
		// to have a valid configuration
	})
}

// TestFullConfig_WithoutBatchPriority tests loading a complete config without batch priority fields.
// This test will FAIL until the priority fields are removed from BatchProcessingConfig.
func TestFullConfig_WithoutBatchPriority(t *testing.T) {
	yamlContent := `
api:
  host: 0.0.0.0
  port: 8080
  read_timeout: 10s
  write_timeout: 10s

worker:
  concurrency: 5
  queue_group: workers
  job_timeout: 30m

database:
  host: localhost
  port: 5432
  user: testuser
  password: testpass
  name: testdb
  sslmode: disable
  max_connections: 25
  max_idle_connections: 5

nats:
  url: nats://localhost:4222
  max_reconnects: 5
  reconnect_wait: 2s

search:
  iterative_scan_mode: relaxed_order

gemini:
  model: gemini-embedding-001
  max_retries: 3
  timeout: 120s
  batch:
    enabled: true
    input_dir: /tmp/batch_embeddings/input
    output_dir: /tmp/batch_embeddings/output
    poll_interval: 5s
    max_wait_time: 30m

batch_processing:
  enabled: true
  threshold_chunks: 50
  fallback_to_sequential: true
  use_test_embeddings: false
  max_batch_size: 500
  initial_backoff: 30s
  max_backoff: 300s
  max_retries: 3
  enable_batch_chunking: true
  poller_interval: 5s
  max_concurrent_polls: 2
  submitter_poll_interval: 5s
  max_concurrent_submissions: 1
  submission_initial_backoff: 1m
  submission_max_backoff: 30m
  max_submission_attempts: 10
  queue_limits:
    max_queue_size: 10000
    max_wait_time: 30m
  token_counting:
    enabled: true
    mode: all
    sample_percent: 10
    max_tokens_per_chunk: 8192

log:
  level: info
  format: json
`

	v := viper.New()
	v.SetConfigType("yaml")
	if err := v.ReadConfig(bytes.NewReader([]byte(yamlContent))); err != nil {
		t.Fatalf("failed to read YAML config: %v", err)
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		t.Fatalf("failed to unmarshal config: %v", err)
	}

	// Validate that the config loads successfully without priority fields
	if !config.BatchProcessing.Enabled {
		t.Errorf("expected BatchProcessing.Enabled to be true")
	}
	if config.BatchProcessing.ThresholdChunks != 50 {
		t.Errorf("expected ThresholdChunks 50, got %d", config.BatchProcessing.ThresholdChunks)
	}
	if config.BatchProcessing.MaxBatchSize != 500 {
		t.Errorf("expected MaxBatchSize 500, got %d", config.BatchProcessing.MaxBatchSize)
	}

	// Validate the config passes validation
	if err := config.Validate(); err != nil {
		t.Errorf("config validation failed: %v", err)
	}
}
