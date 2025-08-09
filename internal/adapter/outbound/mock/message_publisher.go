package mock

import (
	"context"
	"log"

	"codechunking/internal/port/outbound"

	"github.com/google/uuid"
)

// MockMessagePublisher provides a mock implementation of MessagePublisher for development
type MockMessagePublisher struct {
	// Published jobs for testing/verification
	publishedJobs []PublishedJob
}

// PublishedJob represents a job that was published
type PublishedJob struct {
	RepositoryID  uuid.UUID
	RepositoryURL string
}

// NewMockMessagePublisher creates a new mock message publisher
func NewMockMessagePublisher() outbound.MessagePublisher {
	return &MockMessagePublisher{
		publishedJobs: make([]PublishedJob, 0),
	}
}

// PublishIndexingJob publishes an indexing job message (mock implementation)
func (m *MockMessagePublisher) PublishIndexingJob(ctx context.Context, repositoryID uuid.UUID, repositoryURL string) error {
	// Log the job publication for development
	log.Printf("Mock: Publishing indexing job for repository %s (ID: %s)", repositoryURL, repositoryID)

	// Store for potential testing/verification
	m.publishedJobs = append(m.publishedJobs, PublishedJob{
		RepositoryID:  repositoryID,
		RepositoryURL: repositoryURL,
	})

	return nil
}

// GetPublishedJobs returns all published jobs (for testing)
func (m *MockMessagePublisher) GetPublishedJobs() []PublishedJob {
	return m.publishedJobs
}

// Reset clears all published jobs
func (m *MockMessagePublisher) Reset() {
	m.publishedJobs = make([]PublishedJob, 0)
}
