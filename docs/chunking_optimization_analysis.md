# Code Chunking Optimization Analysis

## Executive Summary

This document analyzes the current code chunking implementation in the codechunking system and provides recommendations for optimizing it for retrieval and querying performance.

## Current Implementation Analysis

### Strengths of Current Implementation

1. **Semantic Integrity**
   - Uses tree-sitter for language-aware parsing, preserving semantic boundaries
   - Functions remain complete with signatures
   - Classes/structs stay intact with methods
   - Import statements and dependencies are preserved
   - Context maintained through `PreservedContext` struct

2. **Rich Metadata for Enhanced Retrieval**
   - **Qualified Names** (e.g., `package.Class.method`) enable precise hierarchical searches
   - **Signatures** allow searching by function parameters and return types
   - **Visibility** (public/private) helps filter by API boundaries
   - **Dependencies** enable relationship-based queries
   - **Chunk Types** (function, class, interface) provide categorical filtering

3. **Multiple Chunking Strategies**
   - **Function-level**: Optimal for code reuse patterns
   - **Class-level**: Good for understanding object-oriented designs
   - **Size-based**: Ensures manageable chunk sizes for embedding models
   - **Hybrid/Adaptive**: Selects optimal strategy based on content characteristics

4. **Production-Ready Features**
   - OpenTelemetry metrics for monitoring chunk quality
   - Quality metrics (cohesion, complexity scores)
   - Circular dependency detection
   - Batch processing for large files

5. **Language Support**
   - Go parser with comprehensive function, struct, and interface extraction
   - Python parser including specialized chunking for large files
   - JavaScript/TypeScript parser for modern web development
   - 300+ language support through go-sitter-forest integration

6. **Scalability Features**
   - Pre-parsing chunking for large files (500KB threshold)
   - Streaming processing for memory efficiency
   - Partitioned embeddings table for performance
   - Vector similarity search with pgvector support

### Areas for Improvement

1. **Chunk Size Optimization**
   ```go
   // Current default: 4000 bytes max, 200 min
   MaxChunkSize:         4000,
   MinChunkSize:         200,
   ```
   This might be suboptimal for modern embedding models which typically handle 8192 tokens effectively.

2. **Overlap Strategy Absence**
   ```go
   OverlapSize: 0, // No overlap between chunks
   ```
   For retrieval, having semantic overlap between adjacent chunks can improve context preservation and reduce fragmentation of related concepts.

3. **Limited Hierarchical Context**
   While chunks have parent/child relationships, the retrieval system doesn't leverage the full hierarchical structure that could enable more sophisticated queries.

4. **Embedding Specificity**
   The system generates one embedding per chunk without considering:
   - Different embedding strategies for different chunk types
   - Summary embeddings for large chunks
   - Specialized embeddings for signatures vs. implementations
   - The embedding service supports `TaskTypeCodeRetrievalQuery` but chunking doesn't leverage specialized task types

5. **Retrieval Pattern Limitations**
   The search system supports rich filtering but doesn't optimize chunking for common patterns:
   - No chunking strategy for "find implementations of interface X"
   - Limited support for type-hierarchy aware chunking
   - No consideration for cross-file references when creating chunks
   - Search supports `Types`, `EntityName`, `Visibility` filters but chunks aren't created with these queries in mind

6. **Vector Storage Constraints**
   - Fixed 768 dimensions for all embeddings regardless of content type
   - No adaptive dimensionality based on chunk complexity
   - Partitioned tables help with performance but aren't leveraged for optimal chunk distribution

## Recommendations for Optimization

### 1. Implement Adaptive Chunk Sizing

Instead of fixed sizes, use language-aware and content-aware sizing:

