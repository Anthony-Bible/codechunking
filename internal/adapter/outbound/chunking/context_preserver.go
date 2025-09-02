package chunking

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ContextPreserver implements context preservation algorithms that maintain semantic
// relationships and dependencies between chunks.
type ContextPreserver struct{}

// NewContextPreserver creates a new context preservation component.
func NewContextPreserver() *ContextPreserver {
	return &ContextPreserver{}
}

// PreserveContext applies context preservation strategies to maintain semantic relationships
// between chunks and their surrounding code.
func (c *ContextPreserver) PreserveContext(
	ctx context.Context,
	chunks []outbound.EnhancedCodeChunk,
	strategy outbound.ContextPreservationStrategy,
) ([]outbound.EnhancedCodeChunk, error) {
	slogger.Info(ctx, "Preserving context for chunks", slogger.Fields{
		"chunk_count": len(chunks),
		"strategy":    string(strategy),
	})

	if len(chunks) == 0 {
		return chunks, nil
	}

	var preservedChunks []outbound.EnhancedCodeChunk

	for _, chunk := range chunks {
		preservedChunk, err := c.preserveChunkContext(ctx, chunk, chunks, strategy)
		if err != nil {
			return nil, fmt.Errorf("failed to preserve context for chunk %s: %w", chunk.ID, err)
		}
		preservedChunks = append(preservedChunks, *preservedChunk)
	}

	slogger.Info(ctx, "Context preservation completed", slogger.Fields{
		"processed_chunks": len(preservedChunks),
		"strategy":         string(strategy),
	})

	return preservedChunks, nil
}

// preserveChunkContext applies context preservation to a single chunk.
func (c *ContextPreserver) preserveChunkContext(
	ctx context.Context,
	chunk outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
	strategy outbound.ContextPreservationStrategy,
) (*outbound.EnhancedCodeChunk, error) {
	preservedChunk := chunk // Create a copy

	// Extract and preserve import statements
	imports, err := c.extractImportStatements(ctx, chunk)
	if err != nil {
		slogger.Error(ctx, "Failed to extract import statements", slogger.Fields{
			"chunk_id": chunk.ID,
			"error":    err.Error(),
		})
	} else {
		preservedChunk.PreservedContext.ImportStatements = imports
	}

	// Apply strategy-specific context preservation
	switch strategy {
	case outbound.PreserveMinimal:
		err = c.applyMinimalContextPreservation(ctx, &preservedChunk)
	case outbound.PreserveModerate:
		err = c.applyModerateContextPreservation(ctx, &preservedChunk, allChunks)
	case outbound.PreserveMaximal:
		err = c.applyMaximalContextPreservation(ctx, &preservedChunk, allChunks)
	default:
		err = fmt.Errorf("unknown context preservation strategy: %s", strategy)
	}

	if err != nil {
		return nil, fmt.Errorf("context preservation failed: %w", err)
	}

	// Extract and preserve semantic relations
	relations := c.extractSemanticRelations(ctx, preservedChunk, allChunks)
	preservedChunk.PreservedContext.SemanticRelations = relations

	// Create metadata about context preservation
	if preservedChunk.PreservedContext.Metadata == nil {
		preservedChunk.PreservedContext.Metadata = make(map[string]interface{})
	}
	preservedChunk.PreservedContext.Metadata["preservation_strategy"] = string(strategy)
	preservedChunk.PreservedContext.Metadata["context_elements_count"] = c.countContextElements(
		preservedChunk.PreservedContext,
	)

	return &preservedChunk, nil
}

