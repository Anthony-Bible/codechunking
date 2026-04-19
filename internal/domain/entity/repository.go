package entity

import (
	"codechunking/internal/domain/valueobject"
	"time"

	"github.com/google/uuid"
)

// Repository represents a Git repository in the system.
type Repository struct {
	id                   uuid.UUID
	url                  valueobject.RepositoryURL
	name                 string
	description          *string
	defaultBranch        *string
	lastIndexedAt        *time.Time
	lastCommitHash       *string
	totalFiles           int
	totalChunks          int
	status               valueobject.RepositoryStatus
	zoektIndexStatus     valueobject.ZoektIndexStatus
	embeddingIndexStatus valueobject.EmbeddingIndexStatus
	zoektLastIndexedAt   *time.Time
	zoektShardCount      int
	zoektCommitHash      *string
	createdAt            time.Time
	updatedAt            time.Time
	deletedAt            *time.Time
}

// NewRepository creates a new Repository entity.
func NewRepository(
	url valueobject.RepositoryURL,
	name string,
	description *string,
	defaultBranch *string,
) *Repository {
	now := time.Now()
	return &Repository{
		id:                   uuid.New(),
		url:                  url,
		name:                 name,
		description:          description,
		defaultBranch:        defaultBranch,
		totalFiles:           0,
		totalChunks:          0,
		status:               valueobject.RepositoryStatusPending,
		zoektIndexStatus:     valueobject.ZoektIndexStatusPending,
		embeddingIndexStatus: valueobject.EmbeddingIndexStatusPending,
		zoektShardCount:      0,
		createdAt:            now,
		updatedAt:            now,
	}
}

// RestoreRepository creates a Repository entity from stored data.
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
	zoektIndexStatus valueobject.ZoektIndexStatus,
	embeddingIndexStatus valueobject.EmbeddingIndexStatus,
	zoektLastIndexedAt *time.Time,
	zoektShardCount int,
	zoektCommitHash *string,
	createdAt time.Time,
	updatedAt time.Time,
	deletedAt *time.Time,
) *Repository {
	return &Repository{
		id:                   id,
		url:                  url,
		name:                 name,
		description:          description,
		defaultBranch:        defaultBranch,
		lastIndexedAt:        lastIndexedAt,
		lastCommitHash:       lastCommitHash,
		totalFiles:           totalFiles,
		totalChunks:          totalChunks,
		status:               status,
		zoektIndexStatus:     zoektIndexStatus,
		embeddingIndexStatus: embeddingIndexStatus,
		zoektLastIndexedAt:   zoektLastIndexedAt,
		zoektShardCount:      zoektShardCount,
		zoektCommitHash:      zoektCommitHash,
		createdAt:            createdAt,
		updatedAt:            updatedAt,
		deletedAt:            deletedAt,
	}
}

// ZoektIndexStatus returns the current Zoekt index status.
func (r *Repository) ZoektIndexStatus() valueobject.ZoektIndexStatus {
	return r.zoektIndexStatus
}

// EmbeddingIndexStatus returns the current embedding index status.
func (r *Repository) EmbeddingIndexStatus() valueobject.EmbeddingIndexStatus {
	return r.embeddingIndexStatus
}

// ZoektLastIndexedAt returns the last Zoekt indexed timestamp.
func (r *Repository) ZoektLastIndexedAt() *time.Time {
	return r.zoektLastIndexedAt
}

// ZoektShardCount returns the number of Zoekt shards.
func (r *Repository) ZoektShardCount() int {
	return r.zoektShardCount
}

// ZoektCommitHash returns the Zoekt commit hash.
func (r *Repository) ZoektCommitHash() *string {
	return r.zoektCommitHash
}

// UpdateZoektStatus updates the Zoekt index status and related metadata.
// Returns a DomainError if the transition is invalid.
func (r *Repository) UpdateZoektStatus(status valueobject.ZoektIndexStatus, shardCount int, commitHash *string) error {
	if !r.zoektIndexStatus.CanTransitionTo(status) {
		return NewDomainError("invalid zoekt status transition", "INVALID_ZOEKT_STATUS_TRANSITION")
	}
	r.zoektIndexStatus = status
	r.zoektShardCount = shardCount
	r.zoektCommitHash = commitHash
	if status == valueobject.ZoektIndexStatusCompleted ||
		status == valueobject.ZoektIndexStatusPartial ||
		status == valueobject.ZoektIndexStatusFailed {
		now := time.Now()
		r.zoektLastIndexedAt = &now
	}
	r.updatedAt = time.Now()
	return nil
}

// UpdateEmbeddingStatus updates the embedding index status.
// Returns a DomainError if the transition is invalid.
func (r *Repository) UpdateEmbeddingStatus(status valueobject.EmbeddingIndexStatus) error {
	if !r.embeddingIndexStatus.CanTransitionTo(status) {
		return NewDomainError("invalid embedding status transition", "INVALID_EMBEDDING_STATUS_TRANSITION")
	}
	r.embeddingIndexStatus = status
	r.updatedAt = time.Now()
	return nil
}

// IsFullyIndexed returns true if both Zoekt and embedding engines are fully indexed.
func (r *Repository) IsFullyIndexed() bool {
	return r.zoektIndexStatus == valueobject.ZoektIndexStatusCompleted &&
		r.embeddingIndexStatus == valueobject.EmbeddingIndexStatusCompleted
}

