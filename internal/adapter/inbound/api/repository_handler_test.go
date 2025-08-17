package api

import (
	"codechunking/internal/adapter/inbound/api/testutil"
	"codechunking/internal/application/dto"
	"codechunking/internal/domain/errors/domain"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryHandler_CreateRepository(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    interface{}
		mockSetup      func(*testutil.MockRepositoryService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name: "successful_creation_returns_202_accepted",
			requestBody: testutil.NewCreateRepositoryRequestBuilder().
				WithURL("https://github.com/golang/go").
				WithName("golang/go").
				Build(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().
					WithURL("https://github.com/golang/go").
					WithName("golang/go").
					WithStatus("pending").
					Build()
				mock.ExpectCreateRepository(&response, nil)
			},
			expectedStatus: http.StatusAccepted,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.RepositoryResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, "https://github.com/golang/go", response.URL)
				assert.Equal(t, "golang/go", response.Name)
				assert.Equal(t, "pending", response.Status)
				assert.NotEmpty(t, response.ID)
			},
		},
		{
			name:           "invalid_json_body_returns_400_bad_request",
			requestBody:    `{"invalid": json}`, // Invalid JSON
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error", // MockErrorHandler returns generic validation error
		},
		{
			name: "missing_required_url_returns_400_bad_request",
			requestBody: map[string]interface{}{
				"name": "test-repo",
			},
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name: "empty_url_returns_400_bad_request",
			requestBody: testutil.NewCreateRepositoryRequestBuilder().
				WithURL("").
				Build(),
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name: "repository_already_exists_returns_409_conflict",
			requestBody: testutil.NewCreateRepositoryRequestBuilder().
				WithURL("https://github.com/golang/go").
				Build(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectCreateRepository(nil, domain.ErrRepositoryAlreadyExists)
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "service error", // Will be handled by mock error handler
		},
		{
			name: "service_error_returns_500_internal_server_error",
			requestBody: testutil.NewCreateRepositoryRequestBuilder().
				WithURL("https://github.com/golang/go").
				Build(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectCreateRepository(nil, errors.New("database connection failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "service error",
		},
		{
			name: "full_request_with_all_fields_succeeds",
			requestBody: testutil.NewCreateRepositoryRequestBuilder().
				WithURL("https://github.com/golang/go").
				WithName("golang/go").
				WithDescription("The Go programming language").
				WithDefaultBranch("master").
				Build(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().Build()
				mock.ExpectCreateRepository(&response, nil)
			},
			expectedStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request
			req := testutil.CreateJSONRequest(http.MethodPost, "/repositories", tt.requestBody)
			recorder := httptest.NewRecorder()

			// Execute
			handler.CreateRepository(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}

			// Verify service was called appropriately
			if tt.expectedStatus == http.StatusAccepted {
				assert.Len(t, mockService.CreateRepositoryCalls, 1)
			}
		})
	}
}

func TestRepositoryHandler_GetRepository(t *testing.T) {
	tests := []struct {
		name           string
		repositoryID   string
		mockSetup      func(*testutil.MockRepositoryService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "successful_get_returns_200_with_repository_data",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := testutil.NewRepositoryResponseBuilder().
					WithID(testutil.TestRepositoryID1()).
					WithStatus("completed").
					WithTotalFiles(1250).
					WithTotalChunks(5000).
					Build()
				mock.ExpectGetRepository(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.RepositoryResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, testutil.TestRepositoryID1(), response.ID)
				assert.Equal(t, "completed", response.Status)
				assert.Equal(t, 1250, response.TotalFiles)
				assert.Equal(t, 5000, response.TotalChunks)
			},
		},
		{
			name:           "missing_id_parameter_returns_400_bad_request",
			repositoryID:   "", // Simulates missing mux var
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_uuid_format_returns_400_bad_request",
			repositoryID:   "invalid-uuid",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:         "repository_not_found_returns_404_not_found",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectGetRepository(nil, domain.ErrRepositoryNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "service error",
		},
		{
			name:         "service_error_returns_500_internal_server_error",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectGetRepository(nil, errors.New("database connection failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "service error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request with mux vars
			vars := make(map[string]string)
			if tt.repositoryID != "" {
				vars["id"] = tt.repositoryID
			}
			req := testutil.CreateRequestWithMuxVars(http.MethodGet, "/repositories/"+tt.repositoryID, vars)
			recorder := httptest.NewRecorder()

			// Execute
			handler.GetRepository(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestRepositoryHandler_ListRepositories(t *testing.T) {
	tests := []struct {
		name           string
		queryParams    string
		mockSetup      func(*testutil.MockRepositoryService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:        "successful_list_with_defaults_returns_200",
			queryParams: "",
			mockSetup: func(mock *testutil.MockRepositoryService) {
				repo1 := testutil.NewRepositoryResponseBuilder().
					WithID(testutil.TestRepositoryID1()).
					WithStatus("completed").
					Build()
				repo2 := testutil.NewRepositoryResponseBuilder().
					WithID(testutil.TestRepositoryID2()).
					WithStatus("pending").
					Build()

				response := &dto.RepositoryListResponse{
					Repositories: []dto.RepositoryResponse{repo1, repo2},
					Pagination: dto.PaginationResponse{
						Limit:   20,
						Offset:  0,
						Total:   2,
						HasMore: false,
					},
				}
				mock.ExpectListRepositories(response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.RepositoryListResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Len(t, response.Repositories, 2)
				assert.Equal(t, 20, response.Pagination.Limit)
				assert.Equal(t, 0, response.Pagination.Offset)
				assert.Equal(t, 2, response.Pagination.Total)
				assert.False(t, response.Pagination.HasMore)
			},
		},
		{
			name:        "list_with_query_parameters_returns_filtered_results",
			queryParams: "?status=completed&limit=10&offset=0&sort=created_at:desc",
			mockSetup: func(mock *testutil.MockRepositoryService) {
				repo := testutil.NewRepositoryResponseBuilder().
					WithStatus("completed").
					Build()

				response := &dto.RepositoryListResponse{
					Repositories: []dto.RepositoryResponse{repo},
					Pagination: dto.PaginationResponse{
						Limit:   10,
						Offset:  0,
						Total:   1,
						HasMore: false,
					},
				}
				mock.ExpectListRepositories(response, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:           "invalid_limit_too_high_returns_400_bad_request",
			queryParams:    "?limit=101",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_limit_too_low_returns_400_bad_request",
			queryParams:    "?limit=0",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_offset_negative_returns_400_bad_request",
			queryParams:    "?offset=-1",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:        "service_error_returns_500_internal_server_error",
			queryParams: "",
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectListRepositories(nil, errors.New("database connection failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "service error",
		},
		{
			name:        "pagination_with_has_more_true",
			queryParams: "?limit=1",
			mockSetup: func(mock *testutil.MockRepositoryService) {
				repo := testutil.NewRepositoryResponseBuilder().Build()
				response := &dto.RepositoryListResponse{
					Repositories: []dto.RepositoryResponse{repo},
					Pagination: dto.PaginationResponse{
						Limit:   1,
						Offset:  0,
						Total:   10,
						HasMore: true,
					},
				}
				mock.ExpectListRepositories(response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.RepositoryListResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)
				assert.True(t, response.Pagination.HasMore)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request
			req := testutil.CreateRequest(http.MethodGet, "/repositories"+tt.queryParams)
			recorder := httptest.NewRecorder()

			// Execute
			handler.ListRepositories(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestRepositoryHandler_DeleteRepository(t *testing.T) {
	tests := []struct {
		name           string
		repositoryID   string
		mockSetup      func(*testutil.MockRepositoryService)
		expectedStatus int
		expectedError  string
	}{
		{
			name:         "successful_delete_returns_204_no_content",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectDeleteRepository(nil)
			},
			expectedStatus: http.StatusNoContent,
		},
		{
			name:           "missing_id_parameter_returns_400_bad_request",
			repositoryID:   "",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_uuid_format_returns_400_bad_request",
			repositoryID:   "invalid-uuid",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:         "repository_not_found_returns_404_not_found",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectDeleteRepository(domain.ErrRepositoryNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "service error",
		},
		{
			name:         "repository_processing_returns_409_conflict",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectDeleteRepository(domain.ErrRepositoryProcessing)
			},
			expectedStatus: http.StatusConflict,
			expectedError:  "service error",
		},
		{
			name:         "service_error_returns_500_internal_server_error",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectDeleteRepository(errors.New("database connection failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "service error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request with mux vars
			vars := make(map[string]string)
			if tt.repositoryID != "" {
				vars["id"] = tt.repositoryID
			}
			req := testutil.CreateRequestWithMuxVars(http.MethodDelete, "/repositories/"+tt.repositoryID, vars)
			recorder := httptest.NewRecorder()

			// Execute
			handler.DeleteRepository(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			// For successful deletes, body should be empty
			if tt.expectedStatus == http.StatusNoContent {
				assert.Empty(t, recorder.Body.String())
			}
		})
	}
}

func TestRepositoryHandler_GetRepositoryJobs(t *testing.T) {
	tests := []struct {
		name           string
		repositoryID   string
		queryParams    string
		mockSetup      func(*testutil.MockRepositoryService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "successful_get_jobs_returns_200_with_jobs_list",
			repositoryID: testutil.TestRepositoryID1().String(),
			queryParams:  "",
			mockSetup: func(mock *testutil.MockRepositoryService) {
				job1 := testutil.NewIndexingJobResponseBuilder().
					WithID(testutil.TestJobID1()).
					WithRepositoryID(testutil.TestRepositoryID1()).
					WithStatus("completed").
					WithFilesProcessed(1250).
					WithChunksCreated(5000).
					Build()

				response := &dto.IndexingJobListResponse{
					Jobs: []dto.IndexingJobResponse{job1},
					Pagination: dto.PaginationResponse{
						Limit:   10,
						Offset:  0,
						Total:   1,
						HasMore: false,
					},
				}
				mock.ExpectGetRepositoryJobs(response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.IndexingJobListResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Len(t, response.Jobs, 1)
				assert.Equal(t, testutil.TestJobID1(), response.Jobs[0].ID)
				assert.Equal(t, testutil.TestRepositoryID1(), response.Jobs[0].RepositoryID)
				assert.Equal(t, "completed", response.Jobs[0].Status)
			},
		},
		{
			name:           "missing_repository_id_returns_400_bad_request",
			repositoryID:   "",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_repository_uuid_returns_400_bad_request",
			repositoryID:   "invalid-uuid",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:         "repository_not_found_returns_404_not_found",
			repositoryID: testutil.TestRepositoryID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectGetRepositoryJobs(nil, domain.ErrRepositoryNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "service error",
		},
		{
			name:           "invalid_limit_exceeds_maximum_returns_400_bad_request",
			repositoryID:   testutil.TestRepositoryID1().String(),
			queryParams:    "?limit=51",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:         "jobs_with_pagination_parameters",
			repositoryID: testutil.TestRepositoryID1().String(),
			queryParams:  "?limit=5&offset=10",
			mockSetup: func(mock *testutil.MockRepositoryService) {
				response := &dto.IndexingJobListResponse{
					Jobs: []dto.IndexingJobResponse{},
					Pagination: dto.PaginationResponse{
						Limit:   5,
						Offset:  10,
						Total:   15,
						HasMore: false,
					},
				}
				mock.ExpectGetRepositoryJobs(response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				var response dto.IndexingJobListResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, 5, response.Pagination.Limit)
				assert.Equal(t, 10, response.Pagination.Offset)
				assert.Equal(t, 15, response.Pagination.Total)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request with mux vars
			vars := make(map[string]string)
			if tt.repositoryID != "" {
				vars["id"] = tt.repositoryID
			}
			req := testutil.CreateRequestWithMuxVars(
				http.MethodGet,
				"/repositories/"+tt.repositoryID+"/jobs"+tt.queryParams,
				vars,
			)
			recorder := httptest.NewRecorder()

			// Execute
			handler.GetRepositoryJobs(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestRepositoryHandler_GetIndexingJob(t *testing.T) {
	tests := []struct {
		name           string
		repositoryID   string
		jobID          string
		mockSetup      func(*testutil.MockRepositoryService)
		expectedStatus int
		expectedError  string
		validateFunc   func(t *testing.T, recorder *httptest.ResponseRecorder)
	}{
		{
			name:         "successful_get_job_returns_200_with_job_details",
			repositoryID: testutil.TestRepositoryID1().String(),
			jobID:        testutil.TestJobID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				startTime := time.Now().Add(-1 * time.Hour)
				endTime := time.Now()

				response := testutil.NewIndexingJobResponseBuilder().
					WithID(testutil.TestJobID1()).
					WithRepositoryID(testutil.TestRepositoryID1()).
					WithStatus("completed").
					WithFilesProcessed(1250).
					WithChunksCreated(5000).
					WithStartedAt(startTime).
					WithCompletedAt(endTime).
					Build()
				mock.ExpectGetIndexingJob(&response, nil)
			},
			expectedStatus: http.StatusOK,
			validateFunc: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				assert.Equal(t, "application/json", recorder.Header().Get("Content-Type"))

				var response dto.IndexingJobResponse
				err := testutil.ParseJSONResponse(recorder, &response)
				require.NoError(t, err)

				assert.Equal(t, testutil.TestJobID1(), response.ID)
				assert.Equal(t, testutil.TestRepositoryID1(), response.RepositoryID)
				assert.Equal(t, "completed", response.Status)
				assert.Equal(t, 1250, response.FilesProcessed)
				assert.Equal(t, 5000, response.ChunksCreated)
				assert.NotNil(t, response.StartedAt)
				assert.NotNil(t, response.CompletedAt)
			},
		},
		{
			name:           "missing_repository_id_returns_400_bad_request",
			repositoryID:   "",
			jobID:          testutil.TestJobID1().String(),
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "missing_job_id_returns_400_bad_request",
			repositoryID:   testutil.TestRepositoryID1().String(),
			jobID:          "",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_repository_uuid_returns_400_bad_request",
			repositoryID:   "invalid-uuid",
			jobID:          testutil.TestJobID1().String(),
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:           "invalid_job_uuid_returns_400_bad_request",
			repositoryID:   testutil.TestRepositoryID1().String(),
			jobID:          "invalid-uuid",
			mockSetup:      func(mock *testutil.MockRepositoryService) {},
			expectedStatus: http.StatusBadRequest,
			expectedError:  "validation error",
		},
		{
			name:         "repository_not_found_returns_404_not_found",
			repositoryID: testutil.TestRepositoryID1().String(),
			jobID:        testutil.TestJobID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectGetIndexingJob(nil, domain.ErrRepositoryNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "service error",
		},
		{
			name:         "job_not_found_returns_404_not_found",
			repositoryID: testutil.TestRepositoryID1().String(),
			jobID:        testutil.TestJobID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectGetIndexingJob(nil, domain.ErrJobNotFound)
			},
			expectedStatus: http.StatusNotFound,
			expectedError:  "service error",
		},
		{
			name:         "service_error_returns_500_internal_server_error",
			repositoryID: testutil.TestRepositoryID1().String(),
			jobID:        testutil.TestJobID1().String(),
			mockSetup: func(mock *testutil.MockRepositoryService) {
				mock.ExpectGetIndexingJob(nil, errors.New("database connection failed"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedError:  "service error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			mockService := testutil.NewMockRepositoryService()
			mockErrorHandler := testutil.NewMockErrorHandler()
			tt.mockSetup(mockService)

			handler := NewRepositoryHandler(mockService, mockErrorHandler)

			// Create request with mux vars
			vars := make(map[string]string)
			if tt.repositoryID != "" {
				vars["id"] = tt.repositoryID
			}
			if tt.jobID != "" {
				vars["job_id"] = tt.jobID
			}
			req := testutil.CreateRequestWithMuxVars(
				http.MethodGet,
				"/repositories/"+tt.repositoryID+"/jobs/"+tt.jobID,
				vars,
			)
			recorder := httptest.NewRecorder()

			// Execute
			handler.GetIndexingJob(recorder, req)

			// Assert
			assert.Equal(t, tt.expectedStatus, recorder.Code)

			if tt.expectedError != "" {
				assert.Contains(t, recorder.Body.String(), tt.expectedError)
			}

			if tt.validateFunc != nil {
				tt.validateFunc(t, recorder)
			}
		})
	}
}

func TestRepositoryHandler_QueryParameterParsing(t *testing.T) {
	t.Run("parseRepositoryListQuery_should_parse_all_parameters", func(t *testing.T) {
		// Setup
		mockService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		// Mock setup to capture the parsed query
		var capturedQuery dto.RepositoryListQuery
		mockService.ListRepositoriesFunc = func(ctx context.Context, query dto.RepositoryListQuery) (*dto.RepositoryListResponse, error) {
			capturedQuery = query
			return &dto.RepositoryListResponse{
				Repositories: []dto.RepositoryResponse{},
				Pagination:   dto.PaginationResponse{},
			}, nil
		}

		handler := NewRepositoryHandler(mockService, mockErrorHandler)

		// Create request with all query parameters
		req := testutil.CreateRequest(http.MethodGet, "/repositories?status=completed&limit=50&offset=20&sort=name:asc")
		recorder := httptest.NewRecorder()

		// Execute
		handler.ListRepositories(recorder, req)

		// Assert query was parsed correctly
		assert.Equal(t, "completed", capturedQuery.Status)
		assert.Equal(t, 50, capturedQuery.Limit)
		assert.Equal(t, 20, capturedQuery.Offset)
		assert.Equal(t, "name:asc", capturedQuery.Sort)
	})

	t.Run("parseIndexingJobListQuery_should_parse_pagination_parameters", func(t *testing.T) {
		// Setup
		mockService := testutil.NewMockRepositoryService()
		mockErrorHandler := testutil.NewMockErrorHandler()

		// Mock setup to capture the parsed query
		var capturedQuery dto.IndexingJobListQuery
		mockService.GetRepositoryJobsFunc = func(ctx context.Context, repositoryID uuid.UUID, query dto.IndexingJobListQuery) (*dto.IndexingJobListResponse, error) {
			capturedQuery = query
			return &dto.IndexingJobListResponse{
				Jobs:       []dto.IndexingJobResponse{},
				Pagination: dto.PaginationResponse{},
			}, nil
		}

		handler := NewRepositoryHandler(mockService, mockErrorHandler)

		// Create request with pagination parameters
		vars := map[string]string{"id": testutil.TestRepositoryID1().String()}
		req := testutil.CreateRequestWithMuxVars(
			http.MethodGet,
			"/repositories/"+testutil.TestRepositoryID1().String()+"/jobs?limit=25&offset=50",
			vars,
		)
		recorder := httptest.NewRecorder()

		// Execute
		handler.GetRepositoryJobs(recorder, req)

		// Assert query was parsed correctly
		assert.Equal(t, 25, capturedQuery.Limit)
		assert.Equal(t, 50, capturedQuery.Offset)
	})
}