// applyMinimalContextPreservation applies minimal context preservation (basic imports and types).
func (c *ContextPreserver) applyMinimalContextPreservation(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
) error {
	// For minimal preservation, we only keep essential context

	// Extract used variables from semantic constructs
	usedVars := c.extractUsedVariables(chunk.SemanticConstructs)
	chunk.PreservedContext.UsedVariables = c.filterEssentialVariables(usedVars)

	// Extract essential type definitions
	typeRefs := c.extractTypeReferences(chunk.SemanticConstructs)
	chunk.PreservedContext.TypeDefinitions = c.filterEssentialTypes(typeRefs)

	// Extract called functions (but only direct calls)
	funcCalls := c.extractFunctionCalls(chunk.SemanticConstructs)
	chunk.PreservedContext.CalledFunctions = c.filterDirectFunctionCalls(funcCalls)

	slogger.Debug(ctx, "Applied minimal context preservation", slogger.Fields{
		"chunk_id":       chunk.ID,
		"variables":      len(chunk.PreservedContext.UsedVariables),
		"types":          len(chunk.PreservedContext.TypeDefinitions),
		"function_calls": len(chunk.PreservedContext.CalledFunctions),
	})

	return nil
}

// applyModerateContextPreservation applies moderate context preservation (includes surrounding context).
func (c *ContextPreserver) applyModerateContextPreservation(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) error {
	// Start with minimal preservation
	if err := c.applyMinimalContextPreservation(ctx, chunk); err != nil {
		return fmt.Errorf("failed to apply minimal context preservation: %w", err)
	}

	// Add surrounding context
	precedingContext, followingContext := c.findSurroundingContext(chunk, allChunks)
	chunk.PreservedContext.PrecedingContext = precedingContext
	chunk.PreservedContext.FollowingContext = followingContext

	// Add more comprehensive variable analysis
	allUsedVars := c.extractUsedVariables(chunk.SemanticConstructs)
	chunk.PreservedContext.UsedVariables = c.filterModerateVariables(allUsedVars)

	// Add documentation links
	docLinks := c.extractDocumentationLinks(chunk.SemanticConstructs)
	chunk.PreservedContext.DocumentationLinks = docLinks

	slogger.Debug(ctx, "Applied moderate context preservation", slogger.Fields{
		"chunk_id":            chunk.ID,
		"preceding_chars":     len(chunk.PreservedContext.PrecedingContext),
		"following_chars":     len(chunk.PreservedContext.FollowingContext),
		"documentation_links": len(chunk.PreservedContext.DocumentationLinks),
	})

	return nil
}

// applyMaximalContextPreservation applies maximal context preservation (comprehensive context).
func (c *ContextPreserver) applyMaximalContextPreservation(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) error {
	// Start with moderate preservation
	if err := c.applyModerateContextPreservation(ctx, chunk, allChunks); err != nil {
		return fmt.Errorf("failed to apply moderate context preservation: %w", err)
	}

	// Add comprehensive dependency analysis
	crossChunkDeps := c.analyzeCrossChunkDependencies(chunk, allChunks)
	chunk.Dependencies = append(chunk.Dependencies, crossChunkDeps...)

	// Add all variable references (not just essential ones)
	allUsedVars := c.extractUsedVariables(chunk.SemanticConstructs)
	chunk.PreservedContext.UsedVariables = allUsedVars

	// Add all type references
	allTypeRefs := c.extractTypeReferences(chunk.SemanticConstructs)
	chunk.PreservedContext.TypeDefinitions = allTypeRefs

	// Add comprehensive semantic relations
	allRelations := c.extractComprehensiveSemanticRelations(chunk, allChunks)
	chunk.PreservedContext.SemanticRelations = append(
		chunk.PreservedContext.SemanticRelations,
		allRelations...,
	)

	// Add extended context for better understanding
	extendedContext := c.buildExtendedContext(chunk, allChunks)
	if chunk.PreservedContext.Metadata == nil {
		chunk.PreservedContext.Metadata = make(map[string]interface{})
	}
	chunk.PreservedContext.Metadata["extended_context"] = extendedContext

	slogger.Debug(ctx, "Applied maximal context preservation", slogger.Fields{
		"chunk_id":           chunk.ID,
		"total_dependencies": len(chunk.Dependencies),
		"semantic_relations": len(chunk.PreservedContext.SemanticRelations),
		"context_metadata":   len(chunk.PreservedContext.Metadata),
	})

	return nil
}

// Helper methods for context extraction

