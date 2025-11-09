package dto

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchRequestDTO_Validation verifies both positive and negative validation paths.
func TestSearchRequestDTO_Validation(t *testing.T) {
	tests := []struct {
		name        string
		setupDTO    func() SearchRequestDTO
		expectValid bool
		expectError string
	}{
		{
			name: "Valid_Search_Request_With_All_Fields",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:               "implement authentication middleware",
					Limit:               20,
					Offset:              10,
					RepositoryIDs:       []uuid.UUID{uuid.New(), uuid.New()},
					Languages:           []string{"go", "javascript"},
					FileTypes:           []string{".go", ".js"},
					SimilarityThreshold: 0.8,
					Sort:                "similarity:desc",
				}
			},
			expectValid: true,
		},
		{
			name: "Valid_Search_Request_With_Only_Required_Fields",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query: "search query",
				}
			},
			expectValid: true,
		},
		{
			name: "Invalid_Empty_Query",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query: "",
				}
			},
			expectValid: false,
			expectError: "query is required",
		},
		{
			name: "Invalid_Whitespace_Only_Query",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query: "   ",
				}
			},
			expectValid: false,
			expectError: "query cannot be empty or whitespace only",
		},
		{
			name: "Invalid_Limit_Too_Large",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query: "test query",
					Limit: 101,
				}
			},
			expectValid: false,
			expectError: "limit cannot exceed 100",
		},
		{
			name: "Invalid_Limit_Zero",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query: "test query",
					Limit: 0,
				}
			},
			expectValid: false,
			expectError: "limit must be at least 1",
		},
		{
			name: "Invalid_Negative_Offset",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:  "test query",
					Offset: -1,
				}
			},
			expectValid: false,
			expectError: "offset must be non-negative",
		},
		{
			name: "Invalid_Similarity_Threshold_Too_High",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:               "test query",
					SimilarityThreshold: 1.1,
				}
			},
			expectValid: false,
			expectError: "similarity_threshold must be between 0.0 and 1.0",
		},
		{
			name: "Invalid_Similarity_Threshold_Negative",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:               "test query",
					SimilarityThreshold: -0.1,
				}
			},
			expectValid: false,
			expectError: "similarity_threshold must be between 0.0 and 1.0",
		},
		{
			name: "Invalid_Sort_Option",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query: "test query",
					Sort:  "invalid_sort",
				}
			},
			expectValid: false,
			expectError: "sort must be one of: similarity:desc, similarity:asc, file_path:asc, file_path:desc",
		},
		{
			name: "Invalid_Repository_ID_Empty_UUID",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:         "test query",
					RepositoryIDs: []uuid.UUID{uuid.Nil},
				}
			},
			expectValid: false,
			expectError: "repository_ids cannot contain empty UUIDs",
		},
		{
			name: "Invalid_Empty_Language_String",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:     "test query",
					Languages: []string{"go", ""},
				}
			},
			expectValid: false,
			expectError: "languages cannot contain empty strings",
		},
		{
			name: "Invalid_Empty_File_Type_String",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:     "test query",
					FileTypes: []string{".go", ""},
				}
			},
			expectValid: false,
			expectError: "file_types cannot contain empty strings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dto := tt.setupDTO()

			// This should call the Validate() method on SearchRequestDTO
			err := dto.Validate()

			if tt.expectValid {
				assert.NoError(t, err, "Expected valid DTO to pass validation")
			} else {
				assert.Error(t, err, "Expected invalid DTO to fail validation")
				if tt.expectError != "" {
					assert.Contains(t, err.Error(), tt.expectError, "Error message should contain expected text")
				}
			}
		})
	}
}

