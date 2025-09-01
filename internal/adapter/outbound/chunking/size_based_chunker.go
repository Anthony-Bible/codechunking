package chunking

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
)

// SizeBasedChunker implements intelligent size-based chunking algorithms that split code
// at semantic boundaries while respecting size constraints.
type SizeBasedChunker struct{}

// NewSizeBasedChunker creates a new size-based chunker.
func NewSizeBasedChunker() *SizeBasedChunker {
	return &SizeBasedChunker{}
}

// ChunkBySize groups semantic chunks based on size constraints while preserving
// semantic boundaries and maintaining code coherence.
func (s *SizeBasedChunker) ChunkBySize(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Info(ctx, "Starting size-based chunking", slogger.Fields{
		"semantic_chunks": len(semanticChunks),
		"max_chunk_size":  config.MaxChunkSize,
		"min_chunk_size":  config.MinChunkSize,
		"overlap_size":    config.OverlapSize,
	})

	if len(semanticChunks) == 0 {
		return []outbound.EnhancedCodeChunk{}, nil
	}

	// Sort chunks by position for logical grouping
	sortedChunks := s.sortChunksByPosition(semanticChunks)

	// Create size-based groups with semantic boundary awareness
	groups := s.createSizeBasedGroups(ctx, sortedChunks, config)

	var enhancedChunks []outbound.EnhancedCodeChunk

	for _, group := range groups {
		chunk, err := s.createEnhancedChunkFromGroup(ctx, group, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create enhanced chunk from size group: %w", err)
		}
		enhancedChunks = append(enhancedChunks, *chunk)
	}

	slogger.Info(ctx, "Size-based chunking completed", slogger.Fields{
		"input_chunks":  len(semanticChunks),
		"output_chunks": len(enhancedChunks),
		"size_groups":   len(groups),
	})

	return enhancedChunks, nil
}

// RefineChunks refines existing chunks by applying size-based splitting where necessary.
// This is used in hybrid chunking strategies.
func (s *SizeBasedChunker) RefineChunks(
	ctx context.Context,
	chunks []outbound.EnhancedCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Debug(ctx, "Refining chunks with size-based analysis", slogger.Fields{
		"input_chunks": len(chunks),
		"max_size":     config.MaxChunkSize,
	})

	var refinedChunks []outbound.EnhancedCodeChunk

	for _, chunk := range chunks {
		if chunk.Size.Bytes > config.MaxChunkSize && config.EnableSplitting {
			// Split oversized chunk
			splitChunks, err := s.splitOversizedChunk(ctx, chunk, config)
			if err != nil {
				slogger.Error(ctx, "Failed to split oversized chunk", slogger.Fields{
					"chunk_id": chunk.ID,
					"size":     chunk.Size.Bytes,
					"error":    err.Error(),
				})
				// Keep original chunk if splitting fails
				refinedChunks = append(refinedChunks, chunk)
			} else {
				refinedChunks = append(refinedChunks, splitChunks...)
			}
		} else {
			refinedChunks = append(refinedChunks, chunk)
		}
	}

	slogger.Debug(ctx, "Chunk refinement completed", slogger.Fields{
		"input_chunks":  len(chunks),
		"output_chunks": len(refinedChunks),
	})

	return refinedChunks, nil
}

// sortChunksByPosition sorts semantic chunks by their position in the source.
func (s *SizeBasedChunker) sortChunksByPosition(chunks []outbound.SemanticCodeChunk) []outbound.SemanticCodeChunk {
	sortedChunks := make([]outbound.SemanticCodeChunk, len(chunks))
	copy(sortedChunks, chunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].StartByte < sortedChunks[j].StartByte
	})
	return sortedChunks
}

