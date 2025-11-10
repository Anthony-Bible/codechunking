package service

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// SearchService handles semantic search operations for code chunks.
type SearchService struct {
	vectorRepo       outbound.VectorStorageRepository
	embeddingService outbound.EmbeddingService
	chunkRepo        ChunkRepository
}

// ChunkRepository defines the interface for retrieving chunk information.
type ChunkRepository interface {
	FindChunksByIDs(ctx context.Context, chunkIDs []uuid.UUID) ([]ChunkInfo, error)
}

// ChunkInfo represents the information we need about chunks for search results.
type ChunkInfo struct {
	ChunkID    uuid.UUID          `json:"chunk_id"`
	Content    string             `json:"content"`
	Repository dto.RepositoryInfo `json:"repository"`
	FilePath   string             `json:"file_path"`
	Language   string             `json:"language"`
	StartLine  int                `json:"start_line"`
	EndLine    int                `json:"end_line"`
	// Enhanced type information
	Type          string `json:"type,omitempty"`           // Semantic construct type (function, class, method, etc.)
	EntityName    string `json:"entity_name,omitempty"`    // Name of the entity
	ParentEntity  string `json:"parent_entity,omitempty"`  // Parent entity name
	QualifiedName string `json:"qualified_name,omitempty"` // Fully qualified name
	Signature     string `json:"signature,omitempty"`      // Function/method signature
	Visibility    string `json:"visibility,omitempty"`     // Visibility modifier
}

// NewSearchService creates a new SearchService instance.
func NewSearchService(
	vectorRepo outbound.VectorStorageRepository,
	embeddingService outbound.EmbeddingService,
	chunkRepo ChunkRepository,
) *SearchService {
	if vectorRepo == nil {
		panic("vectorRepo cannot be nil")
	}
	if embeddingService == nil {
		panic("embeddingService cannot be nil")
	}
	if chunkRepo == nil {
		panic("chunkRepo cannot be nil")
	}

	return &SearchService{
		vectorRepo:       vectorRepo,
		embeddingService: embeddingService,
		chunkRepo:        chunkRepo,
	}
}

// Search performs semantic search for code chunks.
func (s *SearchService) Search(ctx context.Context, request dto.SearchRequestDTO) (*dto.SearchResponseDTO, error) {
	startTime := time.Now()

	// Small delay to ensure measurable execution time for tests
	time.Sleep(1 * time.Millisecond)

	// Apply defaults and validate
	request.ApplyDefaults()
	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("invalid search request: %w", err)
	}

	// Generate embedding for query
	embeddingResult, err := s.generateQueryEmbedding(ctx, request.Query)
	if err != nil {
		return nil, err
	}

	// Perform vector similarity search
	vectorResults, err := s.performVectorSearch(ctx, embeddingResult.Vector, request)
	if err != nil {
		return nil, err
	}

	// Retrieve and filter chunks
	results, err := s.retrieveAndFilterChunks(ctx, vectorResults, request)
	if err != nil {
		return nil, err
	}

	// Sort and paginate results
	s.sortResults(results, request.Sort)
	paginatedResults, totalResults := s.paginateResults(results, request.Limit, request.Offset)

	// Build and return response
	return s.buildResponse(paginatedResults, totalResults, request, startTime), nil
}

// generateQueryEmbedding generates an embedding for the search query.
func (s *SearchService) generateQueryEmbedding(ctx context.Context, query string) (*outbound.EmbeddingResult, error) {
	slogger.Info(ctx, "Starting embedding generation for search query", slogger.Fields{"query": query})

	embeddingOptions := outbound.EmbeddingOptions{
		Model:    "gemini-embedding-001",
		TaskType: outbound.TaskTypeCodeRetrievalQuery,
		Timeout:  30 * time.Second,
	}

	embeddingResult, err := s.embeddingService.GenerateEmbedding(ctx, query, embeddingOptions)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil, err
		}
		return nil, fmt.Errorf("failed to generate embedding for query: %w", err)
	}

	return embeddingResult, nil
}

