package chunking

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"
	"math"
	"strings"
)

// QualityAnalyzer implements quality metrics calculation for code chunks,
// providing comprehensive analysis of cohesion, coupling, complexity, and maintainability.
type QualityAnalyzer struct{}

// NewQualityAnalyzer creates a new quality analyzer.
func NewQualityAnalyzer() *QualityAnalyzer {
	return &QualityAnalyzer{}
}

// CalculateQualityMetrics calculates comprehensive quality metrics for a code chunk.
func (q *QualityAnalyzer) CalculateQualityMetrics(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
) (*outbound.ChunkQualityMetrics, error) {
	slogger.Debug(ctx, "Calculating quality metrics for chunk", slogger.Fields{
		"chunk_id":   chunk.ID,
		"chunk_size": chunk.Size.Bytes,
		"constructs": len(chunk.SemanticConstructs),
	})

	// Calculate individual metrics
	cohesionScore := q.calculateCohesionScore(ctx, chunk)
	couplingScore := q.calculateCouplingScore(ctx, chunk)
	complexityScore := q.calculateComplexityScore(ctx, chunk)
	completenessScore := q.calculateCompletenessScore(ctx, chunk)
	readabilityScore := q.calculateReadabilityScore(ctx, chunk)
	maintainabilityScore := q.calculateMaintainabilityScore(ctx, chunk)

	// Calculate overall quality as a weighted average
	overallQuality := q.calculateOverallQuality(
		cohesionScore,
		couplingScore,
		complexityScore,
		completenessScore,
		readabilityScore,
		maintainabilityScore,
	)

	metrics := &outbound.ChunkQualityMetrics{
		CohesionScore:        cohesionScore,
		CouplingScore:        couplingScore,
		ComplexityScore:      complexityScore,
		CompletenessScore:    completenessScore,
		ReadabilityScore:     readabilityScore,
		MaintainabilityScore: maintainabilityScore,
		OverallQuality:       overallQuality,
	}

	slogger.Debug(ctx, "Quality metrics calculated", slogger.Fields{
		"chunk_id":        chunk.ID,
		"overall_quality": overallQuality,
		"cohesion":        cohesionScore,
		"coupling":        couplingScore,
		"complexity":      complexityScore,
	})

	return metrics, nil
}

// calculateCohesionScore measures how closely related the elements within a chunk are.
// Higher cohesion indicates better design with related functionality grouped together.
func (q *QualityAnalyzer) calculateCohesionScore(ctx context.Context, chunk *outbound.EnhancedCodeChunk) float64 {
	if len(chunk.SemanticConstructs) <= 1 {
		return 1.0 // Single constructs are perfectly cohesive
	}

	var totalRelatedPairs float64
	var totalPairs float64

	// Analyze pairwise relationships between semantic constructs
	for i := 0; i < len(chunk.SemanticConstructs); i++ {
		for j := i + 1; j < len(chunk.SemanticConstructs); j++ {
			construct1 := chunk.SemanticConstructs[i]
			construct2 := chunk.SemanticConstructs[j]

			totalPairs++

			// Check various relationship indicators
			relationshipScore := q.calculateConstructRelationship(construct1, construct2)
			if relationshipScore > 0.3 { // Threshold for considering constructs related
				totalRelatedPairs += relationshipScore
			}
		}
	}

	if totalPairs == 0 {
		return 1.0
	}

	cohesion := totalRelatedPairs / totalPairs
	return math.Min(cohesion, 1.0)
}

// calculateCouplingScore measures dependencies between this chunk and external elements.
// Lower coupling indicates better design with fewer external dependencies.
func (q *QualityAnalyzer) calculateCouplingScore(ctx context.Context, chunk *outbound.EnhancedCodeChunk) float64 {
	externalDependencies := 0

	// Count external dependencies (those not resolved within the chunk)
	for _, dep := range chunk.Dependencies {
		if !dep.IsResolved {
			externalDependencies++
		}
	}

	// Calculate coupling based on external dependencies vs chunk size
	chunkComplexity := float64(len(chunk.SemanticConstructs))
	if chunkComplexity == 0 {
		chunkComplexity = 1.0
	}

	// Normalize coupling score (lower is better, so we invert it)
	coupling := float64(externalDependencies) / chunkComplexity

	// Convert to 0-1 scale where 1 is good (low coupling)
	couplingScore := math.Max(0.0, 1.0-coupling)

	return couplingScore
}

