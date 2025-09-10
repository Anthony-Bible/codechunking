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
	"time"

	"github.com/google/uuid"
)

// ClassChunker implements class-level chunking algorithms that group related classes/structs
// and maintain class boundaries with proper inheritance and interface relationships.
type ClassChunker struct{}

// NewClassChunker creates a new class-level chunker.
func NewClassChunker() *ClassChunker {
	return &ClassChunker{}
}

// ChunkByClass groups semantic chunks based on class boundaries and hierarchical relationships.
// This strategy prioritizes keeping complete classes together with their methods and properties.
func (c *ClassChunker) ChunkByClass(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Info(ctx, "Starting class-based chunking", slogger.Fields{
		"semantic_chunks": len(semanticChunks),
		"max_chunk_size":  config.MaxChunkSize,
		"min_chunk_size":  config.MinChunkSize,
	})

	if len(semanticChunks) == 0 {
		return []outbound.EnhancedCodeChunk{}, nil
	}

	// Analyze class hierarchies and relationships
	classGroups := c.groupByClassHierarchy(ctx, semanticChunks)

	var enhancedChunks []outbound.EnhancedCodeChunk

	for _, group := range classGroups {
		chunks, err := c.createEnhancedChunksFromClassGroup(ctx, group, config)
		if err != nil {
			return nil, fmt.Errorf("failed to create enhanced chunks from class group: %w", err)
		}
		enhancedChunks = append(enhancedChunks, chunks...)
	}

	slogger.Info(ctx, "Class-based chunking completed", slogger.Fields{
		"input_chunks":  len(semanticChunks),
		"output_chunks": len(enhancedChunks),
		"class_groups":  len(classGroups),
	})

	return enhancedChunks, nil
}

// groupByClassHierarchy analyzes semantic chunks to identify class hierarchies and relationships.
func (c *ClassChunker) groupByClassHierarchy(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
) [][]outbound.SemanticCodeChunk {
	// Sort chunks by position for logical grouping
	sortedChunks := make([]outbound.SemanticCodeChunk, len(semanticChunks))
	copy(sortedChunks, semanticChunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].StartByte < sortedChunks[j].StartByte
	})

	// Build class relationship map
	classMap := c.buildClassRelationshipMap(ctx, sortedChunks)

	// Group chunks based on class relationships
	groups := c.createGroupsFromClassMap(ctx, classMap, sortedChunks)

	slogger.Debug(ctx, "Created class hierarchy groups", slogger.Fields{
		"total_groups":       len(groups),
		"input_chunks":       len(semanticChunks),
		"classes_identified": len(classMap),
	})

	return groups
}

// buildClassRelationshipMap creates a map of class relationships from semantic chunks.
func (c *ClassChunker) buildClassRelationshipMap(
	ctx context.Context,
	semanticChunks []outbound.SemanticCodeChunk,
) map[string]*ClassInfo {
	classMap := make(map[string]*ClassInfo)

	for _, chunk := range semanticChunks {
		if c.isClassLikeConstruct(chunk.Type) {
			classInfo := &ClassInfo{
				Name:          chunk.Name,
				QualifiedName: chunk.QualifiedName,
				Chunk:         chunk,
				Methods:       []outbound.SemanticCodeChunk{},
				Properties:    []outbound.SemanticCodeChunk{},
				Language:      chunk.Language,
			}
			classMap[chunk.QualifiedName] = classInfo
		}
	}

	// Handle nested classes - merge nested classes into their parent
	c.handleNestedClasses(classMap, semanticChunks)

	// Second pass: associate methods and properties with classes
	for _, chunk := range semanticChunks {
		switch chunk.Type {
		case outbound.ConstructMethod, outbound.ConstructFunction:
			// Try to find the parent class for this method
			if parentClass := c.findParentClass(chunk, classMap); parentClass != nil {
				parentClass.Methods = append(parentClass.Methods, chunk)
			}
		case outbound.ConstructField, outbound.ConstructProperty:
			// Try to find the parent class for this property
			if parentClass := c.findParentClass(chunk, classMap); parentClass != nil {
				parentClass.Properties = append(parentClass.Properties, chunk)
			}
		case outbound.ConstructClass, outbound.ConstructStruct, outbound.ConstructInterface:
			// Already handled in first pass
		case outbound.ConstructEnum, outbound.ConstructVariable, outbound.ConstructConstant,
			outbound.ConstructModule, outbound.ConstructPackage, outbound.ConstructNamespace,
			outbound.ConstructType, outbound.ConstructComment, outbound.ConstructDecorator,
			outbound.ConstructAttribute, outbound.ConstructLambda, outbound.ConstructClosure,
			outbound.ConstructGenerator, outbound.ConstructAsyncFunction:
			// These types are not associated with classes in this chunking strategy
		}
	}

	return classMap
}

