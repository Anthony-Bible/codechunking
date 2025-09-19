package service

import (
	"codechunking/internal/application/dto"
	"codechunking/internal/port/outbound"
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// TestSearchService_FiltersByType tests that the SearchService properly filters
// results based on the Types field in the search request.
func TestSearchService_FiltersByType(t *testing.T) {
	tests := []struct {
		name               string
		availableChunks    []ChunkInfo
		searchRequest      dto.SearchRequestDTO
		expectedChunkCount int
		expectedChunkTypes []string
		expectedChunkNames []string
	}{
		{
			name: "FilterByFunction_ReturnsOnlyFunctions",
			availableChunks: []ChunkInfo{
				{
					ChunkID:       uuid.New(),
					Content:       "func calculateTotal() float64 { return 0.0 }",
					Type:          "function",
					EntityName:    "calculateTotal",
					QualifiedName: "com.example.calculateTotal",
					Signature:     "func calculateTotal() float64",
					Visibility:    "public",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:       uuid.New(),
					Content:       "type Calculator struct { precision int }",
					Type:          "class",
					EntityName:    "Calculator",
					QualifiedName: "com.example.Calculator",
					Signature:     "type Calculator struct",
					Visibility:    "public",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:       uuid.New(),
					Content:       "func (c *Calculator) Add(a, b float64) float64 { return a + b }",
					Type:          "method",
					EntityName:    "Add",
					ParentEntity:  "Calculator",
					QualifiedName: "com.example.Calculator.Add",
					Signature:     "func (c *Calculator) Add(a, b float64) float64",
					Visibility:    "public",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query: "calculate",
				Types: []string{"function"}, // Filter by function type only
				Limit: 10,
			},
			expectedChunkCount: 1,
			expectedChunkTypes: []string{"function"},
			expectedChunkNames: []string{"calculateTotal"},
		},
		{
			name: "FilterByMultipleTypes_ReturnsMatchingTypes",
			availableChunks: []ChunkInfo{
				{
					ChunkID:       uuid.New(),
					Content:       "func helper() {}",
					Type:          "function",
					EntityName:    "helper",
					QualifiedName: "com.example.helper",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:       uuid.New(),
					Content:       "type User struct {}",
					Type:          "class",
					EntityName:    "User",
					QualifiedName: "com.example.User",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:       uuid.New(),
					Content:       "func (u *User) GetName() string {}",
					Type:          "method",
					EntityName:    "GetName",
					ParentEntity:  "User",
					QualifiedName: "com.example.User.GetName",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:       uuid.New(),
					Content:       "const MaxSize = 100",
					Type:          "constant",
					EntityName:    "MaxSize",
					QualifiedName: "com.example.MaxSize",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query: "test",
				Types: []string{"function", "class"}, // Filter by multiple types
				Limit: 10,
			},
			expectedChunkCount: 2,
			expectedChunkTypes: []string{"function", "class"},
			expectedChunkNames: []string{"helper", "User"},
		},
		{
			name: "FilterByMethod_ReturnsOnlyMethods",
			availableChunks: []ChunkInfo{
				{
					ChunkID:       uuid.New(),
					Content:       "func globalFunction() {}",
					Type:          "function",
					EntityName:    "globalFunction",
					QualifiedName: "com.example.globalFunction",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:       uuid.New(),
					Content:       "func (c *Calculator) Calculate() float64 {}",
					Type:          "method",
					EntityName:    "Calculate",
					ParentEntity:  "Calculator",
					QualifiedName: "com.example.Calculator.Calculate",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:       uuid.New(),
					Content:       "func (c *Calculator) Validate() bool {}",
					Type:          "method",
					EntityName:    "Validate",
					ParentEntity:  "Calculator",
					QualifiedName: "com.example.Calculator.Validate",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query: "calc",
				Types: []string{"method"}, // Filter by method type only
				Limit: 10,
			},
			expectedChunkCount: 2,
			expectedChunkTypes: []string{"method", "method"},
			expectedChunkNames: []string{"Calculate", "Validate"},
		},
		{
			name: "NoTypeFilter_ReturnsAllTypes",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func test() {}",
					Type:       "function",
					EntityName: "test",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "type Test struct {}",
					Type:       "class",
					EntityName: "Test",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query: "test",
				Types: []string{}, // No type filter - should return all
				Limit: 10,
			},
			expectedChunkCount: 2,
			expectedChunkTypes: []string{"function", "class"},
			expectedChunkNames: []string{"test", "Test"},
		},
		{
			name: "FilterByNonexistentType_ReturnsEmpty",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func test() {}",
					Type:       "function",
					EntityName: "test",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query: "test",
				Types: []string{"interface"}, // Filter by type that doesn't exist in available chunks
				Limit: 10,
			},
			expectedChunkCount: 0,
			expectedChunkTypes: []string{},
			expectedChunkNames: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Create mock dependencies
			mockVectorRepo := new(MockVectorStorageRepository)
			mockEmbeddingService := new(MockEmbeddingService)
			mockChunkRepo := new(MockChunkRepository)

			// Set up mock expectations - this will fail initially since we need to implement proper filtering
			setupMockSearchExpectations(mockVectorRepo, mockEmbeddingService, mockChunkRepo, tt.availableChunks)

			searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)
			ctx := context.Background()

			// Act: Perform search with type filter
			response, err := searchService.Search(ctx, tt.searchRequest)

			// This will fail initially - we need to verify the SearchService implements type filtering
			require.NoError(t, err, "Search should succeed")
			require.NotNil(t, response, "Response should not be nil")

			// Assert: Should return only chunks matching the type filter
			assert.Len(t, response.Results, tt.expectedChunkCount, "Should return expected number of chunks")

			if tt.expectedChunkCount > 0 {
				// Assert: All returned chunks should have expected types
				actualTypes := make([]string, len(response.Results))
				actualNames := make([]string, len(response.Results))
				for i, result := range response.Results {
					actualTypes[i] = result.Type
					actualNames[i] = result.EntityName
				}

				assert.ElementsMatch(
					t,
					tt.expectedChunkTypes,
					actualTypes,
					"Returned chunk types should match expected",
				)
				assert.ElementsMatch(
					t,
					tt.expectedChunkNames,
					actualNames,
					"Returned chunk names should match expected",
				)

				// Assert: Each result should include all type information fields
				for i, result := range response.Results {
					assert.NotEmpty(t, result.Type, "Result %d should have Type field populated", i)
					assert.NotEmpty(t, result.EntityName, "Result %d should have EntityName field populated", i)
					// Note: ParentEntity, QualifiedName, Signature, Visibility can be empty but should be present as fields
					assert.NotNil(t, result.ParentEntity, "Result %d should have ParentEntity field present", i)
					assert.NotNil(t, result.QualifiedName, "Result %d should have QualifiedName field present", i)
					assert.NotNil(t, result.Signature, "Result %d should have Signature field present", i)
					assert.NotNil(t, result.Visibility, "Result %d should have Visibility field present", i)
				}
			}
		})
	}
}