// TestSearchRequestDTO_Defaults ensures default values are applied when optional fields are omitted.
func TestSearchRequestDTO_Defaults(t *testing.T) {
	tests := []struct {
		name              string
		input             SearchRequestDTO
		expectedLimit     int
		expectedOffset    int
		expectedThreshold float64
		expectedSort      string
	}{
		{
			name: "Apply_Default_Limit_When_Zero",
			input: SearchRequestDTO{
				Query: "test query",
			},
			expectedLimit:     DefaultSearchLimit,
			expectedOffset:    0,
			expectedThreshold: DefaultSimilarityThreshold,
			expectedSort:      DefaultSearchSort,
		},
		{
			name: "Preserve_Custom_Values",
			input: SearchRequestDTO{
				Query:               "test query",
				Limit:               25,
				Offset:              50,
				SimilarityThreshold: 0.9,
				Sort:                "file_path:asc",
			},
			expectedLimit:     25,
			expectedOffset:    50,
			expectedThreshold: 0.9,
			expectedSort:      "file_path:asc",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// This should call ApplyDefaults() method on SearchRequestDTO
			dto := tc.input
			dto.ApplyDefaults()

			assert.Equal(t, tc.expectedLimit, dto.Limit, "Limit should match expected value")
			assert.Equal(t, tc.expectedOffset, dto.Offset, "Offset should match expected value")
			assert.Equal(
				t,
				tc.expectedThreshold,
				dto.SimilarityThreshold,
				"SimilarityThreshold should match expected value",
			)
			assert.Equal(t, tc.expectedSort, dto.Sort, "Sort should match expected value")
		})
	}
}