```go
// Recommended optimization
func (c *ChunkingStrategyAdapter) GetOptimalChunkSize(
    ctx context.Context,
    language valueobject.Language,
    contentType SemanticConstructType, // NEW: Consider chunk type
    contentSize int,
) (ChunkSizeRecommendation, error) {

    // Base sizes by embedding model token limits (8192 tokens ~ 32KB)
    var optimalSize int

    switch contentType {
    case ConstructFunction:
        // Keep functions whole but cap at 6KB for complex functions
        optimalSize = min(contentSize, 6000)
    case ConstructClass:
        // Allow larger for classes with multiple methods
        optimalSize = min(contentSize, 10000)
    default:
        // Use 30% of content or 8KB, whichever is smaller
        optimalSize = min(contentSize/3, 8192)
    }

    // Add strategic overlap for context preservation
    return ChunkSizeRecommendation{
        OptimalSize: optimalSize,
        OverlapSize: 200, // NEW: Add overlap for better retrieval
    }
}
```

### 2. Add Semantic Overlap Strategy

Implement overlap at logical boundaries rather than arbitrary text:

```go
type OverlapStrategy struct {
    PreserveFunctionCalls bool
    PreserveImports       bool
    PreserveDocstrings    bool
    OverlapLines         int
}

// Create overlapping chunks that include:
// - Import statements of dependent functions
// - Function signatures that are called within the chunk
// - Related class methods that are frequently used together
```

### 3. Implement Multi-Level Embedding Strategy

Generate multiple embeddings per chunk for different retrieval patterns:

```go
type MultiLevelEmbedding struct {
    FullEmbedding     []float64 // Entire chunk
    SignatureEmbedding []float64 // Just signatures and types
    SummaryEmbedding   []float64 // Generated summary for large chunks
}

// Benefit: Enables different search strategies:
// - Full embedding for semantic similarity
// - Signature embedding for API matching
// - Summary embedding for high-level concepts search
```

### 4. Enhance Retrieval with Graph-Based Relationships

Leverage the dependency information for graph-based retrieval:

```go
// Extend ChunkDependency with strength scoring
type ChunkDependency struct {
    Type         DependencyType `json:"type"`
    Strength     float64       `json:"strength"` // NEW: Weight relationships
    CallFrequency int          `json:"call_frequency"` // NEW: Usage patterns
}

// Enable retrieval queries like:
// "Find functions that use this struct AND are called frequently"
```

### 5. Implement Query-Aware Chunking

Store chunks in ways that optimize for common query patterns:

```sql
-- Add composite indexes for common query patterns
CREATE INDEX idx_chunks_entity_lookup ON code_chunks
(qualified_name, chunk_type, visibility)
WHERE deleted_at IS NULL;

-- Partition by repository for multi-tenant efficiency
CREATE TABLE code_chunks_2024_01 PARTITION OF code_chunks
FOR VALUES FROM ('2024-01-01') TO ('2024-02-01');

-- Materialized view for frequently accessed combinations
CREATE MATERIALIZED VIEW chunk_signatures AS
SELECT id, qualified_name, signature, chunk_type, language
FROM code_chunks
WHERE chunk_type IN ('function', 'method');
```

### 6. Add Dynamic Quality Metrics for Retrieval

Enhance quality scoring based on retrieval effectiveness:

```go
type RetrievalQualityMetrics struct {
    SearchHitRate      float64 `json:"search_hit_rate"`       // How often this chunk appears in results
    AverageSimilarity  float64 `json:"average_similarity"`    // Similarity scores when retrieved
    ClickThroughRate   float64 `json:"click_through_rate"`    // User interaction metrics
    UsefulnessScore    float64 `json:"usefulness_score"`      // Combined utility metric
}

// Use these metrics to:
// 1. Prioritize high-quality chunks in search results
// 2. Identify chunks that need re-chunking
// 3. Optimize chunking strategies over time
```

### 7. Implement Contextual Window Expansion

For retrieval, provide expanding context windows:

```go
type ContextualChunk struct {
    CoreChunk       EnhancedCodeChunk    `json:"core_chunk"`
    ImmediateContext []EnhancedCodeChunk `json:"immediate_context"` // ±1 chunk
    ExtendedContext  []EnhancedCodeChunk `json:"extended_context"`  // Related dependencies

    // Benefit: Return just the core chunk initially,
    // then expand context as needed
}
```

## Additional Considerations Based on Code Review