// TestSearchService_FiltersByEntityName tests that the SearchService properly filters
// results based on the EntityName field in the search request.
func TestSearchService_FiltersByEntityName(t *testing.T) {
	tests := []struct {
		name                string
		availableChunks     []ChunkInfo
		searchRequest       dto.SearchRequestDTO
		expectedChunkCount  int
		expectedEntityNames []string
	}{
		{
			name: "FilterByExactEntityName_ReturnsMatching",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func calculateTotal() {}",
					Type:       "function",
					EntityName: "calculateTotal",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func calculateAverage() {}",
					Type:       "function",
					EntityName: "calculateAverage",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func processData() {}",
					Type:       "function",
					EntityName: "processData",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query:      "function",
				EntityName: "calculate", // Should match both functions with "calculate" in name
				Limit:      10,
			},
			expectedChunkCount:  2,
			expectedEntityNames: []string{"calculateTotal", "calculateAverage"},
		},
		{
			name: "FilterByPartialEntityName_ReturnsMatching",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "type User struct {}",
					Type:       "class",
					EntityName: "User",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "type UserManager struct {}",
					Type:       "class",
					EntityName: "UserManager",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "type Product struct {}",
					Type:       "class",
					EntityName: "Product",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query:      "struct",
				EntityName: "User", // Should match both User and UserManager (partial match)
				Limit:      10,
			},
			expectedChunkCount:  2,
			expectedEntityNames: []string{"User", "UserManager"},
		},
		{
			name: "FilterByNonexistentEntityName_ReturnsEmpty",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func test() {}",
					Type:       "function",
					EntityName: "test",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query:      "function",
				EntityName: "nonexistent", // Should match nothing
				Limit:      10,
			},
			expectedChunkCount:  0,
			expectedEntityNames: []string{},
		},
		{
			name: "NoEntityNameFilter_ReturnsAll",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func first() {}",
					Type:       "function",
					EntityName: "first",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func second() {}",
					Type:       "function",
					EntityName: "second",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query:      "function",
				EntityName: "", // No entity name filter - should return all
				Limit:      10,
			},
			expectedChunkCount:  2,
			expectedEntityNames: []string{"first", "second"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Create mock dependencies
			mockVectorRepo := new(MockVectorStorageRepository)
			mockEmbeddingService := new(MockEmbeddingService)
			mockChunkRepo := new(MockChunkRepository)

			setupMockSearchExpectations(mockVectorRepo, mockEmbeddingService, mockChunkRepo, tt.availableChunks)

			searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)
			ctx := context.Background()

			// Act: Perform search with entity name filter
			response, err := searchService.Search(ctx, tt.searchRequest)

			// This will fail initially - we need to verify the SearchService implements entity name filtering
			require.NoError(t, err, "Search should succeed")

			// Assert: Should return only chunks matching the entity name filter
			assert.Len(t, response.Results, tt.expectedChunkCount, "Should return expected number of chunks")

			if tt.expectedChunkCount > 0 {
				actualEntityNames := make([]string, len(response.Results))
				for i, result := range response.Results {
					actualEntityNames[i] = result.EntityName
				}
				assert.ElementsMatch(
					t,
					tt.expectedEntityNames,
					actualEntityNames,
					"Returned entity names should match expected",
				)
			}
		})
	}
}

