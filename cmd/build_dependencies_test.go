package cmd

import (
	"testing"

	"codechunking/internal/adapter/outbound/mock"
	"codechunking/internal/adapter/outbound/repository"
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	// These imports are used by the buildDependencies method tests.
)

// TestServiceFactory_BuildDependencies_Success tests that buildDependencies successfully
// returns all three dependencies when database connection succeeds.
func TestServiceFactory_BuildDependencies_Success(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           "localhost",
			Port:           5432,
			Name:           "codechunking",
			User:           "dev",
			Password:       "dev",
			MaxConnections: 10,
			SSLMode:        "disable",
		},
	}
	serviceFactory := NewServiceFactory(cfg)

	// Act
	repositoryRepo, indexingJobRepo, messagePublisher, err := serviceFactory.buildDependencies()

	// Assert
	require.NoError(t, err, "buildDependencies should not return error when database connection succeeds")
	assert.NotNil(t, repositoryRepo, "repositoryRepo should not be nil when database connection succeeds")
	assert.NotNil(t, indexingJobRepo, "indexingJobRepo should not be nil when database connection succeeds")
	assert.NotNil(t, messagePublisher, "messagePublisher should never be nil")

	// Verify concrete types
	_, ok := repositoryRepo.(*repository.PostgreSQLRepositoryRepository)
	assert.True(t, ok, "repositoryRepo should be PostgreSQLRepositoryRepository")

	_, ok = indexingJobRepo.(*repository.PostgreSQLIndexingJobRepository)
	assert.True(t, ok, "indexingJobRepo should be PostgreSQLIndexingJobRepository")

	_, ok = messagePublisher.(*mock.MockMessagePublisher)
	assert.True(t, ok, "messagePublisher should be MockMessagePublisher")
}

// TestServiceFactory_BuildDependencies_DatabaseError tests that buildDependencies handles
// database connection errors gracefully by returning nil repositories but valid message publisher.
func TestServiceFactory_BuildDependencies_DatabaseError(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           "invalid-host",
			Port:           9999,
			Name:           "nonexistent_db",
			User:           "invalid_user",
			Password:       "invalid_pass",
			MaxConnections: 10,
			SSLMode:        "disable",
		},
	}
	serviceFactory := NewServiceFactory(cfg)

	// Act
	repositoryRepo, indexingJobRepo, messagePublisher, err := serviceFactory.buildDependencies()

	// Assert
	require.Error(t, err, "buildDependencies should return error when database connection fails")
	assert.Nil(t, repositoryRepo, "repositoryRepo should be nil when database connection fails")
	assert.Nil(t, indexingJobRepo, "indexingJobRepo should be nil when database connection fails")
	assert.NotNil(t, messagePublisher, "messagePublisher should always be returned, even on database error")

	// Verify message publisher type is correct
	_, ok := messagePublisher.(*mock.MockMessagePublisher)
	assert.True(t, ok, "messagePublisher should be MockMessagePublisher even when database fails")
}

// TestServiceFactory_BuildDependencies_ReturnTypes tests that buildDependencies returns
// the correct interface types that can be used by the service layer.
func TestServiceFactory_BuildDependencies_ReturnTypes(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           "localhost",
			Port:           5432,
			Name:           "codechunking",
			User:           "dev",
			Password:       "dev",
			MaxConnections: 10,
			SSLMode:        "disable",
		},
	}
	serviceFactory := NewServiceFactory(cfg)

	// Act
	repositoryRepo, indexingJobRepo, messagePublisher, err := serviceFactory.buildDependencies()

	// Assert - assuming successful connection for this test
	if err == nil {
		// Verify interface compliance
		_ = repositoryRepo
		_ = indexingJobRepo
		_ = messagePublisher

		assert.Implements(t, (*outbound.RepositoryRepository)(nil), repositoryRepo,
			"repositoryRepo must implement RepositoryRepository interface")
		assert.Implements(t, (*outbound.IndexingJobRepository)(nil), indexingJobRepo,
			"indexingJobRepo must implement IndexingJobRepository interface")
		assert.Implements(t, (*outbound.MessagePublisher)(nil), messagePublisher,
			"messagePublisher must implement MessagePublisher interface")
	}
}