func (c *ContextPreserver) extractImportStatements(
	ctx context.Context,
	chunk outbound.EnhancedCodeChunk,
) ([]outbound.ImportDeclaration, error) {
	var imports []outbound.ImportDeclaration

	// Phase 4.3 Step 2: Enhanced import extraction from dependencies and content
	for _, construct := range chunk.SemanticConstructs {
		// Extract from dependencies (Phase 4.3 Step 2 requirement)
		for _, dep := range construct.Dependencies {
			if dep.Type == "import" {
				import_ := outbound.ImportDeclaration{
					Path:        dep.Path,
					Content:     fmt.Sprintf("import \"%s\"", dep.Path),
					Hash:        hex.EncodeToString([]byte(dep.Path)),
					ExtractedAt: time.Now(),
				}
				imports = append(imports, import_)
			}
		}

		// Look for import-like patterns in the content
		importLines := c.findImportLines(construct.Content, chunk.Language)
		for _, importLine := range importLines {
			import_ := c.parseImportLine(importLine, chunk.Language)
			if import_.Path != "" {
				imports = append(imports, import_)
			}
		}
	}

	// Phase 4.3 Step 2: Extract imports from chunk dependencies
	for _, dep := range chunk.Dependencies {
		if dep.Type == outbound.DependencyImport {
			import_ := outbound.ImportDeclaration{
				Path:        dep.TargetSymbol,
				Content:     fmt.Sprintf("// Dependency import: %s", dep.TargetSymbol),
				Hash:        hex.EncodeToString([]byte(dep.TargetSymbol)),
				ExtractedAt: time.Now(),
			}
			imports = append(imports, import_)
		}
	}

	return imports, nil
}

func (c *ContextPreserver) findImportLines(content string, language valueobject.Language) []string {
	var importLines []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)

		switch language.Name() {
		case valueobject.LanguageGo:
			if strings.HasPrefix(trimmedLine, "import ") ||
				(strings.HasPrefix(trimmedLine, "import(") && strings.Contains(trimmedLine, ")")) {
				importLines = append(importLines, line)
			}
		case valueobject.LanguagePython:
			if strings.HasPrefix(trimmedLine, "import ") ||
				strings.HasPrefix(trimmedLine, "from ") {
				importLines = append(importLines, line)
			}
		case valueobject.LanguageJavaScript, valueobject.LanguageTypeScript:
			if strings.HasPrefix(trimmedLine, "import ") ||
				strings.HasPrefix(trimmedLine, "const ") && strings.Contains(trimmedLine, "require(") {
				importLines = append(importLines, line)
			}
		}
	}

	return importLines
}

func (c *ContextPreserver) parseImportLine(line string, language valueobject.Language) outbound.ImportDeclaration {
	trimmedLine := strings.TrimSpace(line)

	import_ := outbound.ImportDeclaration{
		Content: line,
		Hash:    hex.EncodeToString([]byte(line)), // Simple hash
	}

	switch language.Name() {
	case valueobject.LanguageGo:
		// Simple Go import parsing: import "path" or import alias "path"
		if strings.Contains(trimmedLine, "\"") {
			parts := strings.Split(trimmedLine, "\"")
			if len(parts) >= 2 {
				import_.Path = parts[1]
			}
		}
	case valueobject.LanguagePython:
		// Simple Python import parsing
		if strings.HasPrefix(trimmedLine, "import ") {
			import_.Path = strings.TrimSpace(strings.TrimPrefix(trimmedLine, "import "))
		} else if strings.HasPrefix(trimmedLine, "from ") {
			parts := strings.Split(trimmedLine, " ")
			if len(parts) >= 2 {
				import_.Path = parts[1]
			}
		}
	case valueobject.LanguageJavaScript, valueobject.LanguageTypeScript:
		// Simple JS/TS import parsing
		if strings.Contains(trimmedLine, "from ") && strings.Contains(trimmedLine, "\"") {
			parts := strings.Split(trimmedLine, "\"")
			if len(parts) >= 2 {
				import_.Path = parts[1]
			}
		}
	}

	return import_
}