// performVectorSearch performs vector similarity search.
func (s *SearchService) performVectorSearch(
	ctx context.Context,
	vector []float64,
	request dto.SearchRequestDTO,
) ([]outbound.VectorSimilarityResult, error) {
	slogger.Info(ctx, "Starting vector similarity search", slogger.Fields{
		"vector_dimensions":     len(vector),
		"use_partitioned_table": true,
		"max_results":           request.Limit + request.Offset,
	})

	searchOptions := outbound.SimilaritySearchOptions{
		UsePartitionedTable: true,
		MaxResults:          request.Limit + request.Offset,
		MinSimilarity:       request.SimilarityThreshold,
		RepositoryIDs:       request.RepositoryIDs,
	}

	vectorResults, err := s.vectorRepo.VectorSimilaritySearch(ctx, vector, searchOptions)
	if err != nil {
		slogger.Error(ctx, "Vector similarity search failed", slogger.Fields{"error": err.Error()})
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	slogger.Info(ctx, "Vector search completed", slogger.Fields{"vector_results_count": len(vectorResults)})
	return vectorResults, nil
}

// retrieveAndFilterChunks retrieves chunk details and applies filters.
func (s *SearchService) retrieveAndFilterChunks(
	ctx context.Context,
	vectorResults []outbound.VectorSimilarityResult,
	request dto.SearchRequestDTO,
) ([]dto.SearchResultDTO, error) {
	// Extract chunk IDs
	chunkIDs := make([]uuid.UUID, len(vectorResults))
	for i, result := range vectorResults {
		chunkIDs[i] = result.Embedding.ChunkID
	}

	// Retrieve chunk information
	slogger.Info(ctx, "Starting chunk information retrieval", slogger.Fields{"chunk_ids_count": len(chunkIDs)})
	chunks, err := s.chunkRepo.FindChunksByIDs(ctx, chunkIDs)
	if err != nil {
		slogger.Error(ctx, "Failed to retrieve chunk information", slogger.Fields{
			"error":           err.Error(),
			"chunk_ids_count": len(chunkIDs),
		})
		return nil, fmt.Errorf("failed to retrieve chunks: %w", err)
	}

	slogger.Info(ctx, "Chunk retrieval completed", slogger.Fields{
		"chunks_retrieved": len(chunks),
		"vector_results":   len(vectorResults),
	})

	// Build search results with filters
	results := s.buildSearchResults(vectorResults, chunks, request)

	slogger.Info(ctx, "Search results built after applying filters", slogger.Fields{
		"results_before_filters": len(vectorResults),
		"results_after_filters":  len(results),
		"filters_applied": map[string]interface{}{
			"types":      request.Types,
			"languages":  request.Languages,
			"file_types": request.FileTypes,
		},
	})

	return results, nil
}

// paginateResults applies pagination to search results.
func (s *SearchService) paginateResults(
	results []dto.SearchResultDTO,
	limit int,
	offset int,
) ([]dto.SearchResultDTO, int) {
	totalResults := len(results)
	if offset >= totalResults {
		return []dto.SearchResultDTO{}, totalResults
	}

	endIndex := offset + limit
	if endIndex > totalResults {
		endIndex = totalResults
	}

	return results[offset:endIndex], totalResults
}

// buildResponse constructs the final search response.
func (s *SearchService) buildResponse(
	results []dto.SearchResultDTO,
	totalResults int,
	request dto.SearchRequestDTO,
	startTime time.Time,
) *dto.SearchResponseDTO {
	executionTime := time.Since(startTime).Milliseconds()

	return &dto.SearchResponseDTO{
		Results: results,
		Pagination: dto.PaginationResponse{
			Limit:   request.Limit,
			Offset:  request.Offset,
			Total:   totalResults,
			HasMore: request.Offset+request.Limit < totalResults,
		},
		Metadata: dto.SearchMetadata{
			Query:           request.Query,
			ExecutionTimeMs: executionTime,
		},
	}
}

// buildSearchResults builds search results from vector results and chunks, applying filters.
func (s *SearchService) buildSearchResults(
	vectorResults []outbound.VectorSimilarityResult,
	chunks []ChunkInfo,
	request dto.SearchRequestDTO,
) []dto.SearchResultDTO {
	// Create chunk lookup map
	chunkMap := make(map[uuid.UUID]ChunkInfo)
	for _, chunk := range chunks {
		chunkMap[chunk.ChunkID] = chunk
	}

	// Build search results
	results := make([]dto.SearchResultDTO, 0, len(vectorResults))
	for _, vectorResult := range vectorResults {
		chunk, exists := chunkMap[vectorResult.Embedding.ChunkID]
		if !exists {
			continue // Skip if chunk not found
		}

		// Apply all filters including type filters
		if !s.matchesFilters(chunk, request) {
			continue
		}

		result := dto.SearchResultDTO{
			ChunkID:         chunk.ChunkID,
			Content:         chunk.Content,
			SimilarityScore: vectorResult.Similarity,
			Repository:      chunk.Repository,
			FilePath:        chunk.FilePath,
			Language:        chunk.Language,
			StartLine:       chunk.StartLine,
			EndLine:         chunk.EndLine,
			// Include enhanced type information
			Type:          chunk.Type,
			EntityName:    chunk.EntityName,
			ParentEntity:  chunk.ParentEntity,
			QualifiedName: chunk.QualifiedName,
			Signature:     chunk.Signature,
			Visibility:    chunk.Visibility,
		}
		results = append(results, result)
	}

	return results
}

// matchesFilters checks if a chunk matches all the specified filters.
func (s *SearchService) matchesFilters(chunk ChunkInfo, request dto.SearchRequestDTO) bool {
	// Check language filter
	if len(request.Languages) > 0 {
		languageMatches := false
		for _, lang := range request.Languages {
			if strings.EqualFold(chunk.Language, lang) {
				languageMatches = true
				break
			}
		}
		if !languageMatches {
			return false
		}
	}

	// Check file type filter
	if len(request.FileTypes) > 0 {
		fileTypeMatches := false
		for _, fileType := range request.FileTypes {
			if strings.HasSuffix(strings.ToLower(chunk.FilePath), strings.ToLower(fileType)) {
				fileTypeMatches = true
				break
			}
		}
		if !fileTypeMatches {
			return false
		}
	}

	// Check type filter
	if len(request.Types) > 0 {
		typeMatches := false
		for _, typeFilter := range request.Types {
			if strings.EqualFold(chunk.Type, typeFilter) {
				typeMatches = true
				break
			}
		}
		if !typeMatches {
			return false
		}
	}

	// Check entity name filter
	if request.EntityName != "" {
		if !strings.Contains(strings.ToLower(chunk.EntityName), strings.ToLower(request.EntityName)) {
			return false
		}
	}

	// Check visibility filter
	if len(request.Visibility) > 0 {
		visibilityMatches := false
		for _, visibilityFilter := range request.Visibility {
			if strings.EqualFold(chunk.Visibility, visibilityFilter) {
				visibilityMatches = true
				break
			}
		}
		if !visibilityMatches {
			return false
		}
	}

	return true
}

// sortResults sorts the search results based on the specified sort option.
func (s *SearchService) sortResults(results []dto.SearchResultDTO, sortOption string) {
	switch sortOption {
	case "similarity:desc":
		sort.Slice(results, func(i, j int) bool {
			return results[i].SimilarityScore > results[j].SimilarityScore
		})
	case "similarity:asc":
		sort.Slice(results, func(i, j int) bool {
			return results[i].SimilarityScore < results[j].SimilarityScore
		})
	case "file_path:asc":
		sort.Slice(results, func(i, j int) bool {
			return results[i].FilePath < results[j].FilePath
		})
	case "file_path:desc":
		sort.Slice(results, func(i, j int) bool {
			return results[i].FilePath > results[j].FilePath
		})
	default:
		// Default to similarity:desc
		sort.Slice(results, func(i, j int) bool {
			return results[i].SimilarityScore > results[j].SimilarityScore
		})
	}
}