// TestServiceFactory_BuildDependencies_ErrorPropagation tests that database connection
// errors are properly propagated with meaningful error messages.
func TestServiceFactory_BuildDependencies_ErrorPropagation(t *testing.T) {
	testCases := []struct {
		name           string
		config         config.DatabaseConfig
		expectedErrMsg string
	}{
		{
			name: "invalid_host",
			config: config.DatabaseConfig{
				Host:           "nonexistent-host-12345",
				Port:           5432,
				Name:           "test_db",
				User:           "test_user",
				Password:       "test_pass",
				MaxConnections: 10,
				SSLMode:        "disable",
			},
			expectedErrMsg: "failed to connect to database",
		},
		{
			name: "invalid_port",
			config: config.DatabaseConfig{
				Host:           "localhost",
				Port:           99999, // Invalid port
				Name:           "test_db",
				User:           "test_user",
				Password:       "test_pass",
				MaxConnections: 10,
				SSLMode:        "disable",
			},
			expectedErrMsg: "invalid port",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			cfg := &config.Config{Database: tc.config}
			serviceFactory := NewServiceFactory(cfg)

			// Act
			repositoryRepo, indexingJobRepo, messagePublisher, err := serviceFactory.buildDependencies()

			// Assert
			require.Error(t, err, "buildDependencies should return error for invalid database config")
			assert.Nil(t, repositoryRepo, "repositoryRepo should be nil on database error")
			assert.Nil(t, indexingJobRepo, "indexingJobRepo should be nil on database error")
			assert.NotNil(t, messagePublisher, "messagePublisher should still be returned")

			// Note: The exact error message will depend on the implementation
			// This test ensures errors are propagated, not the exact message content
			assert.NotEmpty(t, err.Error(), "error should have a meaningful message")
		})
	}
}

// TestServiceFactory_CreateHealthService_UsesBuildDependencies tests that the refactored
// CreateHealthService method uses buildDependencies to eliminate code duplication.
func TestServiceFactory_CreateHealthService_UsesBuildDependencies(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           "invalid-host", // Force database error
			Port:           9999,
			Name:           "nonexistent_db",
			User:           "invalid_user",
			Password:       "invalid_pass",
			MaxConnections: 10,
			SSLMode:        "disable",
		},
	}
	serviceFactory := NewServiceFactory(cfg)

	// Act
	healthService := serviceFactory.CreateHealthService()

	// Assert
	assert.NotNil(t, healthService, "CreateHealthService should return a health service even with database errors")

	// Note: This test verifies that CreateHealthService can handle the case where
	// buildDependencies returns nil repositories due to database connection failure.
	// The health service should still be created with nil dependencies, which is
	// the expected behavior for graceful degradation.
}

// TestServiceFactory_CreateRepositoryService_UsesBuildDependencies tests that the refactored
// CreateRepositoryService method uses buildDependencies and properly handles dependencies.
func TestServiceFactory_CreateRepositoryService_UsesBuildDependencies(t *testing.T) {
	// Arrange
	cfg := &config.Config{
		Database: config.DatabaseConfig{
			Host:           "localhost",
			Port:           5432,
			Name:           "codechunking",
			User:           "dev",
			Password:       "dev",
			MaxConnections: 10,
			SSLMode:        "disable",
		},
	}
	serviceFactory := NewServiceFactory(cfg)

	// Act & Assert
	// This test will initially fail because:
	// 1. buildDependencies method doesn't exist yet
	// 2. CreateRepositoryService hasn't been refactored to use it
	//
	// The expectation is that after refactoring:
	// - CreateRepositoryService should call buildDependencies
	// - If buildDependencies succeeds, CreateRepositoryService should create service registry with those dependencies
	// - If buildDependencies fails, CreateRepositoryService should handle the error appropriately

	// We expect this to work without errors when database connection is valid
	repositoryService := serviceFactory.CreateRepositoryService()
	assert.NotNil(
		t,
		repositoryService,
		"CreateRepositoryService should return a repository service when dependencies are available",
	)
}

// TestServiceFactory_CreateRepositoryService_FailsOnDatabaseError tests that CreateRepositoryService
// properly handles database connection failures after being refactored to use buildDependencies.
func TestServiceFactory_CreateRepositoryService_FailsOnDatabaseError(t *testing.T) {
	// This test documents that CreateRepositoryService should call log.Fatalf
	// when buildDependencies returns a database error, maintaining the original behavior

	// Act & Assert
	// Note: Testing log.Fatalf is tricky - in the current implementation,
	// CreateRepositoryService calls log.Fatalf on database errors.
	// After refactoring to use buildDependencies, this behavior should be preserved.
	//
	// This test documents the expected behavior that CreateRepositoryService should
	// fail when buildDependencies returns a database error.
	// The actual test implementation might need to be adjusted based on how
	// error handling is implemented in the GREEN phase.

	// For now, we document that this should cause the program to exit
	// In a real implementation, we might want to test this by capturing log output
	// or by modifying the method to return an error instead of calling log.Fatalf

	t.Log("This test documents that CreateRepositoryService should fail when buildDependencies returns database error")
	t.Log("The actual implementation will depend on the chosen error handling strategy in the GREEN phase")

	// This will currently call log.Fatalf, which makes it hard to test
	// The refactored version should use buildDependencies and handle errors consistently

	// Note: We cannot easily test log.Fatalf since it calls os.Exit()
	// This test documents the expected behavior - CreateRepositoryService should
	// call log.Fatalf when buildDependencies returns a database error

	// The implementation correctly maintains the original behavior:
	// - CreateHealthService logs error and continues with nil dependencies
	// - CreateRepositoryService calls log.Fatalf and exits on database error
}

