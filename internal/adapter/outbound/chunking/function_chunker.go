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

// FunctionChunker implements function-level chunking algorithms that group related functions
// and preserve semantic boundaries while respecting size constraints.
type FunctionChunker struct{}

// NewFunctionChunker creates a new function-level chunker.
func NewFunctionChunker() *FunctionChunker {
	return &FunctionChunker{}
}

// ChunkByFunction groups semantic chunks based on function boundaries and relationships.
// This strategy prioritizes keeping complete functions together while respecting size limits.
func (f *FunctionChunker) ChunkByFunction(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Info(ctx, "Starting function-based chunking", slogger.Fields{
		"semantic_chunks": len(semanticChunks),
		"max_chunk_size":  config.MaxChunkSize,
		"min_chunk_size":  config.MinChunkSize,
	})

	if len(semanticChunks) == 0 {
		return []outbound.EnhancedCodeChunk{}, nil
	}

	// Group semantic chunks by function relationships
	functionGroups := f.groupByFunctionRelationships(ctx, semanticChunks)

	var enhancedChunks []outbound.EnhancedCodeChunk

	for _, group := range functionGroups {
		chunks, err := f.createEnhancedChunksFromGroup(ctx, group, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create enhanced chunks from function group: %w", err)
		}
		enhancedChunks = append(enhancedChunks, chunks...)
	}

	slogger.Info(ctx, "Function-based chunking completed", slogger.Fields{
		"input_chunks":   len(semanticChunks),
		"output_chunks":  len(enhancedChunks),
		"groups_created": len(functionGroups),
	})

	return enhancedChunks, nil
}

// groupByFunctionRelationships analyzes semantic chunks to identify function relationships
// and groups them accordingly.
func (f *FunctionChunker) groupByFunctionRelationships(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
) [][]outbound.SemanticCodeChunk {
	// Sort chunks by position for logical grouping
	sortedChunks := make([]outbound.SemanticCodeChunk, len(semanticChunks))
	copy(sortedChunks, semanticChunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].StartByte < sortedChunks[j].StartByte
	})

	var groups [][]outbound.SemanticCodeChunk
	var currentGroup []outbound.SemanticCodeChunk

	for _, chunk := range sortedChunks {
		if f.shouldStartNewFunctionGroup(ctx, currentGroup, chunk) {
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
			}
			currentGroup = []outbound.SemanticCodeChunk{chunk}
		} else {
			currentGroup = append(currentGroup, chunk)
		}
	}

	// Add the final group if it's not empty
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	slogger.Debug(ctx, "Created function groups", slogger.Fields{
		"total_groups": len(groups),
		"input_chunks": len(semanticChunks),
	})

	return groups
}

// shouldStartNewFunctionGroup determines if a new function group should be started
// based on semantic analysis and size constraints.
func (f *FunctionChunker) shouldStartNewFunctionGroup(
	ctx context.Context,
	currentGroup []outbound.SemanticCodeChunk,
	newChunk outbound.SemanticCodeChunk,
) bool {
	if len(currentGroup) == 0 {
		return true
	}

	// Check if the new chunk represents a major semantic boundary
	if f.isMajorSemanticBoundary(newChunk.Type) {
		return true
	}

	// Check if adding this chunk would exceed reasonable group size
	currentSize := f.calculateGroupSize(currentGroup)
	newChunkSize := len(newChunk.Content)

	// Use a heuristic to determine if the group is getting too large
	if currentSize+newChunkSize > 1500 { // Reasonable function group size
		return true
	}

	// Check for language-specific function boundaries
	return f.isLanguageSpecificBoundary(ctx, currentGroup, newChunk)
}

// isMajorSemanticBoundary identifies major semantic boundaries that should typically
// start new function groups.
func (f *FunctionChunker) isMajorSemanticBoundary(constructType outbound.SemanticConstructType) bool {
	switch constructType {
	case outbound.ConstructClass, outbound.ConstructStruct, outbound.ConstructInterface:
		return true
	case outbound.ConstructModule, outbound.ConstructPackage, outbound.ConstructNamespace:
		return true
	default:
		return false
	}
}

// isLanguageSpecificBoundary checks for language-specific boundaries that should
// trigger new function groups.
func (f *FunctionChunker) isLanguageSpecificBoundary(
	ctx context.Context,
	currentGroup []outbound.SemanticCodeChunk,
	newChunk outbound.SemanticCodeChunk,
) bool {
	if len(currentGroup) == 0 {
		return false
	}

	lastChunk := currentGroup[len(currentGroup)-1]

	// Different languages should generally be in separate groups
	if !lastChunk.Language.Equal(newChunk.Language) {
		return true
	}

	// Check for significant gaps in file positions (indicates logical separation)
	if newChunk.StartByte > lastChunk.EndByte+500 { // 500 byte gap threshold
		return true
	}

	return false
}

// calculateGroupSize calculates the total size of a group of semantic chunks.
func (f *FunctionChunker) calculateGroupSize(group []outbound.SemanticCodeChunk) int {
	totalSize := 0
	for _, chunk := range group {
		totalSize += len(chunk.Content)
	}
	return totalSize
}

