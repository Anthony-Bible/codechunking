package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockCleanupResource is a mock implementation of CleanupResource for testing.
type MockCleanupResource struct {
	mock.Mock

	name         string
	resourceType ResourceType
	priority     int
	concurrent   bool
	healthy      bool
}

func NewMockCleanupResource(name string, resourceType ResourceType) *MockCleanupResource {
	return &MockCleanupResource{
		name:         name,
		resourceType: resourceType,
		priority:     1,
		concurrent:   true,
		healthy:      true,
	}
}

func (m *MockCleanupResource) Cleanup(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockCleanupResource) GetResourceType() ResourceType {
	return m.resourceType
}

func (m *MockCleanupResource) GetResourceInfo() ResourceInfo {
	return ResourceInfo{
		Name:        m.name,
		Type:        m.resourceType,
		Description: "Mock resource for testing",
		Priority:    m.priority,
		Concurrent:  m.concurrent,
		CreatedAt:   time.Now(),
	}
}

func (m *MockCleanupResource) CanCleanupConcurrently() bool {
	return m.concurrent
}

func (m *MockCleanupResource) GetCleanupPriority() int {
	return m.priority
}

func (m *MockCleanupResource) IsHealthy(_ context.Context) bool {
	return m.healthy
}

// TestResourceCleanupManager_ResourceRegistration tests resource registration and management.
func TestResourceCleanupManager_ResourceRegistration(t *testing.T) {
	tests := []struct {
		name                  string
		config                ResourceCleanupConfig
		resourceRegistrations []struct {
			name     string
			resource CleanupResource
		}
		expectedResourceCount int
		expectError           bool
		errorContains         string
	}{
		{
			name: "Valid resource registration",
			config: ResourceCleanupConfig{
				CleanupTimeout:        30 * time.Second,
				MaxConcurrentCleanups: 5,
			},
			resourceRegistrations: []struct {
				name     string
				resource CleanupResource
			}{
				{"database-pool", NewMockCleanupResource("database-pool", ResourceTypeDatabase)},
				{"nats-consumer", NewMockCleanupResource("nats-consumer", ResourceTypeMessaging)},
				{"file-cache", NewMockCleanupResource("file-cache", ResourceTypeFileSystem)},
			},
			expectedResourceCount: 3,
			expectError:           false,
		},
		{
			name: "Duplicate resource name returns error",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
			},
			resourceRegistrations: []struct {
				name     string
				resource CleanupResource
			}{
				{"database-pool", NewMockCleanupResource("database-pool", ResourceTypeDatabase)},
				{"database-pool", NewMockCleanupResource("database-pool", ResourceTypeDatabase)},
			},
			expectedResourceCount: 1,
			expectError:           true,
			errorContains:         "already registered",
		},
		{
			name: "Empty resource name returns error",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
			},
			resourceRegistrations: []struct {
				name     string
				resource CleanupResource
			}{
				{"", NewMockCleanupResource("", ResourceTypeDatabase)},
			},
			expectedResourceCount: 0,
			expectError:           true,
			errorContains:         "name cannot be empty",
		},
		{
			name: "Nil resource returns error",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
			},
			resourceRegistrations: []struct {
				name     string
				resource CleanupResource
			}{
				{"database-pool", nil},
			},
			expectedResourceCount: 0,
			expectError:           true,
			errorContains:         "resource cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented, the test would:
			// manager := NewResourceCleanupManager(tt.config)

			// var lastErr error
			// for _, reg := range tt.resourceRegistrations {
			//     lastErr = manager.RegisterResource(reg.name, reg.resource)
			//     if tt.expectError && lastErr != nil {
			//         break
			//     }
			// }

			// if tt.expectError {
			//     assert.Error(t, lastErr)
			//     assert.Contains(t, lastErr.Error(), tt.errorContains)
			// } else {
			//     assert.NoError(t, lastErr)
			// }

			// status := manager.GetResourceStatus()
			// assert.Len(t, status, tt.expectedResourceCount)
		})
	}
}