// handleNestedClasses identifies nested classes and merges them into their parent classes.
func (c *ClassChunker) handleNestedClasses(
	classMap map[string]*ClassInfo,
	semanticChunks []outbound.SemanticCodeChunk,
) {
	// Find classes that are nested inside others based on byte ranges
	var nestedClasses []*ClassInfo

	for _, classInfo := range classMap {
		// Check if this class is nested inside another class
		parentClass := c.findParentClassForNestedClass(classInfo, classMap)
		if parentClass != nil {
			// Add nested class as a method to the parent (for content inclusion)
			parentClass.Methods = append(parentClass.Methods, classInfo.Chunk)
			nestedClasses = append(nestedClasses, classInfo)
		}
	}

	// Remove nested classes from the main class map since they're now part of parent classes
	for _, nestedClass := range nestedClasses {
		delete(classMap, nestedClass.QualifiedName)
	}
}

// findParentClassForNestedClass finds the parent class for a potentially nested class.
func (c *ClassChunker) findParentClassForNestedClass(
	nestedClass *ClassInfo,
	classMap map[string]*ClassInfo,
) *ClassInfo {
	var bestParent *ClassInfo
	smallestRange := ^uint32(0)

	for _, potentialParent := range classMap {
		// Skip self
		if potentialParent.QualifiedName == nestedClass.QualifiedName {
			continue
		}

		// Check if nested class is contained within potential parent's byte range
		if nestedClass.Chunk.StartByte >= potentialParent.Chunk.StartByte &&
			nestedClass.Chunk.EndByte <= potentialParent.Chunk.EndByte {
			// Prefer the parent with smallest containing range (most immediate parent)
			parentRange := potentialParent.Chunk.EndByte - potentialParent.Chunk.StartByte
			if parentRange < smallestRange {
				smallestRange = parentRange
				bestParent = potentialParent
			}
		}
	}

	// If no containment found, check for indentation-based nesting (for Python)
	if bestParent == nil && nestedClass.Chunk.Language.Name() == valueobject.LanguagePython {
		// Look for classes that start with indentation suggesting nesting
		if c.isIndentedClass(nestedClass.Chunk) {
			// Find the closest preceding class as potential parent
			var closestParent *ClassInfo
			minDistance := ^uint32(0)

			for _, potentialParent := range classMap {
				if potentialParent.QualifiedName == nestedClass.QualifiedName {
					continue
				}

				// Check if potential parent comes before nested class
				if potentialParent.Chunk.EndByte <= nestedClass.Chunk.StartByte {
					distance := nestedClass.Chunk.StartByte - potentialParent.Chunk.EndByte
					if distance < minDistance {
						minDistance = distance
						closestParent = potentialParent
					}
				}
			}

			// Only consider if they're close (within 100 bytes)
			if closestParent != nil && minDistance <= 100 {
				bestParent = closestParent
			}
		}
	}

	return bestParent
}

// isIndentedClass checks if a class appears to be nested based on indentation.
func (c *ClassChunker) isIndentedClass(chunk outbound.SemanticCodeChunk) bool {
	// Simple heuristic: check if content starts with spaces (indicating indentation)
	content := chunk.Content
	if len(content) > 0 && (content[0] == ' ' || content[0] == '\t') {
		return true
	}
	return false
}