// createSizeBasedGroups creates groups of semantic chunks based on size constraints
// while respecting semantic boundaries.
func (s *SizeBasedChunker) createSizeBasedGroups(
	ctx context.Context,
	sortedChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) [][]outbound.SemanticCodeChunk {
	var groups [][]outbound.SemanticCodeChunk
	var currentGroup []outbound.SemanticCodeChunk
	currentSize := 0

	for _, chunk := range sortedChunks {
		chunkSize := len(chunk.Content)

		// Check if adding this chunk would exceed the size limit
		if currentSize+chunkSize > config.MaxChunkSize {
			// If current group meets minimum size requirements, finalize it
			if currentSize >= config.MinChunkSize && len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
				currentGroup = s.createNewGroupWithOverlap(currentGroup, chunk, config)
				currentSize = s.calculateGroupSize(currentGroup)
			} else if len(currentGroup) == 0 {
				// Start new group with this chunk even if it's large
				currentGroup = []outbound.SemanticCodeChunk{chunk}
				currentSize = chunkSize
			} else {
				// Current group is too small, but adding this chunk makes it too big
				// Check if we should split the current chunk
				if s.shouldSplitLargeChunk(chunk, config) {
					// Split the large chunk and add parts
					splitChunks := s.splitSemanticChunk(ctx, chunk, config)
					for _, splitChunk := range splitChunks {
						splitSize := len(splitChunk.Content)
						if currentSize+splitSize <= config.MaxChunkSize {
							currentGroup = append(currentGroup, splitChunk)
							currentSize += splitSize
						} else {
							if len(currentGroup) > 0 {
								groups = append(groups, currentGroup)
							}
							currentGroup = []outbound.SemanticCodeChunk{splitChunk}
							currentSize = splitSize
						}
					}
				} else {
					// Finalize current group and start new one
					if len(currentGroup) > 0 {
						groups = append(groups, currentGroup)
					}
					currentGroup = []outbound.SemanticCodeChunk{chunk}
					currentSize = chunkSize
				}
			}
		} else {
			// Add chunk to current group
			currentGroup = append(currentGroup, chunk)
			currentSize += chunkSize
		}
	}

	// Add the final group if it's not empty
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	slogger.Debug(ctx, "Created size-based groups", slogger.Fields{
		"total_groups":       len(groups),
		"average_group_size": s.calculateAverageGroupSize(groups),
	})

	return groups
}

// createNewGroupWithOverlap creates a new group with potential overlap from the previous group.
func (s *SizeBasedChunker) createNewGroupWithOverlap(
	previousGroup []outbound.SemanticCodeChunk,
	newChunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) []outbound.SemanticCodeChunk {
	newGroup := []outbound.SemanticCodeChunk{newChunk}

	// Add overlap if configured and beneficial
	if config.OverlapSize > 0 && len(previousGroup) > 0 {
		overlapSize := 0
		// Add chunks from the end of previous group as context
		for i := len(previousGroup) - 1; i >= 0 && overlapSize < config.OverlapSize; i-- {
			chunk := previousGroup[i]
			if overlapSize+len(chunk.Content) <= config.OverlapSize {
				// Insert at beginning to maintain order
				newGroup = append([]outbound.SemanticCodeChunk{chunk}, newGroup...)
				overlapSize += len(chunk.Content)
			} else {
				break
			}
		}
	}

	return newGroup
}

// shouldSplitLargeChunk determines if a large semantic chunk should be split.
func (s *SizeBasedChunker) shouldSplitLargeChunk(
	chunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) bool {
	chunkSize := len(chunk.Content)

	// Only split if the chunk is significantly larger than max size
	if chunkSize < config.MaxChunkSize*2 {
		return false
	}

	// Some construct types should not be split
	switch chunk.Type {
	case outbound.ConstructInterface:
		return false // Keep interfaces intact
	case outbound.ConstructEnum:
		return false // Keep enums intact
	case outbound.ConstructConstant:
		return false // Keep constants intact
	default:
		return config.EnableSplitting
	}
}

