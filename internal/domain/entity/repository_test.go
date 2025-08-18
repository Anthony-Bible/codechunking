package entity

import (
	"codechunking/internal/domain/valueobject"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNewRepository(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	name := "Test Repository"
	description := "A test repository"
	defaultBranch := "main"

	repo := NewRepository(testURL, name, &description, &defaultBranch)

	// Test basic properties
	if repo == nil {
		t.Fatal("Expected non-nil repository")
	}

	if repo.ID() == uuid.Nil {
		t.Error("Expected non-nil UUID")
	}

	if !repo.URL().Equal(testURL) {
		t.Errorf("Expected URL %s, got %s", testURL, repo.URL())
	}

	if repo.Name() != name {
		t.Errorf("Expected name %s, got %s", name, repo.Name())
	}

	if repo.Description() == nil || *repo.Description() != description {
		t.Errorf("Expected description %s, got %v", description, repo.Description())
	}

	if repo.DefaultBranch() == nil || *repo.DefaultBranch() != defaultBranch {
		t.Errorf("Expected default branch %s, got %v", defaultBranch, repo.DefaultBranch())
	}

	// Test initial state
	if repo.Status() != valueobject.RepositoryStatusPending {
		t.Errorf("Expected initial status %s, got %s", valueobject.RepositoryStatusPending, repo.Status())
	}

	if repo.TotalFiles() != 0 {
		t.Errorf("Expected initial total files 0, got %d", repo.TotalFiles())
	}

	if repo.TotalChunks() != 0 {
		t.Errorf("Expected initial total chunks 0, got %d", repo.TotalChunks())
	}

	if repo.LastIndexedAt() != nil {
		t.Error("Expected LastIndexedAt to be nil initially")
	}

	if repo.LastCommitHash() != nil {
		t.Error("Expected LastCommitHash to be nil initially")
	}

	if repo.IsDeleted() {
		t.Error("Expected repository to not be deleted initially")
	}

	// Test timestamps
	now := time.Now()
	if repo.CreatedAt().After(now) || repo.CreatedAt().Before(now.Add(-time.Minute)) {
		t.Error("Expected CreatedAt to be close to current time")
	}

	if repo.UpdatedAt().After(now) || repo.UpdatedAt().Before(now.Add(-time.Minute)) {
		t.Error("Expected UpdatedAt to be close to current time")
	}
}

func TestNewRepository_WithNilValues(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	name := "Test Repository"

	repo := NewRepository(testURL, name, nil, nil)

	if repo.Description() != nil {
		t.Error("Expected nil description")
	}

	if repo.DefaultBranch() != nil {
		t.Error("Expected nil default branch")
	}
}

func TestNewRepository_GeneratesUniqueIDs(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	name := "Test Repository"

	repo1 := NewRepository(testURL, name, nil, nil)
	repo2 := NewRepository(testURL, name, nil, nil)

	if repo1.ID() == repo2.ID() {
		t.Error("Expected different repositories to have different IDs")
	}
}

func TestRestoreRepository(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	id := uuid.New()
	name := "Test Repository"
	description := "A test repository"
	defaultBranch := "main"
	lastIndexedAt := time.Now().Add(-time.Hour)
	lastCommitHash := "abc123"
	totalFiles := 100
	totalChunks := 500
	status := valueobject.RepositoryStatusCompleted
	createdAt := time.Now().Add(-24 * time.Hour)
	updatedAt := time.Now().Add(-time.Hour)
	deletedAt := time.Now().Add(-30 * time.Minute)

	repo := RestoreRepository(
		id, testURL, name, &description, &defaultBranch,
		&lastIndexedAt, &lastCommitHash, totalFiles, totalChunks,
		status, createdAt, updatedAt, &deletedAt,
	)

	// Verify all fields are restored correctly
	if repo.ID() != id {
		t.Errorf("Expected ID %s, got %s", id, repo.ID())
	}

	if !repo.URL().Equal(testURL) {
		t.Errorf("Expected URL %s, got %s", testURL, repo.URL())
	}

	if repo.Name() != name {
		t.Errorf("Expected name %s, got %s", name, repo.Name())
	}

	if repo.Description() == nil || *repo.Description() != description {
		t.Errorf("Expected description %s, got %v", description, repo.Description())
	}

	if repo.DefaultBranch() == nil || *repo.DefaultBranch() != defaultBranch {
		t.Errorf("Expected default branch %s, got %v", defaultBranch, repo.DefaultBranch())
	}

	if repo.LastIndexedAt() == nil || !repo.LastIndexedAt().Equal(lastIndexedAt) {
		t.Errorf("Expected last indexed at %s, got %v", lastIndexedAt, repo.LastIndexedAt())
	}

	if repo.LastCommitHash() == nil || *repo.LastCommitHash() != lastCommitHash {
		t.Errorf("Expected last commit hash %s, got %v", lastCommitHash, repo.LastCommitHash())
	}

	if repo.TotalFiles() != totalFiles {
		t.Errorf("Expected total files %d, got %d", totalFiles, repo.TotalFiles())
	}

	if repo.TotalChunks() != totalChunks {
		t.Errorf("Expected total chunks %d, got %d", totalChunks, repo.TotalChunks())
	}

	if repo.Status() != status {
		t.Errorf("Expected status %s, got %s", status, repo.Status())
	}

	if !repo.CreatedAt().Equal(createdAt) {
		t.Errorf("Expected created at %s, got %s", createdAt, repo.CreatedAt())
	}

	if !repo.UpdatedAt().Equal(updatedAt) {
		t.Errorf("Expected updated at %s, got %s", updatedAt, repo.UpdatedAt())
	}

	if repo.DeletedAt() == nil || !repo.DeletedAt().Equal(deletedAt) {
		t.Errorf("Expected deleted at %s, got %v", deletedAt, repo.DeletedAt())
	}

	if !repo.IsDeleted() {
		t.Error("Expected repository to be deleted")
	}
}

func TestRepository_UpdateName(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Original Name", nil, nil)

	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond) // Ensure timestamp difference

	newName := "Updated Name"
	repo.UpdateName(newName)

	if repo.Name() != newName {
		t.Errorf("Expected name %s, got %s", newName, repo.Name())
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestRepository_UpdateDescription(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	originalDesc := "Original Description"
	repo := NewRepository(testURL, "Test", &originalDesc, nil)

	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond)

	newDesc := "Updated Description"
	repo.UpdateDescription(&newDesc)

	if repo.Description() == nil || *repo.Description() != newDesc {
		t.Errorf("Expected description %s, got %v", newDesc, repo.Description())
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}

	// Test setting to nil
	repo.UpdateDescription(nil)
	if repo.Description() != nil {
		t.Error("Expected description to be nil")
	}
}

func TestRepository_UpdateDefaultBranch(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	originalBranch := "main"
	repo := NewRepository(testURL, "Test", nil, &originalBranch)

	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond)

	newBranch := "develop"
	repo.UpdateDefaultBranch(&newBranch)

	if repo.DefaultBranch() == nil || *repo.DefaultBranch() != newBranch {
		t.Errorf("Expected default branch %s, got %v", newBranch, repo.DefaultBranch())
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}

	// Test setting to nil
	repo.UpdateDefaultBranch(nil)
	if repo.DefaultBranch() != nil {
		t.Error("Expected default branch to be nil")
	}
}