// ClassInfo represents information about a class and its associated chunks.
type ClassInfo struct {
	Name          string
	QualifiedName string
	Chunk         outbound.SemanticCodeChunk
	Methods       []outbound.SemanticCodeChunk
	Properties    []outbound.SemanticCodeChunk
	Language      valueobject.Language
}

// isClassLikeConstruct determines if a semantic construct represents a class-like entity.
func (c *ClassChunker) isClassLikeConstruct(constructType outbound.SemanticConstructType) bool {
	switch constructType {
	case outbound.ConstructClass, outbound.ConstructStruct, outbound.ConstructInterface:
		return true
	case outbound.ConstructFunction, outbound.ConstructMethod, outbound.ConstructEnum,
		outbound.ConstructVariable, outbound.ConstructConstant, outbound.ConstructField,
		outbound.ConstructProperty, outbound.ConstructModule, outbound.ConstructPackage,
		outbound.ConstructNamespace, outbound.ConstructType, outbound.ConstructComment,
		outbound.ConstructDecorator, outbound.ConstructAttribute, outbound.ConstructLambda,
		outbound.ConstructClosure, outbound.ConstructGenerator, outbound.ConstructAsyncFunction:
		return false
	default:
		return false
	}
}

// findParentClass attempts to find the parent class for a method or property chunk.
func (c *ClassChunker) findParentClass(
	chunk outbound.SemanticCodeChunk,
	classMap map[string]*ClassInfo,
) *ClassInfo {
	// For methods/functions, check qualified name first
	if chunk.Type == outbound.ConstructMethod || chunk.Type == outbound.ConstructFunction {
		// Extract receiver type from qualified name (e.g., "Calculator.Add" -> "Calculator")
		for _, classInfo := range classMap {
			// Check if the method's qualified name suggests it belongs to this class
			if chunk.QualifiedName != "" {
				// Look for patterns like "ClassName.MethodName" or "*ClassName.MethodName"
				if len(chunk.QualifiedName) > len(classInfo.Name) {
					if chunk.QualifiedName[len(chunk.QualifiedName)-len(classInfo.Name)-1:] == classInfo.Name+"."+chunk.Name ||
						chunk.QualifiedName == classInfo.QualifiedName+"."+chunk.Name ||
						chunk.QualifiedName == "*"+classInfo.Name+"."+chunk.Name ||
						chunk.QualifiedName == classInfo.Name+"."+chunk.Name {
						return classInfo
					}
				}
			}
		}
	}

	// Fallback: find the closest class that contains this chunk's byte range
	var bestMatch *ClassInfo
	smallestRange := ^uint32(0) // Max uint32

	for _, classInfo := range classMap {
		// Check if the chunk is within the class's byte range
		if chunk.StartByte >= classInfo.Chunk.StartByte &&
			chunk.EndByte <= classInfo.Chunk.EndByte {
			// Prefer the class with the smallest containing range (most specific)
			classRange := classInfo.Chunk.EndByte - classInfo.Chunk.StartByte
			if classRange < smallestRange {
				smallestRange = classRange
				bestMatch = classInfo
			}
		}
	}

	// Second fallback: find the closest class by proximity (for methods defined outside class body)
	if bestMatch == nil {
		var closestClass *ClassInfo
		minDistance := ^uint32(0)

		for _, classInfo := range classMap {
			var distance uint32
			if chunk.StartByte > classInfo.Chunk.EndByte {
				distance = chunk.StartByte - classInfo.Chunk.EndByte
			} else if classInfo.Chunk.StartByte > chunk.EndByte {
				distance = classInfo.Chunk.StartByte - chunk.EndByte
			} else {
				// Overlapping - prefer this
				distance = 0
			}

			if distance < minDistance {
				minDistance = distance
				closestClass = classInfo
			}
		}

		// Only use proximity if within reasonable distance (1000 bytes)
		if closestClass != nil && minDistance <= 1000 {
			bestMatch = closestClass
		}
	}

	return bestMatch
}