// TestSearchService_FiltersByVisibility tests that the SearchService properly filters
// results based on the Visibility field in the search request.
func TestSearchService_FiltersByVisibility(t *testing.T) {
	tests := []struct {
		name                 string
		availableChunks      []ChunkInfo
		searchRequest        dto.SearchRequestDTO
		expectedChunkCount   int
		expectedVisibilities []string
		expectedEntityNames  []string
	}{
		{
			name: "FilterByPublicVisibility_ReturnsOnlyPublic",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func PublicFunction() {}",
					Type:       "function",
					EntityName: "PublicFunction",
					Visibility: "public",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func privateFunction() {}",
					Type:       "function",
					EntityName: "privateFunction",
					Visibility: "private",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func ProtectedFunction() {}",
					Type:       "function",
					EntityName: "ProtectedFunction",
					Visibility: "protected",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query:      "function",
				Visibility: []string{"public"}, // Filter by public visibility only
				Limit:      10,
			},
			expectedChunkCount:   1,
			expectedVisibilities: []string{"public"},
			expectedEntityNames:  []string{"PublicFunction"},
		},
		{
			name: "FilterByMultipleVisibilities_ReturnsMatching",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func PublicMethod() {}",
					Type:       "method",
					EntityName: "PublicMethod",
					Visibility: "public",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func privateMethod() {}",
					Type:       "method",
					EntityName: "privateMethod",
					Visibility: "private",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func ProtectedMethod() {}",
					Type:       "method",
					EntityName: "ProtectedMethod",
					Visibility: "protected",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func InternalMethod() {}",
					Type:       "method",
					EntityName: "InternalMethod",
					Visibility: "internal",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query:      "method",
				Visibility: []string{"public", "private"}, // Filter by multiple visibilities
				Limit:      10,
			},
			expectedChunkCount:   2,
			expectedVisibilities: []string{"public", "private"},
			expectedEntityNames:  []string{"PublicMethod", "privateMethod"},
		},
		{
			name: "NoVisibilityFilter_ReturnsAll",
			availableChunks: []ChunkInfo{
				{
					ChunkID:    uuid.New(),
					Content:    "func Public() {}",
					Type:       "function",
					EntityName: "Public",
					Visibility: "public",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
				{
					ChunkID:    uuid.New(),
					Content:    "func private() {}",
					Type:       "function",
					EntityName: "private",
					Visibility: "private",
					Repository: dto.RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo",
					},
				},
			},
			searchRequest: dto.SearchRequestDTO{
				Query:      "function",
				Visibility: []string{}, // No visibility filter - should return all
				Limit:      10,
			},
			expectedChunkCount:   2,
			expectedVisibilities: []string{"public", "private"},
			expectedEntityNames:  []string{"Public", "private"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Create mock dependencies
			mockVectorRepo := new(MockVectorStorageRepository)
			mockEmbeddingService := new(MockEmbeddingService)
			mockChunkRepo := new(MockChunkRepository)

			setupMockSearchExpectations(mockVectorRepo, mockEmbeddingService, mockChunkRepo, tt.availableChunks)

			searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)
			ctx := context.Background()

			// Act: Perform search with visibility filter
			response, err := searchService.Search(ctx, tt.searchRequest)

			// This will fail initially - we need to verify the SearchService implements visibility filtering
			require.NoError(t, err, "Search should succeed")

			// Assert: Should return only chunks matching the visibility filter
			assert.Len(t, response.Results, tt.expectedChunkCount, "Should return expected number of chunks")

			if tt.expectedChunkCount > 0 {
				actualVisibilities := make([]string, len(response.Results))
				actualEntityNames := make([]string, len(response.Results))
				for i, result := range response.Results {
					actualVisibilities[i] = result.Visibility
					actualEntityNames[i] = result.EntityName
				}
				assert.ElementsMatch(
					t,
					tt.expectedVisibilities,
					actualVisibilities,
					"Returned visibilities should match expected",
				)
				assert.ElementsMatch(
					t,
					tt.expectedEntityNames,
					actualEntityNames,
					"Returned entity names should match expected",
				)
			}
		})
	}
}

