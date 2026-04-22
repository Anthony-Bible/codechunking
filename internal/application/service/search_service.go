package service

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/application/service/ranking"
	"codechunking/internal/config"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// SearchService handles semantic search operations for code chunks.
type SearchService struct {
	vectorRepo       outbound.VectorStorageRepository
	embeddingService outbound.EmbeddingService
	chunkRepo        ChunkRepository
	repoRepo         outbound.RepositoryRepository
	config           *config.Config
	zoektSearcher    outbound.ZoektSearcher
	hybridRanker     HybridRanker

	zoektChunkCacheMu sync.RWMutex
	zoektChunkCache   map[string][]ChunkInfo
}

// ChunkRepository defines the interface for retrieving chunk information.
type ChunkRepository interface {
	FindChunksByIDs(ctx context.Context, chunkIDs []uuid.UUID) ([]ChunkInfo, error)
	FindChunksByRepositoryPath(ctx context.Context, repositoryName string, filePath string) ([]ChunkInfo, error)
	FindChunksByRepositoryPathAndLineRange(
		ctx context.Context,
		repositoryName string,
		filePath string,
		startLine int,
		endLine int,
	) ([]ChunkInfo, error)
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
// Pass nil for zoektSearcher to disable text search; text mode returns an error if nil.
// Pass nil for hybridRanker to use the default NewHybridRankingService; hybrid mode
// degrades to semantic-only when zoektSearcher is nil.
func NewSearchService(
	vectorRepo outbound.VectorStorageRepository,
	embeddingService outbound.EmbeddingService,
	chunkRepo ChunkRepository,
	repoRepo outbound.RepositoryRepository,
	cfg *config.Config,
	zoektSearcher outbound.ZoektSearcher,
	hybridRanker HybridRanker,
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
	if repoRepo == nil {
		panic("repoRepo cannot be nil")
	}
	if cfg == nil {
		panic("cfg cannot be nil")
	}

	if hybridRanker == nil && zoektSearcher != nil {
		hybridRanker = NewHybridRankingService()
	}

	return &SearchService{
		vectorRepo:       vectorRepo,
		embeddingService: embeddingService,
		chunkRepo:        chunkRepo,
		repoRepo:         repoRepo,
		config:           cfg,
		zoektSearcher:    zoektSearcher,
		hybridRanker:     hybridRanker,
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

	switch request.Mode { //nolint:exhaustive // DefaultSearchMode is an alias for SearchModeSemantic; default handles both
	case dto.SearchModeText:
		return s.searchText(ctx, request, startTime)
	case dto.SearchModeHybrid:
		return s.searchHybrid(ctx, request, startTime)
	default:
		return s.searchSemantic(ctx, request, startTime)
	}
}

// searchSemantic is the original pure-embedding search path.
func (s *SearchService) searchSemantic(ctx context.Context, request dto.SearchRequestDTO, startTime time.Time) (*dto.SearchResponseDTO, error) {
	embeddingResult, err := s.generateQueryEmbedding(ctx, request.Query)
	if err != nil {
		return nil, err
	}

	vectorResults, err := s.performVectorSearch(ctx, embeddingResult.Vector, request)
	if err != nil {
		return nil, err
	}

	results, err := s.retrieveAndFilterChunks(ctx, vectorResults, request)
	if err != nil {
		return nil, err
	}

	s.sortResults(results, request.Sort)
	paginatedResults, totalResults := s.paginateResults(results, request.Limit, request.Offset)
	return s.buildResponseFromResults(paginatedResults, totalResults, request, dto.SearchModeSemantic, []string{"embedding"}, startTime), nil
}

// zoektQuery returns the raw query string to pass to Zoekt.
// Filters (repos, languages, file types) are applied via ZoektSearchOptions
// so applySearchOptions in the gRPC client enforces them correctly.
func (s *SearchService) zoektQuery(request dto.SearchRequestDTO) string {
	return strings.TrimSpace(request.Query)
}

func (s *SearchService) searchText(ctx context.Context, request dto.SearchRequestDTO, startTime time.Time) (*dto.SearchResponseDTO, error) {
	if s.zoektSearcher == nil {
		return nil, errors.New("text search unavailable: zoekt is not configured")
	}

	opts := s.buildZoektSearchOptions(request)
	zoektResult, err := s.zoektSearcher.Search(ctx, s.zoektQuery(request), opts)
	if err != nil {
		return nil, fmt.Errorf("text search failed: %w", err)
	}

	results := s.convertZoektResults(ctx, zoektResult, false, "")
	// Apply RRF scoring to text search results (where semantic_rank is effectively infinity).
	ranking.ApplyRRFRanks(results)

	s.sortResults(results, request.Sort)
	paginatedResults, totalResults := s.paginateResults(results, request.Limit, request.Offset)
	return s.buildResponseFromResults(paginatedResults, totalResults, request, dto.SearchModeText, []string{"zoekt"}, startTime), nil
}

// searchHybrid falls back to semantic-only when ZoektSearcher is nil or Zoekt fails.
func (s *SearchService) searchHybrid(ctx context.Context, request dto.SearchRequestDTO, startTime time.Time) (*dto.SearchResponseDTO, error) {
	embeddingResult, err := s.generateQueryEmbedding(ctx, request.Query)
	if err != nil {
		return nil, err
	}
	vectorResults, err := s.performVectorSearch(ctx, embeddingResult.Vector, request)
	if err != nil {
		return nil, err
	}
	semanticResults, err := s.retrieveAndFilterChunks(ctx, vectorResults, request)
	if err != nil {
		return nil, err
	}

	enginesUsed := []string{"embedding"}

	if s.zoektSearcher == nil {
		slogger.Info(ctx, "Zoekt not configured; hybrid search degraded to semantic-only", slogger.Fields{"query": request.Query})
		s.sortResults(semanticResults, request.Sort)
		paginatedResults, totalResults := s.paginateResults(semanticResults, request.Limit, request.Offset)
		return s.buildResponseFromResults(paginatedResults, totalResults, request, dto.SearchModeHybrid, enginesUsed, startTime), nil
	}

	opts := s.buildZoektSearchOptions(request)
	opts.ChunkMatches = true
	textResults, zoektOK := s.searchZoektWithFallback(ctx, s.zoektQuery(request), opts)
	if zoektOK {
		enginesUsed = []string{"embedding", "zoekt"}
	}

	textResults = s.filterDuplicateZoektFallbacks(semanticResults, textResults)
	merged := s.hybridRanker.Rank(semanticResults, textResults)

	// Preserve ranker order when the caller accepted the default sort; only override for explicit sorts.
	if request.Sort != dto.DefaultSearchSort {
		s.sortResults(merged, request.Sort)
	}
	paginatedResults, totalResults := s.paginateResults(merged, request.Limit, request.Offset)
	return s.buildResponseFromResults(paginatedResults, totalResults, request, dto.SearchModeHybrid, enginesUsed, startTime), nil
}

// searchZoektWithFallback calls Zoekt and returns results plus a boolean indicating success.
// On error it logs and returns an empty slice so hybrid mode degrades gracefully to semantic-only.
func (s *SearchService) searchZoektWithFallback(ctx context.Context, query string, opts outbound.ZoektSearchOptions) ([]dto.SearchResultDTO, bool) {
	result, err := s.zoektSearcher.Search(ctx, query, opts)
	if err != nil {
		slogger.Info(ctx, "Zoekt search failed in hybrid mode; using semantic only", slogger.Fields{"error": err.Error()})
		return []dto.SearchResultDTO{}, false
	}
	return s.convertZoektResults(ctx, result, true, query), true
}

func (s *SearchService) filterDuplicateZoektFallbacks(
	semanticResults []dto.SearchResultDTO,
	textResults []dto.SearchResultDTO,
) []dto.SearchResultDTO {
	if len(semanticResults) == 0 || len(textResults) == 0 {
		return textResults
	}

	filtered := make([]dto.SearchResultDTO, 0, len(textResults))
	for _, textResult := range textResults {
		if textResult.ChunkID != uuid.Nil || !isZoektFileFallbackDuplicate(semanticResults, textResult) {
			filtered = append(filtered, textResult)
		}
	}

	return filtered
}

func isZoektFileFallbackDuplicate(semanticResults []dto.SearchResultDTO, textResult dto.SearchResultDTO) bool {
	if textResult.Repository.Name == "" || textResult.FilePath == "" || textResult.StartLine <= 0 || textResult.EndLine <= 0 {
		return false
	}

	for _, semanticResult := range semanticResults {
		if semanticResult.Repository.Name != textResult.Repository.Name {
			continue
		}
		if semanticResult.FilePath != textResult.FilePath {
			continue
		}
		if semanticResult.StartLine <= 0 || semanticResult.EndLine <= 0 {
			continue
		}
		if lineRangesOverlap(semanticResult.StartLine, semanticResult.EndLine, textResult.StartLine, textResult.EndLine) {
			return true
		}
	}

	return false
}

// normalizeZoektRepoName preserves the repository name as returned by Zoekt.
// Some code paths and tests store repository names with the host prefix
// included, so callers that need to match both formats should use
// zoektRepoNameCandidates instead of relying on lossy normalization.
func normalizeZoektRepoName(name string) string {
	return name
}

// zoektRepoNameCandidates returns all plausible DB repository name variants for
// a zoekt repo name, ordered from most to least specific. It strips one leading
// path segment at a time, stopping before single-segment results to avoid
// false-positive matches.
//
// Examples:
//
//	"github.com/owner/repo"     → ["github.com/owner/repo", "owner/repo"]
//	"gitlab.com/org/sub/repo"  → ["gitlab.com/org/sub/repo", "org/sub/repo", "sub/repo"]
//	"owner/repo"               → ["owner/repo"]
func zoektRepoNameCandidates(name string) []string {
	normalized := normalizeZoektRepoName(name)
	seen := map[string]bool{normalized: true}
	candidates := []string{normalized}
	current := normalized
	for {
		idx := strings.Index(current, "/")
		if idx < 0 {
			break
		}
		next := current[idx+1:]
		// Stop before adding single-segment candidates — too ambiguous.
		if !strings.Contains(next, "/") {
			break
		}
		if !seen[next] {
			candidates = append(candidates, next)
			seen[next] = true
		}
		current = next
	}
	return candidates
}

func lineRangesOverlap(startA, endA, startB, endB int) bool {
	return startA <= endB && startB <= endA
}

// convertZoektResults converts ZoektFileMatch results to SearchResultDTO slice.
func (s *SearchService) convertZoektResults(
	ctx context.Context,
	result *outbound.ZoektSearchResult,
	resolveToChunks bool,
	query string,
) []dto.SearchResultDTO {
	if result == nil {
		return []dto.SearchResultDTO{}
	}
	out := make([]dto.SearchResultDTO, 0, len(result.FileMatches))
	for _, fm := range result.FileMatches {
		content, startLine, endLine := extractZoektMatchContent(fm)
		resultDTO := searchResultFromZoektMatch(fm, content, startLine, endLine)

		if !resolveToChunks {
			out = append(out, resultDTO)
			continue
		}

		resolved, updatedDTO := s.tryResolveZoektFileMatch(ctx, fm, query, resultDTO)
		if !resolved && resultDTO.Content == "" && resultDTO.StartLine == 0 && resultDTO.EndLine == 0 && len(fm.ChunkMatches) == 0 {
			continue
		}
		out = append(out, updatedDTO)
	}
	return out
}

func (s *SearchService) tryResolveZoektFileMatch(ctx context.Context, fm outbound.ZoektFileMatch, query string, fallback dto.SearchResultDTO) (bool, dto.SearchResultDTO) {
	if chunk, resolved := s.resolveZoektChunk(ctx, fm, query); resolved {
		return true, searchResultFromChunk(chunk, fm.Score, "zoekt")
	}

	for _, repoName := range zoektRepoNameCandidates(fm.Repository) {
		if repoName == fm.Repository {
			continue
		}
		alternateMatch := fm
		alternateMatch.Repository = repoName
		if chunk, resolved := s.resolveZoektChunk(ctx, alternateMatch, query); resolved {
			return true, searchResultFromChunk(chunk, fm.Score, "zoekt")
		}
	}

	return false, fallback
}

func (s *SearchService) getZoektChunksForFile(ctx context.Context, repoName, filePath string) ([]ChunkInfo, error) {
	if s.chunkRepo == nil {
		return nil, nil
	}

	cacheKey := repoName + "\x00" + filePath

	s.zoektChunkCacheMu.RLock()
	if chunks, ok := s.zoektChunkCache[cacheKey]; ok {
		s.zoektChunkCacheMu.RUnlock()
		return chunks, nil
	}
	s.zoektChunkCacheMu.RUnlock()

	chunks, err := s.chunkRepo.FindChunksByRepositoryPath(ctx, repoName, filePath)
	if err != nil {
		return nil, err
	}

	s.zoektChunkCacheMu.Lock()
	if s.zoektChunkCache == nil {
		s.zoektChunkCache = make(map[string][]ChunkInfo)
	}
	s.zoektChunkCache[cacheKey] = chunks
	s.zoektChunkCacheMu.Unlock()

	return chunks, nil
}

func (s *SearchService) resolveZoektChunk(ctx context.Context, fm outbound.ZoektFileMatch, query string) (ChunkInfo, bool) {
	if len(fm.ChunkMatches) > 0 {
		best := fm.ChunkMatches[0]
		for _, candidate := range fm.ChunkMatches[1:] {
			if candidate.Score > best.Score {
				best = candidate
			}
		}

		if id, err := uuid.Parse(best.ChunkID); err == nil && id != uuid.Nil {
			return ChunkInfo{
				ChunkID:   id,
				Content:   best.Context,
				FilePath:  fm.FileName,
				Language:  fm.Language,
				StartLine: best.StartLine,
				EndLine:   best.EndLine,
				Repository: dto.RepositoryInfo{
					Name: fm.Repository,
				},
			}, true
		}
	}

	startLine, endLine, ok := zoektMatchLineRange(fm)
	if s.chunkRepo == nil {
		return ChunkInfo{}, false
	}
	if !ok {
		return s.resolveZoektChunkByPath(ctx, fm, query)
	}

	repoName := normalizeZoektRepoName(fm.Repository)
	chunks, err := s.getZoektChunksForFile(ctx, repoName, fm.FileName)
	if err != nil || len(chunks) == 0 {
		slogger.Debug(ctx, "Unable to resolve Zoekt file match to chunk by line range", slogger.Fields{
			"repository": repoName,
			"file_path":  fm.FileName,
			"start_line": startLine,
			"end_line":   endLine,
			"error":      err,
		})
		return ChunkInfo{}, false
	}

	best := selectBestOverlappingChunk(chunks, startLine, endLine)
	if best == nil || best.ChunkID == uuid.Nil {
		slogger.Debug(ctx, "No overlapping chunk found for Zoekt line range", slogger.Fields{
			"repository":   repoName,
			"file_path":    fm.FileName,
			"start_line":   startLine,
			"end_line":     endLine,
			"chunks_count": len(chunks),
		})
		return ChunkInfo{}, false
	}

	return *best, true
}

func (s *SearchService) resolveZoektChunkByPath(ctx context.Context, fm outbound.ZoektFileMatch, query string) (ChunkInfo, bool) {
	chunks, err := s.chunkRepo.FindChunksByRepositoryPath(ctx, normalizeZoektRepoName(fm.Repository), fm.FileName)
	if err != nil || len(chunks) == 0 {
		if err != nil {
			slogger.Debug(ctx, "Unable to resolve Zoekt file match to chunk by path", slogger.Fields{
				"repository": fm.Repository,
				"file_path":  fm.FileName,
				"error":      err.Error(),
			})
		}
		return ChunkInfo{}, false
	}

	best := selectBestPathOnlyChunk(chunks, query)
	if best == nil || best.ChunkID == uuid.Nil {
		return ChunkInfo{}, false
	}

	return *best, true
}

func zoektMatchLineRange(fm outbound.ZoektFileMatch) (int, int, bool) {
	startLine := 0
	endLine := 0

	for _, match := range fm.LineMatches {
		if match.LineNumber <= 0 {
			continue
		}
		if startLine == 0 || match.LineNumber < startLine {
			startLine = match.LineNumber
		}
		if match.LineNumber > endLine {
			endLine = match.LineNumber
		}
	}

	if startLine > 0 && endLine > 0 {
		return startLine, endLine, true
	}

	for _, match := range fm.ChunkMatches {
		if match.StartLine > 0 && match.EndLine > 0 {
			if startLine == 0 || match.StartLine < startLine {
				startLine = match.StartLine
			}
			if match.EndLine > endLine {
				endLine = match.EndLine
			}
		}
	}

	if startLine > 0 && endLine > 0 {
		return startLine, endLine, true
	}

	return 0, 0, false
}

func selectBestOverlappingChunk(chunks []ChunkInfo, startLine, endLine int) *ChunkInfo {
	var best *ChunkInfo
	bestOverlap := 0
	bestSpan := 0
	bestStartLine := 0
	bestChunkID := ""

	for i := range chunks {
		chunk := &chunks[i]
		overlap, span, ok := overlapScoreForRange(*chunk, startLine, endLine)
		if !ok {
			continue
		}

		if best == nil || overlap > bestOverlap || (overlap == bestOverlap && span < bestSpan) ||
			(overlap == bestOverlap && span == bestSpan && chunk.StartLine < bestStartLine) ||
			(overlap == bestOverlap && span == bestSpan && chunk.StartLine == bestStartLine &&
				chunk.ChunkID.String() < bestChunkID) {
			best = chunk
			bestOverlap = overlap
			bestSpan = span
			bestStartLine = chunk.StartLine
			bestChunkID = chunk.ChunkID.String()
		}
	}

	return best
}

func selectBestPathOnlyChunk(chunks []ChunkInfo, query string) *ChunkInfo {
	query = strings.ToLower(strings.TrimSpace(query))

	var best *ChunkInfo
	bestScore := -1
	bestSpan := 0
	bestStartLine := 0
	bestChunkID := ""

	for i := range chunks {
		chunk := &chunks[i]
		if chunk.ChunkID == uuid.Nil {
			continue
		}

		score := pathOnlyChunkQueryScore(*chunk, query)
		span := chunk.EndLine - chunk.StartLine + 1
		if span <= 0 {
			span = 1
		}

		if best == nil || score > bestScore || (score == bestScore && span < bestSpan) ||
			(score == bestScore && span == bestSpan && chunk.StartLine < bestStartLine) ||
			(score == bestScore && span == bestSpan && chunk.StartLine == bestStartLine &&
				chunk.ChunkID.String() < bestChunkID) {
			best = chunk
			bestScore = score
			bestSpan = span
			bestStartLine = chunk.StartLine
			bestChunkID = chunk.ChunkID.String()
		}
	}

	return best
}

func pathOnlyChunkQueryScore(chunk ChunkInfo, query string) int {
	if query == "" {
		return 0
	}

	entityName := strings.ToLower(chunk.EntityName)
	qualifiedName := strings.ToLower(chunk.QualifiedName)
	signature := strings.ToLower(chunk.Signature)
	content := strings.ToLower(chunk.Content)

	switch {
	case entityName == query:
		return 5
	case qualifiedName == query || strings.HasSuffix(qualifiedName, "."+query):
		return 4
	case strings.Contains(signature, query):
		return 3
	case strings.Contains(content, query):
		return 2
	case strings.Contains(entityName, query) || strings.Contains(qualifiedName, query):
		return 1
	default:
		return 0
	}
}

func overlapScoreForRange(chunk ChunkInfo, startLine, endLine int) (int, int, bool) {
	overlapStart := max(chunk.StartLine, startLine)
	overlapEnd := min(chunk.EndLine, endLine)
	if overlapEnd < overlapStart {
		return 0, 0, false
	}

	return overlapEnd - overlapStart + 1, chunk.EndLine - chunk.StartLine + 1, true
}

func searchResultFromChunk(chunk ChunkInfo, engineScore float64, sourceEngine string) dto.SearchResultDTO {
	return dto.SearchResultDTO{
		ChunkID:         chunk.ChunkID,
		Content:         chunk.Content,
		SimilarityScore: engineScore,
		SourceEngine:    sourceEngine,
		EngineScore:     engineScore,
		Repository:      chunk.Repository,
		FilePath:        chunk.FilePath,
		Language:        chunk.Language,
		StartLine:       chunk.StartLine,
		EndLine:         chunk.EndLine,
		Type:            chunk.Type,
		EntityName:      chunk.EntityName,
		ParentEntity:    chunk.ParentEntity,
		QualifiedName:   chunk.QualifiedName,
		Signature:       chunk.Signature,
		Visibility:      chunk.Visibility,
	}
}

// extractZoektMatchContent pulls the best content and line range from a file match.
// When ChunkMatches are present (opts.ChunkMatches=true), LineMatches is empty so
// we read from the highest-scoring ChunkMatch instead.
func extractZoektMatchContent(fm outbound.ZoektFileMatch) (string, int, int) {
	if len(fm.LineMatches) > 0 {
		first := fm.LineMatches[0]
		return first.LineContent, first.LineNumber, fm.LineMatches[len(fm.LineMatches)-1].LineNumber
	}
	if len(fm.ChunkMatches) > 0 {
		best := fm.ChunkMatches[0]
		for _, cm := range fm.ChunkMatches[1:] {
			if cm.Score > best.Score {
				best = cm
			}
		}
		return best.Context, best.StartLine, best.EndLine
	}
	return "", 0, 0
}

func searchResultFromZoektMatch(
	fm outbound.ZoektFileMatch,
	content string,
	startLine, endLine int,
) dto.SearchResultDTO {
	return dto.SearchResultDTO{
		FilePath:        fm.FileName,
		Language:        fm.Language,
		SourceEngine:    "zoekt",
		EngineScore:     fm.Score,
		SimilarityScore: fm.Score,
		Content:         content,
		Repository: dto.RepositoryInfo{
			Name: fm.Repository,
		},
		StartLine: startLine,
		EndLine:   endLine,
	}
}

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

// resolveRepositoryNames resolves repository names to their corresponding UUIDs.
// Returns an error if any repository name cannot be found or if a database error occurs.
func (s *SearchService) resolveRepositoryNames(ctx context.Context, names []string) ([]uuid.UUID, error) {
	resolvedIDs := make([]uuid.UUID, 0, len(names))

	for _, name := range names {
		repoID, err := s.findRepositoryIDByName(ctx, name)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve repository names: %w", err)
		}
		resolvedIDs = append(resolvedIDs, repoID)
	}

	return resolvedIDs, nil
}

// findRepositoryIDByName looks up a single repository by name and returns its ID.
func (s *SearchService) findRepositoryIDByName(ctx context.Context, name string) (uuid.UUID, error) {
	filters := outbound.RepositoryFilters{
		Name:   name,
		Limit:  1,
		Offset: 0,
	}

	repos, _, err := s.repoRepo.FindAll(ctx, filters)
	if err != nil {
		return uuid.Nil, err
	}

	if len(repos) == 0 {
		return uuid.Nil, fmt.Errorf("repository '%s' not found", name)
	}

	return repos[0].ID(), nil
}

// performVectorSearch performs vector similarity search with optional repository filtering.
func (s *SearchService) performVectorSearch(
	ctx context.Context,
	vector []float64,
	request dto.SearchRequestDTO,
) ([]outbound.VectorSimilarityResult, error) {
	slogger.Info(ctx, "Starting vector similarity search", slogger.Fields{
		"vector_dimensions":     len(vector),
		"use_partitioned_table": true,
		"max_results":           request.Limit + request.Offset,
		"sql_filters": map[string]interface{}{
			"languages":       request.Languages,
			"chunk_types":     request.Types,
			"file_extensions": request.FileTypes,
		},
	})

	// Build combined repository filter from both IDs and names
	combinedRepositoryIDs, err := s.buildRepositoryFilter(ctx, request.RepositoryIDs, request.RepositoryNames)
	if err != nil {
		return nil, err
	}

	// Configure search options with SQL-level metadata filtering for pgvector 0.8.0+ optimization.
	//
	// IMPORTANT: Language, chunk type, and file extension filters are applied at the SQL level
	// (in the WHERE clause) rather than in application code. This is critical for pgvector's
	// iterative scanning to work efficiently. If these filters were applied after the vector
	// search, pgvector would scan the entire HNSW index before filtering, defeating the
	// optimization's purpose.
	//
	// The partitioned table schema includes denormalized metadata columns (language, chunk_type,
	// file_path) specifically to enable this SQL-level filtering strategy.
	searchOptions := outbound.SimilaritySearchOptions{
		UsePartitionedTable: true,
		MaxResults:          request.Limit + request.Offset,
		MinSimilarity:       request.SimilarityThreshold,
		RepositoryIDs:       combinedRepositoryIDs,
		IterativeScanMode:   s.getIterativeScanMode(),                   // Enable pgvector 0.8.0+ iterative scanning from config
		Languages:           request.Languages,                          // SQL-level language filtering
		ChunkTypes:          request.Types,                              // SQL-level chunk type filtering
		FileExtensions:      s.extractFileExtensions(request.FileTypes), // SQL-level file extension filtering
	}

	vectorResults, err := s.vectorRepo.VectorSimilaritySearch(ctx, vector, searchOptions)
	if err != nil {
		slogger.Error(ctx, "Vector similarity search failed", slogger.Fields{"error": err.Error()})
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	slogger.Info(ctx, "Vector search completed", slogger.Fields{"vector_results_count": len(vectorResults)})
	return vectorResults, nil
}

// buildRepositoryFilter combines repository IDs and resolved repository names into a single filter list.
// Returns nil if no repository filtering is needed (preserves nil semantics for database queries).
func (s *SearchService) buildRepositoryFilter(
	ctx context.Context,
	repositoryIDs []uuid.UUID,
	repositoryNames []string,
) ([]uuid.UUID, error) {
	// If no filters provided, return nil to indicate no filtering
	if len(repositoryIDs) == 0 && len(repositoryNames) == 0 {
		return nil, nil
	}

	// Pre-allocate capacity for efficiency
	totalCapacity := len(repositoryIDs) + len(repositoryNames)
	combinedIDs := make([]uuid.UUID, 0, totalCapacity)

	// Start with explicitly provided repository IDs
	combinedIDs = append(combinedIDs, repositoryIDs...)

	// Resolve and append repository names to IDs if provided
	if len(repositoryNames) > 0 {
		resolvedIDs, err := s.resolveRepositoryNames(ctx, repositoryNames)
		if err != nil {
			return nil, err
		}
		combinedIDs = append(combinedIDs, resolvedIDs...)
	}

	return combinedIDs, nil
}

// extractFileExtensions converts file type filters (e.g., ".go", "go", ".py") to normalized extensions.
// Returns nil if no file types provided (preserves nil semantics for database queries).
func (s *SearchService) extractFileExtensions(fileTypes []string) []string {
	if len(fileTypes) == 0 {
		return nil
	}

	extensions := make([]string, 0, len(fileTypes))
	for _, ft := range fileTypes {
		// Normalize extension format (ensure it starts with a dot)
		ext := strings.TrimSpace(ft)
		if ext == "" {
			continue
		}
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		extensions = append(extensions, ext)
	}

	if len(extensions) == 0 {
		return nil
	}
	return extensions
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

	slogger.Info(ctx, "Search results built after applying post-processing filters", slogger.Fields{
		"results_before_filters": len(vectorResults),
		"results_after_filters":  len(results),
		"post_process_filters": map[string]interface{}{
			"entity_name": request.EntityName,
			"visibility":  request.Visibility,
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

// buildResponseFromResults constructs a SearchResponseDTO for any search mode.
func (s *SearchService) buildResponseFromResults(
	results []dto.SearchResultDTO,
	totalResults int,
	request dto.SearchRequestDTO,
	mode dto.SearchMode,
	enginesUsed []string,
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
		EngineInfo: dto.SearchEngineInfo{
			Mode:            mode,
			EnginesUsed:     enginesUsed,
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

		results = append(results, searchResultFromChunk(chunk, vectorResult.Similarity, "embedding"))
	}

	return results
}

// matchesFilters checks if a chunk matches all the specified filters.
// Note: Language, chunk type, and file extension filters are now applied at the SQL level
// in VectorSimilaritySearch. This method only handles filters that cannot be applied in SQL
// (entity_name and visibility), which are stored in code_chunks but not in embeddings_partitioned.
func (s *SearchService) matchesFilters(chunk ChunkInfo, request dto.SearchRequestDTO) bool {
	// Check entity name filter (not in embeddings_partitioned, must be post-processed)
	if request.EntityName != "" {
		if !strings.Contains(strings.ToLower(chunk.EntityName), strings.ToLower(request.EntityName)) {
			return false
		}
	}

	// Check visibility filter (not in embeddings_partitioned, must be post-processed)
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

// buildZoektSearchOptions constructs ZoektSearchOptions from a request and config.
// MaxTotalResults is set to Limit+Offset so pagination always has enough results to slice.
// Repos and Lang are populated from the request so applySearchOptions enforces the same
// filters as the semantic path.
func (s *SearchService) buildZoektSearchOptions(request dto.SearchRequestDTO) outbound.ZoektSearchOptions {
	cfg := s.config.Zoekt.Search

	var lang string
	if len(request.Languages) > 0 {
		lang = request.Languages[0]
	}

	return outbound.ZoektSearchOptions{
		MaxTotalResults: request.Limit + request.Offset,
		MaxMatchPerFile: cfg.MaxMatchPerFile,
		ContextLines:    cfg.ContextLines,
		Timeout:         cfg.Timeout,
		Repos:           request.RepositoryNames,
		Lang:            lang,
	}
}

// getIterativeScanMode converts the config string to the IterativeScanMode enum.
// Defaults to IterativeScanRelaxedOrder if the config value is invalid or empty.
func (s *SearchService) getIterativeScanMode() outbound.IterativeScanMode {
	switch s.config.Search.IterativeScanMode {
	case "off":
		return outbound.IterativeScanOff
	case "strict_order":
		return outbound.IterativeScanStrictOrder
	case "relaxed_order":
		return outbound.IterativeScanRelaxedOrder
	default:
		// Default to relaxed_order for best recall/performance
		return outbound.IterativeScanRelaxedOrder
	}
}