func (c *ContextPreserver) extractUsedVariables(constructs []outbound.SemanticCodeChunk) []outbound.VariableReference {
	var variables []outbound.VariableReference

	for _, construct := range constructs {
		// Simple variable extraction - look for common variable patterns
		vars := c.findVariableReferences(construct.Content, construct.Language)
		variables = append(variables, vars...)
	}

	return variables
}

func (c *ContextPreserver) findVariableReferences(
	content string,
	language valueobject.Language,
) []outbound.VariableReference {
	var variables []outbound.VariableReference

	// This is a simplified implementation - a real implementation would use proper parsing
	words := strings.Fields(content)

	for _, word := range words {
		// Simple heuristic: words that look like variable names
		if c.looksLikeVariable(word, language) {
			variable := outbound.VariableReference{
				Name:  word,
				Scope: "unknown", // Would need proper scope analysis
			}
			variables = append(variables, variable)
		}
	}

	return variables
}

func (c *ContextPreserver) looksLikeVariable(word string, _ valueobject.Language) bool {
	// Simple heuristics for variable identification
	if len(word) < 2 || len(word) > 50 {
		return false
	}

	// Must start with letter or underscore
	if !((word[0] >= 'a' && word[0] <= 'z') ||
		(word[0] >= 'A' && word[0] <= 'Z') ||
		word[0] == '_') {
		return false
	}

	// Check for common non-variable patterns
	nonVariables := []string{"if", "else", "for", "while", "function", "class", "import", "return"}
	for _, nonVar := range nonVariables {
		if word == nonVar {
			return false
		}
	}

	return true
}

func (c *ContextPreserver) extractTypeReferences(constructs []outbound.SemanticCodeChunk) []outbound.TypeReference {
	var types []outbound.TypeReference

	// Phase 4.3 Step 2: Enhanced type reference extraction
	for _, construct := range constructs {
		// Extract used types from semantic construct
		for _, typeRef := range construct.UsedTypes {
			types = append(types, outbound.TypeReference{
				Name:          typeRef.Name,
				QualifiedName: typeRef.QualifiedName,
				IsGeneric:     typeRef.IsGeneric,
				GenericArgs:   typeRef.GenericArgs,
			})
		}

		// Phase 4.3 Step 2: Extract type information from function signatures
		if construct.ReturnType != "" {
			returnType := outbound.TypeReference{
				Name:          construct.ReturnType,
				QualifiedName: construct.ReturnType,
				IsGeneric: strings.Contains(construct.ReturnType, "[") ||
					strings.Contains(construct.ReturnType, "<"),
			}
			types = append(types, returnType)
		}

		// Extract parameter types
		for _, param := range construct.Parameters {
			if param.Type != "" {
				paramType := outbound.TypeReference{
					Name:          param.Type,
					QualifiedName: param.Type,
					IsGeneric:     strings.Contains(param.Type, "[") || strings.Contains(param.Type, "<"),
				}
				types = append(types, paramType)
			}
		}
	}

	return types
}

func (c *ContextPreserver) extractFunctionCalls(constructs []outbound.SemanticCodeChunk) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Phase 4.3 Step 2: Enhanced function call extraction
	for _, construct := range constructs {
		// Extract existing called functions
		for _, call := range construct.CalledFunctions {
			calls = append(calls, outbound.FunctionCall{
				Name:      call.Name,
				Arguments: call.Arguments,
				StartByte: call.StartByte,
				EndByte:   call.EndByte,
			})
		}

		// Phase 4.3 Step 2: Enhanced function call extraction from content analysis
		additionalCalls := c.extractFunctionCallsFromContent(construct.Content, construct.Language)
		calls = append(calls, additionalCalls...)
	}

	return calls
}

// extractFunctionCallsFromContent extracts function calls by analyzing code content.
// Phase 4.3 Step 2: Enhanced called function preservation.
func (c *ContextPreserver) extractFunctionCallsFromContent(
	content string,
	language valueobject.Language,
) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Simple pattern matching for function calls (basic implementation)
		switch language.Name() {
		case valueobject.LanguageGo:
			calls = append(calls, c.extractGoFunctionCalls(line, i)...)
		case valueobject.LanguagePython:
			calls = append(calls, c.extractPythonFunctionCalls(line, i)...)
		case valueobject.LanguageJavaScript, valueobject.LanguageTypeScript:
			calls = append(calls, c.extractJSFunctionCalls(line, i)...)
		}
	}

	return calls
}