// splitSemanticChunk splits a large semantic chunk into smaller pieces at logical boundaries.
func (s *SizeBasedChunker) splitSemanticChunk(
	ctx context.Context,
	chunk outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) []outbound.SemanticCodeChunk {
	content := chunk.Content
	targetSize := config.MaxChunkSize * 3 / 4 // Aim for 75% of max size

	var splitChunks []outbound.SemanticCodeChunk

	// Simple line-based splitting with semantic awareness
	lines := s.splitIntoLines(content)

	var currentLines []string
	currentSize := 0
	splitIndex := 0

	for _, line := range lines {
		lineSize := len(line) + 1 // +1 for newline

		if currentSize+lineSize > targetSize && len(currentLines) > 0 {
			// Create a split chunk
			splitContent := s.joinLines(currentLines)
			splitChunk := s.createSplitChunk(chunk, splitContent, splitIndex)
			splitChunks = append(splitChunks, splitChunk)

			// Start new chunk
			currentLines = []string{line}
			currentSize = lineSize
			splitIndex++
		} else {
			currentLines = append(currentLines, line)
			currentSize += lineSize
		}
	}

	// Add the final split chunk
	if len(currentLines) > 0 {
		splitContent := s.joinLines(currentLines)
		splitChunk := s.createSplitChunk(chunk, splitContent, splitIndex)
		splitChunks = append(splitChunks, splitChunk)
	}

	slogger.Debug(ctx, "Split large semantic chunk", slogger.Fields{
		"original_size": len(chunk.Content),
		"split_count":   len(splitChunks),
		"chunk_type":    string(chunk.Type),
	})

	return splitChunks
}

// createSplitChunk creates a new semantic chunk from a portion of an original chunk.
func (s *SizeBasedChunker) createSplitChunk(
	originalChunk outbound.SemanticCodeChunk,
	content string,
	splitIndex int,
) outbound.SemanticCodeChunk {
	return outbound.SemanticCodeChunk{
		ID:            fmt.Sprintf("%s_split_%d", originalChunk.ID, splitIndex),
		Type:          outbound.ConstructVariable, // Mark as fragment (using variable as closest equivalent)
		Name:          fmt.Sprintf("%s_part_%d", originalChunk.Name, splitIndex+1),
		QualifiedName: fmt.Sprintf("%s_part_%d", originalChunk.QualifiedName, splitIndex+1),
		Language:      originalChunk.Language,
		StartByte:     originalChunk.StartByte, // Approximate - would need more precise calculation
		EndByte:       originalChunk.StartByte + uint32(len(content)),
		StartPosition: originalChunk.StartPosition,
		EndPosition:   originalChunk.EndPosition,
		Content:       content,
		Signature:     originalChunk.Signature,
		Documentation: originalChunk.Documentation,
		Annotations:   originalChunk.Annotations,
		ExtractedAt:   time.Now(),
		Hash:          fmt.Sprintf("%x", sha256.Sum256([]byte(content))),
	}
}

// splitOversizedChunk splits an already-created enhanced chunk that exceeds size limits.
func (s *SizeBasedChunker) splitOversizedChunk(
	ctx context.Context,
	chunk outbound.EnhancedCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	// Convert back to semantic chunks for splitting
	var semanticChunks []outbound.SemanticCodeChunk

	if len(chunk.SemanticConstructs) > 0 {
		semanticChunks = chunk.SemanticConstructs
	} else {
		// Create a single semantic chunk from the enhanced chunk
		semanticChunk := outbound.SemanticCodeChunk{
			ID:            chunk.ID + "_semantic",
			Type:          s.mapChunkTypeToSemanticType(chunk.ChunkType),
			Name:          "enhanced_chunk_content",
			QualifiedName: "enhanced_chunk_content",
			Language:      chunk.Language,
			StartByte:     chunk.StartByte,
			EndByte:       chunk.EndByte,
			StartPosition: chunk.StartPosition,
			EndPosition:   chunk.EndPosition,
			Content:       chunk.Content,
			ExtractedAt:   chunk.CreatedAt,
			Hash:          chunk.Hash,
		}
		semanticChunks = []outbound.SemanticCodeChunk{semanticChunk}
	}

	// Split the semantic chunks
	var allSplitChunks []outbound.SemanticCodeChunk
	for _, semChunk := range semanticChunks {
		if len(semChunk.Content) > config.MaxChunkSize {
			splitChunks := s.splitSemanticChunk(ctx, semChunk, config)
			allSplitChunks = append(allSplitChunks, splitChunks...)
		} else {
			allSplitChunks = append(allSplitChunks, semChunk)
		}
	}

	// Create new enhanced chunks from split semantic chunks
	var enhancedChunks []outbound.EnhancedCodeChunk
	for i, splitChunk := range allSplitChunks {
		enhancedChunk, err := s.createEnhancedChunkFromSplitSemantic(ctx, splitChunk, chunk, i, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create enhanced chunk from split semantic chunk: %w", err)
		}
		enhancedChunks = append(enhancedChunks, *enhancedChunk)
	}

	return enhancedChunks, nil
}

