package entity

import (
	"codechunking/internal/domain/valueobject"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewGitCloneMetrics(t *testing.T) {
	tests := []struct {
		name          string
		repositoryID  uuid.UUID
		repositoryURL string
		cloneOptions  valueobject.CloneOptions
		expectError   bool
	}{
		{
			name:          "should create metrics for shallow clone",
			repositoryID:  uuid.New(),
			repositoryURL: "https://github.com/user/repo.git",
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectError:   false,
		},
		{
			name:          "should create metrics for full clone",
			repositoryID:  uuid.New(),
			repositoryURL: "https://github.com/user/large-repo.git",
			cloneOptions:  valueobject.NewFullCloneOptions(),
			expectError:   false,
		},
		{
			name:          "should create metrics with custom depth",
			repositoryID:  uuid.New(),
			repositoryURL: "https://gitlab.com/user/project.git",
			cloneOptions:  valueobject.NewShallowCloneOptions(50, "develop"),
			expectError:   false,
		},
		{
			name:          "should reject nil repository ID",
			repositoryID:  uuid.Nil,
			repositoryURL: "https://github.com/user/repo.git",
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectError:   true,
		},
		{
			name:          "should reject empty repository URL",
			repositoryID:  uuid.New(),
			repositoryURL: "",
			cloneOptions:  valueobject.NewShallowCloneOptions(1, "main"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics, err := NewGitCloneMetrics(tt.repositoryID, tt.repositoryURL, tt.cloneOptions)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				if metrics != nil {
					t.Errorf("expected nil metrics but got %v", metrics)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics == nil {
				t.Errorf("expected GitCloneMetrics but got nil")
				return
			}

			// Validate initial state
			if metrics.RepositoryID() != tt.repositoryID {
				t.Errorf("expected repository ID %v but got %v", tt.repositoryID, metrics.RepositoryID())
			}
			if metrics.RepositoryURL() != tt.repositoryURL {
				t.Errorf("expected repository URL %s but got %s", tt.repositoryURL, metrics.RepositoryURL())
			}
			if !metrics.CloneOptions().Equal(tt.cloneOptions) {
				t.Errorf("expected clone options %v but got %v", tt.cloneOptions, metrics.CloneOptions())
			}
			if metrics.Status() != CloneMetricsStatusPending {
				t.Errorf("expected pending status but got %v", metrics.Status())
			}
		})
	}
}

func TestGitCloneMetrics_StartClone(t *testing.T) {
	metrics := mustCreateGitCloneMetrics()

	err := metrics.StartClone()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if metrics.Status() != CloneMetricsStatusInProgress {
		t.Errorf("expected in-progress status but got %v", metrics.Status())
	}

	if metrics.StartedAt() == nil {
		t.Errorf("expected started time to be set")
	}

	if time.Since(*metrics.StartedAt()) > time.Second {
		t.Errorf("started time seems too old: %v", metrics.StartedAt())
	}

	// Test that starting already started clone fails
	err = metrics.StartClone()
	if err == nil {
		t.Errorf("expected error when starting already started clone")
	}
}

func TestGitCloneMetrics_CompleteClone(t *testing.T) {
	tests := []struct {
		name           string
		setupStatus    func(*GitCloneMetrics)
		cloneDuration  time.Duration
		repositorySize int64
		fileCount      int
		expectError    bool
	}{
		{
			name: "should complete clone from in-progress status",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			cloneDuration:  5 * time.Second,
			repositorySize: 1024 * 1024, // 1MB
			fileCount:      100,
			expectError:    false,
		},
		{
			name: "should fail to complete clone from pending status",
			setupStatus: func(_ *GitCloneMetrics) {
				// Leave in pending status
			},
			cloneDuration:  2 * time.Second,
			repositorySize: 512 * 1024,
			fileCount:      50,
			expectError:    true,
		},
		{
			name: "should reject negative clone duration",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			cloneDuration:  -1 * time.Second,
			repositorySize: 1024 * 1024,
			fileCount:      100,
			expectError:    true,
		},
		{
			name: "should reject negative repository size",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			cloneDuration:  5 * time.Second,
			repositorySize: -1,
			fileCount:      100,
			expectError:    true,
		},
		{
			name: "should reject negative file count",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			cloneDuration:  5 * time.Second,
			repositorySize: 1024 * 1024,
			fileCount:      -1,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := mustCreateGitCloneMetrics()
			tt.setupStatus(metrics)

			err := metrics.CompleteClone(tt.cloneDuration, tt.repositorySize, tt.fileCount)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.Status() != CloneMetricsStatusCompleted {
				t.Errorf("expected completed status but got %v", metrics.Status())
			}
			if metrics.CloneDuration() != tt.cloneDuration {
				t.Errorf("expected clone duration %v but got %v", tt.cloneDuration, metrics.CloneDuration())
			}
			if metrics.RepositorySize() != tt.repositorySize {
				t.Errorf("expected repository size %d but got %d", tt.repositorySize, metrics.RepositorySize())
			}
			if metrics.FileCount() != tt.fileCount {
				t.Errorf("expected file count %d but got %d", tt.fileCount, metrics.FileCount())
			}
			if metrics.CompletedAt() == nil {
				t.Errorf("expected completed time to be set")
			}
		})
	}
}

func TestGitCloneMetrics_FailClone(t *testing.T) {
	tests := []struct {
		name        string
		setupStatus func(*GitCloneMetrics)
		errorMsg    string
		expectError bool
	}{
		{
			name: "should fail clone from in-progress status",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			errorMsg:    "network timeout",
			expectError: false,
		},
		{
			name: "should fail clone from pending status",
			setupStatus: func(_ *GitCloneMetrics) {
				// Leave in pending status
			},
			errorMsg:    "invalid repository URL",
			expectError: false,
		},
		{
			name: "should reject empty error message",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			errorMsg:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := mustCreateGitCloneMetrics()
			tt.setupStatus(metrics)

			err := metrics.FailClone(tt.errorMsg)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.Status() != CloneMetricsStatusFailed {
				t.Errorf("expected failed status but got %v", metrics.Status())
			}
			if metrics.ErrorMessage() == nil || *metrics.ErrorMessage() != tt.errorMsg {
				t.Errorf("expected error message %q but got %v", tt.errorMsg, metrics.ErrorMessage())
			}
			if metrics.CompletedAt() == nil {
				t.Errorf("expected completed time to be set")
			}
		})
	}
}