// createGroupsFromClassMap creates chunk groups based on the class relationship map.
func (c *ClassChunker) createGroupsFromClassMap(
	ctx context.Context,
	classMap map[string]*ClassInfo,
	allChunks []outbound.SemanticCodeChunk,
) [][]outbound.SemanticCodeChunk {
	var groups [][]outbound.SemanticCodeChunk
	processedChunks := make(map[string]bool)

	// Create groups for each class
	for _, classInfo := range classMap {
		var group []outbound.SemanticCodeChunk

		// Add the class chunk itself
		group = append(group, classInfo.Chunk)
		processedChunks[classInfo.Chunk.ID()] = true

		// Add associated methods
		for _, method := range classInfo.Methods {
			group = append(group, method)
			processedChunks[method.ID()] = true
		}

		// Add associated properties
		for _, property := range classInfo.Properties {
			group = append(group, property)
			processedChunks[property.ID()] = true
		}

		if len(group) > 0 {
			groups = append(groups, group)
		}
	}

	// Handle remaining chunks that don't belong to any class
	var orphanGroup []outbound.SemanticCodeChunk
	for _, chunk := range allChunks {
		if !processedChunks[chunk.ID()] {
			orphanGroup = append(orphanGroup, chunk)
		}
	}

	// Group orphan chunks by proximity
	if len(orphanGroup) > 0 {
		orphanGroups := c.groupOrphanChunks(ctx, orphanGroup)
		groups = append(groups, orphanGroups...)
	}

	return groups
}

// groupOrphanChunks groups chunks that don't belong to any class by proximity and type.
func (c *ClassChunker) groupOrphanChunks(
	ctx context.Context,
	orphanChunks []outbound.SemanticCodeChunk,
) [][]outbound.SemanticCodeChunk {
	if len(orphanChunks) == 0 {
		return nil
	}

	// Sort orphan chunks by position
	sort.Slice(orphanChunks, func(i, j int) bool {
		return orphanChunks[i].StartByte < orphanChunks[j].StartByte
	})

	var groups [][]outbound.SemanticCodeChunk
	var currentGroup []outbound.SemanticCodeChunk

	for _, chunk := range orphanChunks {
		if c.shouldStartNewOrphanGroup(currentGroup, chunk) {
			if len(currentGroup) > 0 {
				groups = append(groups, currentGroup)
			}
			currentGroup = []outbound.SemanticCodeChunk{chunk}
		} else {
			currentGroup = append(currentGroup, chunk)
		}
	}

	// Add the final group
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	return groups
}

// shouldStartNewOrphanGroup determines if orphan chunks should be grouped together.
func (c *ClassChunker) shouldStartNewOrphanGroup(
	currentGroup []outbound.SemanticCodeChunk,
	newChunk outbound.SemanticCodeChunk,
) bool {
	if len(currentGroup) == 0 {
		return true
	}

	lastChunk := currentGroup[len(currentGroup)-1]

	// Different languages should be in separate groups
	if !lastChunk.Language.Equal(newChunk.Language) {
		return true
	}

	// Large gaps suggest logical separation
	if newChunk.StartByte > lastChunk.EndByte+1000 { // 1KB gap threshold
		return true
	}

	// Different construct types might warrant separation
	if c.areIncompatibleConstructTypes(lastChunk.Type, newChunk.Type) {
		return true
	}

	return false
}

// areIncompatibleConstructTypes determines if two construct types should not be grouped together.
func (c *ClassChunker) areIncompatibleConstructTypes(
	type1, type2 outbound.SemanticConstructType,
) bool {
	// Module-level constructs should be separate from others
	if type1 == outbound.ConstructModule || type2 == outbound.ConstructModule {
		return type1 != type2
	}

	// Package-level constructs should be separate from others
	if type1 == outbound.ConstructPackage || type2 == outbound.ConstructPackage {
		return type1 != type2
	}

	return false
}