// TestSearchService_CombinedTypeFilters tests that the SearchService properly applies
// multiple type-based filters together (Types + EntityName + Visibility).
func TestSearchService_CombinedTypeFilters(t *testing.T) {
	// Arrange: Create diverse chunks with different type information
	availableChunks := []ChunkInfo{
		{
			ChunkID:       uuid.New(),
			Content:       "func PublicCalculate() float64 {}",
			Type:          "function",
			EntityName:    "PublicCalculate",
			ParentEntity:  "",
			QualifiedName: "com.example.PublicCalculate",
			Signature:     "func PublicCalculate() float64",
			Visibility:    "public",
			Repository:    dto.RepositoryInfo{ID: uuid.New(), Name: "test-repo", URL: "https://github.com/test/repo"},
		},
		{
			ChunkID:       uuid.New(),
			Content:       "func privateCalculate() float64 {}",
			Type:          "function",
			EntityName:    "privateCalculate",
			ParentEntity:  "",
			QualifiedName: "com.example.privateCalculate",
			Signature:     "func privateCalculate() float64",
			Visibility:    "private",
			Repository:    dto.RepositoryInfo{ID: uuid.New(), Name: "test-repo", URL: "https://github.com/test/repo"},
		},
		{
			ChunkID:       uuid.New(),
			Content:       "func (c *Calculator) PublicAdd() float64 {}",
			Type:          "method",
			EntityName:    "PublicAdd",
			ParentEntity:  "Calculator",
			QualifiedName: "com.example.Calculator.PublicAdd",
			Signature:     "func (c *Calculator) PublicAdd() float64",
			Visibility:    "public",
			Repository:    dto.RepositoryInfo{ID: uuid.New(), Name: "test-repo", URL: "https://github.com/test/repo"},
		},
		{
			ChunkID:       uuid.New(),
			Content:       "func PublicProcess() {}",
			Type:          "function",
			EntityName:    "PublicProcess",
			ParentEntity:  "",
			QualifiedName: "com.example.PublicProcess",
			Signature:     "func PublicProcess()",
			Visibility:    "public",
			Repository:    dto.RepositoryInfo{ID: uuid.New(), Name: "test-repo", URL: "https://github.com/test/repo"},
		},
	}

	tests := []struct {
		name               string
		searchRequest      dto.SearchRequestDTO
		expectedChunkCount int
		expectedEntityName string
	}{
		{
			name: "FilterByTypeAndEntityNameAndVisibility_ReturnsExactMatch",
			searchRequest: dto.SearchRequestDTO{
				Query:      "calculate",
				Types:      []string{"function"}, // Only functions
				EntityName: "Calculate",          // Name contains "Calculate"
				Visibility: []string{"public"},   // Only public visibility
				Limit:      10,
			},
			expectedChunkCount: 1,
			expectedEntityName: "PublicCalculate", // Only this matches all criteria
		},
		{
			name: "FilterByTypeAndVisibility_ReturnsMultiple",
			searchRequest: dto.SearchRequestDTO{
				Query:      "function",
				Types:      []string{"function"}, // Only functions
				Visibility: []string{"public"},   // Only public visibility
				Limit:      10,
			},
			expectedChunkCount: 2, // PublicCalculate and PublicProcess
		},
		{
			name: "FilterByEntityNameOnly_ReturnsMatching",
			searchRequest: dto.SearchRequestDTO{
				Query:      "code",
				EntityName: "Public", // Name contains "Public"
				Limit:      10,
			},
			expectedChunkCount: 3, // PublicCalculate, PublicAdd, PublicProcess
		},
		{
			name: "ConflictingFilters_ReturnsEmpty",
			searchRequest: dto.SearchRequestDTO{
				Query:      "function",
				Types:      []string{"method"}, // Only methods
				EntityName: "Calculate",        // Name contains "Calculate"
				Visibility: []string{"public"}, // Only public
				Limit:      10,
			},
			expectedChunkCount: 0, // No functions with "Calculate" that are methods
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: Create mock dependencies
			mockVectorRepo := new(MockVectorStorageRepository)
			mockEmbeddingService := new(MockEmbeddingService)
			mockChunkRepo := new(MockChunkRepository)

			setupMockSearchExpectations(mockVectorRepo, mockEmbeddingService, mockChunkRepo, availableChunks)

			searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)
			ctx := context.Background()

			// Act: Perform search with combined filters
			response, err := searchService.Search(ctx, tt.searchRequest)

			// This will fail initially - we need to verify the SearchService implements combined filtering
			require.NoError(t, err, "Search should succeed")

			// Assert: Should return only chunks matching all specified filters
			assert.Len(t, response.Results, tt.expectedChunkCount, "Should return expected number of chunks")

			if tt.expectedChunkCount == 1 && tt.expectedEntityName != "" {
				assert.Equal(
					t,
					tt.expectedEntityName,
					response.Results[0].EntityName,
					"Should return the expected entity",
				)
			}

			// Assert: All returned results should include complete type information
			for i, result := range response.Results {
				assert.NotEmpty(t, result.Type, "Result %d should have Type", i)
				assert.NotEmpty(t, result.EntityName, "Result %d should have EntityName", i)
				assert.NotNil(t, result.ParentEntity, "Result %d should have ParentEntity field", i)
				assert.NotEmpty(t, result.QualifiedName, "Result %d should have QualifiedName", i)
				assert.NotEmpty(t, result.Signature, "Result %d should have Signature", i)
				assert.NotEmpty(t, result.Visibility, "Result %d should have Visibility", i)
			}
		})
	}
}