// IsPartiallyIndexed returns true if at least one engine is indexed but not both.
func (r *Repository) IsPartiallyIndexed() bool {
	zoektIndexed := r.zoektIndexStatus == valueobject.ZoektIndexStatusCompleted ||
		r.zoektIndexStatus == valueobject.ZoektIndexStatusPartial
	embeddingIndexed := r.embeddingIndexStatus == valueobject.EmbeddingIndexStatusCompleted ||
		r.embeddingIndexStatus == valueobject.EmbeddingIndexStatusPartial
	return (zoektIndexed != embeddingIndexed) && (zoektIndexed || embeddingIndexed)
}

// AvailableEngines returns a list of available search engines based on status.
func (r *Repository) AvailableEngines() []string {
	engines := make([]string, 0)
	if r.zoektIndexStatus == valueobject.ZoektIndexStatusCompleted ||
		r.zoektIndexStatus == valueobject.ZoektIndexStatusPartial {
		engines = append(engines, "zoekt")
	}
	if r.embeddingIndexStatus == valueobject.EmbeddingIndexStatusCompleted ||
		r.embeddingIndexStatus == valueobject.EmbeddingIndexStatusPartial {
		engines = append(engines, "embedding")
	}
	return engines
}

// ID returns the repository ID.
func (r *Repository) ID() uuid.UUID {
	return r.id
}

// URL returns the repository URL.
func (r *Repository) URL() valueobject.RepositoryURL {
	return r.url
}

// Name returns the repository name.
func (r *Repository) Name() string {
	return r.name
}

// Description returns the repository description.
func (r *Repository) Description() *string {
	return r.description
}

// DefaultBranch returns the default branch.
func (r *Repository) DefaultBranch() *string {
	return r.defaultBranch
}

// LastIndexedAt returns the last indexed timestamp.
func (r *Repository) LastIndexedAt() *time.Time {
	return r.lastIndexedAt
}

// LastCommitHash returns the last indexed commit hash.
func (r *Repository) LastCommitHash() *string {
	return r.lastCommitHash
}

// TotalFiles returns the total number of indexed files.
func (r *Repository) TotalFiles() int {
	return r.totalFiles
}

// TotalChunks returns the total number of generated chunks.
func (r *Repository) TotalChunks() int {
	return r.totalChunks
}

// Status returns the current repository status.
func (r *Repository) Status() valueobject.RepositoryStatus {
	return r.status
}

// CreatedAt returns the creation timestamp.
func (r *Repository) CreatedAt() time.Time {
	return r.createdAt
}

// UpdatedAt returns the last update timestamp.
func (r *Repository) UpdatedAt() time.Time {
	return r.updatedAt
}

// DeletedAt returns the deletion timestamp.
func (r *Repository) DeletedAt() *time.Time {
	return r.deletedAt
}

// IsDeleted returns true if the repository is soft-deleted.
func (r *Repository) IsDeleted() bool {
	return r.deletedAt != nil
}

// UpdateName updates the repository name.
func (r *Repository) UpdateName(name string) {
	r.name = name
	r.updatedAt = time.Now()
}

// UpdateDescription updates the repository description.
func (r *Repository) UpdateDescription(description *string) {
	r.description = description
	r.updatedAt = time.Now()
}

// UpdateDefaultBranch updates the default branch.
func (r *Repository) UpdateDefaultBranch(branch *string) {
	r.defaultBranch = branch
	r.updatedAt = time.Now()
}

// UpdateStatus updates the repository status if the transition is valid.
func (r *Repository) UpdateStatus(newStatus valueobject.RepositoryStatus) error {
	if !r.status.CanTransitionTo(newStatus) {
		return NewDomainError("invalid status transition", "INVALID_STATUS_TRANSITION")
	}
	r.status = newStatus
	r.updatedAt = time.Now()
	return nil
}

// ResetForReindexing resets the repository status to pending to allow a full re-index.
func (r *Repository) ResetForReindexing() error {
	return r.UpdateStatus(valueobject.RepositoryStatusPending)
}

// MarkIndexingCompleted marks the repository as successfully indexed.
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

// MarkIndexingFailed marks the repository indexing as failed.
func (r *Repository) MarkIndexingFailed() error {
	return r.UpdateStatus(valueobject.RepositoryStatusFailed)
}

// Archive soft-deletes the repository.
func (r *Repository) Archive() error {
	if r.IsDeleted() {
		return NewDomainError("repository is already archived", "ALREADY_ARCHIVED")
	}

	// Only allow archiving repositories in final states (completed or failed)
	if r.status == valueobject.RepositoryStatusPending ||
		r.status == valueobject.RepositoryStatusCloning ||
		r.status == valueobject.RepositoryStatusProcessing {
		return NewDomainError("cannot archive repository in active state", "INVALID_STATE_FOR_ARCHIVE")
	}

	r.status = valueobject.RepositoryStatusArchived
	now := time.Now()
	r.deletedAt = &now
	r.updatedAt = now
	return nil
}

// Restore undeletes the repository.
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

// CanBeDeleted returns true if the repository can be deleted.
func (r *Repository) CanBeDeleted() bool {
	// Cannot delete if currently being processed
	return r.status != valueobject.RepositoryStatusCloning &&
		r.status != valueobject.RepositoryStatusProcessing
}

// Equal compares two Repository entities.
func (r *Repository) Equal(other *Repository) bool {
	if other == nil {
		return false
	}
	return r.id == other.id
}