func TestGitCloneMetrics_UpdateProgress(t *testing.T) {
	tests := []struct {
		name           string
		setupStatus    func(*GitCloneMetrics)
		bytesReceived  int64
		totalBytes     int64
		filesProcessed int
		currentPhase   string
		expectError    bool
	}{
		{
			name: "should update progress during clone",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			bytesReceived:  512 * 1024,  // 512KB
			totalBytes:     1024 * 1024, // 1MB
			filesProcessed: 50,
			currentPhase:   "cloning",
			expectError:    false,
		},
		{
			name: "should update progress with zero values",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			bytesReceived:  0,
			totalBytes:     0,
			filesProcessed: 0,
			currentPhase:   "preparing",
			expectError:    false,
		},
		{
			name: "should fail to update progress on completed clone",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
				m.CompleteClone(5*time.Second, 1024*1024, 100)
			},
			bytesReceived:  512 * 1024,
			totalBytes:     1024 * 1024,
			filesProcessed: 50,
			currentPhase:   "cloning",
			expectError:    true,
		},
		{
			name: "should reject negative bytes received",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			bytesReceived:  -1,
			totalBytes:     1024 * 1024,
			filesProcessed: 50,
			currentPhase:   "cloning",
			expectError:    true,
		},
		{
			name: "should reject negative total bytes",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			bytesReceived:  512 * 1024,
			totalBytes:     -1,
			filesProcessed: 50,
			currentPhase:   "cloning",
			expectError:    true,
		},
		{
			name: "should reject negative files processed",
			setupStatus: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			bytesReceived:  512 * 1024,
			totalBytes:     1024 * 1024,
			filesProcessed: -1,
			currentPhase:   "cloning",
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := mustCreateGitCloneMetrics()
			tt.setupStatus(metrics)

			err := metrics.UpdateProgress(tt.bytesReceived, tt.totalBytes, tt.filesProcessed, tt.currentPhase)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if metrics.BytesReceived() != tt.bytesReceived {
				t.Errorf("expected bytes received %d but got %d", tt.bytesReceived, metrics.BytesReceived())
			}
			if metrics.TotalBytes() != tt.totalBytes {
				t.Errorf("expected total bytes %d but got %d", tt.totalBytes, metrics.TotalBytes())
			}
			if metrics.FilesProcessed() != tt.filesProcessed {
				t.Errorf("expected files processed %d but got %d", tt.filesProcessed, metrics.FilesProcessed())
			}
			if metrics.CurrentPhase() != tt.currentPhase {
				t.Errorf("expected current phase %q but got %q", tt.currentPhase, metrics.CurrentPhase())
			}
		})
	}
}