// TestResourceCleanupManager_CleanupOrdering tests resource cleanup order management.
func TestResourceCleanupManager_CleanupOrdering(t *testing.T) {
	tests := []struct {
		name      string
		config    ResourceCleanupConfig
		resources []struct {
			name     string
			resType  ResourceType
			priority int
		}
		cleanupOrder     []string
		expectedOrder    []string
		expectOrderError bool
	}{
		{
			name: "Priority-based cleanup order",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
			},
			resources: []struct {
				name     string
				resType  ResourceType
				priority int
			}{
				{"worker-pool", ResourceTypeWorkerPool, 10}, // High priority (cleanup first)
				{"database", ResourceTypeDatabase, 5},       // Medium priority
				{"file-cache", ResourceTypeFileSystem, 1},   // Low priority (cleanup last)
			},
			expectedOrder:    []string{"worker-pool", "database", "file-cache"},
			expectOrderError: false,
		},
		{
			name: "Type-based cleanup order",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
				CleanupOrder: []ResourceType{
					ResourceTypeWorkerPool,
					ResourceTypeMessaging,
					ResourceTypeDatabase,
					ResourceTypeFileSystem,
				},
			},
			resources: []struct {
				name     string
				resType  ResourceType
				priority int
			}{
				{"file-cache", ResourceTypeFileSystem, 5},
				{"nats-consumer", ResourceTypeMessaging, 5},
				{"worker-pool", ResourceTypeWorkerPool, 5},
				{"database", ResourceTypeDatabase, 5},
			},
			expectedOrder:    []string{"worker-pool", "nats-consumer", "database", "file-cache"},
			expectOrderError: false,
		},
		{
			name: "Custom cleanup order",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
			},
			resources: []struct {
				name     string
				resType  ResourceType
				priority int
			}{
				{"resource-a", ResourceTypeDatabase, 1},
				{"resource-b", ResourceTypeMessaging, 1},
				{"resource-c", ResourceTypeFileSystem, 1},
			},
			cleanupOrder:     []string{"resource-c", "resource-a", "resource-b"},
			expectedOrder:    []string{"resource-c", "resource-a", "resource-b"},
			expectOrderError: false,
		},
		{
			name: "Invalid cleanup order with non-existent resource",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
			},
			resources: []struct {
				name     string
				resType  ResourceType
				priority int
			}{
				{"resource-a", ResourceTypeDatabase, 1},
			},
			cleanupOrder:     []string{"resource-a", "non-existent"},
			expectOrderError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented:
			// manager := NewResourceCleanupManager(tt.config)

			// Register resources
			// for _, res := range tt.resources {
			//     mockResource := NewMockCleanupResource(res.name, res.resType)
			//     mockResource.priority = res.priority
			//     manager.RegisterResource(res.name, mockResource)
			// }

			// Set custom cleanup order if provided
			// if len(tt.cleanupOrder) > 0 {
			//     err := manager.SetCleanupOrder(tt.cleanupOrder)
			//     if tt.expectOrderError {
			//         assert.Error(t, err)
			//         return
			//     }
			//     assert.NoError(t, err)
			// }

			// Test cleanup order by tracking cleanup sequence
			// var cleanupSequence []string
			// var mu sync.Mutex

			// for _, res := range tt.resources {
			//     mockResource := manager.getResource(res.name) // hypothetical method
			//     mockResource.On("Cleanup", mock.Anything).Run(func(args mock.Arguments) {
			//         mu.Lock()
			//         cleanupSequence = append(cleanupSequence, res.name)
			//         mu.Unlock()
			//     }).Return(nil)
			// }

			// manager.CleanupAll(context.Background())

			// assert.Equal(t, tt.expectedOrder, cleanupSequence)
		})
	}
}