// calculateComplexityScore measures the cognitive complexity of the chunk.
// Lower complexity indicates code that is easier to understand and maintain.
func (q *QualityAnalyzer) calculateComplexityScore(ctx context.Context, chunk *outbound.EnhancedCodeChunk) float64 {
	totalComplexity := 0.0

	// Analyze complexity of each semantic construct
	for _, construct := range chunk.SemanticConstructs {
		constructComplexity := q.calculateConstructComplexity(construct)
		totalComplexity += constructComplexity
	}

	// Add complexity based on dependencies
	dependencyComplexity := float64(len(chunk.Dependencies)) * 0.1
	totalComplexity += dependencyComplexity

	// Add complexity based on chunk size
	sizeComplexity := math.Log10(float64(chunk.Size.Bytes)/1000.0 + 1) // Logarithmic scaling
	totalComplexity += sizeComplexity

	// Normalize to 0-1 scale (lower complexity is better, so we invert)
	maxExpectedComplexity := float64(len(chunk.SemanticConstructs)) * 5.0 // Assuming max 5 complexity per construct
	normalizedComplexity := totalComplexity / maxExpectedComplexity

	complexityScore := math.Max(0.0, 1.0-math.Min(normalizedComplexity, 1.0))

	return complexityScore
}

// calculateCompletenessScore measures how complete and self-contained the chunk is.
// Higher completeness indicates the chunk contains all necessary context.
func (q *QualityAnalyzer) calculateCompletenessScore(ctx context.Context, chunk *outbound.EnhancedCodeChunk) float64 {
	completenessFactors := 0.0
	maxFactors := 6.0 // Number of completeness factors we check

	// Factor 1: Has meaningful content
	if len(strings.TrimSpace(chunk.Content)) > 0 {
		completenessFactors += 1.0
	}

	// Factor 2: Has semantic constructs
	if len(chunk.SemanticConstructs) > 0 {
		completenessFactors += 1.0
	}

	// Factor 3: Has context information (if preservation strategy is not minimal)
	if chunk.ContextStrategy != outbound.PreserveMinimal {
		contextElements := len(chunk.PreservedContext.ImportStatements) +
			len(chunk.PreservedContext.UsedVariables) +
			len(chunk.PreservedContext.TypeDefinitions)
		if contextElements > 0 {
			completenessFactors += 1.0
		}
	} else {
		completenessFactors += 1.0 // Minimal context is acceptable for minimal strategy
	}

	// Factor 4: Dependencies are identified
	if len(chunk.Dependencies) > 0 || q.hasNoDependencies(chunk) {
		completenessFactors += 1.0
	}

	// Factor 5: Has appropriate chunk type classification
	if chunk.ChunkType != outbound.ChunkMixed || len(chunk.SemanticConstructs) <= 1 {
		completenessFactors += 1.0
	}

	// Factor 6: Size is within reasonable bounds
	if chunk.Size.Bytes >= 50 && chunk.Size.Bytes <= 5000 { // Reasonable size range
		completenessFactors += 1.0
	}

	return completenessFactors / maxFactors
}

// calculateReadabilityScore measures how readable and well-structured the code is.
func (q *QualityAnalyzer) calculateReadabilityScore(ctx context.Context, chunk *outbound.EnhancedCodeChunk) float64 {
	readabilityFactors := 0.0
	maxFactors := 5.0

	content := chunk.Content
	lines := strings.Split(content, "\n")

	// Factor 1: Reasonable line length
	longLines := 0
	for _, line := range lines {
		if len(line) > 120 { // Typical max line length
			longLines++
		}
	}
	if len(lines) == 0 || float64(longLines)/float64(len(lines)) < 0.1 { // Less than 10% long lines
		readabilityFactors += 1.0
	}

	// Factor 2: Appropriate indentation (basic check)
	if q.hasConsistentIndentation(lines) {
		readabilityFactors += 1.0
	}

	// Factor 3: Has comments or documentation
	if q.hasDocumentation(chunk) {
		readabilityFactors += 1.0
	}

	// Factor 4: Reasonable construct naming
	if q.hasReasonableNaming(chunk) {
		readabilityFactors += 1.0
	}

	// Factor 5: Balanced construct distribution
	if q.hasBalancedConstructs(chunk) {
		readabilityFactors += 1.0
	}

	return readabilityFactors / maxFactors
}