func TestRepository_UpdateStatus_ValidTransitions(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Test valid transition: pending -> cloning
	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond)

	err := repo.UpdateStatus(valueobject.RepositoryStatusCloning)
	if err != nil {
		t.Errorf("Expected no error for valid transition, got: %v", err)
	}

	if repo.Status() != valueobject.RepositoryStatusCloning {
		t.Errorf("Expected status %s, got %s", valueobject.RepositoryStatusCloning, repo.Status())
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestRepository_UpdateStatus_InvalidTransitions(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Test invalid transition: pending -> completed (should go through cloning and processing)
	originalStatus := repo.Status()
	originalUpdatedAt := repo.UpdatedAt()

	err := repo.UpdateStatus(valueobject.RepositoryStatusCompleted)
	if err == nil {
		t.Error("Expected error for invalid status transition")
	}

	domainErr := &DomainError{}
	ok := errors.As(err, &domainErr)
	if !ok {
		t.Errorf("Expected DomainError, got %T", err)
	} else if domainErr.Code() != "INVALID_STATUS_TRANSITION" {
		t.Errorf("Expected error code 'INVALID_STATUS_TRANSITION', got '%s'", domainErr.Code())
	}

	// Verify status and timestamp weren't changed
	if repo.Status() != originalStatus {
		t.Errorf("Expected status to remain %s, got %s", originalStatus, repo.Status())
	}

	if repo.UpdatedAt() != originalUpdatedAt {
		t.Error("Expected UpdatedAt to not change on failed transition")
	}
}

func TestRepository_MarkIndexingCompleted(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Set up repository in a state where it can be completed
	_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)

	commitHash := "abc123def456"
	totalFiles := 100
	totalChunks := 500

	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond)

	err := repo.MarkIndexingCompleted(commitHash, totalFiles, totalChunks)
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	// Verify status changed
	if repo.Status() != valueobject.RepositoryStatusCompleted {
		t.Errorf("Expected status %s, got %s", valueobject.RepositoryStatusCompleted, repo.Status())
	}

	// Verify indexing data was set
	if repo.LastCommitHash() == nil || *repo.LastCommitHash() != commitHash {
		t.Errorf("Expected last commit hash %s, got %v", commitHash, repo.LastCommitHash())
	}

	if repo.TotalFiles() != totalFiles {
		t.Errorf("Expected total files %d, got %d", totalFiles, repo.TotalFiles())
	}

	if repo.TotalChunks() != totalChunks {
		t.Errorf("Expected total chunks %d, got %d", totalChunks, repo.TotalChunks())
	}

	if repo.LastIndexedAt() == nil {
		t.Error("Expected LastIndexedAt to be set")
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestRepository_MarkIndexingCompleted_InvalidState(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Try to mark as completed from pending state (invalid)
	err := repo.MarkIndexingCompleted("abc123", 100, 500)
	if err == nil {
		t.Error("Expected error when marking as completed from pending state")
	}

	// Verify no changes were made
	if repo.Status() != valueobject.RepositoryStatusPending {
		t.Error("Expected status to remain pending")
	}

	if repo.LastCommitHash() != nil {
		t.Error("Expected LastCommitHash to remain nil")
	}

	if repo.TotalFiles() != 0 {
		t.Error("Expected TotalFiles to remain 0")
	}

	if repo.TotalChunks() != 0 {
		t.Error("Expected TotalChunks to remain 0")
	}
}

func TestRepository_MarkIndexingFailed(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond)

	err := repo.MarkIndexingFailed()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if repo.Status() != valueobject.RepositoryStatusFailed {
		t.Errorf("Expected status %s, got %s", valueobject.RepositoryStatusFailed, repo.Status())
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestRepository_Archive(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Set to a state that can be archived (must go through proper transitions)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusCompleted)

	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond)

	err := repo.Archive()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if repo.Status() != valueobject.RepositoryStatusArchived {
		t.Errorf("Expected status %s, got %s", valueobject.RepositoryStatusArchived, repo.Status())
	}

	if !repo.IsDeleted() {
		t.Error("Expected repository to be marked as deleted")
	}

	if repo.DeletedAt() == nil {
		t.Error("Expected DeletedAt to be set")
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestRepository_Archive_AlreadyArchived(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Archive the repository first (must go through proper transitions)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusCompleted)
	_ = repo.Archive()

	// Try to archive again
	err := repo.Archive()
	if err == nil {
		t.Error("Expected error when archiving already archived repository")
	}

	domainErr := &DomainError{}
	ok := errors.As(err, &domainErr)
	if !ok {
		t.Errorf("Expected DomainError, got %T", err)
	} else if domainErr.Code() != "ALREADY_ARCHIVED" {
		t.Errorf("Expected error code 'ALREADY_ARCHIVED', got '%s'", domainErr.Code())
	}
}

func TestRepository_Restore(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Archive the repository first (must go through proper transitions)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)
	_ = repo.UpdateStatus(valueobject.RepositoryStatusCompleted)
	_ = repo.Archive()

	originalUpdatedAt := repo.UpdatedAt()
	time.Sleep(1 * time.Millisecond)

	err := repo.Restore()
	if err != nil {
		t.Errorf("Expected no error, got: %v", err)
	}

	if repo.Status() != valueobject.RepositoryStatusPending {
		t.Errorf("Expected status %s, got %s", valueobject.RepositoryStatusPending, repo.Status())
	}

	if repo.IsDeleted() {
		t.Error("Expected repository to not be marked as deleted")
	}

	if repo.DeletedAt() != nil {
		t.Error("Expected DeletedAt to be nil")
	}

	if !repo.UpdatedAt().After(originalUpdatedAt) {
		t.Error("Expected UpdatedAt to be updated")
	}
}