func TestGitCloneMetrics_CalculateEfficiency(t *testing.T) {
	tests := []struct {
		name          string
		setupMetrics  func(*GitCloneMetrics)
		expectedRatio float64
		expectError   bool
	}{
		{
			name: "should calculate efficiency for completed shallow clone",
			setupMetrics: func(m *GitCloneMetrics) {
				m.StartClone()
				m.CompleteClone(2*time.Second, 500*1024, 50) // 500KB, 50 files in 2 seconds
			},
			expectedRatio: 256000.0, // 500KB / 2s = 256000 bytes/s
			expectError:   false,
		},
		{
			name: "should calculate efficiency for completed full clone",
			setupMetrics: func(m *GitCloneMetrics) {
				m.StartClone()
				m.CompleteClone(10*time.Second, 5*1024*1024, 500) // 5MB, 500 files in 10 seconds
			},
			expectedRatio: 5242880.0 / 10.0, // 5MB / 10s = 524288 bytes/s
			expectError:   false,
		},
		{
			name: "should fail for incomplete clone",
			setupMetrics: func(m *GitCloneMetrics) {
				m.StartClone()
				// Don't complete the clone
			},
			expectedRatio: 0,
			expectError:   true,
		},
		{
			name: "should fail for failed clone",
			setupMetrics: func(m *GitCloneMetrics) {
				m.StartClone()
				m.FailClone("network error")
			},
			expectedRatio: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := mustCreateGitCloneMetrics()
			tt.setupMetrics(metrics)

			efficiency, err := metrics.CalculateEfficiency()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if efficiency.BytesPerSecond != tt.expectedRatio {
				t.Errorf("expected bytes per second %f but got %f", tt.expectedRatio, efficiency.BytesPerSecond)
			}

			// Validate other efficiency metrics
			if efficiency.FilesPerSecond <= 0 {
				t.Errorf("expected positive files per second")
			}
			if efficiency.CompressionRatio <= 0 {
				t.Errorf("expected positive compression ratio")
			}
		})
	}
}

func TestGitCloneMetrics_GetPerformanceInsights(t *testing.T) {
	tests := []struct {
		name         string
		setupMetrics func(*GitCloneMetrics)
		expectError  bool
	}{
		{
			name: "should provide insights for completed shallow clone",
			setupMetrics: func(m *GitCloneMetrics) {
				m.StartClone()
				m.CompleteClone(1*time.Second, 100*1024, 10) // Small, fast clone
			},
			expectError: false,
		},
		{
			name: "should provide insights for completed full clone",
			setupMetrics: func(m *GitCloneMetrics) {
				m.StartClone()
				m.CompleteClone(30*time.Second, 10*1024*1024, 1000) // Large, slow clone
			},
			expectError: false,
		},
		{
			name: "should fail for incomplete clone",
			setupMetrics: func(m *GitCloneMetrics) {
				m.StartClone()
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metrics := mustCreateGitCloneMetrics()
			tt.setupMetrics(metrics)

			insights, err := metrics.GetPerformanceInsights()

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if insights == nil {
				t.Errorf("expected PerformanceInsights but got nil")
				return
			}

			// Validate insights structure
			if insights.RecommendedStrategy == "" {
				t.Errorf("expected recommended strategy but got empty string")
			}
			if len(insights.OptimizationSuggestions) == 0 {
				t.Errorf("expected optimization suggestions")
			}
			if insights.PerformanceScore < 0 || insights.PerformanceScore > 100 {
				t.Errorf("expected performance score between 0 and 100 but got %f", insights.PerformanceScore)
			}
		})
	}
}

func TestGitCloneMetrics_CompareTo(t *testing.T) {
	baseMetrics := mustCreateGitCloneMetrics()
	baseMetrics.StartClone()
	baseMetrics.CompleteClone(5*time.Second, 1024*1024, 100)

	tests := []struct {
		name         string
		otherMetrics func() *GitCloneMetrics
		expectError  bool
	}{
		{
			name: "should compare with faster clone",
			otherMetrics: func() *GitCloneMetrics {
				m := mustCreateGitCloneMetrics()
				m.StartClone()
				m.CompleteClone(3*time.Second, 1024*1024, 100) // Same size, faster
				return m
			},
			expectError: false,
		},
		{
			name: "should compare with slower clone",
			otherMetrics: func() *GitCloneMetrics {
				m := mustCreateGitCloneMetrics()
				m.StartClone()
				m.CompleteClone(10*time.Second, 1024*1024, 100) // Same size, slower
				return m
			},
			expectError: false,
		},
		{
			name: "should fail with incomplete metrics",
			otherMetrics: func() *GitCloneMetrics {
				m := mustCreateGitCloneMetrics()
				m.StartClone() // Don't complete
				return m
			},
			expectError: true,
		},
		{
			name: "should fail with nil metrics",
			otherMetrics: func() *GitCloneMetrics {
				return nil
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			otherMetrics := tt.otherMetrics()

			comparison, err := baseMetrics.CompareTo(otherMetrics)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if comparison == nil {
				t.Errorf("expected MetricsComparison but got nil")
				return
			}

			// Validate comparison structure
			if comparison.SpeedRatio <= 0 {
				t.Errorf("expected positive speed ratio but got %f", comparison.SpeedRatio)
			}
			if comparison.Winner == "" {
				t.Errorf("expected winner but got empty string")
			}
			if len(comparison.Improvements) == 0 {
				t.Errorf("expected improvement suggestions")
			}
		})
	}
}

// Helper function to create a valid GitCloneMetrics for testing.
func mustCreateGitCloneMetrics() *GitCloneMetrics {
	metrics, err := NewGitCloneMetrics(
		uuid.New(),
		"https://github.com/user/repo.git",
		valueobject.NewShallowCloneOptions(1, "main"),
	)
	if err != nil {
		panic(err)
	}
	return metrics
}
