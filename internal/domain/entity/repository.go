package entity

import (
	"time"

	"codechunking/internal/domain/valueobject"

	"github.com/google/uuid"
)

// Repository represents a Git repository in the system
type Repository struct {
	id             uuid.UUID
	url            valueobject.RepositoryURL
	name           string
	description    *string
	defaultBranch  *string
	lastIndexedAt  *time.Time
	lastCommitHash *string
	totalFiles     int
	totalChunks    int
	status         valueobject.RepositoryStatus
	createdAt      time.Time
	updatedAt      time.Time
	deletedAt      *time.Time
}

// NewRepository creates a new Repository entity
func NewRepository(
	url valueobject.RepositoryURL,
	name string,
	description *string,
	defaultBranch *string,
) *Repository {
	now := time.Now()
	return &Repository{
		id:            uuid.New(),
		url:           url,
		name:          name,
		description:   description,
		defaultBranch: defaultBranch,
		totalFiles:    0,
		totalChunks:   0,
		status:        valueobject.RepositoryStatusPending,
		createdAt:     now,
		updatedAt:     now,
	}
}

// RestoreRepository creates a Repository entity from stored data
func RestoreRepository(
	id uuid.UUID,
	url valueobject.RepositoryURL,
	name string,
	description *string,
	defaultBranch *string,
	lastIndexedAt *time.Time,
	lastCommitHash *string,
	totalFiles int,
	totalChunks int,
	status valueobject.RepositoryStatus,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) *Repository {
	return &Repository{
		id:             id,
		url:            url,
		name:           name,
		description:    description,
		defaultBranch:  defaultBranch,
		lastIndexedAt:  lastIndexedAt,
		lastCommitHash: lastCommitHash,
		totalFiles:     totalFiles,
		totalChunks:    totalChunks,
		status:         status,
		createdAt:      createdAt,
		updatedAt:      updatedAt,
		deletedAt:      deletedAt,
	}
}

// ID returns the repository ID
func (r *Repository) ID() uuid.UUID {
	return r.id
}

// URL returns the repository URL
func (r *Repository) URL() valueobject.RepositoryURL {
	return r.url
}

// Name returns the repository name
func (r *Repository) Name() string {
	return r.name
}

// Description returns the repository description
func (r *Repository) Description() *string {
	return r.description
}

// DefaultBranch returns the default branch
func (r *Repository) DefaultBranch() *string {
	return r.defaultBranch
}

// LastIndexedAt returns the last indexed timestamp
func (r *Repository) LastIndexedAt() *time.Time {
	return r.lastIndexedAt
}

// LastCommitHash returns the last indexed commit hash
func (r *Repository) LastCommitHash() *string {
	return r.lastCommitHash
}

// TotalFiles returns the total number of indexed files
func (r *Repository) TotalFiles() int {
	return r.totalFiles
}

// TotalChunks returns the total number of generated chunks
func (r *Repository) TotalChunks() int {
	return r.totalChunks
}

// Status returns the current repository status
func (r *Repository) Status() valueobject.RepositoryStatus {
	return r.status
}

// CreatedAt returns the creation timestamp
func (r *Repository) CreatedAt() time.Time {
	return r.createdAt
}

// UpdatedAt returns the last update timestamp
func (r *Repository) UpdatedAt() time.Time {
	return r.updatedAt
}

// DeletedAt returns the deletion timestamp
func (r *Repository) DeletedAt() *time.Time {
	return r.deletedAt
}

// IsDeleted returns true if the repository is soft-deleted
func (r *Repository) IsDeleted() bool {
	return r.deletedAt != nil
}

// UpdateName updates the repository name
func (r *Repository) UpdateName(name string) {
	r.name = name
	r.updatedAt = time.Now()
}

// UpdateDescription updates the repository description
func (r *Repository) UpdateDescription(description *string) {
	r.description = description
	r.updatedAt = time.Now()
}

// UpdateDefaultBranch updates the default branch
func (r *Repository) UpdateDefaultBranch(branch *string) {
	r.defaultBranch = branch
	r.updatedAt = time.Now()
}

// UpdateStatus updates the repository status if the transition is valid
func (r *Repository) UpdateStatus(newStatus valueobject.RepositoryStatus) error {
	if !r.status.CanTransitionTo(newStatus) {
		return NewDomainError("invalid status transition", "INVALID_STATUS_TRANSITION")
	}
	r.status = newStatus
	r.updatedAt = time.Now()
	return nil
}

// MarkIndexingCompleted marks the repository as successfully indexed
func (r *Repository) MarkIndexingCompleted(commitHash string, totalFiles, totalChunks int) error {
	if err := r.UpdateStatus(valueobject.RepositoryStatusCompleted); err != nil {
		return err
	}
	now := time.Now()
	r.lastIndexedAt = &now
	r.lastCommitHash = &commitHash
	r.totalFiles = totalFiles
	r.totalChunks = totalChunks
	r.updatedAt = now
	return nil
}

// MarkIndexingFailed marks the repository indexing as failed
func (r *Repository) MarkIndexingFailed() error {
	return r.UpdateStatus(valueobject.RepositoryStatusFailed)
}

// Archive soft-deletes the repository
func (r *Repository) Archive() error {
	if r.IsDeleted() {
		return NewDomainError("repository is already archived", "ALREADY_ARCHIVED")
	}

	// Archive is a special administrative action that can be done from any status
	// except when already archived
	r.status = valueobject.RepositoryStatusArchived
	now := time.Now()
	r.deletedAt = &now
	r.updatedAt = now
	return nil
}

// Restore undeletes the repository
func (r *Repository) Restore() error {
	if !r.IsDeleted() {
		return NewDomainError("repository is not archived", "NOT_ARCHIVED")
	}

	if err := r.UpdateStatus(valueobject.RepositoryStatusPending); err != nil {
		return err
	}

	r.deletedAt = nil
	r.updatedAt = time.Now()
	return nil
}

// CanBeDeleted returns true if the repository can be deleted
func (r *Repository) CanBeDeleted() bool {
	// Cannot delete if currently being processed
	return r.status != valueobject.RepositoryStatusCloning &&
		r.status != valueobject.RepositoryStatusProcessing
}

// Equal compares two Repository entities
func (r *Repository) Equal(other *Repository) bool {
	if other == nil {
		return false
	}
	return r.id == other.id
}
