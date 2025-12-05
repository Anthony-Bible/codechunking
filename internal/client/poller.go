package client

import (
	"codechunking/internal/application/dto"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultPollInterval is the default time to wait between status checks
	// when polling for repository indexing completion.
	DefaultPollInterval = 5 * time.Second

	// DefaultMaxWait is the default maximum time to wait for repository
	// indexing to complete before timing out.
	DefaultMaxWait = 30 * time.Minute
)

// Error messages for polling operations.
const (
	errMsgPollingFailed  = "repository indexing failed"
	errMsgPollingTimeout = "polling timeout exceeded"
)

// Progress status constants for JSON output.
const (
	progressStatusPolling = "polling"
)

// PollerConfig configures the behavior of a Poller.
// Zero values for fields will use defaults from DefaultPollInterval and DefaultMaxWait.
type PollerConfig struct {
	// Interval is the duration to wait between consecutive status polls.
	// Default: 5 seconds (DefaultPollInterval).
	Interval time.Duration

	// MaxWait is the maximum total duration to wait for completion.
	// Default: 30 minutes (DefaultMaxWait).
	MaxWait time.Duration
}

// Poller polls for repository indexing job completion.
// It periodically checks the status of a repository until it reaches
// a terminal state (completed, failed, or archived) or times out.
type Poller struct {
	client   *Client
	interval time.Duration
	maxWait  time.Duration
}

// NewPoller creates a new Poller with the given client and configuration.
// If config is nil or has zero values, defaults are used from DefaultPollInterval
// and DefaultMaxWait.
//
// Returns an error if client is nil.
func NewPoller(client *Client, config *PollerConfig) (*Poller, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}

	interval := DefaultPollInterval
	maxWait := DefaultMaxWait

	if config != nil {
		if config.Interval > 0 {
			interval = config.Interval
		}
		if config.MaxWait > 0 {
			maxWait = config.MaxWait
		}
	}

	return &Poller{
		client:   client,
		interval: interval,
		maxWait:  maxWait,
	}, nil
}

// WaitForCompletion polls for repository indexing completion.
// It returns when the repository reaches a terminal status (completed, failed, or archived),
// the context is cancelled, or the maximum wait time is exceeded.
//
// Progress updates are written as JSON to progressWriter in the format:
//
//	{
//	  "status": "polling",
//	  "repository_id": "uuid",
//	  "current_status": "processing",
//	  "elapsed": "5s",
//	  "poll_count": 1
//	}
//
// Returns the final repository state and an error if:
//   - The repository status is "failed" (error: "repository indexing failed")
//   - Polling exceeds maxWait duration (error: "polling timeout exceeded")
//   - The context is cancelled (error wraps context.Err())
//   - Network errors occur repeatedly until timeout
func (p *Poller) WaitForCompletion(
	ctx context.Context,
	repoID uuid.UUID,
	progressWriter io.Writer,
) (*dto.RepositoryResponse, error) {
	startTime := time.Now()
	pollCount := 0
	var lastRepo *dto.RepositoryResponse

	for {
		repo, err := p.client.GetRepository(ctx, repoID)
		if err != nil {
			if err := p.handlePollingError(ctx, &pollCount, startTime, lastRepo, repoID, progressWriter); err != nil {
				return lastRepo, err
			}
			continue
		}

		lastRepo = repo

		if IsTerminalStatus(repo.Status) {
			if repo.Status == dto.StatusFailed {
				return repo, errors.New(errMsgPollingFailed)
			}
			if pollCount == 0 {
				return repo, nil
			}
			return repo, nil
		}

		pollCount++
		elapsed := time.Since(startTime)

		progress := map[string]interface{}{
			"status":         progressStatusPolling,
			"repository_id":  repoID.String(),
			"current_status": repo.Status,
			"elapsed":        elapsed.String(),
			"poll_count":     pollCount,
		}
		_ = json.NewEncoder(progressWriter).Encode(progress)

		if elapsed >= p.maxWait {
			return repo, errors.New(errMsgPollingTimeout)
		}

		select {
		case <-ctx.Done():
			return repo, fmt.Errorf("context cancelled: %w", ctx.Err())
		case <-time.After(p.interval):
		}
	}
}

// handlePollingError processes errors during polling and determines whether to retry.
// It increments the poll count, writes progress updates, and checks for timeout conditions.
// Returns an error if the context is cancelled or the maximum wait time is exceeded.
func (p *Poller) handlePollingError(
	ctx context.Context,
	pollCount *int,
	startTime time.Time,
	lastRepo *dto.RepositoryResponse,
	repoID uuid.UUID,
	progressWriter io.Writer,
) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled: %w", ctx.Err())
	default:
	}

	*pollCount++
	elapsed := time.Since(startTime)

	progress := map[string]interface{}{
		"status":        progressStatusPolling,
		"repository_id": repoID.String(),
		"elapsed":       elapsed.String(),
		"poll_count":    *pollCount,
	}
	if lastRepo != nil {
		progress["current_status"] = lastRepo.Status
	}
	_ = json.NewEncoder(progressWriter).Encode(progress)

	if elapsed >= p.maxWait {
		return errors.New(errMsgPollingTimeout)
	}

	time.Sleep(p.interval)
	return nil
}

// IsTerminalStatus returns true if the given status represents a terminal state
// for repository indexing (completed, failed, or archived).
func IsTerminalStatus(status string) bool {
	return status == dto.StatusCompleted || status == dto.StatusFailed || status == dto.StatusArchived
}