// TestSearchService_TypeInformationInResults tests that search results include
// all enhanced type information fields in the response.
func TestSearchService_TypeInformationInResults(t *testing.T) {
	// Arrange: Create chunks with comprehensive type information
	testChunks := []ChunkInfo{
		{
			ChunkID:       uuid.New(),
			Content:       "func (c *Calculator) CalculateTotal(items []Item) float64 { return 0.0 }",
			Type:          "method",
			EntityName:    "CalculateTotal",
			ParentEntity:  "Calculator",
			QualifiedName: "com.example.Calculator.CalculateTotal",
			Signature:     "func (c *Calculator) CalculateTotal(items []Item) float64",
			Visibility:    "public",
			Repository:    dto.RepositoryInfo{ID: uuid.New(), Name: "test-repo", URL: "https://github.com/test/repo"},
			FilePath:      "/src/calculator.go",
			Language:      "go",
			StartLine:     10,
			EndLine:       15,
		},
	}

	// Arrange: Create mock dependencies
	mockVectorRepo := new(MockVectorStorageRepository)
	mockEmbeddingService := new(MockEmbeddingService)
	mockChunkRepo := new(MockChunkRepository)

	setupMockSearchExpectations(mockVectorRepo, mockEmbeddingService, mockChunkRepo, testChunks)

	searchService := NewSearchService(mockVectorRepo, mockEmbeddingService, mockChunkRepo)
	ctx := context.Background()

	searchRequest := dto.SearchRequestDTO{
		Query: "calculate",
		Limit: 10,
	}

	// Act: Perform search
	response, err := searchService.Search(ctx, searchRequest)

	// This will fail initially - we need to verify the SearchService includes type information in results
	require.NoError(t, err, "Search should succeed")
	require.NotNil(t, response, "Response should not be nil")
	require.Len(t, response.Results, 1, "Should return one result")

	result := response.Results[0]

	// Assert: All type information fields should be present and populated
	assert.Equal(t, "method", result.Type, "Type should be preserved in search result")
	assert.Equal(t, "CalculateTotal", result.EntityName, "EntityName should be preserved in search result")
	assert.Equal(t, "Calculator", result.ParentEntity, "ParentEntity should be preserved in search result")
	assert.Equal(
		t,
		"com.example.Calculator.CalculateTotal",
		result.QualifiedName,
		"QualifiedName should be preserved in search result",
	)
	assert.Equal(
		t,
		"func (c *Calculator) CalculateTotal(items []Item) float64",
		result.Signature,
		"Signature should be preserved in search result",
	)
	assert.Equal(t, "public", result.Visibility, "Visibility should be preserved in search result")

	// Assert: Basic search result fields should also be present
	assert.NotEmpty(t, result.ChunkID, "ChunkID should be present")
	assert.NotEmpty(t, result.Content, "Content should be present")
	assert.Positive(t, result.SimilarityScore, "SimilarityScore should be present")
	assert.Equal(t, "/src/calculator.go", result.FilePath, "FilePath should be preserved")
	assert.Equal(t, "go", result.Language, "Language should be preserved")
	assert.Equal(t, 10, result.StartLine, "StartLine should be preserved")
	assert.Equal(t, 15, result.EndLine, "EndLine should be preserved")
}