// calculateMaintainabilityScore measures how maintainable the chunk is.
func (q *QualityAnalyzer) calculateMaintainabilityScore(
	ctx context.Context,
	chunk *outbound.EnhancedCodeChunk,
) float64 {
	// Maintainability is derived from other metrics
	cohesion := chunk.CohesionScore
	if cohesion == 0 { // If not set, calculate it
		cohesion = q.calculateCohesionScore(ctx, chunk)
	}

	coupling := chunk.QualityMetrics.CouplingScore
	if coupling == 0 { // If not set, calculate it
		coupling = q.calculateCouplingScore(ctx, chunk)
	}

	complexity := chunk.ComplexityScore
	if complexity == 0 { // If not set, calculate it
		complexity = q.calculateComplexityScore(ctx, chunk)
	}

	completeness := q.calculateCompletenessScore(ctx, chunk)
	readability := q.calculateReadabilityScore(ctx, chunk)

	// Weighted average with emphasis on factors that most affect maintainability
	maintainability := (cohesion*0.25 + coupling*0.25 + (1.0-complexity+1.0)*0.25 +
		completeness*0.15 + readability*0.1)

	return math.Min(maintainability, 1.0)
}

// calculateOverallQuality computes the overall quality score from individual metrics.
func (q *QualityAnalyzer) calculateOverallQuality(
	cohesion, coupling, complexity, completeness, readability, maintainability float64,
) float64 {
	// Weighted average with different priorities for different metrics
	weights := map[string]float64{
		"cohesion":        0.20,
		"coupling":        0.20,
		"complexity":      0.15, // Lower complexity is better, but we've already inverted it
		"completeness":    0.15,
		"readability":     0.15,
		"maintainability": 0.15,
	}

	overallQuality := cohesion*weights["cohesion"] +
		coupling*weights["coupling"] +
		complexity*weights["complexity"] +
		completeness*weights["completeness"] +
		readability*weights["readability"] +
		maintainability*weights["maintainability"]

	return math.Min(overallQuality, 1.0)
}

// Helper methods for quality analysis

func (q *QualityAnalyzer) calculateConstructRelationship(
	construct1, construct2 outbound.SemanticCodeChunk,
) float64 {
	relationshipScore := 0.0

	// Same type constructs are related
	if construct1.Type == construct2.Type {
		relationshipScore += 0.3
	}

	// Shared function calls indicate relationship
	sharedCalls := q.countSharedFunctionCalls(construct1, construct2)
	relationshipScore += float64(sharedCalls) * 0.2

	// Shared types indicate relationship
	sharedTypes := q.countSharedTypes(construct1, construct2)
	relationshipScore += float64(sharedTypes) * 0.15

	// Proximity in code indicates relationship
	if q.areConstructsClose(construct1, construct2) {
		relationshipScore += 0.25
	}

	// Similar naming indicates relationship
	if q.haveSimilarNames(construct1.Name, construct2.Name) {
		relationshipScore += 0.1
	}

	return math.Min(relationshipScore, 1.0)
}

func (q *QualityAnalyzer) calculateConstructComplexity(construct outbound.SemanticCodeChunk) float64 {
	complexity := 0.0

	// Base complexity by construct type
	switch construct.Type {
	case outbound.ConstructFunction, outbound.ConstructMethod:
		complexity += 1.0
		// Add complexity for parameters
		complexity += float64(len(construct.Parameters)) * 0.1
	case outbound.ConstructClass, outbound.ConstructStruct:
		complexity += 2.0
		// Add complexity for child constructs
		complexity += float64(len(construct.ChildChunks)) * 0.2
	case outbound.ConstructInterface:
		complexity += 1.5
	case outbound.ConstructVariable, outbound.ConstructConstant:
		complexity += 0.5
	default:
		complexity += 0.8
	}

	// Add complexity for function calls
	complexity += float64(len(construct.CalledFunctions)) * 0.05

	// Add complexity for type usage
	complexity += float64(len(construct.UsedTypes)) * 0.05

	// Add complexity for annotations/decorators
	complexity += float64(len(construct.Annotations)) * 0.1

	return complexity
}

func (q *QualityAnalyzer) hasNoDependencies(chunk *outbound.EnhancedCodeChunk) bool {
	// Check if chunk genuinely has no dependencies (e.g., constants, simple utilities)
	for _, construct := range chunk.SemanticConstructs {
		if construct.Type == outbound.ConstructConstant ||
			construct.Type == outbound.ConstructVariable ||
			(construct.Type == outbound.ConstructFunction && len(construct.CalledFunctions) == 0) {
			return true
		}
	}
	return false
}

func (q *QualityAnalyzer) hasConsistentIndentation(lines []string) bool {
	// Simple heuristic: check if indentation uses consistent spacing
	indentationPattern := ""
	indentationSet := false

	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if len(trimmed) == 0 || trimmed == line { // Empty line or no indentation
			continue
		}

		indent := line[:len(line)-len(trimmed)]
		if !indentationSet {
			if len(indent) > 0 {
				indentationPattern = indent
				indentationSet = true
			}
		} else {
			// Check if indentation is consistent (multiple of the base pattern)
			if len(indent) > 0 && len(indent)%len(indentationPattern) != 0 {
				return false
			}
		}
	}

	return true
}