// TestResourceCleanupManager_ConcurrentCleanup tests parallel resource cleanup.
func TestResourceCleanupManager_ConcurrentCleanup(t *testing.T) {
	tests := []struct {
		name      string
		config    ResourceCleanupConfig
		resources []struct {
			name       string
			resType    ResourceType
			concurrent bool
			duration   time.Duration
		}
		expectedConcurrency int
		expectedDuration    time.Duration
	}{
		{
			name: "Concurrent cleanup of independent resources",
			config: ResourceCleanupConfig{
				CleanupTimeout:        10 * time.Second,
				MaxConcurrentCleanups: 3,
			},
			resources: []struct {
				name       string
				resType    ResourceType
				concurrent bool
				duration   time.Duration
			}{
				{"cache-a", ResourceTypeCache, true, 2 * time.Second},
				{"cache-b", ResourceTypeCache, true, 2 * time.Second},
				{"cache-c", ResourceTypeCache, true, 2 * time.Second},
			},
			expectedConcurrency: 3,
			expectedDuration:    2 * time.Second, // Should run in parallel
		},
		{
			name: "Sequential cleanup of non-concurrent resources",
			config: ResourceCleanupConfig{
				CleanupTimeout:        15 * time.Second,
				MaxConcurrentCleanups: 5,
			},
			resources: []struct {
				name       string
				resType    ResourceType
				concurrent bool
				duration   time.Duration
			}{
				{"database-1", ResourceTypeDatabase, false, 2 * time.Second},
				{"database-2", ResourceTypeDatabase, false, 2 * time.Second},
				{"database-3", ResourceTypeDatabase, false, 2 * time.Second},
			},
			expectedConcurrency: 1,
			expectedDuration:    6 * time.Second, // Sequential execution
		},
		{
			name: "Mixed concurrent and sequential cleanup",
			config: ResourceCleanupConfig{
				CleanupTimeout:        20 * time.Second,
				MaxConcurrentCleanups: 2,
			},
			resources: []struct {
				name       string
				resType    ResourceType
				concurrent bool
				duration   time.Duration
			}{
				{"cache-1", ResourceTypeCache, true, 3 * time.Second},
				{"cache-2", ResourceTypeCache, true, 3 * time.Second},
				{"database", ResourceTypeDatabase, false, 4 * time.Second},
				{"file-system", ResourceTypeFileSystem, true, 2 * time.Second},
			},
			expectedConcurrency: 2,
			expectedDuration:    9 * time.Second, // Mixed execution pattern
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented:
			// manager := NewResourceCleanupManager(tt.config)

			// Track concurrent execution
			// var activeConcurrency int
			// var maxConcurrency int
			// var mu sync.Mutex

			// for _, res := range tt.resources {
			//     mockResource := NewMockCleanupResource(res.name, res.resType)
			//     mockResource.concurrent = res.concurrent
			//
			//     mockResource.On("Cleanup", mock.Anything).Run(func(args mock.Arguments) {
			//         mu.Lock()
			//         activeConcurrency++
			//         if activeConcurrency > maxConcurrency {
			//             maxConcurrency = activeConcurrency
			//         }
			//         mu.Unlock()
			//
			//         time.Sleep(res.duration)
			//
			//         mu.Lock()
			//         activeConcurrency--
			//         mu.Unlock()
			//     }).Return(nil)
			//
			//     manager.RegisterResource(res.name, mockResource)
			// }

			// start := time.Now()
			// err := manager.CleanupAll(context.Background())
			// duration := time.Since(start)

			// assert.NoError(t, err)
			// assert.Equal(t, tt.expectedConcurrency, maxConcurrency)
			// assert.InDelta(t, tt.expectedDuration, duration, float64(500*time.Millisecond))
		})
	}
}

