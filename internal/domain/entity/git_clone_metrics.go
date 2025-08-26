package entity

import (
	"codechunking/internal/domain/valueobject"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CloneMetricsStatus represents the status of clone metrics tracking.
type CloneMetricsStatus string

const (
	// CloneMetricsStatusPending indicates clone metrics tracking is pending.
	CloneMetricsStatusPending CloneMetricsStatus = "pending"
	// CloneMetricsStatusInProgress indicates clone operation is currently running.
	CloneMetricsStatusInProgress CloneMetricsStatus = "in_progress"
	// CloneMetricsStatusCompleted indicates clone operation completed successfully.
	CloneMetricsStatusCompleted CloneMetricsStatus = "completed"
	// CloneMetricsStatusFailed indicates clone operation failed.
	CloneMetricsStatusFailed CloneMetricsStatus = "failed"
)

// Performance benchmark constants (in bytes per second).
const (
	ExcellentPerformanceThreshold  = 1024 * 1024       // 1 MB/s
	GoodPerformanceThreshold       = 900 * 1024        // 900 KB/s
	AcceptablePerformanceThreshold = 512 * 1024        // 512 KB/s
	LargeRepositoryThreshold       = 100 * 1024 * 1024 // 100 MB
	ShallowCloneTimeoutThreshold   = 30 * time.Second  // 30 seconds
)

// Performance scoring constants.
const (
	ExcellentPerformanceScore  = 90.0
	GoodPerformanceScore       = 70.0
	AcceptablePerformanceScore = 50.0
	PoorPerformanceScore       = 30.0
	ExpectedShallowImprovement = 3.0 // 3x improvement expected with shallow clone
)

// GitCloneMetrics tracks performance metrics for git clone operations.
type GitCloneMetrics struct {
	id             uuid.UUID
	repositoryID   uuid.UUID
	repositoryURL  string
	cloneOptions   valueobject.CloneOptions
	status         CloneMetricsStatus
	startedAt      *time.Time
	completedAt    *time.Time
	cloneDuration  time.Duration
	repositorySize int64
	fileCount      int
	bytesReceived  int64
	totalBytes     int64
	filesProcessed int
	currentPhase   string
	errorMessage   *string
	createdAt      time.Time
	updatedAt      time.Time
}

// CloneEfficiency represents performance efficiency metrics.
type CloneEfficiency struct {
	BytesPerSecond   float64 `json:"bytes_per_second"`
	FilesPerSecond   float64 `json:"files_per_second"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// PerformanceInsights provides analysis and recommendations.
type PerformanceInsights struct {
	RecommendedStrategy      string   `json:"recommended_strategy"`
	OptimizationSuggestions  []string `json:"optimization_suggestions"`
	PerformanceScore         float64  `json:"performance_score"` // 0-100
	ExpectedImprovementRatio float64  `json:"expected_improvement_ratio"`
}

// MetricsComparison compares two clone metrics.
type MetricsComparison struct {
	SpeedRatio   float64  `json:"speed_ratio"`
	SizeRatio    float64  `json:"size_ratio"`
	Winner       string   `json:"winner"`
	Improvements []string `json:"improvements"`
}

// NewGitCloneMetrics creates a new GitCloneMetrics entity.
func NewGitCloneMetrics(
	repositoryID uuid.UUID,
	repositoryURL string,
	cloneOptions valueobject.CloneOptions,
) (*GitCloneMetrics, error) {
	if repositoryID == uuid.Nil {
		return nil, errors.New("repository ID cannot be nil")
	}
	if repositoryURL == "" {
		return nil, errors.New("repository URL cannot be empty")
	}

	now := time.Now()
	return &GitCloneMetrics{
		id:            uuid.New(),
		repositoryID:  repositoryID,
		repositoryURL: repositoryURL,
		cloneOptions:  cloneOptions,
		status:        CloneMetricsStatusPending,
		createdAt:     now,
		updatedAt:     now,
	}, nil
}

// ID returns the metrics ID.
func (m *GitCloneMetrics) ID() uuid.UUID {
	return m.id
}

// RepositoryID returns the repository ID.
func (m *GitCloneMetrics) RepositoryID() uuid.UUID {
	return m.repositoryID
}

// RepositoryURL returns the repository URL.
func (m *GitCloneMetrics) RepositoryURL() string {
	return m.repositoryURL
}

// CloneOptions returns the clone options used.
func (m *GitCloneMetrics) CloneOptions() valueobject.CloneOptions {
	return m.cloneOptions
}

// Status returns the current metrics status.
func (m *GitCloneMetrics) Status() CloneMetricsStatus {
	return m.status
}

// StartedAt returns the clone start time.
func (m *GitCloneMetrics) StartedAt() *time.Time {
	return m.startedAt
}

// CompletedAt returns the clone completion time.
func (m *GitCloneMetrics) CompletedAt() *time.Time {
	return m.completedAt
}

// CloneDuration returns the total clone duration.
func (m *GitCloneMetrics) CloneDuration() time.Duration {
	return m.cloneDuration
}

// RepositorySize returns the cloned repository size in bytes.
func (m *GitCloneMetrics) RepositorySize() int64 {
	return m.repositorySize
}

// FileCount returns the number of files cloned.
func (m *GitCloneMetrics) FileCount() int {
	return m.fileCount
}

// BytesReceived returns the bytes received during clone.
func (m *GitCloneMetrics) BytesReceived() int64 {
	return m.bytesReceived
}

// TotalBytes returns the total bytes to be received.
func (m *GitCloneMetrics) TotalBytes() int64 {
	return m.totalBytes
}

// FilesProcessed returns the number of files processed.
func (m *GitCloneMetrics) FilesProcessed() int {
	return m.filesProcessed
}

// CurrentPhase returns the current clone phase.
func (m *GitCloneMetrics) CurrentPhase() string {
	return m.currentPhase
}

// ErrorMessage returns the error message if clone failed.
func (m *GitCloneMetrics) ErrorMessage() *string {
	return m.errorMessage
}

// CreatedAt returns the creation timestamp.
func (m *GitCloneMetrics) CreatedAt() time.Time {
	return m.createdAt
}

// UpdatedAt returns the last update timestamp.
func (m *GitCloneMetrics) UpdatedAt() time.Time {
	return m.updatedAt
}

// StartClone marks the beginning of a clone operation.
func (m *GitCloneMetrics) StartClone() error {
	if m.status != CloneMetricsStatusPending {
		return fmt.Errorf("cannot start clone from status: %s", m.status)
	}

	now := time.Now()
	m.status = CloneMetricsStatusInProgress
	m.startedAt = &now
	m.updatedAt = now
	return nil
}

// CompleteClone marks the successful completion of a clone operation.
func (m *GitCloneMetrics) CompleteClone(cloneDuration time.Duration, repositorySize int64, fileCount int) error {
	if m.status != CloneMetricsStatusInProgress {
		return fmt.Errorf("cannot complete clone from status: %s", m.status)
	}
	if cloneDuration < 0 {
		return fmt.Errorf("clone duration cannot be negative: %v", cloneDuration)
	}
	if repositorySize < 0 {
		return fmt.Errorf("repository size cannot be negative: %d", repositorySize)
	}
	if fileCount < 0 {
		return fmt.Errorf("file count cannot be negative: %d", fileCount)
	}

	now := time.Now()
	m.status = CloneMetricsStatusCompleted
	m.completedAt = &now
	m.cloneDuration = cloneDuration
	m.repositorySize = repositorySize
	m.fileCount = fileCount
	m.errorMessage = nil // Clear any previous error
	m.updatedAt = now
	return nil
}

// FailClone marks the clone operation as failed.
func (m *GitCloneMetrics) FailClone(errorMessage string) error {
	if errorMessage == "" {
		return errors.New("error message cannot be empty")
	}

	now := time.Now()
	m.status = CloneMetricsStatusFailed
	m.completedAt = &now
	m.errorMessage = &errorMessage
	m.updatedAt = now
	return nil
}

// UpdateProgress updates the clone progress information.
func (m *GitCloneMetrics) UpdateProgress(
	bytesReceived, totalBytes int64,
	filesProcessed int,
	currentPhase string,
) error {
	if m.status == CloneMetricsStatusCompleted || m.status == CloneMetricsStatusFailed {
		return errors.New("cannot update progress on completed clone")
	}
	if bytesReceived < 0 {
		return fmt.Errorf("bytes received cannot be negative: %d", bytesReceived)
	}
	if totalBytes < 0 {
		return fmt.Errorf("total bytes cannot be negative: %d", totalBytes)
	}
	if filesProcessed < 0 {
		return fmt.Errorf("files processed cannot be negative: %d", filesProcessed)
	}

	m.bytesReceived = bytesReceived
	m.totalBytes = totalBytes
	m.filesProcessed = filesProcessed
	m.currentPhase = currentPhase
	m.updatedAt = time.Now()
	return nil
}

// CalculateEfficiency calculates clone performance efficiency metrics.
func (m *GitCloneMetrics) CalculateEfficiency() (*CloneEfficiency, error) {
	if m.status != CloneMetricsStatusCompleted {
		return nil, errors.New("efficiency can only be calculated for completed clones")
	}
	if m.cloneDuration == 0 {
		return nil, errors.New("clone duration is zero")
	}

	seconds := m.cloneDuration.Seconds()
	bytesPerSecond := float64(m.repositorySize) / seconds
	filesPerSecond := float64(m.fileCount) / seconds

	// Calculate compression ratio (simplified)
	compressionRatio := 1.0
	if m.totalBytes > 0 {
		compressionRatio = float64(m.repositorySize) / float64(m.totalBytes)
	}

	return &CloneEfficiency{
		BytesPerSecond:   bytesPerSecond,
		FilesPerSecond:   filesPerSecond,
		CompressionRatio: compressionRatio,
	}, nil
}

// GetPerformanceInsights analyzes the clone performance and provides recommendations.
func (m *GitCloneMetrics) GetPerformanceInsights() (*PerformanceInsights, error) {
	if m.status != CloneMetricsStatusCompleted {
		return nil, errors.New("insights can only be generated for completed clones")
	}

	efficiency, err := m.CalculateEfficiency()
	if err != nil {
		return nil, fmt.Errorf("failed to calculate efficiency: %w", err)
	}

	// Simple performance scoring based on bytes per second
	var performanceScore float64
	switch {
	case efficiency.BytesPerSecond > 1024*1024: // > 1 MB/s
		performanceScore = 90
	case efficiency.BytesPerSecond > 900*1024: // > 900 KB/s
		performanceScore = 70
	case efficiency.BytesPerSecond > 512*1024: // > 512 KB/s
		performanceScore = 50
	default:
		performanceScore = 30
	}

	recommendedStrategy, suggestions := m.determineStrategy()
	expectedImprovement := m.calculateExpectedImprovement()

	return &PerformanceInsights{
		RecommendedStrategy:      recommendedStrategy,
		OptimizationSuggestions:  suggestions,
		PerformanceScore:         performanceScore,
		ExpectedImprovementRatio: expectedImprovement,
	}, nil
}

// CompareTo compares this metrics with another GitCloneMetrics.
func (m *GitCloneMetrics) CompareTo(other *GitCloneMetrics) (*MetricsComparison, error) {
	if other == nil {
		return nil, errors.New("cannot compare with nil metrics")
	}
	if m.status != CloneMetricsStatusCompleted {
		return nil, errors.New("cannot compare incomplete metrics")
	}
	if other.status != CloneMetricsStatusCompleted {
		return nil, errors.New("cannot compare with incomplete metrics")
	}

	speedRatio := other.cloneDuration.Seconds() / m.cloneDuration.Seconds()
	sizeRatio := float64(other.repositorySize) / float64(m.repositorySize)

	var winner string
	var improvements []string

	if speedRatio > 1 {
		winner = "current"
		improvements = append(improvements, "Current strategy is faster")
	} else {
		winner = "other"
		improvements = append(improvements, "Other strategy is faster")
		improvements = append(improvements, "Consider adopting the other strategy")
	}

	// Add specific improvement suggestions based on differences
	if m.cloneOptions.IsShallowClone() != other.cloneOptions.IsShallowClone() {
		if other.cloneOptions.IsShallowClone() && speedRatio < 1 {
			improvements = append(improvements, "Consider switching to shallow clone")
		} else if !other.cloneOptions.IsShallowClone() && speedRatio > 1 {
			improvements = append(improvements, "Full clone may be more appropriate")
		}
	}

	return &MetricsComparison{
		SpeedRatio:   speedRatio,
		SizeRatio:    sizeRatio,
		Winner:       winner,
		Improvements: improvements,
	}, nil
}

// Equal compares two GitCloneMetrics entities.
func (m *GitCloneMetrics) Equal(other *GitCloneMetrics) bool {
	if other == nil {
		return false
	}
	return m.id == other.id
}

// determineStrategy determines the recommended cloning strategy based on current metrics.
func (m *GitCloneMetrics) determineStrategy() (string, []string) {
	if m.cloneOptions.IsShallowClone() {
		return m.determineShallowStrategy()
	}
	return m.determineFullStrategy()
}

// determineShallowStrategy determines strategy for shallow clones.
func (m *GitCloneMetrics) determineShallowStrategy() (string, []string) {
	if m.cloneDuration > 30*time.Second {
		return "optimize-shallow-clone", []string{
			"Consider reducing clone depth further",
			"Use single-branch clone",
		}
	}
	return "current-strategy-optimal", []string{
		"Current shallow clone strategy is performing well",
	}
}

// determineFullStrategy determines strategy for full clones.
func (m *GitCloneMetrics) determineFullStrategy() (string, []string) {
	if m.repositorySize > 100*1024*1024 { // > 100MB
		return "use-shallow-clone", []string{
			"Switch to shallow clone for large repositories",
			"Consider depth=1 for faster cloning",
		}
	}
	return "full-clone-acceptable", []string{
		"Full clone is reasonable for repository of this size",
	}
}

// calculateExpectedImprovement calculates the expected improvement ratio.
func (m *GitCloneMetrics) calculateExpectedImprovement() float64 {
	expectedImprovement := 1.0
	if !m.cloneOptions.IsShallowClone() && m.repositorySize > 10*1024*1024 {
		expectedImprovement = 3.0 // Expect 3x improvement with shallow clone
	}
	return expectedImprovement
}