// createEnhancedChunksFromGroup converts a group of semantic chunks into enhanced chunks
// with proper context preservation and metadata.
func (f *FunctionChunker) createEnhancedChunksFromGroup(
	ctx context.Context,
	group []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	if len(group) == 0 {
		return []outbound.EnhancedCodeChunk{}, nil
	}

	// Check if the group should be split based on size constraints
	groupSize := f.calculateGroupSize(group)
	if groupSize > config.MaxChunkSize && config.EnableSplitting {
		return f.splitLargeFunctionGroup(ctx, group, config)
	}

	// Create a single enhanced chunk from the group
	chunk, err := f.createSingleEnhancedChunk(ctx, group, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create enhanced chunk: %w", err)
	}

	return []outbound.EnhancedCodeChunk{*chunk}, nil
}

// splitLargeFunctionGroup splits a large function group into smaller chunks while
// maintaining semantic integrity.
func (f *FunctionChunker) splitLargeFunctionGroup(
	ctx context.Context,
	group []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Debug(ctx, "Splitting large function group", slogger.Fields{
		"group_size": len(group),
		"max_size":   config.MaxChunkSize,
	})

	var enhancedChunks []outbound.EnhancedCodeChunk
	var currentSubGroup []outbound.SemanticCodeChunk
	currentSize := 0

	for _, chunk := range group {
		chunkSize := len(chunk.Content)

		// If adding this chunk would exceed max size, create a chunk from current subgroup
		if currentSize+chunkSize > config.MaxChunkSize && len(currentSubGroup) > 0 {
			enhancedChunk, err := f.createSingleEnhancedChunk(ctx, currentSubGroup, config)
			if err != nil {
				return nil, fmt.Errorf("failed to create split enhanced chunk: %w", err)
			}
			enhancedChunks = append(enhancedChunks, *enhancedChunk)

			// Start new subgroup
			currentSubGroup = []outbound.SemanticCodeChunk{chunk}
			currentSize = chunkSize
		} else {
			currentSubGroup = append(currentSubGroup, chunk)
			currentSize += chunkSize
		}
	}

	// Handle the remaining subgroup
	if len(currentSubGroup) > 0 {
		enhancedChunk, err := f.createSingleEnhancedChunk(ctx, currentSubGroup, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create final split enhanced chunk: %w", err)
		}
		enhancedChunks = append(enhancedChunks, *enhancedChunk)
	}

	return enhancedChunks, nil
}

// createSingleEnhancedChunk creates an enhanced chunk from a group of semantic chunks.
func (f *FunctionChunker) createSingleEnhancedChunk(
	ctx context.Context,
	group []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) (*outbound.EnhancedCodeChunk, error) {
	if len(group) == 0 {
		return nil, fmt.Errorf("cannot create enhanced chunk from empty group")
	}

	// Calculate boundaries
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

	// Determine primary language (most common in group)
	var primaryLanguage string
	if len(allLanguages) == 1 {
		for lang := range allLanguages {
			primaryLanguage = lang
		}
	} else {
		// For mixed language groups, use the first chunk's language as primary
		primaryLanguage = group[0].Language.Name()
	}

	// Create chunk ID and hash
	chunkID := uuid.New().String()
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(allContent)))

	// Determine chunk type based on group contents
	chunkType := f.determineChunkType(group)

	// Extract dependencies from all chunks in the group
	dependencies := f.extractDependencies(group)

	// Calculate size metrics
	size := outbound.ChunkSize{
		Bytes:      len(allContent),
		Lines:      f.countLines(allContent),
		Characters: len(allContent),
		Constructs: len(group),
	}

	// Calculate complexity and cohesion scores (basic implementation)
	complexityScore := f.calculateComplexityScore(group)
	cohesionScore := f.calculateCohesionScore(group)

	chunk := &outbound.EnhancedCodeChunk{
		ID:                 chunkID,
		SourceFile:         f.determineSourceFile(group),
		Language:           f.parseLanguage(primaryLanguage),
		StartByte:          minStart,
		EndByte:            maxEnd,
		StartPosition:      group[0].StartPosition,
		EndPosition:        group[len(group)-1].EndPosition,
		Content:            allContent,
		PreservedContext:   outbound.PreservedContext{}, // Will be populated by context preserver
		SemanticConstructs: group,
		Dependencies:       dependencies,
		ChunkType:          chunkType,
		ChunkingStrategy:   config.Strategy,
		ContextStrategy:    config.ContextPreservation,
		QualityMetrics:     outbound.ChunkQualityMetrics{}, // Will be populated by quality analyzer
		CreatedAt:          time.Now(),
		Hash:               contentHash,
		Size:               size,
		ComplexityScore:    complexityScore,
		CohesionScore:      cohesionScore,
	}

	return chunk, nil
}

// Helper methods for chunk creation