// createEnhancedChunksFromClassGroup converts a class group into enhanced chunks.
func (c *ClassChunker) createEnhancedChunksFromClassGroup(
	ctx context.Context,
	group []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	if len(group) == 0 {
		return []outbound.EnhancedCodeChunk{}, nil
	}

	// Check if the group should be split based on size constraints
	groupSize := c.calculateGroupSize(group)
	if groupSize > config.MaxChunkSize && config.EnableSplitting {
		return c.splitLargeClassGroup(ctx, group, config)
	}

	// Create a single enhanced chunk from the group
	chunk, err := c.createSingleEnhancedChunk(ctx, group, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create enhanced chunk: %w", err)
	}

	return []outbound.EnhancedCodeChunk{*chunk}, nil
}

// splitLargeClassGroup splits a large class group while maintaining class integrity.
func (c *ClassChunker) splitLargeClassGroup(
	ctx context.Context,
	group []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Debug(ctx, "Splitting large class group", slogger.Fields{
		"group_size": len(group),
		"max_size":   config.MaxChunkSize,
	})

	// Strategy: Try to keep class definitions with their most important methods
	var enhancedChunks []outbound.EnhancedCodeChunk

	// Find class definitions in the group
	var classChunks []outbound.SemanticCodeChunk
	var otherChunks []outbound.SemanticCodeChunk

	for _, chunk := range group {
		if c.isClassLikeConstruct(chunk.Type) {
			classChunks = append(classChunks, chunk)
		} else {
			otherChunks = append(otherChunks, chunk)
		}
	}

	// If we have class definitions, create chunks around them
	if len(classChunks) > 0 {
		for _, classChunk := range classChunks {
			subGroup := []outbound.SemanticCodeChunk{classChunk}
			currentSize := len(classChunk.Content)

			// Add related methods/properties that fit
			for _, otherChunk := range otherChunks {
				if currentSize+len(otherChunk.Content) <= config.MaxChunkSize {
					subGroup = append(subGroup, otherChunk)
					currentSize += len(otherChunk.Content)
				}
			}

			enhancedChunk, err := c.createSingleEnhancedChunk(ctx, subGroup, config)
			if err != nil {
				return nil, fmt.Errorf("failed to create class-based split chunk: %w", err)
			}
			enhancedChunks = append(enhancedChunks, *enhancedChunk)
		}
	} else {
		// No class definitions, split by size
		var currentSubGroup []outbound.SemanticCodeChunk
		currentSize := 0

		for _, chunk := range otherChunks {
			chunkSize := len(chunk.Content)

			if currentSize+chunkSize > config.MaxChunkSize && len(currentSubGroup) > 0 {
				enhancedChunk, err := c.createSingleEnhancedChunk(ctx, currentSubGroup, config)
				if err != nil {
					return nil, fmt.Errorf("failed to create size-based split chunk: %w", err)
				}
				enhancedChunks = append(enhancedChunks, *enhancedChunk)

				currentSubGroup = []outbound.SemanticCodeChunk{chunk}
				currentSize = chunkSize
			} else {
				currentSubGroup = append(currentSubGroup, chunk)
				currentSize += chunkSize
			}
		}

		// Handle remaining subgroup
		if len(currentSubGroup) > 0 {
			enhancedChunk, err := c.createSingleEnhancedChunk(ctx, currentSubGroup, config)
			if err != nil {
				return nil, fmt.Errorf("failed to create final split chunk: %w", err)
			}
			enhancedChunks = append(enhancedChunks, *enhancedChunk)
		}
	}

	return enhancedChunks, nil
}

