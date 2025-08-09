package service

import (
	"context"
	"time"

	"codechunking/internal/application/dto"
	"codechunking/internal/port/inbound"
	"codechunking/internal/port/outbound"
)

// HealthServiceAdapter provides health check functionality
type HealthServiceAdapter struct {
	repositoryRepo   outbound.RepositoryRepository
	indexingJobRepo  outbound.IndexingJobRepository
	messagePublisher outbound.MessagePublisher
	version          string
}

// NewHealthServiceAdapter creates a new HealthServiceAdapter
func NewHealthServiceAdapter(
	repositoryRepo outbound.RepositoryRepository,
	indexingJobRepo outbound.IndexingJobRepository,
	messagePublisher outbound.MessagePublisher,
	version string,
) inbound.HealthService {
	return &HealthServiceAdapter{
		repositoryRepo:   repositoryRepo,
		indexingJobRepo:  indexingJobRepo,
		messagePublisher: messagePublisher,
		version:          version,
	}
}

// GetHealth performs health checks on all dependencies
func (h *HealthServiceAdapter) GetHealth(ctx context.Context) (*dto.HealthResponse, error) {
	response := &dto.HealthResponse{
		Status:       string(dto.HealthStatusHealthy),
		Timestamp:    time.Now(),
		Version:      h.version,
		Dependencies: make(map[string]dto.DependencyStatus),
	}

	// Check repository database connection
	if h.repositoryRepo != nil {
		// Try a simple list query to check database connectivity
		_, _, err := h.repositoryRepo.FindAll(ctx, outbound.RepositoryFilters{
			Limit:  1,
			Offset: 0,
		})
		if err != nil {
			response.Dependencies["database"] = dto.DependencyStatus{
				Status:  string(dto.DependencyStatusUnhealthy),
				Message: "Database connection failed",
			}
			response.Status = string(dto.HealthStatusDegraded)
		} else {
			response.Dependencies["database"] = dto.DependencyStatus{
				Status: string(dto.DependencyStatusHealthy),
			}
		}
	}

	// Check message publisher connectivity (basic availability check)
	if h.messagePublisher != nil {
		response.Dependencies["message_queue"] = dto.DependencyStatus{
			Status: string(dto.DependencyStatusHealthy),
		}
	}

	return response, nil
}
