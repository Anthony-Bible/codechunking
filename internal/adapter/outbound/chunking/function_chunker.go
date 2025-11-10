package chunking

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// FunctionChunker implements function-level chunking algorithms that group related functions
// and preserve semantic boundaries while respecting size constraints.
type FunctionChunker struct {
	analysisUtils *FunctionAnalysisUtils
}

// NewFunctionChunker creates a new function-level chunker.
func NewFunctionChunker() *FunctionChunker {
	return &FunctionChunker{
		analysisUtils: &FunctionAnalysisUtils{},
	}
}

// ChunkByFunction creates individual discrete chunks for functions with context preservation.
// Phase 4.3 Step 2: Implements function-level chunking with context preservation.
func (f *FunctionChunker) ChunkByFunction(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Info(ctx, "Starting function-based chunking Phase 4.3 Step 2", slogger.Fields{
		"semantic_chunks": len(semanticChunks),
		"strategy":        string(config.Strategy),
		"preserve_deps":   config.PreserveDependencies,
	})

	if len(semanticChunks) == 0 {
		return []outbound.EnhancedCodeChunk{}, nil
	}

	// Phase 4.3 Step 2: Create individual discrete chunks for each function
	var enhancedChunks []outbound.EnhancedCodeChunk

	for _, semanticChunk := range semanticChunks {
		// Create individual chunk for each function/method
		chunk, err := f.createDiscreteFunction(ctx, semanticChunk, semanticChunks, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create discrete function chunk: %w", err)
		}
		enhancedChunks = append(enhancedChunks, *chunk)
	}

	// Handle related helper functions if PreserveDependencies is enabled
	if config.PreserveDependencies {
		enhancedChunks = f.includeHelperFunctions(ctx, enhancedChunks, semanticChunks, config)
	}

	slogger.Info(ctx, "Function-based chunking completed", slogger.Fields{
		"input_chunks":  len(semanticChunks),
		"output_chunks": len(enhancedChunks),
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
		return nil, errors.New("cannot create enhanced chunk from empty group")
	}

	// Calculate boundaries
	minStart := group[0].StartByte
	maxEnd := group[0].EndByte
	var allContent string
	allLanguages := make(map[string]bool)

	var allContentSb280 strings.Builder
	for _, chunk := range group {
		if chunk.StartByte < minStart {
			minStart = chunk.StartByte
		}
		if chunk.EndByte > maxEnd {
			maxEnd = chunk.EndByte
		}
		allContentSb280.WriteString(chunk.Content + "\n")
		allLanguages[chunk.Language.Name()] = true
	}
	allContent += allContentSb280.String()

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

	for i := range group {
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

// createDiscreteFunction creates an individual enhanced chunk for a single function/method.
// Phase 4.3 Step 2: Creates discrete chunks for individual functions with context preservation.
func (f *FunctionChunker) createDiscreteFunction(
	ctx context.Context,
	semanticChunk outbound.SemanticCodeChunk,
	allChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) (*outbound.EnhancedCodeChunk, error) {
	// Phase 4.3 Step 2: Validate chunk boundaries
	if semanticChunk.EndByte < semanticChunk.StartByte {
		return nil, fmt.Errorf("invalid boundaries: end byte %d is less than start byte %d",
			semanticChunk.EndByte, semanticChunk.StartByte)
	}

	// Phase 4.3 Step 2: Check for size limits
	contentSize := len(semanticChunk.Content)
	if contentSize > 1000000 { // 1MB limit
		return nil, fmt.Errorf("size limit exceeded: function size %d bytes exceeds maximum allowed size", contentSize)
	}

	// Phase 4.3 Step 2: Detect circular dependencies
	if f.hasCircularDependency(semanticChunk, allChunks) {
		return nil, fmt.Errorf("circular dependency detected involving function %s", semanticChunk.Name)
	}

	// Create chunk ID and hash
	chunkID := uuid.New().String()
	contentHash := fmt.Sprintf("%x", sha256.Sum256([]byte(semanticChunk.Content)))

	// Preserve documentation if requested
	content := semanticChunk.Content
	if config.IncludeDocumentation && semanticChunk.Documentation != "" {
		content = semanticChunk.Documentation + "\n" + content
	}

	// Determine chunk type based on semantic construct type
	chunkType := f.mapSemanticToChunkType(semanticChunk.Type)

	// Calculate size metrics
	size := outbound.ChunkSize{
		Bytes:      len(content),
		Lines:      f.countLines(content),
		Characters: len(content),
		Constructs: 1,
	}

	// Create enhanced chunk
	chunk := &outbound.EnhancedCodeChunk{
		ID:                 chunkID,
		SourceFile:         fmt.Sprintf("function_%s", semanticChunk.Name),
		Language:           semanticChunk.Language,
		StartByte:          semanticChunk.StartByte,
		EndByte:            semanticChunk.EndByte,
		StartPosition:      semanticChunk.StartPosition,
		EndPosition:        semanticChunk.EndPosition,
		Content:            content,
		PreservedContext:   outbound.PreservedContext{},
		SemanticConstructs: []outbound.SemanticCodeChunk{semanticChunk},
		Dependencies:       f.extractSingleChunkDependencies(semanticChunk),
		ChunkType:          chunkType,
		ChunkingStrategy:   config.Strategy,
		ContextStrategy:    config.ContextPreservation,
		QualityMetrics:     outbound.ChunkQualityMetrics{},
		CreatedAt:          time.Now(),
		Hash:               contentHash,
		Size:               size,
		ComplexityScore:    f.calculateSingleChunkComplexity(semanticChunk),
		CohesionScore:      1.0, // Single functions have perfect cohesion
	}

	return chunk, nil
}

// includeHelperFunctions identifies and includes related helper functions with main functions.
// Phase 4.3 Step 2: Implements helper function inclusion for context preservation.
func (f *FunctionChunker) includeHelperFunctions(
	ctx context.Context,
	enhancedChunks []outbound.EnhancedCodeChunk,
	allSemanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) []outbound.EnhancedCodeChunk {
	// Create a map of function names to chunks for quick lookup
	chunkMap := make(map[string]*outbound.EnhancedCodeChunk)
	for i := range enhancedChunks {
		if len(enhancedChunks[i].SemanticConstructs) > 0 {
			name := enhancedChunks[i].SemanticConstructs[0].Name
			chunkMap[name] = &enhancedChunks[i]
		}
	}

	// Analyze function calls to identify helper relationships
	for i := range enhancedChunks {
		chunk := &enhancedChunks[i]
		if len(chunk.SemanticConstructs) > 0 {
			mainFunction := chunk.SemanticConstructs[0]

			// Phase 4.3 Step 2: Extract function calls from content analysis
			allCalledFunctions := append([]outbound.FunctionCall{}, mainFunction.CalledFunctions...)

			// Analyze content for additional function calls
			contentCalls := f.analysisUtils.ExtractFunctionCallsFromContent(mainFunction.Content, mainFunction.Language)
			allCalledFunctions = append(allCalledFunctions, contentCalls...)

			// Find called functions that exist in our chunk set
			for _, funcCall := range allCalledFunctions {
				if helperChunk, exists := chunkMap[funcCall.Name]; exists && helperChunk.ID != chunk.ID {
					// Add helper function as dependency (external helper)
					dependency := outbound.ChunkDependency{
						Type:         outbound.DependencyCall,
						TargetChunk:  helperChunk.ID,
						TargetSymbol: funcCall.Name,
						Relationship: "helper_function",
						Strength:     0.8,
						IsResolved:   true,
					}
					chunk.Dependencies = append(chunk.Dependencies, dependency)

					// Update relationship count for cohesion
					chunk.CohesionScore = f.calculateHelperCohesion(chunk, helperChunk)
				} else {
					// Phase 4.3 Step 2: Handle internal closures/nested functions
					if f.analysisUtils.IsInternalFunction(funcCall.Name, mainFunction) {
						dependency := outbound.ChunkDependency{
							Type:         outbound.DependencyCall,
							TargetSymbol: funcCall.Name,
							Relationship: "internal_closure",
							Strength:     0.9, // Internal closures have high strength
							IsResolved:   true,
						}
						chunk.Dependencies = append(chunk.Dependencies, dependency)

						// Internal closures also increase cohesion
						chunk.CohesionScore = 0.95
					}
				}
			}
		}
	}

	return enhancedChunks
}

// mapSemanticToChunkType maps semantic construct types to chunk types.
func (f *FunctionChunker) mapSemanticToChunkType(semanticType outbound.SemanticConstructType) outbound.ChunkType {
	switch semanticType {
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

// extractSingleChunkDependencies extracts dependencies from a single semantic chunk.
func (f *FunctionChunker) extractSingleChunkDependencies(chunk outbound.SemanticCodeChunk) []outbound.ChunkDependency {
	var dependencies []outbound.ChunkDependency

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

	return dependencies
}

// calculateSingleChunkComplexity calculates complexity for a single semantic chunk.
func (f *FunctionChunker) calculateSingleChunkComplexity(chunk outbound.SemanticCodeChunk) float64 {
	complexity := 0.0

	// Base complexity by construct type
	switch chunk.Type {
	case outbound.ConstructFunction, outbound.ConstructMethod:
		complexity = 1.0
	case outbound.ConstructClass, outbound.ConstructStruct:
		complexity = 2.0
	case outbound.ConstructInterface:
		complexity = 1.5
	default:
		complexity = 0.5
	}

	// Add complexity for dependencies
	complexity += float64(len(chunk.CalledFunctions)) * 0.1
	complexity += float64(len(chunk.UsedTypes)) * 0.1

	// Normalize to 0-1 range
	return complexity / 3.0 // Max expected complexity for a single function
}

// calculateHelperCohesion calculates cohesion score when including helper functions.
func (f *FunctionChunker) calculateHelperCohesion(mainChunk, helperChunk *outbound.EnhancedCodeChunk) float64 {
	// Helper functions that are called increase cohesion
	return 0.9 // High cohesion for helper relationship
}

// hasCircularDependency detects circular dependencies between functions.
// Phase 4.3 Step 2: Circular dependency detection for error handling.
func (f *FunctionChunker) hasCircularDependency(
	chunk outbound.SemanticCodeChunk,
	allChunks []outbound.SemanticCodeChunk,
) bool {
	// Create a map of function names to their called functions
	functionCalls := make(map[string][]string)

	// Build the call graph
	for _, c := range allChunks {
		var calls []string
		for _, funcCall := range c.CalledFunctions {
			calls = append(calls, funcCall.Name)
		}
		functionCalls[c.Name] = calls
	}

	// Add current chunk to the map
	var currentCalls []string
	for _, funcCall := range chunk.CalledFunctions {
		currentCalls = append(currentCalls, funcCall.Name)
	}
	functionCalls[chunk.Name] = currentCalls

	// Use DFS to detect cycles
	visited := make(map[string]bool)
	recursionStack := make(map[string]bool)

	return f.detectCycleFromNode(chunk.Name, functionCalls, visited, recursionStack)
}

// detectCycleFromNode performs DFS to detect cycles starting from a given node.
func (f *FunctionChunker) detectCycleFromNode(
	node string,
	graph map[string][]string,
	visited map[string]bool,
	recursionStack map[string]bool,
) bool {
	visited[node] = true
	recursionStack[node] = true

	// Visit all adjacent nodes
	for _, neighbor := range graph[node] {
		// If neighbor is in recursion stack, we found a cycle
		if recursionStack[neighbor] {
			return true
		}

		// If neighbor hasn't been visited, recurse
		if !visited[neighbor] {
			if f.detectCycleFromNode(neighbor, graph, visited, recursionStack) {
				return true
			}
		}
	}

	// Remove from recursion stack before returning
	recursionStack[node] = false
	return false
}

// extractFunctionCallsFromContent extracts function calls by analyzing code content.
// Phase 4.3 Step 2: Enhanced called function detection for helper relationship analysis.
func (f *FunctionChunker) extractFunctionCallsFromContent(
	content string,
	language valueobject.Language,
) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Simple pattern matching for function calls (basic implementation)
		switch language.Name() {
		case valueobject.LanguageGo:
			calls = append(calls, f.extractGoFunctionCalls(line, i)...)
		case valueobject.LanguagePython:
			calls = append(calls, f.extractPythonFunctionCalls(line, i)...)
		case valueobject.LanguageJavaScript, valueobject.LanguageTypeScript:
			calls = append(calls, f.extractJSFunctionCalls(line, i)...)
		}
	}

	return calls
}

// extractGoFunctionCalls extracts Go function calls from a line of code.
func (f *FunctionChunker) extractGoFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Simple pattern: functionName( or variable.functionName(
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if f.isValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: valueobject.ClampToUint32(lineNum * 80), // Rough estimate
					EndByte:   valueobject.ClampToUint32(lineNum*80 + len(word)),
				})
			}
		}
	}

	return calls
}