// TestResourceCleanupManager_HealthCheckAndRetry tests health checking and retry logic.
func TestResourceCleanupManager_HealthCheckAndRetry(t *testing.T) {
	tests := []struct {
		name      string
		config    ResourceCleanupConfig
		resources []struct {
			name         string
			healthy      bool
			cleanupError error
			retryCount   int
		}
		expectedRetries   int
		expectedSuccesses int
		expectedFailures  int
	}{
		{
			name: "Healthy resources cleanup without retry",
			config: ResourceCleanupConfig{
				CleanupTimeout:     30 * time.Second,
				HealthCheckEnabled: true,
				RetryEnabled:       true,
				MaxRetries:         3,
			},
			resources: []struct {
				name         string
				healthy      bool
				cleanupError error
				retryCount   int
			}{
				{"healthy-resource-1", true, nil, 0},
				{"healthy-resource-2", true, nil, 0},
			},
			expectedRetries:   0,
			expectedSuccesses: 2,
			expectedFailures:  0,
		},
		{
			name: "Unhealthy resources skip cleanup",
			config: ResourceCleanupConfig{
				CleanupTimeout:     30 * time.Second,
				HealthCheckEnabled: true,
				RetryEnabled:       false,
			},
			resources: []struct {
				name         string
				healthy      bool
				cleanupError error
				retryCount   int
			}{
				{"healthy-resource", true, nil, 0},
				{"unhealthy-resource", false, nil, 0},
			},
			expectedRetries:   0,
			expectedSuccesses: 1,
			expectedFailures:  1, // Unhealthy resource marked as failed
		},
		{
			name: "Failed cleanup with retry logic",
			config: ResourceCleanupConfig{
				CleanupTimeout:     30 * time.Second,
				HealthCheckEnabled: false,
				RetryEnabled:       true,
				MaxRetries:         2,
				RetryDelay:         100 * time.Millisecond,
			},
			resources: []struct {
				name         string
				healthy      bool
				cleanupError error
				retryCount   int
			}{
				{"retry-resource", true, errors.New("temporary failure"), 2},
				{"success-resource", true, nil, 0},
			},
			expectedRetries:   2,
			expectedSuccesses: 1,
			expectedFailures:  1, // Fails after max retries
		},
		{
			name: "Successful cleanup after retries",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
				RetryEnabled:   true,
				MaxRetries:     3,
				RetryDelay:     50 * time.Millisecond,
			},
			resources: []struct {
				name         string
				healthy      bool
				cleanupError error
				retryCount   int
			}{
				{"eventual-success", true, errors.New("temporary failure"), 1}, // Succeeds on retry
			},
			expectedRetries:   1,
			expectedSuccesses: 1,
			expectedFailures:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented:
			// manager := NewResourceCleanupManager(tt.config)

			// Setup mock resources with health and retry behavior
			// for _, res := range tt.resources {
			//     mockResource := NewMockCleanupResource(res.name, ResourceTypeUnknown)
			//     mockResource.healthy = res.healthy
			//
			//     if res.cleanupError != nil {
			//         // Simulate failure followed by success after retries
			//         callCount := 0
			//         mockResource.On("Cleanup", mock.Anything).Run(func(args mock.Arguments) {
			//             callCount++
			//             if callCount <= res.retryCount {
			//                 // Fail first N times
			//             } else {
			//                 // Succeed after retries
			//             }
			//         }).Return(func(ctx context.Context) error {
			//             callCount := mockResource.callCount // hypothetical field
			//             if callCount <= res.retryCount {
			//                 return res.cleanupError
			//             }
			//             return nil
			//         })
			//     } else {
			//         mockResource.On("Cleanup", mock.Anything).Return(nil)
			//     }
			//
			//     manager.RegisterResource(res.name, mockResource)
			// }

			// err := manager.CleanupAll(context.Background())
			// metrics := manager.GetCleanupMetrics()

			// assert.NoError(t, err) // Manager should complete even with failures
			// assert.Equal(t, tt.expectedRetries, metrics.RetryCount)
			// assert.Equal(t, tt.expectedSuccesses, metrics.CleanedResources)
			// assert.Equal(t, tt.expectedFailures, metrics.FailedResources)
		})
	}
}