// createEnhancedChunkFromGroup creates an enhanced chunk from a group of semantic chunks.
func (s *SizeBasedChunker) createEnhancedChunkFromGroup(
	ctx context.Context,
	group []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) (*outbound.EnhancedCodeChunk, error) {
	if len(group) == 0 {
		return nil, fmt.Errorf("cannot create enhanced chunk from empty group")
	}

	// Calculate boundaries and aggregate content
	minStart := group[0].StartByte
	maxEnd := group[0].EndByte
	var allContent string
	allLanguages := make(map[string]bool)

	for _, chunk := range group {
		if chunk.StartByte < minStart {
			minStart = chunk.StartByte
		}
		if chunk.EndByte > maxEnd {
			maxEnd = chunk.EndByte
		}
		allContent += chunk.Content + "\n"
		allLanguages[chunk.Language.Name()] = true
	}

	// Determine primary language
	var primaryLanguage string
	if len(allLanguages) == 1 {
		for lang := range allLanguages {
			primaryLanguage = lang
		}
	} else {
		primaryLanguage = group[0].Language.Name()
	}

	// Create chunk metadata
	chunkID := uuid.New().String()
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(allContent)))
	chunkType := s.determineChunkType(group)

	// Calculate size metrics
	size := outbound.ChunkSize{
		Bytes:      len(allContent),
		Lines:      s.countLines(allContent),
		Characters: len(allContent),
		Constructs: len(group),
	}

	// Calculate basic scores
	complexityScore := s.calculateComplexityScore(group)
	cohesionScore := s.calculateCohesionScore(group)

	chunk := &outbound.EnhancedCodeChunk{
		ID:                 chunkID,
		SourceFile:         s.determineSourceFile(group),
		Language:           s.parseLanguage(primaryLanguage),
		StartByte:          minStart,
		EndByte:            maxEnd,
		StartPosition:      group[0].StartPosition,
		EndPosition:        group[len(group)-1].EndPosition,
		Content:            allContent,
		PreservedContext:   outbound.PreservedContext{},
		SemanticConstructs: group,
		Dependencies:       s.extractDependencies(group),
		ChunkType:          chunkType,
		ChunkingStrategy:   config.Strategy,
		ContextStrategy:    config.ContextPreservation,
		QualityMetrics:     outbound.ChunkQualityMetrics{},
		CreatedAt:          time.Now(),
		Hash:               contentHash,
		Size:               size,
		ComplexityScore:    complexityScore,
		CohesionScore:      cohesionScore,
	}

	return chunk, nil
}