// TestSearchRequestDTO_JSONSerialization ensures round-trip JSON marshaling/unmarshaling works.
func TestSearchRequestDTO_JSONSerialization(t *testing.T) {
	originalDTO := SearchRequestDTO{
		Query:               "implement authentication",
		Limit:               15,
		Offset:              30,
		RepositoryIDs:       []uuid.UUID{uuid.New()},
		Languages:           []string{"go", "python"},
		FileTypes:           []string{".go", ".py"},
		SimilarityThreshold: 0.85,
		Sort:                "similarity:asc",
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(originalDTO)
	require.NoError(t, err, "Should marshal SearchRequestDTO to JSON without error")

	// Unmarshal from JSON
	var unmarshaledDTO SearchRequestDTO
	err = json.Unmarshal(jsonData, &unmarshaledDTO)
	require.NoError(t, err, "Should unmarshal JSON to SearchRequestDTO without error")

	// Verify all fields match
	assert.Equal(t, originalDTO.Query, unmarshaledDTO.Query)
	assert.Equal(t, originalDTO.Limit, unmarshaledDTO.Limit)
	assert.Equal(t, originalDTO.Offset, unmarshaledDTO.Offset)
	assert.Equal(t, originalDTO.RepositoryIDs, unmarshaledDTO.RepositoryIDs)
	assert.Equal(t, originalDTO.Languages, unmarshaledDTO.Languages)
	assert.Equal(t, originalDTO.FileTypes, unmarshaledDTO.FileTypes)
	assert.Equal(t, originalDTO.SimilarityThreshold, unmarshaledDTO.SimilarityThreshold)
	assert.Equal(t, originalDTO.Sort, unmarshaledDTO.Sort)
}

// TestSearchResponseDTO verifies the response DTO shape and JSON wiring.
func TestSearchResponseDTO(t *testing.T) {
	t.Run("Valid_Search_Response_Creation", func(t *testing.T) {
		// Create sample search results
		results := []SearchResultDTO{
			{
				ChunkID:         uuid.New(),
				Content:         "func authenticateUser() error { return nil }",
				SimilarityScore: 0.95,
				Repository: RepositoryInfo{
					ID:   uuid.New(),
					Name: "auth-service",
					URL:  "https://github.com/example/auth-service.git",
				},
				FilePath:  "/auth/middleware.go",
				Language:  "go",
				StartLine: 10,
				EndLine:   25,
			},
		}

		pagination := PaginationResponse{
			Limit:   10,
			Offset:  0,
			Total:   42,
			HasMore: true,
		}

		searchMetadata := SearchMetadata{
			Query:           "authentication middleware",
			ExecutionTimeMs: 150,
		}

		response := SearchResponseDTO{
			Results:    results,
			Pagination: pagination,
			Metadata:   searchMetadata,
		}

		// Verify structure is correctly formed
		assert.Len(t, response.Results, 1, "Should have exactly one result")
		assert.Equal(t, 10, response.Pagination.Limit, "Pagination limit should match")
		assert.Equal(t, 42, response.Pagination.Total, "Pagination total should match")
		assert.Equal(t, "authentication middleware", response.Metadata.Query, "Metadata query should match")
		assert.Equal(t, int64(150), response.Metadata.ExecutionTimeMs, "Execution time should match")
	})

	t.Run("Search_Response_JSON_Serialization", func(t *testing.T) {
		response := SearchResponseDTO{
			Results: []SearchResultDTO{
				{
					ChunkID:         uuid.New(),
					Content:         "test content",
					SimilarityScore: 0.8,
					Repository: RepositoryInfo{
						ID:   uuid.New(),
						Name: "test-repo",
						URL:  "https://github.com/test/repo.git",
					},
					FilePath:  "/test.go",
					Language:  "go",
					StartLine: 1,
					EndLine:   10,
				},
			},
			Pagination: PaginationResponse{
				Limit:   10,
				Offset:  0,
				Total:   1,
				HasMore: false,
			},
			Metadata: SearchMetadata{
				Query:           "test query",
				ExecutionTimeMs: 100,
			},
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(response)
		require.NoError(t, err, "Should marshal SearchResponseDTO to JSON")

		// Unmarshal back
		var unmarshaledResponse SearchResponseDTO
		err = json.Unmarshal(jsonData, &unmarshaledResponse)
		require.NoError(t, err, "Should unmarshal JSON to SearchResponseDTO")

		// Verify critical fields
		assert.Len(t, unmarshaledResponse.Results, 1, "Should have one result after unmarshaling")
		assert.Equal(t, response.Results[0].Content, unmarshaledResponse.Results[0].Content)
		assert.Equal(t, response.Pagination.Total, unmarshaledResponse.Pagination.Total)
		assert.Equal(t, response.Metadata.Query, unmarshaledResponse.Metadata.Query)
	})
}

// TestSearchResultDTO covers result DTO validation and helper behaviors.
func TestSearchResultDTO(t *testing.T) {
	t.Run("Valid_Search_Result_Creation", func(t *testing.T) {
		chunkID := uuid.New()
		repoID := uuid.New()

		result := SearchResultDTO{
			ChunkID:         chunkID,
			Content:         "func Process() error { return nil }",
			SimilarityScore: 0.92,
			Repository: RepositoryInfo{
				ID:   repoID,
				Name: "processing-service",
				URL:  "https://github.com/example/processing.git",
			},
			FilePath:  "/internal/processor.go",
			Language:  "go",
			StartLine: 15,
			EndLine:   30,
		}

		// Verify all fields are properly set
		assert.Equal(t, chunkID, result.ChunkID, "ChunkID should match")
		assert.Contains(t, result.Content, "func Process()", "Content should contain function signature")
		assert.Equal(t, 0.92, result.SimilarityScore, "SimilarityScore should match")
		assert.Equal(t, repoID, result.Repository.ID, "Repository ID should match")
		assert.Equal(t, "processing-service", result.Repository.Name, "Repository name should match")
		assert.Equal(t, "/internal/processor.go", result.FilePath, "FilePath should match")
		assert.Equal(t, "go", result.Language, "Language should match")
		assert.Equal(t, 15, result.StartLine, "StartLine should match")
		assert.Equal(t, 30, result.EndLine, "EndLine should match")
	})

	t.Run("Search_Result_Validation", func(t *testing.T) {
		validationTests := []struct {
			name        string
			setupResult func() SearchResultDTO
			expectValid bool
			expectError string
		}{
			{
				name: "Valid_Result",
				setupResult: func() SearchResultDTO {
					return SearchResultDTO{
						ChunkID:         uuid.New(),
						Content:         "valid content",
						SimilarityScore: 0.8,
						Repository: RepositoryInfo{
							ID:   uuid.New(),
							Name: "repo",
							URL:  "https://github.com/test/repo.git",
						},
						FilePath:  "/test.go",
						Language:  "go",
						StartLine: 1,
						EndLine:   10,
					}
				},
				expectValid: true,
			},
			{
				name: "Invalid_Empty_ChunkID",
				setupResult: func() SearchResultDTO {
					return SearchResultDTO{
						ChunkID:         uuid.Nil,
						Content:         "content",
						SimilarityScore: 0.8,
						Repository: RepositoryInfo{
							ID:   uuid.New(),
							Name: "repo",
							URL:  "https://github.com/test/repo.git",
						},
						FilePath:  "/test.go",
						Language:  "go",
						StartLine: 1,
						EndLine:   10,
					}
				},
				expectValid: false,
				expectError: "chunk_id cannot be empty",
			},
			{
				name: "Invalid_Empty_Content",
				setupResult: func() SearchResultDTO {
					return SearchResultDTO{
						ChunkID:         uuid.New(),
						Content:         "",
						SimilarityScore: 0.8,
						Repository: RepositoryInfo{
							ID:   uuid.New(),
							Name: "repo",
							URL:  "https://github.com/test/repo.git",
						},
						FilePath:  "/test.go",
						Language:  "go",
						StartLine: 1,
						EndLine:   10,
					}
				},
				expectValid: false,
				expectError: "content cannot be empty",
			},
			{
				name: "Invalid_Similarity_Score_Out_Of_Range",
				setupResult: func() SearchResultDTO {
					return SearchResultDTO{
						ChunkID:         uuid.New(),
						Content:         "content",
						SimilarityScore: 1.5,
						Repository: RepositoryInfo{
							ID:   uuid.New(),
							Name: "repo",
							URL:  "https://github.com/test/repo.git",
						},
						FilePath:  "/test.go",
						Language:  "go",
						StartLine: 1,
						EndLine:   10,
					}
				},
				expectValid: false,
				expectError: "similarity_score must be between 0.0 and 1.0",
			},
			{
				name: "Invalid_Start_Line_Greater_Than_End_Line",
				setupResult: func() SearchResultDTO {
					return SearchResultDTO{
						ChunkID:         uuid.New(),
						Content:         "content",
						SimilarityScore: 0.8,
						Repository: RepositoryInfo{
							ID:   uuid.New(),
							Name: "repo",
							URL:  "https://github.com/test/repo.git",
						},
						FilePath:  "/test.go",
						Language:  "go",
						StartLine: 20,
						EndLine:   10,
					}
				},
				expectValid: false,
				expectError: "start_line must be less than or equal to end_line",
			},
		}

		for _, tt := range validationTests {
			t.Run(tt.name, func(t *testing.T) {
				result := tt.setupResult()

				// This should call the Validate() method on SearchResultDTO
				err := result.Validate()

				if tt.expectValid {
					assert.NoError(t, err, "Expected valid result to pass validation")
				} else {
					assert.Error(t, err, "Expected invalid result to fail validation")
					if tt.expectError != "" {
						assert.Contains(t, err.Error(), tt.expectError, "Error message should contain expected text")
					}
				}
			})
		}
	})
}

// TestRepositoryInfo validates repository metadata hydration.
func TestRepositoryInfo(t *testing.T) {
	t.Run("Valid_Repository_Info_Creation", func(t *testing.T) {
		repoID := uuid.New()

		info := RepositoryInfo{
			ID:   repoID,
			Name: "test-repository",
			URL:  "https://github.com/example/test-repository.git",
		}

		assert.Equal(t, repoID, info.ID, "Repository ID should match")
		assert.Equal(t, "test-repository", info.Name, "Repository name should match")
		assert.Equal(t, "https://github.com/example/test-repository.git", info.URL, "Repository URL should match")
	})

	t.Run("Repository_Info_Validation", func(t *testing.T) {
		tests := []struct {
			name        string
			setupInfo   func() RepositoryInfo
			expectValid bool
			expectError string
		}{
			{
				name: "Valid_Repository_Info",
				setupInfo: func() RepositoryInfo {
					return RepositoryInfo{
						ID:   uuid.New(),
						Name: "valid-repo",
						URL:  "https://github.com/example/repo.git",
					}
				},
				expectValid: true,
			},
			{
				name: "Invalid_Empty_Repository_ID",
				setupInfo: func() RepositoryInfo {
					return RepositoryInfo{
						ID:   uuid.Nil,
						Name: "repo",
						URL:  "https://github.com/example/repo.git",
					}
				},
				expectValid: false,
				expectError: "repository id cannot be empty",
			},
			{
				name: "Invalid_Empty_Repository_Name",
				setupInfo: func() RepositoryInfo {
					return RepositoryInfo{
						ID:   uuid.New(),
						Name: "",
						URL:  "https://github.com/example/repo.git",
					}
				},
				expectValid: false,
				expectError: "repository name cannot be empty",
			},
			{
				name: "Invalid_Empty_Repository_URL",
				setupInfo: func() RepositoryInfo {
					return RepositoryInfo{
						ID:   uuid.New(),
						Name: "repo",
						URL:  "",
					}
				},
				expectValid: false,
				expectError: "repository url cannot be empty",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				info := tt.setupInfo()

				// This should call the Validate() method on RepositoryInfo
				err := info.Validate()

				if tt.expectValid {
					assert.NoError(t, err, "Expected valid repository info to pass validation")
				} else {
					assert.Error(t, err, "Expected invalid repository info to fail validation")
					if tt.expectError != "" {
						assert.Contains(t, err.Error(), tt.expectError, "Error message should contain expected text")
					}
				}
			})
		}
	})
}

// TestSearchMetadata validates search metadata defaults.
func TestSearchMetadata(t *testing.T) {
	t.Run("Valid_Search_Metadata_Creation", func(t *testing.T) {
		metadata := SearchMetadata{
			Query:           "test search query",
			ExecutionTimeMs: 250,
		}

		assert.Equal(t, "test search query", metadata.Query, "Query should match")
		assert.Equal(t, int64(250), metadata.ExecutionTimeMs, "ExecutionTimeMs should match")
	})

	t.Run("Search_Metadata_With_Timing", func(t *testing.T) {
		startTime := time.Now()

		// Simulate some processing time
		time.Sleep(1 * time.Millisecond)

		endTime := time.Now()
		executionTime := endTime.Sub(startTime).Milliseconds()

		metadata := SearchMetadata{
			Query:           "timing test",
			ExecutionTimeMs: executionTime,
		}

		assert.Positive(t, metadata.ExecutionTimeMs, "ExecutionTimeMs should be positive")
		assert.Equal(t, "timing test", metadata.Query, "Query should match")
	})
}

// TestSearchDTO_Constants ensures the exported constants remain stable.
func TestSearchDTO_Constants(t *testing.T) {
	t.Run("Default_Constants_Should_Exist", func(t *testing.T) {
		// These constants should be defined in the search DTO file
		assert.Equal(t, 10, DefaultSearchLimit, "DefaultSearchLimit should be 10")
		assert.Equal(t, 0.7, DefaultSimilarityThreshold, "DefaultSimilarityThreshold should be 0.7")
		assert.Equal(t, "similarity:desc", DefaultSearchSort, "DefaultSearchSort should be 'similarity:desc'")

		// Maximum limits
		assert.Equal(t, 100, MaxSearchLimit, "MaxSearchLimit should be 100")
	})
}

// TestSearchRequestDTO_RepositoryNamesValidation tests repository name filtering functionality.
func TestSearchRequestDTO_RepositoryNamesValidation(t *testing.T) {
	tests := []struct {
		name        string
		setupDTO    func() SearchRequestDTO
		expectValid bool
		expectError string
	}{
		{
			name: "Valid_Repository_Names_Single",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"golang/go"},
				}
			},
			expectValid: true,
		},
		{
			name: "Valid_Repository_Names_Multiple",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"golang/go", "facebook/react", "microsoft/vscode"},
				}
			},
			expectValid: true,
		},
		{
			name: "Valid_Repository_Names_With_Repository_IDs",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryIDs:   []uuid.UUID{uuid.New()},
					RepositoryNames: []string{"golang/go"},
				}
			},
			expectValid: true,
		},
		{
			name: "Invalid_Repository_Name_Empty_String",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{""},
				}
			},
			expectValid: false,
			expectError: "repository_names cannot contain empty strings",
		},
		{
			name: "Invalid_Repository_Name_Whitespace_Only",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"   "},
				}
			},
			expectValid: false,
			expectError: "repository_names cannot contain empty strings",
		},
		{
			name: "Invalid_Repository_Name_No_Slash",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"golanggo"},
				}
			},
			expectValid: false,
			expectError: "repository name 'golanggo' must be in format 'org/repo'",
		},
		{
			name: "Invalid_Repository_Name_Multiple_Slashes",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"golang/go/extra"},
				}
			},
			expectValid: false,
			expectError: "repository name 'golang/go/extra' must be in format 'org/repo'",
		},
		{
			name: "Invalid_Repository_Name_Starts_With_Slash",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"/golang/go"},
				}
			},
			expectValid: false,
			expectError: "repository name '/golang/go' must be in format 'org/repo'",
		},
		{
			name: "Invalid_Repository_Name_Ends_With_Slash",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"golang/"},
				}
			},
			expectValid: false,
			expectError: "repository name 'golang/' must be in format 'org/repo'",
		},
		{
			name: "Invalid_Repository_Name_Empty_Org",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"/go"},
				}
			},
			expectValid: false,
			expectError: "repository name '/go' must be in format 'org/repo'",
		},
		{
			name: "Invalid_Repository_Name_Empty_Repo",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"golang/"},
				}
			},
			expectValid: false,
			expectError: "repository name 'golang/' must be in format 'org/repo'",
		},
		{
			name: "Invalid_Repository_Names_Too_Many",
			setupDTO: func() SearchRequestDTO {
				names := make([]string, 51) // Exceeds max limit of 50
				for i := range names {
					names[i] = "org/repo"
				}
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: names,
				}
			},
			expectValid: false,
			expectError: "repository_names cannot exceed 50 items",
		},
		{
			name: "Invalid_Repository_Names_Duplicate",
			setupDTO: func() SearchRequestDTO {
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: []string{"golang/go", "golang/go"},
				}
			},
			expectValid: false,
			expectError: "repository_names cannot contain duplicates",
		},
		{
			name: "Valid_Repository_Names_Max_Limit",
			setupDTO: func() SearchRequestDTO {
				names := make([]string, 50) // Exactly at max limit
				for i := range names {
					names[i] = fmt.Sprintf("org%d/repo%d", i, i)
				}
				return SearchRequestDTO{
					Query:           "test query",
					RepositoryNames: names,
				}
			},
			expectValid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dto := tt.setupDTO()

			// This should call the Validate() method on SearchRequestDTO
			err := dto.Validate()

			if tt.expectValid {
				assert.NoError(t, err, "Expected valid DTO to pass validation")
			} else {
				assert.Error(t, err, "Expected invalid DTO to fail validation")
				if tt.expectError != "" {
					assert.Contains(t, err.Error(), tt.expectError, "Error message should contain expected text")
				}
			}
		})
	}
}