// extractGoFunctionCalls extracts Go function calls from a line of code.
func (c *ContextPreserver) extractGoFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Simple pattern: functionName( or variable.functionName(
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if c.isValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: uint32(lineNum * 80), // Rough estimate
					EndByte:   uint32(lineNum*80 + len(word)),
				})
			}
		}
	}

	return calls
}

// extractPythonFunctionCalls extracts Python function calls from a line of code.
func (c *ContextPreserver) extractPythonFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Similar logic for Python
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if c.isValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: uint32(lineNum * 80),
					EndByte:   uint32(lineNum*80 + len(word)),
				})
			}
		}
	}

	return calls
}

// extractJSFunctionCalls extracts JavaScript function calls from a line of code.
func (c *ContextPreserver) extractJSFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Similar logic for JavaScript
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if c.isValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: uint32(lineNum * 80),
					EndByte:   uint32(lineNum*80 + len(word)),
				})
			}
		}
	}

	return calls
}

// isValidFunctionName checks if a string looks like a valid function name.
func (c *ContextPreserver) isValidFunctionName(name string) bool {
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

func (c *ContextPreserver) extractDocumentationLinks(
	constructs []outbound.SemanticCodeChunk,
) []outbound.DocumentationLink {
	var links []outbound.DocumentationLink

	for _, construct := range constructs {
		if construct.Documentation != "" {
			link := outbound.DocumentationLink{
				Type:        "construct_documentation",
				Target:      construct.Name,
				Description: construct.Documentation,
			}
			links = append(links, link)
		}
	}

	return links
}

func (c *ContextPreserver) findSurroundingContext(
	chunk *outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) (string, string) {
	// Sort chunks by position
	sortedChunks := make([]outbound.EnhancedCodeChunk, len(allChunks))
	copy(sortedChunks, allChunks)
	sort.Slice(sortedChunks, func(i, j int) bool {
		return sortedChunks[i].StartByte < sortedChunks[j].StartByte
	})

	var precedingContext, followingContext string
	const contextSize = 200 // Max characters of context

	// Find preceding context
	for i, otherChunk := range sortedChunks {
		if otherChunk.ID == chunk.ID {
			if i > 0 {
				prevChunk := sortedChunks[i-1]
				if len(prevChunk.Content) <= contextSize {
					precedingContext = prevChunk.Content
				} else {
					precedingContext = prevChunk.Content[len(prevChunk.Content)-contextSize:]
				}
			}
			break
		}
	}

	// Find following context
	for i, otherChunk := range sortedChunks {
		if otherChunk.ID == chunk.ID {
			if i < len(sortedChunks)-1 {
				nextChunk := sortedChunks[i+1]
				if len(nextChunk.Content) <= contextSize {
					followingContext = nextChunk.Content
				} else {
					followingContext = nextChunk.Content[:contextSize]
				}
			}
			break
		}
	}

	return precedingContext, followingContext
}

func (c *ContextPreserver) extractSemanticRelations(
	ctx context.Context,
	chunk outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) []outbound.SemanticRelation {
	var relations []outbound.SemanticRelation

	// Find chunks that this chunk depends on
	for _, otherChunk := range allChunks {
		if otherChunk.ID == chunk.ID {
			continue
		}

		// Check for function call relationships
		strength := c.calculateRelationshipStrength(chunk, otherChunk)
		if strength > 0.3 {
			relation := outbound.SemanticRelation{
				Type:        "dependency",
				Source:      chunk.ID,
				Target:      otherChunk.ID,
				Strength:    strength,
				Description: "Chunk dependency relationship",
			}
			relations = append(relations, relation)
		}
	}

	return relations
}

func (c *ContextPreserver) calculateRelationshipStrength(
	chunk1, chunk2 outbound.EnhancedCodeChunk,
) float64 {
	strength := 0.0

	// Check for shared dependencies
	sharedDeps := c.countSharedDependencies(chunk1.Dependencies, chunk2.Dependencies)
	strength += float64(sharedDeps) * 0.2

	// Check for proximity (closer chunks are more related)
	if c.areChunksClose(chunk1, chunk2) {
		strength += 0.3
	}

	// Check for same language
	if chunk1.Language.Equal(chunk2.Language) {
		strength += 0.1
	}

	return strength
}

// Filter methods for different preservation levels

func (c *ContextPreserver) filterEssentialVariables(vars []outbound.VariableReference) []outbound.VariableReference {
	// For minimal preservation, keep only variables that look important
	var essential []outbound.VariableReference
	for _, variable := range vars {
		if c.isEssentialVariable(variable) {
			essential = append(essential, variable)
		}
	}
	return essential
}

func (c *ContextPreserver) filterModerateVariables(vars []outbound.VariableReference) []outbound.VariableReference {
	// For moderate preservation, be more inclusive
	var moderate []outbound.VariableReference
	for _, variable := range vars {
		if c.isEssentialVariable(variable) || c.isImportantVariable(variable) {
			moderate = append(moderate, variable)
		}
	}
	return moderate
}

func (c *ContextPreserver) filterEssentialTypes(types []outbound.TypeReference) []outbound.TypeReference {
	// Keep only built-in and commonly used types for minimal preservation
	var essential []outbound.TypeReference
	for _, typeRef := range types {
		if c.isEssentialType(typeRef) {
			essential = append(essential, typeRef)
		}
	}
	return essential
}

func (c *ContextPreserver) filterDirectFunctionCalls(calls []outbound.FunctionCall) []outbound.FunctionCall {
	// For minimal preservation, keep only direct function calls (not nested)
	return calls // Simplified - would implement proper filtering
}

// Helper methods for analysis

func (c *ContextPreserver) analyzeCrossChunkDependencies(
	chunk *outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) []outbound.ChunkDependency {
	var deps []outbound.ChunkDependency

	for _, otherChunk := range allChunks {
		if otherChunk.ID == chunk.ID {
			continue
		}

		// Analyze if this chunk depends on the other chunk
		if c.hasSemanticDependency(*chunk, otherChunk) {
			dep := outbound.ChunkDependency{
				Type:         outbound.DependencyRef,
				TargetChunk:  otherChunk.ID,
				TargetSymbol: otherChunk.ID, // Simplified
				Relationship: "cross_chunk_dependency",
				Strength:     c.calculateRelationshipStrength(*chunk, otherChunk),
				IsResolved:   true,
			}
			deps = append(deps, dep)
		}
	}

	return deps
}

func (c *ContextPreserver) extractComprehensiveSemanticRelations(
	chunk *outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) []outbound.SemanticRelation {
	// For maximal preservation, extract all possible semantic relations
	var relations []outbound.SemanticRelation

	for _, otherChunk := range allChunks {
		if otherChunk.ID == chunk.ID {
			continue
		}

		strength := c.calculateRelationshipStrength(*chunk, otherChunk)
		if strength > 0.1 { // Lower threshold for maximal preservation
			relation := outbound.SemanticRelation{
				Type:        "comprehensive_relation",
				Source:      chunk.ID,
				Target:      otherChunk.ID,
				Strength:    strength,
				Description: "Comprehensive semantic relationship analysis",
			}
			relations = append(relations, relation)
		}
	}

	return relations
}

func (c *ContextPreserver) buildExtendedContext(
	chunk *outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) map[string]interface{} {
	context := make(map[string]interface{})

	context["chunk_position"] = c.calculateChunkPosition(*chunk, allChunks)
	context["related_chunks"] = c.findRelatedChunks(*chunk, allChunks)
	context["dependency_graph"] = c.buildDependencyGraph(*chunk, allChunks)

	return context
}

func (c *ContextPreserver) countContextElements(context outbound.PreservedContext) int {
	count := 0
	count += len(context.ImportStatements)
	count += len(context.TypeDefinitions)
	count += len(context.UsedVariables)
	count += len(context.CalledFunctions)
	count += len(context.DocumentationLinks)
	count += len(context.SemanticRelations)

	if context.PrecedingContext != "" {
		count++
	}
	if context.FollowingContext != "" {
		count++
	}

	return count
}

// Helper predicates

func (c *ContextPreserver) isEssentialVariable(variable outbound.VariableReference) bool {
	// Variables with certain names are considered essential
	essentialPatterns := []string{"config", "ctx", "err", "result", "data"}
	for _, pattern := range essentialPatterns {
		if strings.Contains(strings.ToLower(variable.Name), pattern) {
			return true
		}
	}
	return false
}

func (c *ContextPreserver) isImportantVariable(variable outbound.VariableReference) bool {
	// More variables are important than essential
	return len(variable.Name) > 2 && !c.isTemporaryVariable(variable)
}

func (c *ContextPreserver) isTemporaryVariable(variable outbound.VariableReference) bool {
	tempPatterns := []string{"i", "j", "k", "tmp", "temp"}
	for _, pattern := range tempPatterns {
		if variable.Name == pattern {
			return true
		}
	}
	return false
}

func (c *ContextPreserver) isEssentialType(typeRef outbound.TypeReference) bool {
	// Built-in and common types are essential
	essentialTypes := []string{"string", "int", "bool", "error", "interface{}", "map", "slice"}
	for _, essential := range essentialTypes {
		if strings.Contains(strings.ToLower(typeRef.Name), essential) {
			return true
		}
	}
	return false
}

func (c *ContextPreserver) countSharedDependencies(deps1, deps2 []outbound.ChunkDependency) int {
	shared := 0
	depMap := make(map[string]bool)

	for _, dep := range deps1 {
		depMap[dep.TargetSymbol] = true
	}

	for _, dep := range deps2 {
		if depMap[dep.TargetSymbol] {
			shared++
		}
	}

	return shared
}

func (c *ContextPreserver) areChunksClose(chunk1, chunk2 outbound.EnhancedCodeChunk) bool {
	// Check if chunks are within reasonable proximity
	gap := uint32(0)
	if chunk2.StartByte > chunk1.EndByte {
		gap = chunk2.StartByte - chunk1.EndByte
	} else if chunk1.StartByte > chunk2.EndByte {
		gap = chunk1.StartByte - chunk2.EndByte
	}

	return gap < 500 // 500 bytes threshold
}

func (c *ContextPreserver) hasSemanticDependency(chunk1, chunk2 outbound.EnhancedCodeChunk) bool {
	// Check if chunk1 semantically depends on chunk2
	for _, dep := range chunk1.Dependencies {
		if dep.TargetChunk == chunk2.ID {
			return true
		}
	}
	return false
}

func (c *ContextPreserver) calculateChunkPosition(
	chunk outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) int {
	position := 0
	for _, otherChunk := range allChunks {
		if otherChunk.StartByte < chunk.StartByte {
			position++
		}
	}
	return position
}

func (c *ContextPreserver) findRelatedChunks(
	chunk outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) []string {
	var related []string
	for _, otherChunk := range allChunks {
		if otherChunk.ID != chunk.ID && c.calculateRelationshipStrength(chunk, otherChunk) > 0.5 {
			related = append(related, otherChunk.ID)
		}
	}
	return related
}

func (c *ContextPreserver) buildDependencyGraph(
	chunk outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) map[string]interface{} {
	graph := make(map[string]interface{})
	graph["dependencies"] = len(chunk.Dependencies)
	graph["dependents"] = c.countDependents(chunk, allChunks)
	return graph
}

func (c *ContextPreserver) countDependents(
	chunk outbound.EnhancedCodeChunk,
	allChunks []outbound.EnhancedCodeChunk,
) int {
	count := 0
	for _, otherChunk := range allChunks {
		if otherChunk.ID != chunk.ID && c.hasSemanticDependency(otherChunk, chunk) {
			count++
		}
	}
	return count
}