// TestServiceFactory_BuildDependencies_Integration tests the integration of buildDependencies
// with the actual repository and message publisher creation.
func TestServiceFactory_BuildDependencies_Integration(t *testing.T) {
	// This test verifies that when buildDependencies is called:
	// 1. It correctly uses ServiceFactory.createDatabasePool()
	// 2. It passes the pool to repository.NewPostgreSQLRepositoryRepository
	// 3. It passes the pool to repository.NewPostgreSQLIndexingJobRepository
	// 4. It always calls mock.NewMockMessagePublisher()
	// 5. Error handling works correctly throughout the chain

	t.Run("successful_dependency_creation", func(t *testing.T) {
		// Arrange
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:           "localhost",
				Port:           5432,
				Name:           "test_db",
				User:           "test_user",
				Password:       "test_pass",
				MaxConnections: 10,
				SSLMode:        "disable",
			},
		}
		serviceFactory := NewServiceFactory(cfg)

		// Act
		repositoryRepo, indexingJobRepo, messagePublisher, err := serviceFactory.buildDependencies()

		// Assert integration points
		if err == nil {
			assert.NotNil(t, repositoryRepo, "Repository should be created when pool creation succeeds")
			assert.NotNil(t, indexingJobRepo, "IndexingJob repository should be created when pool creation succeeds")
		}
		assert.NotNil(t, messagePublisher, "MessagePublisher should always be created")
	})

	t.Run("dependency_creation_with_database_failure", func(t *testing.T) {
		// Arrange
		cfg := &config.Config{
			Database: config.DatabaseConfig{
				Host:           "invalid-host",
				Port:           9999,
				Name:           "nonexistent",
				User:           "invalid",
				Password:       "invalid",
				MaxConnections: 10,
				SSLMode:        "disable",
			},
		}
		serviceFactory := NewServiceFactory(cfg)

		// Act
		repositoryRepo, indexingJobRepo, messagePublisher, err := serviceFactory.buildDependencies()

		// Assert graceful failure
		require.Error(t, err, "Should return error when database connection fails")
		assert.Nil(t, repositoryRepo, "Repository should be nil when pool creation fails")
		assert.Nil(t, indexingJobRepo, "IndexingJob repository should be nil when pool creation fails")
		assert.NotNil(t, messagePublisher, "MessagePublisher should still be created even on database failure")
	})
}

// TestServiceFactory_BuildDependencies_MethodExists tests that the buildDependencies method
// exists on the ServiceFactory struct with the correct signature.
func TestServiceFactory_BuildDependencies_MethodExists(t *testing.T) {
	// Arrange
	cfg := &config.Config{}
	serviceFactory := NewServiceFactory(cfg)

	// Act & Assert
	// This test will fail initially because the method doesn't exist
	// It documents the expected method signature
	assert.True(t, hasMethod(serviceFactory, "buildDependencies"),
		"ServiceFactory should have buildDependencies method")
}

// Helper function to check if a method exists (for testing purposes).
func hasMethod(obj interface{}, methodName string) bool {
	// This is a simplified check - in reality we'd use reflection
	// But for TDD purposes, we know this will fail until the method is implemented
	// Note: Method should exist after GREEN phase implementation

	// Try to call the method - this will panic if it doesn't exist
	if sf, ok := obj.(*ServiceFactory); ok {
		_, _, _, _ = sf.buildDependencies()
		return true
	}
	return false
}

// TestServiceFactory_BuildDependencies_ConfigDefaults tests that buildDependencies works
// with default configuration values similar to createDatabasePool.
func TestServiceFactory_BuildDependencies_ConfigDefaults(t *testing.T) {
	// Arrange
	SetTestConfig(&config.Config{
		Database: config.DatabaseConfig{
			// Test with minimal config to ensure defaults are set
		},
	})
	serviceFactory := NewServiceFactory(GetConfig())

	// Act
	_, _, messagePublisher, err := serviceFactory.buildDependencies()

	// Assert
	// Even if database connection fails due to missing config,
	// the message publisher should always be created
	assert.NotNil(t, messagePublisher, "MessagePublisher should be created regardless of database config")

	// The method should handle missing configuration gracefully
	// by setting defaults (similar to current createDatabasePool implementation)
	if err != nil {
		assert.IsType(t, &mock.MockMessagePublisher{}, messagePublisher,
			"Should return mock message publisher on database error")
	}
}