// TestSearchRequestDTO_RepositoryNamesJSONSerialization tests JSON serialization with repository names.
func TestSearchRequestDTO_RepositoryNamesJSONSerialization(t *testing.T) {
	t.Run("Repository_Names_JSON_Roundtrip", func(t *testing.T) {
		originalDTO := SearchRequestDTO{
			Query:           "test query",
			RepositoryNames: []string{"golang/go", "facebook/react"},
			Limit:           10,
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(originalDTO)
		require.NoError(t, err, "Should marshal SearchRequestDTO with repository names to JSON")

		// Unmarshal from JSON
		var unmarshaledDTO SearchRequestDTO
		err = json.Unmarshal(jsonData, &unmarshaledDTO)
		require.NoError(t, err, "Should unmarshal JSON to SearchRequestDTO with repository names")

		// Verify repository names field is preserved
		assert.Equal(t, originalDTO.Query, unmarshaledDTO.Query)
		assert.Equal(t, originalDTO.RepositoryNames, unmarshaledDTO.RepositoryNames)
		assert.Equal(t, originalDTO.Limit, unmarshaledDTO.Limit)
	})

	t.Run("Repository_Names_And_Repository_IDs_JSON", func(t *testing.T) {
		originalDTO := SearchRequestDTO{
			Query:           "test query",
			RepositoryIDs:   []uuid.UUID{uuid.New(), uuid.New()},
			RepositoryNames: []string{"golang/go", "facebook/react"},
		}

		// Marshal to JSON
		jsonData, err := json.Marshal(originalDTO)
		require.NoError(t, err, "Should marshal DTO with both repository IDs and names")

		// Unmarshal from JSON
		var unmarshaledDTO SearchRequestDTO
		err = json.Unmarshal(jsonData, &unmarshaledDTO)
		require.NoError(t, err, "Should unmarshal DTO with both repository IDs and names")

		// Verify both fields are preserved
		assert.Equal(t, originalDTO.RepositoryIDs, unmarshaledDTO.RepositoryIDs)
		assert.Equal(t, originalDTO.RepositoryNames, unmarshaledDTO.RepositoryNames)
	})
}