// createEnhancedChunkFromSplitSemantic creates an enhanced chunk from a split semantic chunk.
func (s *SizeBasedChunker) createEnhancedChunkFromSplitSemantic(
	ctx context.Context,
	splitChunk outbound.SemanticCodeChunk,
	originalChunk outbound.EnhancedCodeChunk,
	splitIndex int,
	config outbound.ChunkingConfiguration,
) (*outbound.EnhancedCodeChunk, error) {
	chunkID := fmt.Sprintf("%s_split_%d", originalChunk.ID, splitIndex)
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(splitChunk.Content)))

	size := outbound.ChunkSize{
		Bytes:      len(splitChunk.Content),
		Lines:      s.countLines(splitChunk.Content),
		Characters: len(splitChunk.Content),
		Constructs: 1,
	}

	chunk := &outbound.EnhancedCodeChunk{
		ID:                 chunkID,
		SourceFile:         originalChunk.SourceFile,
		Language:           splitChunk.Language,
		StartByte:          splitChunk.StartByte,
		EndByte:            splitChunk.EndByte,
		StartPosition:      splitChunk.StartPosition,
		EndPosition:        splitChunk.EndPosition,
		Content:            splitChunk.Content,
		PreservedContext:   originalChunk.PreservedContext, // Inherit context
		SemanticConstructs: []outbound.SemanticCodeChunk{splitChunk},
		Dependencies:       s.filterDependencies(originalChunk.Dependencies),
		ChunkType:          outbound.ChunkFragment,
		ChunkingStrategy:   config.Strategy,
		ContextStrategy:    config.ContextPreservation,
		QualityMetrics:     originalChunk.QualityMetrics, // Inherit metrics
		ParentChunk:        &outbound.CodeChunk{ID: originalChunk.ID},
		CreatedAt:          time.Now(),
		Hash:               contentHash,
		Size:               size,
		ComplexityScore:    0.5, // Fragments have moderate complexity
		CohesionScore:      0.3, // Fragments have lower cohesion
	}

	return chunk, nil
}

// Helper methods

func (s *SizeBasedChunker) calculateGroupSize(group []outbound.SemanticCodeChunk) int {
	totalSize := 0
	for _, chunk := range group {
		totalSize += len(chunk.Content)
	}
	return totalSize
}

func (s *SizeBasedChunker) calculateAverageGroupSize(groups [][]outbound.SemanticCodeChunk) float64 {
	if len(groups) == 0 {
		return 0.0
	}

	totalSize := 0
	for _, group := range groups {
		totalSize += s.calculateGroupSize(group)
	}

	return float64(totalSize) / float64(len(groups))
}

func (s *SizeBasedChunker) splitIntoLines(content string) []string {
	var lines []string
	var currentLine string

	for _, char := range content {
		if char == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
		} else {
			currentLine += string(char)
		}
	}

	// Add the final line if it's not empty
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

func (s *SizeBasedChunker) joinLines(lines []string) string {
	result := ""
	for i, line := range lines {
		result += line
		if i < len(lines)-1 {
			result += "\n"
		}
	}
	return result
}

func (s *SizeBasedChunker) mapChunkTypeToSemanticType(chunkType outbound.ChunkType) outbound.SemanticConstructType {
	switch chunkType {
	case outbound.ChunkFunction:
		return outbound.ConstructFunction
	case outbound.ChunkClass:
		return outbound.ConstructClass
	case outbound.ChunkStruct:
		return outbound.ConstructStruct
	case outbound.ChunkInterface:
		return outbound.ConstructInterface
	case outbound.ChunkVariable:
		return outbound.ConstructVariable
	case outbound.ChunkConstant:
		return outbound.ConstructConstant
	default:
		return outbound.ConstructFunction // Default fallback
	}
}

func (s *SizeBasedChunker) determineChunkType(group []outbound.SemanticCodeChunk) outbound.ChunkType {
	if len(group) == 1 {
		switch group[0].Type {
		case outbound.ConstructFunction, outbound.ConstructMethod:
			return outbound.ChunkFunction
		case outbound.ConstructClass:
			return outbound.ChunkClass
		case outbound.ConstructStruct:
			return outbound.ChunkStruct
		case outbound.ConstructInterface:
			return outbound.ChunkInterface
		case outbound.ConstructVariable:
			return outbound.ChunkVariable
		case outbound.ConstructConstant:
			return outbound.ChunkConstant
		default:
			return outbound.ChunkMixed
		}
	}

	return outbound.ChunkMixed // Multiple constructs = mixed
}