func (f *FunctionChunker) determineChunkType(group []outbound.SemanticCodeChunk) outbound.ChunkType {
	// Analyze the types in the group to determine the overall chunk type
	typeCounts := make(map[outbound.SemanticConstructType]int)
	for _, chunk := range group {
		typeCounts[chunk.Type]++
	}

	// Find the most common type
	var maxCount int
	var mostCommonType outbound.SemanticConstructType
	for constructType, count := range typeCounts {
		if count > maxCount {
			maxCount = count
			mostCommonType = constructType
		}
	}

	// Map semantic construct type to chunk type
	switch mostCommonType {
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

func (f *FunctionChunker) extractDependencies(group []outbound.SemanticCodeChunk) []outbound.ChunkDependency {
	var dependencies []outbound.ChunkDependency

	for _, chunk := range group {
		// Extract function call dependencies
		for _, funcCall := range chunk.CalledFunctions {
			dependencies = append(dependencies, outbound.ChunkDependency{
				Type:         outbound.DependencyCall,
				TargetSymbol: funcCall.Name,
				Relationship: "function_call",
				Strength:     0.8,
				IsResolved:   false,
			})
		}

		// Extract type dependencies
		for _, typeRef := range chunk.UsedTypes {
			dependencies = append(dependencies, outbound.ChunkDependency{
				Type:         outbound.DependencyRef,
				TargetSymbol: typeRef.Name,
				Relationship: "type_reference",
				Strength:     0.6,
				IsResolved:   false,
			})
		}
	}

	return dependencies
}

func (f *FunctionChunker) determineSourceFile(group []outbound.SemanticCodeChunk) string {
	// Use the source file from the first chunk, or aggregate if multiple files
	if len(group) > 0 {
		// For now, return a simple representation
		return "aggregated_function_chunk"
	}
	return "unknown_source"
}

func (f *FunctionChunker) parseLanguage(languageStr string) valueobject.Language {
	// Convert string to Language value object (this is a simplified implementation)
	language, err := valueobject.NewLanguage(languageStr)
	if err != nil {
		// Fallback to Unknown language if parsing fails
		unknownLang, _ := valueobject.NewLanguage(valueobject.LanguageUnknown)
		return unknownLang
	}
	return language
}

func (f *FunctionChunker) countLines(content string) int {
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

func (f *FunctionChunker) calculateComplexityScore(group []outbound.SemanticCodeChunk) float64 {
	// Basic complexity calculation based on number of constructs and their types
	totalComplexity := 0.0

	for _, chunk := range group {
		// Different construct types have different complexity weights
		switch chunk.Type {
		case outbound.ConstructFunction, outbound.ConstructMethod:
			totalComplexity += 1.0
		case outbound.ConstructClass, outbound.ConstructStruct:
			totalComplexity += 2.0
		case outbound.ConstructInterface:
			totalComplexity += 1.5
		default:
			totalComplexity += 0.5
		}

		// Add complexity for function calls and dependencies
		totalComplexity += float64(len(chunk.CalledFunctions)) * 0.1
		totalComplexity += float64(len(chunk.UsedTypes)) * 0.1
	}

	// Normalize to 0-1 range
	maxPossibleComplexity := float64(len(group)) * 3.0 // Assuming max complexity per chunk
	if maxPossibleComplexity == 0 {
		return 0.0
	}

	normalized := totalComplexity / maxPossibleComplexity
	if normalized > 1.0 {
		normalized = 1.0
	}

	return normalized
}

func (f *FunctionChunker) calculateCohesionScore(group []outbound.SemanticCodeChunk) float64 {
	if len(group) <= 1 {
		return 1.0 // Single chunks are perfectly cohesive
	}

	// Calculate cohesion based on shared dependencies and similar names/types
	totalRelations := 0
	sharedDependencies := 0

	for i := 0; i < len(group); i++ {
		for j := i + 1; j < len(group); j++ {
			totalRelations++

			// Check for shared function calls
			if f.hasSharedFunctionCalls(group[i], group[j]) {
				sharedDependencies++
			}

			// Check for shared types
			if f.hasSharedTypes(group[i], group[j]) {
				sharedDependencies++
			}
		}
	}

	if totalRelations == 0 {
		return 1.0
	}

	cohesion := float64(sharedDependencies) / float64(totalRelations)
	return cohesion
}

func (f *FunctionChunker) hasSharedFunctionCalls(chunk1, chunk2 outbound.SemanticCodeChunk) bool {
	calls1 := make(map[string]bool)
	for _, call := range chunk1.CalledFunctions {
		calls1[call.Name] = true
	}

	for _, call := range chunk2.CalledFunctions {
		if calls1[call.Name] {
			return true
		}
	}

	return false
}

func (f *FunctionChunker) hasSharedTypes(chunk1, chunk2 outbound.SemanticCodeChunk) bool {
	types1 := make(map[string]bool)
	for _, typeRef := range chunk1.UsedTypes {
		types1[typeRef.Name] = true
	}

	for _, typeRef := range chunk2.UsedTypes {
		if types1[typeRef.Name] {
			return true
		}
	}

	return false
}