### Search-Driven Chunking Opportunities
The search service reveals specific patterns that should influence chunking:
- **Similarity Threshold Default**: 0.7 suggests current embeddings may need better precision
- **Type Filtering**: Search explicitly filters by `Types` (function, class, method) - chunks should be optimized for these categories
- **Entity-Based Search**: `EntityName` filtering suggests chunks should preserve clear entity boundaries
- **Visibility Filtering**: Public/private filtering indicates importance of maintaining access modifiers

### Embedding Service Integration Gaps
- The embedding service supports specialized task types (`TaskTypeCodeRetrievalQuery`) but chunking doesn't leverage them
- No integration between chunk quality metrics and embedding generation parameters
- Vector dimensions are fixed at 768 regardless of content complexity

### Performance Monitoring Recommendations
Current metrics track chunk creation but not retrieval effectiveness:
- Add search hit rate tracking per chunk type
- Monitor similarity score distributions by chunking strategy
- Track query latency vs. chunk size patterns
- Implement A/B testing for different chunking approaches

## Implementation Priority

### Immediate (Quick Wins)
1. Increase `MaxChunkSize` to 8000-10000 bytes (better utilizes embedding model capacity)
2. Add `OverlapSize` of 200 bytes to configuration
3. Leverage `TaskTypeCodeRetrievalQuery` for code-specific embeddings

### Short-term (Next Sprint)
1. Implement multi-level embeddings (full + signature)
2. Add basic semantic overlap for adjacent chunks
3. Optimize chunk boundaries based on common search filters (Types, EntityName)

### Medium-term (Next Quarter)
1. Implement adaptive sizing based on chunk type and complexity
2. Add graph-based relationship tracking for cross-file references
3. Create retrieval-pattern aware chunking strategies
4. Implement per-chunk-type embedding optimization

### Long-term (Next Half-Year)
1. Implement retrieval-quality feedback loop with A/B testing
2. Add contextual window expansion for search results
3. Develop dynamic dimensionality based on content complexity
4. Create ML-model for optimal chunking strategy selection

## Conclusion

The current implementation demonstrates a sophisticated understanding of code chunking challenges. Its semantic awareness through tree-sitter parsing puts it ahead of typical text-splitting approaches. The key is evolving from static chunking to retrieval-aware, adaptive chunking that learns from usage patterns.

The foundation is solid - these optimizations will make it genuinely optimal for modern code retrieval and querying scenarios.

## Appendix: Key Code References

### Current Configuration
- Default chunking configuration: `internal/port/outbound/chunking_strategy.go:76-92`
- Function chunker implementation: `internal/adapter/outbound/chunking/function_chunker.go`
- Chunking strategy adapter: `internal/adapter/outbound/chunking/chunking_strategy_adapter.go`

### Search Implementation
- Search request DTO with filtering options: `internal/application/dto/search.go:78-92`
- Search service logic: `internal/application/service/search_service.go`
- Chunk repository interface: `internal/application/service/search_service.go:25-45`

### Vector Storage
- Vector storage interface: `internal/port/outbound/vector_storage_repository.go`
- PostgreSQL implementation: `internal/adapter/outbound/repository/vector_storage_repository.go`
- Vector dimensions validation: line 67 (fixed at 768)

### Embedding Service
- Embedding service interface: `internal/port/outbound/embedding_service.go`
- Supported task types: lines 63-70 (including `TaskTypeCodeRetrievalQuery`)
- Code chunk embedding structure: lines 96-108

### Database Schema
- Code chunks table query: `internal/adapter/outbound/repository/chunk_repository.go:85-105`
- Embeddings partitioning: constants at lines 18-21 in vector_storage_repository.go

## Test Coverage Considerations
The system has comprehensive tests for chunking scenarios:
- Function chunking tests: `internal/adapter/outbound/chunking/*_test.go`
- Search service tests: `internal/application/service/search_service_test.go`
- Vector storage tests: `internal/adapter/outbound/repository/*_test.go`

When implementing optimizations, ensure:
1. Existing test coverage is maintained
2. New retrieval patterns have corresponding tests
3. Performance benchmarks are added for chunk size variations
4. A/B testing framework is considered for strategy comparison

---

**Document Version**: 1.1
**Updated**: 2025-11-09 (Added code references and search-driven insights)
**Author**: Claude Code Analysis