// extractPythonFunctionCalls extracts Python function calls from a line of code.
func (f *FunctionChunker) extractPythonFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Similar logic for Python
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if f.isValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: valueobject.ClampToUint32(lineNum * 80),
					EndByte:   valueobject.ClampToUint32(lineNum*80 + len(word)),
				})
			}
		}
	}

	return calls
}

// extractJSFunctionCalls extracts JavaScript function calls from a line of code.
func (f *FunctionChunker) extractJSFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Similar logic for JavaScript
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if f.isValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: valueobject.ClampToUint32(lineNum * 80),
					EndByte:   valueobject.ClampToUint32(lineNum*80 + len(word)),
				})
			}
		}
	}

	return calls
}

// isValidFunctionName checks if a string looks like a valid function name.
func (f *FunctionChunker) isValidFunctionName(name string) bool {
	if len(name) < 1 || len(name) > 100 {
		return false
	}

	// Must start with letter or underscore
	if !((name[0] >= 'a' && name[0] <= 'z') ||
		(name[0] >= 'A' && name[0] <= 'Z') ||
		name[0] == '_') {
		return false
	}

	// Common non-function words
	nonFunctions := []string{"if", "else", "for", "while", "return", "var", "let", "const"}
	for _, nonFunc := range nonFunctions {
		if name == nonFunc {
			return false
		}
	}

	return true
}

// isInternalFunction checks if a function call refers to an internal function/closure within the same chunk.
// Phase 4.3 Step 2: Internal closure and nested function detection.
func (f *FunctionChunker) isInternalFunction(funcName string, chunk outbound.SemanticCodeChunk) bool {
	// Check if the function is defined within the chunk content
	content := strings.ToLower(chunk.Content)
	funcName = strings.ToLower(funcName)

	// Language-specific patterns for internal function definitions
	switch chunk.Language.Name() {
	case valueobject.LanguageJavaScript, valueobject.LanguageTypeScript:
		// JavaScript patterns: "const funcName =", "function funcName", "let funcName ="
		patterns := []string{
			"const " + funcName + " =",
			"let " + funcName + " =",
			"var " + funcName + " =",
			"function " + funcName,
		}
		for _, pattern := range patterns {
			if strings.Contains(content, pattern) {
				return true
			}
		}
	case valueobject.LanguagePython:
		// Python patterns: "def funcName("
		if strings.Contains(content, "def "+funcName+"(") {
			return true
		}
	case valueobject.LanguageGo:
		// Go patterns: "func funcName(" (but usually Go doesn't have nested functions)
		if strings.Contains(content, "func "+funcName+"(") {
			return true
		}
	}

	return false
}