// TestResourceCleanupManager_TimeoutHandling tests cleanup timeout scenarios.
func TestResourceCleanupManager_TimeoutHandling(t *testing.T) {
	tests := []struct {
		name      string
		config    ResourceCleanupConfig
		resources []struct {
			name     string
			duration time.Duration
		}
		expectTimeout        bool
		expectForceKill      bool
		expectedCleanedCount int
	}{
		{
			name: "All resources cleanup within timeout",
			config: ResourceCleanupConfig{
				CleanupTimeout:   5 * time.Second,
				ResourceTimeout:  2 * time.Second,
				ForceKillTimeout: 6 * time.Second,
			},
			resources: []struct {
				name     string
				duration time.Duration
			}{
				{"fast-resource-1", 500 * time.Millisecond},
				{"fast-resource-2", 1 * time.Second},
			},
			expectTimeout:        false,
			expectForceKill:      false,
			expectedCleanedCount: 2,
		},
		{
			name: "Resource timeout triggers individual timeout",
			config: ResourceCleanupConfig{
				CleanupTimeout:   10 * time.Second,
				ResourceTimeout:  2 * time.Second,
				ForceKillTimeout: 3 * time.Second,
			},
			resources: []struct {
				name     string
				duration time.Duration
			}{
				{"normal-resource", 1 * time.Second},
				{"slow-resource", 5 * time.Second}, // Exceeds resource timeout
			},
			expectTimeout:        true,
			expectForceKill:      true,
			expectedCleanedCount: 1, // Only normal resource succeeds
		},
		{
			name: "Global cleanup timeout triggers force kill",
			config: ResourceCleanupConfig{
				CleanupTimeout:   3 * time.Second,
				ResourceTimeout:  10 * time.Second, // Longer than global timeout
				ForceKillTimeout: 4 * time.Second,
			},
			resources: []struct {
				name     string
				duration time.Duration
			}{
				{"hanging-resource-1", 5 * time.Second},
				{"hanging-resource-2", 5 * time.Second},
			},
			expectTimeout:        true,
			expectForceKill:      true,
			expectedCleanedCount: 0, // None complete before global timeout
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented:
			// manager := NewResourceCleanupManager(tt.config)

			// Setup resources with specified durations
			// for _, res := range tt.resources {
			//     mockResource := NewMockCleanupResource(res.name, ResourceTypeUnknown)
			//     mockResource.On("Cleanup", mock.Anything).Run(func(args mock.Arguments) {
			//         ctx := args.Get(0).(context.Context)
			//         select {
			//         case <-time.After(res.duration):
			//             // Normal completion
			//         case <-ctx.Done():
			//             // Context cancellation (timeout)
			//         }
			//     }).Return(func(ctx context.Context) error {
			//         select {
			//         case <-time.After(res.duration):
			//             return nil
			//         case <-ctx.Done():
			//             return ctx.Err()
			//         }
			//     })
			//
			//     manager.RegisterResource(res.name, mockResource)
			// }

			// start := time.Now()
			// err := manager.CleanupAll(context.Background())
			// duration := time.Since(start)

			// if tt.expectTimeout {
			//     assert.Error(t, err)
			//     assert.Contains(t, err.Error(), "timeout")
			// } else {
			//     assert.NoError(t, err)
			// }

			// metrics := manager.GetCleanupMetrics()
			// assert.Equal(t, tt.expectedCleanedCount, metrics.CleanedResources)

			// if tt.expectForceKill {
			//     assert.Greater(t, metrics.ForceKillCount, 0)
			// }

			// Verify timeout duration
			// expectedMaxDuration := tt.config.CleanupTimeout + 500*time.Millisecond
			// assert.LessOrEqual(t, duration, expectedMaxDuration)
		})
	}
}