func TestRepository_Restore_NotArchived(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
	repo := NewRepository(testURL, "Test", nil, nil)

	// Try to restore without archiving first
	err := repo.Restore()
	if err == nil {
		t.Error("Expected error when restoring non-archived repository")
	}

	domainErr := &DomainError{}
	ok := errors.As(err, &domainErr)
	if !ok {
		t.Errorf("Expected DomainError, got %T", err)
	} else if domainErr.Code() != "NOT_ARCHIVED" {
		t.Errorf("Expected error code 'NOT_ARCHIVED', got '%s'", domainErr.Code())
	}
}

func TestRepository_CanBeDeleted(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")

	testCases := []struct {
		name         string
		status       valueobject.RepositoryStatus
		canBeDeleted bool
	}{
		{"Pending can be deleted", valueobject.RepositoryStatusPending, true},
		{"Cloning cannot be deleted", valueobject.RepositoryStatusCloning, false},
		{"Processing cannot be deleted", valueobject.RepositoryStatusProcessing, false},
		{"Completed can be deleted", valueobject.RepositoryStatusCompleted, true},
		{"Failed can be deleted", valueobject.RepositoryStatusFailed, true},
		{"Archived can be deleted", valueobject.RepositoryStatusArchived, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo := NewRepository(testURL, "Test", nil, nil)

			// Set the desired status (may need multiple transitions)
			switch tc.status {
			case valueobject.RepositoryStatusCloning:
				_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
			case valueobject.RepositoryStatusProcessing:
				_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
				_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)
			case valueobject.RepositoryStatusCompleted:
				_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
				_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)
				_ = repo.UpdateStatus(valueobject.RepositoryStatusCompleted)
			case valueobject.RepositoryStatusFailed:
				_ = repo.UpdateStatus(valueobject.RepositoryStatusFailed)
			case valueobject.RepositoryStatusArchived:
				_ = repo.UpdateStatus(valueobject.RepositoryStatusCompleted)
				_ = repo.Archive()
			}

			canDelete := repo.CanBeDeleted()
			if canDelete != tc.canBeDeleted {
				t.Errorf("Expected CanBeDeleted() to be %v for status %s, got %v",
					tc.canBeDeleted, tc.status, canDelete)
			}
		})
	}
}