func (s *SizeBasedChunker) extractDependencies(group []outbound.SemanticCodeChunk) []outbound.ChunkDependency {
	var dependencies []outbound.ChunkDependency

	for _, chunk := range group {
		// Extract function call dependencies
		for _, funcCall := range chunk.CalledFunctions {
			dependencies = append(dependencies, outbound.ChunkDependency{
				Type:         outbound.DependencyCall,
				TargetSymbol: funcCall.Name,
				Relationship: "function_call",
				Strength:     0.6,
				IsResolved:   false,
			})
		}

		// Extract type dependencies
		for _, typeRef := range chunk.UsedTypes {
			dependencies = append(dependencies, outbound.ChunkDependency{
				Type:         outbound.DependencyRef,
				TargetSymbol: typeRef.Name,
				Relationship: "type_reference",
				Strength:     0.5,
				IsResolved:   false,
			})
		}
	}

	return dependencies
}

func (s *SizeBasedChunker) filterDependencies(dependencies []outbound.ChunkDependency) []outbound.ChunkDependency {
	// For split chunks, keep only the most relevant dependencies
	var filtered []outbound.ChunkDependency
	for _, dep := range dependencies {
		if dep.Strength >= 0.5 { // Keep dependencies with reasonable strength
			filtered = append(filtered, dep)
		}
	}
	return filtered
}

func (s *SizeBasedChunker) determineSourceFile(group []outbound.SemanticCodeChunk) string {
	if len(group) > 0 {
		return "aggregated_size_chunk"
	}
	return "unknown_source"
}

func (s *SizeBasedChunker) parseLanguage(languageStr string) valueobject.Language {
	// Convert string to Language value object
	language, err := valueobject.NewLanguage(languageStr)
	if err != nil {
		// Fallback to Unknown language if parsing fails
		unknownLang, _ := valueobject.NewLanguage(valueobject.LanguageUnknown)
		return unknownLang
	}
	return language
}

func (s *SizeBasedChunker) countLines(content string) int {
	if content == "" {
		return 0
	}
	lines := 1
	for _, char := range content {
		if char == '\n' {
			lines++
		}
	}
	return lines
}

func (s *SizeBasedChunker) calculateComplexityScore(group []outbound.SemanticCodeChunk) float64 {
	// Size-based complexity: larger groups and fragments have moderate complexity
	totalSize := s.calculateGroupSize(group)
	constructCount := len(group)

	// Base complexity on size and construct diversity
	sizeComplexity := float64(totalSize) / 2000.0         // Normalize against 2KB
	constructComplexity := float64(constructCount) / 10.0 // Normalize against 10 constructs

	complexity := (sizeComplexity + constructComplexity) / 2.0

	// Cap at 1.0
	if complexity > 1.0 {
		complexity = 1.0
	}

	return complexity
}

func (s *SizeBasedChunker) calculateCohesionScore(group []outbound.SemanticCodeChunk) float64 {
	if len(group) <= 1 {
		return 1.0
	}

	// For size-based chunking, cohesion is lower since chunks are grouped by size, not semantics
	// But sequential chunks in the same file should have some cohesion
	totalGaps := 0
	smallGaps := 0

	for i := 1; i < len(group); i++ {
		prevChunk := group[i-1]
		currChunk := group[i]

		gap := currChunk.StartByte - prevChunk.EndByte
		totalGaps++

		if gap < 100 { // Small gap indicates related code
			smallGaps++
		}
	}

	if totalGaps == 0 {
		return 1.0
	}

	cohesion := float64(smallGaps) / float64(totalGaps)
	return cohesion * 0.7 // Reduce base cohesion for size-based chunking
}