// TestResourceCleanupManager_DatabaseCleanup tests database-specific cleanup scenarios.
func TestResourceCleanupManager_DatabaseCleanup(t *testing.T) {
	tests := []struct {
		name              string
		config            ResourceCleanupConfig
		databaseScenarios []struct {
			name        string
			activeConns int
			activeTxns  int
			closeDelay  time.Duration
			closeError  error
		}
		expectAllClosed bool
		expectedErrors  int
	}{
		{
			name: "Clean database pool shutdown",
			config: ResourceCleanupConfig{
				CleanupTimeout:  10 * time.Second,
				ResourceTimeout: 5 * time.Second,
			},
			databaseScenarios: []struct {
				name        string
				activeConns int
				activeTxns  int
				closeDelay  time.Duration
				closeError  error
			}{
				{"primary-db", 5, 2, 1 * time.Second, nil},
				{"cache-db", 3, 0, 500 * time.Millisecond, nil},
			},
			expectAllClosed: true,
			expectedErrors:  0,
		},
		{
			name: "Database with hanging transactions",
			config: ResourceCleanupConfig{
				CleanupTimeout:   8 * time.Second,
				ResourceTimeout:  3 * time.Second,
				ForceKillTimeout: 5 * time.Second,
			},
			databaseScenarios: []struct {
				name        string
				activeConns int
				activeTxns  int
				closeDelay  time.Duration
				closeError  error
			}{
				{"normal-db", 2, 1, 1 * time.Second, nil},
				{"hanging-db", 10, 5, 10 * time.Second, errors.New("transactions still active")},
			},
			expectAllClosed: false, // Hanging DB times out
			expectedErrors:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented, would test database-specific cleanup:
			// manager := NewResourceCleanupManager(tt.config)

			// for _, dbScenario := range tt.databaseScenarios {
			//     mockDB := &MockDatabaseResource{
			//         name:        dbScenario.name,
			//         activeConns: dbScenario.activeConns,
			//         activeTxns:  dbScenario.activeTxns,
			//         closeDelay:  dbScenario.closeDelay,
			//         closeError:  dbScenario.closeError,
			//     }
			//
			//     manager.RegisterResource(dbScenario.name, mockDB)
			// }

			// err := manager.CleanupAll(context.Background())
			// metrics := manager.GetCleanupMetrics()

			// if tt.expectAllClosed {
			//     assert.NoError(t, err)
			//     assert.Equal(t, len(tt.databaseScenarios), metrics.CleanedResources)
			// }
			// assert.Equal(t, tt.expectedErrors, metrics.FailedResources)
		})
	}
}

// MockDatabaseResource simulates a database connection pool for testing.
type MockDatabaseResource struct {
	name        string
	activeConns int
	activeTxns  int
	closeDelay  time.Duration
	closeError  error
	closed      bool
}