func TestRepository_Equal(t *testing.T) {
	testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")

	repo1 := NewRepository(testURL, "Test1", nil, nil)
	repo2 := NewRepository(testURL, "Test2", nil, nil)

	// Different repositories should not be equal
	if repo1.Equal(repo2) {
		t.Error("Expected different repositories to not be equal")
	}

	// Same repository should be equal to itself
	if !repo1.Equal(repo1) { //nolint:gocritic // valid reflexivity test for equality
		t.Error("Expected repository to be equal to itself")
	}

	// Comparison with nil should return false
	if repo1.Equal(nil) {
		t.Error("Expected repository to not be equal to nil")
	}

	// Test with restored repository with same ID
	restoredRepo := RestoreRepository(
		repo1.ID(), testURL, "Different Name", nil, nil,
		nil, nil, 0, 0, valueobject.RepositoryStatusPending,
		time.Now(), time.Now(), nil,
	)

	if !repo1.Equal(restoredRepo) {
		t.Error("Expected repositories with same ID to be equal")
	}
}

func TestRepository_EdgeCases(t *testing.T) {
	t.Run("Empty name", func(t *testing.T) {
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
		repo := NewRepository(testURL, "", nil, nil)

		if repo.Name() != "" {
			t.Errorf("Expected empty name to be preserved, got %s", repo.Name())
		}
	})

	t.Run("Negative values in MarkIndexingCompleted", func(t *testing.T) {
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
		repo := NewRepository(testURL, "Test", nil, nil)

		// Set up for completion
		_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
		_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)

		err := repo.MarkIndexingCompleted("abc123", -10, -20)
		if err != nil {
			t.Errorf("Expected no error for negative values, got: %v", err)
		}

		// Values should be preserved even if negative
		if repo.TotalFiles() != -10 {
			t.Errorf("Expected total files -10, got %d", repo.TotalFiles())
		}

		if repo.TotalChunks() != -20 {
			t.Errorf("Expected total chunks -20, got %d", repo.TotalChunks())
		}
	})

	t.Run("Empty commit hash", func(t *testing.T) {
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
		repo := NewRepository(testURL, "Test", nil, nil)

		// Set up for completion
		_ = repo.UpdateStatus(valueobject.RepositoryStatusCloning)
		_ = repo.UpdateStatus(valueobject.RepositoryStatusProcessing)

		err := repo.MarkIndexingCompleted("", 100, 500)
		if err != nil {
			t.Errorf("Expected no error for empty commit hash, got: %v", err)
		}

		if repo.LastCommitHash() == nil || *repo.LastCommitHash() != "" {
			t.Errorf("Expected empty commit hash to be preserved")
		}
	})

	t.Run("Multiple consecutive updates", func(t *testing.T) {
		testURL, _ := valueobject.NewRepositoryURL("https://github.com/owner/repo")
		repo := NewRepository(testURL, "Test", nil, nil)

		// Multiple name updates
		repo.UpdateName("Name1")
		repo.UpdateName("Name2")
		repo.UpdateName("Name3")

		if repo.Name() != "Name3" {
			t.Errorf("Expected final name 'Name3', got %s", repo.Name())
		}

		// Multiple description updates
		desc1 := "Desc1"
		desc2 := "Desc2"
		repo.UpdateDescription(&desc1)
		repo.UpdateDescription(&desc2)
		repo.UpdateDescription(nil)

		if repo.Description() != nil {
			t.Error("Expected final description to be nil")
		}
	})
}