// createSingleEnhancedChunk creates an enhanced chunk from a class group.
func (c *ClassChunker) createSingleEnhancedChunk(
	ctx context.Context,
	group []outbound.SemanticCodeChunk,
	config outbound.ChunkingConfiguration,
) (*outbound.EnhancedCodeChunk, error) {
	if len(group) == 0 {
		return nil, errors.New("cannot create enhanced chunk from empty group")
	}

	// Calculate boundaries and content
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
	chunkType := c.determineChunkType(group)
	dependencies := c.extractDependencies(group)

	// Calculate size metrics
	size := outbound.ChunkSize{
		Bytes:      len(allContent),
		Lines:      c.countLines(allContent),
		Characters: len(allContent),
		Constructs: len(group),
	}

	// Calculate metrics
	complexityScore := c.calculateComplexityScore(group)
	cohesionScore := c.calculateCohesionScore(group)

	chunk := &outbound.EnhancedCodeChunk{
		ID:                 chunkID,
		SourceFile:         c.determineSourceFile(group),
		Language:           c.parseLanguage(primaryLanguage),
		StartByte:          minStart,
		EndByte:            maxEnd,
		StartPosition:      group[0].StartPosition,
		EndPosition:        group[len(group)-1].EndPosition,
		Content:            allContent,
		PreservedContext:   outbound.PreservedContext{},
		SemanticConstructs: group,
		Dependencies:       dependencies,
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

// Helper methods

func (c *ClassChunker) calculateGroupSize(group []outbound.SemanticCodeChunk) int {
	totalSize := 0
	for _, chunk := range group {
		totalSize += len(chunk.Content)
	}
	return totalSize
}

func (c *ClassChunker) determineChunkType(group []outbound.SemanticCodeChunk) outbound.ChunkType {
	typeCounts := make(map[outbound.SemanticConstructType]int)
	for _, chunk := range group {
		typeCounts[chunk.Type]++
	}

	// Prioritize class-like types
	if typeCounts[outbound.ConstructClass] > 0 {
		return outbound.ChunkClass
	}
	if typeCounts[outbound.ConstructStruct] > 0 {
		return outbound.ChunkStruct
	}
	if typeCounts[outbound.ConstructInterface] > 0 {
		return outbound.ChunkInterface
	}

	// Fall back to most common type
	var maxCount int
	var mostCommonType outbound.SemanticConstructType
	for constructType, count := range typeCounts {
		if count > maxCount {
			maxCount = count
			mostCommonType = constructType
		}
	}

	switch mostCommonType {
	case outbound.ConstructFunction, outbound.ConstructMethod:
		return outbound.ChunkFunction
	case outbound.ConstructVariable:
		return outbound.ChunkVariable
	case outbound.ConstructConstant:
		return outbound.ChunkConstant
	case outbound.ConstructClass, outbound.ConstructStruct, outbound.ConstructInterface:
		// These should have been handled earlier, but handle as class type
		return outbound.ChunkClass
	case outbound.ConstructEnum, outbound.ConstructField, outbound.ConstructProperty,
		outbound.ConstructModule, outbound.ConstructPackage, outbound.ConstructNamespace,
		outbound.ConstructType, outbound.ConstructComment, outbound.ConstructDecorator,
		outbound.ConstructAttribute, outbound.ConstructLambda, outbound.ConstructClosure,
		outbound.ConstructGenerator, outbound.ConstructAsyncFunction:
		return outbound.ChunkMixed
	default:
		return outbound.ChunkMixed
	}
}

func (c *ClassChunker) extractDependencies(group []outbound.SemanticCodeChunk) []outbound.ChunkDependency {
	var dependencies []outbound.ChunkDependency

	for _, chunk := range group {
		// Extract inheritance dependencies
		for _, dep := range chunk.Dependencies {
			dependencies = append(dependencies, outbound.ChunkDependency{
				Type:         outbound.DependencyInherit,
				TargetSymbol: dep.Name,
				Relationship: "inheritance",
				Strength:     0.9,
				IsResolved:   false,
			})
		}

		// Extract interface implementation dependencies
		for _, typeRef := range chunk.UsedTypes {
			dependencies = append(dependencies, outbound.ChunkDependency{
				Type:         outbound.DependencyInterface,
				TargetSymbol: typeRef.Name,
				Relationship: "interface_implementation",
				Strength:     0.85,
				IsResolved:   false,
			})
		}

		// Extract composition dependencies
		for _, funcCall := range chunk.CalledFunctions {
			dependencies = append(dependencies, outbound.ChunkDependency{
				Type:         outbound.DependencyCompose,
				TargetSymbol: funcCall.Name,
				Relationship: "composition",
				Strength:     0.7,
				IsResolved:   false,
			})
		}
	}

	return dependencies
}

func (c *ClassChunker) determineSourceFile(group []outbound.SemanticCodeChunk) string {
	if len(group) > 0 {
		return "aggregated_class_chunk"
	}
	return "unknown_source"
}

func (c *ClassChunker) parseLanguage(languageStr string) valueobject.Language {
	lang, err := valueobject.NewLanguage(languageStr)
	if err != nil {
		// If we can't create the language, return an unknown language
		unknownLang, _ := valueobject.NewLanguage(valueobject.LanguageUnknown)
		return unknownLang
	}
	return lang
}

func (c *ClassChunker) countLines(content string) int {
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

func (c *ClassChunker) calculateComplexityScore(group []outbound.SemanticCodeChunk) float64 {
	totalComplexity := 0.0

	for _, chunk := range group {
		// Class-based complexity weighting
		switch chunk.Type {
		case outbound.ConstructClass:
			totalComplexity += 3.0 // Classes are inherently more complex
		case outbound.ConstructInterface:
			totalComplexity += 2.5
		case outbound.ConstructStruct:
			totalComplexity += 2.0
		case outbound.ConstructMethod:
			totalComplexity += 1.5
		case outbound.ConstructFunction:
			totalComplexity += 1.0
		case outbound.ConstructEnum, outbound.ConstructVariable, outbound.ConstructConstant,
			outbound.ConstructField, outbound.ConstructProperty, outbound.ConstructModule,
			outbound.ConstructPackage, outbound.ConstructNamespace, outbound.ConstructType,
			outbound.ConstructComment, outbound.ConstructDecorator, outbound.ConstructAttribute,
			outbound.ConstructLambda, outbound.ConstructClosure, outbound.ConstructGenerator,
			outbound.ConstructAsyncFunction:
			totalComplexity += 0.5
		default:
			totalComplexity += 0.5
		}

		// Add complexity for relationships
		totalComplexity += float64(len(chunk.Dependencies)) * 0.2
		totalComplexity += float64(len(chunk.CalledFunctions)) * 0.1
		totalComplexity += float64(len(chunk.UsedTypes)) * 0.15
	}

	// Normalize to 0-1 range
	maxPossibleComplexity := float64(len(group)) * 4.0
	if maxPossibleComplexity == 0 {
		return 0.0
	}

	normalized := totalComplexity / maxPossibleComplexity
	if normalized > 1.0 {
		normalized = 1.0
	}

	return normalized
}

func (c *ClassChunker) calculateCohesionScore(group []outbound.SemanticCodeChunk) float64 {
	if len(group) <= 1 {
		return 1.0
	}

	// For class-based chunking, cohesion is higher when chunks belong to the same class
	totalRelations := 0
	classRelations := 0

	for i := range group {
		for j := i + 1; j < len(group); j++ {
			totalRelations++

			// Check if chunks are from the same class context
			if c.areFromSameClassContext(group[i], group[j]) {
				classRelations++
			}

			// Check for shared dependencies
			if c.hasSharedDependencies(group[i], group[j]) {
				classRelations++
			}
		}
	}

	if totalRelations == 0 {
		return 1.0
	}

	cohesion := float64(classRelations) / float64(totalRelations)
	return cohesion
}

func (c *ClassChunker) areFromSameClassContext(chunk1, chunk2 outbound.SemanticCodeChunk) bool {
	// Simple heuristic: check if chunks are close in byte range (indicating same class)
	gap := uint32(0)
	if chunk2.StartByte > chunk1.EndByte {
		gap = chunk2.StartByte - chunk1.EndByte
	} else if chunk1.StartByte > chunk2.EndByte {
		gap = chunk1.StartByte - chunk2.EndByte
	}

	return gap < 200 // 200 bytes threshold for same class context
}

func (c *ClassChunker) hasSharedDependencies(chunk1, chunk2 outbound.SemanticCodeChunk) bool {
	deps1 := make(map[string]bool)
	for _, dep := range chunk1.Dependencies {
		deps1[dep.Name] = true
	}

	for _, dep := range chunk2.Dependencies {
		if deps1[dep.Name] {
			return true
		}
	}

	return false
}