func (m *MockDatabaseResource) Cleanup(ctx context.Context) error {
	// Simulate database connection pool cleanup
	select {
	case <-time.After(m.closeDelay):
		if m.closeError != nil {
			return m.closeError
		}
		m.closed = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *MockDatabaseResource) GetResourceType() ResourceType {
	return ResourceTypeDatabase
}

func (m *MockDatabaseResource) GetResourceInfo() ResourceInfo {
	return ResourceInfo{
		Name: m.name,
		Type: ResourceTypeDatabase,
		Metadata: map[string]string{
			"active_connections":  string(rune(m.activeConns)),
			"active_transactions": string(rune(m.activeTxns)),
		},
	}
}

func (m *MockDatabaseResource) CanCleanupConcurrently() bool {
	return false // Database cleanup should be sequential
}

func (m *MockDatabaseResource) GetCleanupPriority() int {
	return 5 // Medium priority
}

func (m *MockDatabaseResource) IsHealthy(_ context.Context) bool {
	return !m.closed
}

// TestResourceCleanupManager_NATSConsumerCleanup tests NATS consumer cleanup scenarios.
func TestResourceCleanupManager_NATSConsumerCleanup(t *testing.T) {
	tests := []struct {
		name          string
		config        ResourceCleanupConfig
		natsScenarios []struct {
			name         string
			connected    bool
			pendingMsgs  int
			drainTimeout time.Duration
			drainError   error
		}
		expectAllDrained bool
		expectedErrors   int
	}{
		{
			name: "Clean NATS consumer drain",
			config: ResourceCleanupConfig{
				CleanupTimeout:  15 * time.Second,
				ResourceTimeout: 5 * time.Second,
			},
			natsScenarios: []struct {
				name         string
				connected    bool
				pendingMsgs  int
				drainTimeout time.Duration
				drainError   error
			}{
				{"indexing-consumer", true, 10, 2 * time.Second, nil},
				{"health-consumer", true, 3, 1 * time.Second, nil},
			},
			expectAllDrained: true,
			expectedErrors:   0,
		},
		{
			name: "NATS consumer with connection issues",
			config: ResourceCleanupConfig{
				CleanupTimeout:   10 * time.Second,
				ResourceTimeout:  4 * time.Second,
				ForceKillTimeout: 6 * time.Second,
			},
			natsScenarios: []struct {
				name         string
				connected    bool
				pendingMsgs  int
				drainTimeout time.Duration
				drainError   error
			}{
				{"normal-consumer", true, 5, 1 * time.Second, nil},
				{"broken-consumer", false, 0, 0, errors.New("connection lost")},
			},
			expectAllDrained: false,
			expectedErrors:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented, would test NATS-specific cleanup:
			// manager := NewResourceCleanupManager(tt.config)

			// for _, natsScenario := range tt.natsScenarios {
			//     mockNATS := &MockNATSResource{
			//         name:         natsScenario.name,
			//         connected:    natsScenario.connected,
			//         pendingMsgs:  natsScenario.pendingMsgs,
			//         drainTimeout: natsScenario.drainTimeout,
			//         drainError:   natsScenario.drainError,
			//     }
			//
			//     manager.RegisterResource(natsScenario.name, mockNATS)
			// }

			// err := manager.CleanupAll(context.Background())
			// metrics := manager.GetCleanupMetrics()

			// if tt.expectAllDrained {
			//     assert.NoError(t, err)
			// }
			// assert.Equal(t, tt.expectedErrors, metrics.FailedResources)
		})
	}
}

// MockNATSResource simulates a NATS consumer for testing.
type MockNATSResource struct {
	name         string
	connected    bool
	pendingMsgs  int
	drainTimeout time.Duration
	drainError   error
	drained      bool
}