func (q *QualityAnalyzer) hasDocumentation(chunk *outbound.EnhancedCodeChunk) bool {
	// Check for documentation in various forms
	for _, construct := range chunk.SemanticConstructs {
		if construct.Documentation != "" {
			return true
		}
	}

	// Check for comments in content
	content := strings.ToLower(chunk.Content)
	commentIndicators := []string{"//", "/*", "#", "\"\"\"", "'''"}
	for _, indicator := range commentIndicators {
		if strings.Contains(content, indicator) {
			return true
		}
	}

	return len(chunk.PreservedContext.DocumentationLinks) > 0
}

func (q *QualityAnalyzer) hasReasonableNaming(chunk *outbound.EnhancedCodeChunk) bool {
	unreasonableNames := 0
	totalNames := 0

	for _, construct := range chunk.SemanticConstructs {
		totalNames++
		if q.isUnreasonableName(construct.Name) {
			unreasonableNames++
		}
	}

	if totalNames == 0 {
		return true
	}

	// Allow up to 20% unreasonable names
	return float64(unreasonableNames)/float64(totalNames) <= 0.2
}

func (q *QualityAnalyzer) hasBalancedConstructs(chunk *outbound.EnhancedCodeChunk) bool {
	// Check if the chunk doesn't have too many constructs of one type
	typeCounts := make(map[outbound.SemanticConstructType]int)
	for _, construct := range chunk.SemanticConstructs {
		typeCounts[construct.Type]++
	}

	if len(typeCounts) <= 1 {
		return true // Single type or empty is balanced
	}

	// Check if any single type dominates (>80% of constructs)
	maxCount := 0
	totalCount := len(chunk.SemanticConstructs)

	for _, count := range typeCounts {
		if count > maxCount {
			maxCount = count
		}
	}

	return float64(maxCount)/float64(totalCount) <= 0.8
}

func (q *QualityAnalyzer) countSharedFunctionCalls(
	construct1, construct2 outbound.SemanticCodeChunk,
) int {
	calls1 := make(map[string]bool)
	for _, call := range construct1.CalledFunctions {
		calls1[call.Name] = true
	}

	shared := 0
	for _, call := range construct2.CalledFunctions {
		if calls1[call.Name] {
			shared++
		}
	}

	return shared
}

func (q *QualityAnalyzer) countSharedTypes(
	construct1, construct2 outbound.SemanticCodeChunk,
) int {
	types1 := make(map[string]bool)
	for _, typeRef := range construct1.UsedTypes {
		types1[typeRef.Name] = true
	}

	shared := 0
	for _, typeRef := range construct2.UsedTypes {
		if types1[typeRef.Name] {
			shared++
		}
	}

	return shared
}

func (q *QualityAnalyzer) areConstructsClose(
	construct1, construct2 outbound.SemanticCodeChunk,
) bool {
	// Check if constructs are within reasonable proximity
	gap := uint32(0)
	if construct2.StartByte > construct1.EndByte {
		gap = construct2.StartByte - construct1.EndByte
	} else if construct1.StartByte > construct2.EndByte {
		gap = construct1.StartByte - construct2.EndByte
	}

	return gap < 200 // 200 bytes threshold for proximity
}

func (q *QualityAnalyzer) haveSimilarNames(name1, name2 string) bool {
	// Simple name similarity check
	if len(name1) == 0 || len(name2) == 0 {
		return false
	}

	// Check for common prefixes or suffixes
	minLen := len(name1)
	if len(name2) < minLen {
		minLen = len(name2)
	}

	if minLen < 3 {
		return false
	}

	// Check prefix similarity
	prefixLen := minLen / 2
	if prefixLen > 0 && strings.ToLower(name1[:prefixLen]) == strings.ToLower(name2[:prefixLen]) {
		return true
	}

	// Check suffix similarity
	if prefixLen > 0 &&
		strings.ToLower(name1[len(name1)-prefixLen:]) == strings.ToLower(name2[len(name2)-prefixLen:]) {
		return true
	}

	return false
}

func (q *QualityAnalyzer) isUnreasonableName(name string) bool {
	// Check for unreasonable naming patterns
	if len(name) == 0 {
		return true
	}

	// Single character names (except for common ones like i, j, k in loops)
	if len(name) == 1 {
		commonSingleChar := []string{"i", "j", "k", "x", "y", "z"}
		for _, common := range commonSingleChar {
			if name == common {
				return false
			}
		}
		return true
	}

	// Very long names
	if len(name) > 50 {
		return true
	}

	// Names with only numbers
	hasLetter := false
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') {
			hasLetter = true
			break
		}
	}

	return !hasLetter
}