// Helper function to set up mock expectations - this will fail initially since we need to
// implement proper mock behavior that respects type filtering.
func setupMockSearchExpectations(
	mockVectorRepo *MockVectorStorageRepository,
	mockEmbeddingService *MockEmbeddingService,
	mockChunkRepo *MockChunkRepository,
	availableChunks []ChunkInfo,
) {
	// This function will fail initially because we need to implement the filtering logic
	// in the actual SearchService. For now, we'll set up basic expectations that return
	// all available chunks, and the filtering should happen in the service.

	// Mock embedding generation
	queryVector := []float64{0.1, 0.2, 0.3, 0.4, 0.5}
	mockEmbedding := &outbound.EmbeddingResult{
		Vector:     queryVector,
		Dimensions: len(queryVector),
		Model:      "gemini-embedding-001",
	}
	mockEmbeddingService.On("GenerateEmbedding", mock.Anything, mock.Anything, mock.AnythingOfType("outbound.EmbeddingOptions")).
		Return(mockEmbedding, nil)

	// Create vector results from available chunks
	vectorResults := make([]outbound.VectorSimilarityResult, len(availableChunks))
	chunkIDs := make([]uuid.UUID, len(availableChunks))
	for i, chunk := range availableChunks {
		vectorResults[i] = outbound.VectorSimilarityResult{
			Embedding: outbound.VectorEmbedding{
				ChunkID: chunk.ChunkID,
			},
			Similarity: 0.9 - float64(i)*0.1, // Decreasing similarity
			Distance:   0.1 + float64(i)*0.1,
			Rank:       i + 1,
		}
		chunkIDs[i] = chunk.ChunkID
	}

	// Mock vector similarity search
	mockVectorRepo.On("VectorSimilaritySearch", mock.Anything, queryVector, mock.AnythingOfType("outbound.SimilaritySearchOptions")).
		Return(vectorResults, nil)

	// Mock chunk retrieval
	mockChunkRepo.On("FindChunksByIDs", mock.Anything, chunkIDs).
		Return(availableChunks, nil)
}