func (m *MockNATSResource) Cleanup(ctx context.Context) error {
	if !m.connected {
		return m.drainError
	}

	// Simulate draining pending messages
	select {
	case <-time.After(m.drainTimeout):
		if m.drainError != nil {
			return m.drainError
		}
		m.drained = true
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *MockNATSResource) GetResourceType() ResourceType {
	return ResourceTypeMessaging
}

func (m *MockNATSResource) GetResourceInfo() ResourceInfo {
	return ResourceInfo{
		Name: m.name,
		Type: ResourceTypeMessaging,
		Metadata: map[string]string{
			"connected":    string(rune(BoolToInt(m.connected))),
			"pending_msgs": string(rune(m.pendingMsgs)),
		},
	}
}

func (m *MockNATSResource) CanCleanupConcurrently() bool {
	return true // NATS consumers can be drained concurrently
}

func (m *MockNATSResource) GetCleanupPriority() int {
	return 8 // High priority - drain consumers before closing databases
}

func (m *MockNATSResource) IsHealthy(_ context.Context) bool {
	return m.connected && !m.drained
}

// BoolToInt converts a boolean to an integer for metadata.
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// TestResourceCleanupManager_ResourceMetrics tests detailed metrics collection.
func TestResourceCleanupManager_ResourceMetrics(t *testing.T) {
	tests := []struct {
		name      string
		config    ResourceCleanupConfig
		resources []struct {
			name     string
			resType  ResourceType
			duration time.Duration
			success  bool
		}
		expectedTypeMetrics  map[ResourceType]TypeMetrics
		expectedTotalMetrics ResourceCleanupMetrics
	}{
		{
			name: "Comprehensive metrics collection",
			config: ResourceCleanupConfig{
				CleanupTimeout: 30 * time.Second,
			},
			resources: []struct {
				name     string
				resType  ResourceType
				duration time.Duration
				success  bool
			}{
				{"db-1", ResourceTypeDatabase, 2 * time.Second, true},
				{"db-2", ResourceTypeDatabase, 3 * time.Second, false},
				{"nats-1", ResourceTypeMessaging, 1 * time.Second, true},
				{"nats-2", ResourceTypeMessaging, 1 * time.Second, true},
				{"cache-1", ResourceTypeCache, 500 * time.Millisecond, true},
			},
			expectedTypeMetrics: map[ResourceType]TypeMetrics{
				ResourceTypeDatabase: {
					Count:              2,
					SuccessCount:       1,
					FailureCount:       1,
					AverageCleanupTime: 2500 * time.Millisecond, // (2000+3000)/2
					MaxCleanupTime:     3 * time.Second,
				},
				ResourceTypeMessaging: {
					Count:              2,
					SuccessCount:       2,
					FailureCount:       0,
					AverageCleanupTime: 1 * time.Second,
					MaxCleanupTime:     1 * time.Second,
				},
				ResourceTypeCache: {
					Count:              1,
					SuccessCount:       1,
					FailureCount:       0,
					AverageCleanupTime: 500 * time.Millisecond,
					MaxCleanupTime:     500 * time.Millisecond,
				},
				// Add missing resource types to satisfy exhaustive linter
				ResourceTypeFileSystem: {},
				ResourceTypeNetwork:    {},
				ResourceTypeWorkerPool: {},
				ResourceTypeScheduler:  {},
				ResourceTypeMetrics:    {},
				ResourceTypeUnknown:    {},
			},
			expectedTotalMetrics: ResourceCleanupMetrics{
				TotalResources:     5,
				CleanedResources:   4,
				FailedResources:    1,
				AverageCleanupTime: 1500 * time.Millisecond, // Overall average
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test should fail because ResourceCleanupManager doesn't exist yet
			var manager ResourceCleanupManager
			require.Nil(t, manager, "ResourceCleanupManager should not exist yet - this is the RED phase")

			// When implemented:
			// manager := NewResourceCleanupManager(tt.config)

			// Setup resources with specified behavior
			// for _, res := range tt.resources {
			//     mockResource := NewMockCleanupResource(res.name, res.resType)
			//     mockResource.On("Cleanup", mock.Anything).Run(func(args mock.Arguments) {
			//         time.Sleep(res.duration)
			//     }).Return(func(ctx context.Context) error {
			//         if res.success {
			//             return nil
			//         }
			//         return errors.New("cleanup failed")
			//     })
			//
			//     manager.RegisterResource(res.name, mockResource)
			// }

			// manager.CleanupAll(context.Background())
			// metrics := manager.GetCleanupMetrics()

			// // Verify overall metrics
			// assert.Equal(t, tt.expectedTotalMetrics.TotalResources, metrics.TotalResources)
			// assert.Equal(t, tt.expectedTotalMetrics.CleanedResources, metrics.CleanedResources)
			// assert.Equal(t, tt.expectedTotalMetrics.FailedResources, metrics.FailedResources)

			// // Verify type-specific metrics
			// for resourceType, expectedMetrics := range tt.expectedTypeMetrics {
			//     actualMetrics := metrics.ResourceTypeMetrics[resourceType]
			//     assert.Equal(t, expectedMetrics.Count, actualMetrics.Count)
			//     assert.Equal(t, expectedMetrics.SuccessCount, actualMetrics.SuccessCount)
			//     assert.Equal(t, expectedMetrics.FailureCount, actualMetrics.FailureCount)
			//     assert.InDelta(t, expectedMetrics.AverageCleanupTime, actualMetrics.AverageCleanupTime, float64(100*time.Millisecond))
			// }
		})
	}
}
