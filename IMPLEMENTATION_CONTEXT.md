# Implementation Context & Design Decisions

This document captures the critical context, design rationale, and lessons learned that cannot be inferred from reading code alone. It serves as "tribal knowledge" for future development sessions.

## Phase 4.3: Chunking Strategy Framework Implementation Context

### Step 1: Language-Agnostic Chunking Strategy - COMPLETE

#### Design Rationale & Key Decisions

**Interface Design (`CodeChunkingStrategy`)**
- **Why `EnhancedCodeChunk` vs extending existing `CodeChunk`**: The existing `CodeChunk` in `job_processor.go` is too simplistic (9 fields) for semantic chunking. `EnhancedCodeChunk` provides 20+ fields including quality metrics, relationships, and semantic context.
- **Why separate quality validation**: `ValidateChunkBoundaries()` method allows post-processing quality checks rather than inline validation, enabling better separation of concerns and testability.
- **Context preservation as separate method**: `PreserveContext()` is isolated to allow different strategies (minimal/moderate/maximal) to be applied independently of chunking algorithms.

**Type System Challenges Resolved**
- **`valueobject.Language` comparison issues**: Cannot use `==` or `switch` directly on Language struct. Must use `.Name()` for string access and `.Equal()` for comparisons. This was discovered during REFACTOR phase compilation fixes.
- **String conversion patterns**: Always use `language.Name()` instead of casting. Use `valueobject.NewLanguage(name)` constructor instead of string constants.

**Architecture Patterns**
- **Hexagonal architecture compliance**: Port interfaces in `internal/port/outbound/`, adapters in `internal/adapter/outbound/chunking/`, no direct dependencies between layers.
- **File size constraint (<500 lines)**: Achieved by extracting specialized chunkers (`function_chunker.go`, `class_chunker.go`, etc.) instead of monolithic implementation.
- **Observable patterns**: All chunkers integrate with structured logging (`slogger`) and OpenTelemetry metrics for production observability.

#### Implementation Strategy & Scope

**What "Language-Agnostic" Means**
- Common interface that works across Go, Python, JavaScript/TypeScript
- Language-specific optimizations handled in specialized chunker implementations
- Context preservation adapts to language semantics (Go interfaces vs Python decorators vs JS closures)

**Chunking Strategy Types Implemented**
1. **Function Strategy**: Groups related functions, preserves call relationships
2. **Class Strategy**: Maintains class boundaries, handles inheritance trees
3. **Size-based Strategy**: Intelligent splitting at semantic boundaries when functions/classes exceed size limits
4. **Hybrid Strategy**: Combines multiple approaches based on content analysis
5. **Adaptive Strategy**: Language-aware optimization (Go struct methods vs Python class methods)
6. **Hierarchy Strategy**: Preserves nested structure relationships

**Quality Metrics Framework**
- **Cohesion Score**: Measures how related code elements are within a chunk
- **Coupling Score**: Measures dependencies between chunks  
- **Complexity Score**: Based on cyclomatic complexity and nesting depth
- **Completeness Score**: Whether chunk boundaries preserve semantic integrity
- **Overall Quality**: Weighted combination of above metrics

#### Lessons Learned & Gotchas

**Tree-sitter Integration Challenges**
- **Language detection**: Use `forest.DetectLanguage()` for file extension mapping, but validate with content analysis
- **Parser creation**: Always check for parser availability before creating - not all languages in forest are complete
- **Memory management**: Parsers should be reused when possible, but ensure thread-safety in concurrent scenarios

**Testing Patterns That Work**
- **Mock-first approach**: Create comprehensive mocks in RED phase, replace with real implementations in GREEN phase
- **Integration test structure**: Test semantic traverser → chunking strategy integration with real parse trees, not synthetic data
- **Error scenario coverage**: Test malformed input, missing languages, oversized content explicitly

**Performance Considerations**
- **Large file handling**: Size-based chunking threshold of 10KB per chunk works well for most languages
- **Context preservation overhead**: Maximal preservation can 3x processing time - use moderate as default
- **Concurrent processing**: Chunkers are thread-safe, but share parser instances carefully

#### Integration Constraints & Patterns

**Semantic Traverser Integration**
- **Input contract**: Expects `[]SemanticCodeChunk` from semantic traverser with populated metadata
- **Language consistency**: All chunks in a batch must be same language - chunking strategy validates this
- **Import preservation**: Context preservation must include import statements from semantic traverser output

**Error Handling Patterns**
- **Validation errors**: Return structured errors with specific field violations, not generic messages
- **Language mismatch**: Explicit error when trying to chunk mixed-language semantic constructs
- **Size violations**: Warn but don't fail when chunks exceed recommended sizes - let quality metrics reflect this

#### Requirements Interpretation

**"Language-Agnostic" Implementation**
- **Core algorithms**: Same chunking logic works across languages
- **Language adaptations**: Specialized behavior for language-specific constructs (Go interfaces, Python decorators, JS closures)
- **Context preservation**: Language-aware import and dependency handling

**"Context Preservation" Definitions**
- **Minimal**: Essential imports and immediate function/variable dependencies
- **Moderate**: Imports, type definitions, directly called functions, documented relationships  
- **Maximal**: Full semantic context including comments, related functions, inheritance chains

**Quality Thresholds & Acceptance Criteria**
- **Minimum chunk size**: 100 bytes (prevent over-fragmentation)
- **Maximum chunk size**: 50KB (prevent oversized chunks)
- **Quality score threshold**: 0.7+ for production use (configurable)
- **Context preservation completeness**: 95%+ for critical dependencies

---

## Next Phase Context (Phase 4.3 Step 2)

### Step 2: Function-Level Chunking with Context Preservation - PENDING

#### Specific Requirements Interpretation

**"Function-Level Chunking" Scope**
- **Primary target**: Individual functions/methods as discrete chunks
- **Context inclusion**: Related helper functions, called functions within same file
- **Documentation**: Include docstrings/comments immediately preceding function
- **Language specifics**: 
  - Go: Include receiver methods with their struct definitions
  - Python: Include decorators and type hints
  - JavaScript: Include arrow functions and closures in same scope

**"Context Preservation" for Functions**
- **Import dependencies**: All imports used by function
- **Type definitions**: Custom types/interfaces used in function signature or body
- **Called functions**: Functions called within the target function (if in same repository)
- **Constants/variables**: Module-level constants/variables referenced
- **Error types**: Custom error types thrown/returned

#### Success Criteria & Acceptance
- Function chunks are semantically complete (can understand function purpose without external context)
- Related functions are grouped intelligently (not randomly split)
- Import statements are preserved accurately
- Performance: <100ms per function for typical codebases
- Quality: 0.8+ cohesion score for function-level chunks

#### Known Challenges to Address
- **Recursive functions**: How to handle mutual recursion without creating oversized chunks
- **Higher-order functions**: Whether to include passed function definitions or just signatures
- **Language-specific patterns**: Go method receivers, Python class methods vs static methods, JS prototype vs class methods

---

## Development Guidelines & Standards

### TDD Methodology with Specialized Agents
- **RED Phase**: Use `@agent-red-phase-tester` - focuses on comprehensive test coverage defining exact behavior
- **GREEN Phase**: Use `@agent-green-phase-implementer` - minimal implementation to make tests pass
- **REFACTOR Phase**: Use `@agent-tdd-refactor-specialist` - production optimization while keeping tests green

### Code Quality Standards
- **File Size**: <500 lines per file for maintainability
- **Architecture**: Strict hexagonal architecture - ports define interfaces, adapters provide implementations
- **Logging**: Use `slogger` package for structured logging with correlation IDs
- **Metrics**: OpenTelemetry integration for production observability
- **Error Handling**: Structured errors with context, not generic error strings
- **Documentation**: Comprehensive comments for public interfaces, minimal for implementation details

### Integration Patterns
- **Semantic Traverser**: Always validate language consistency in input
- **Tree-sitter**: Reuse parsers when possible, handle parser creation failures gracefully
- **Language Detection**: Use file extension first, content analysis for validation
- **Context Preservation**: Separate concern from chunking algorithm - apply as post-processing step

---

## Performance Benchmarks & Targets

### Current Performance (Phase 4.3 Step 1)
- **Small files (<1KB)**: <10ms per file
- **Medium files (1-10KB)**: <50ms per file  
- **Large files (10-100KB)**: <200ms per file
- **Concurrent processing**: 10+ files simultaneously without degradation

### Target Performance (Phase 4.3 Step 2)
- **Function-level chunking**: <5ms per function
- **Context preservation**: <20ms per function (moderate level)
- **Quality analysis**: <10ms per chunk
- **Memory usage**: <100MB for typical repository processing

---

## Technology Stack Decisions & Rationale

### Tree-sitter Integration
- **Library Choice**: `github.com/alexaandru/go-tree-sitter-bare` - lightweight, zero dependencies
- **Language Parsers**: `go-sitter-forest` - 490+ language parsers, actively maintained
- **Why not alternatives**: Considered direct tree-sitter bindings but bare version provides cleaner Go integration

### Architecture Choices
- **Hexagonal Architecture**: Clean separation enables testing, language-specific adapters, future extension
- **Observable Pattern**: Production readiness requires structured logging and metrics from day one
- **Modular Chunkers**: Allows language-specific optimizations without breaking common interface

### Quality & Testing
- **TDD with Specialized Agents**: Ensures comprehensive coverage and prevents regression
- **Integration Testing**: Real tree-sitter parsing instead of mocks for confidence in production behavior
- **Performance Testing**: Early performance characterization prevents scalability issues later

---

## Common Pitfalls & Solutions

### Type System Issues
**Problem**: `valueobject.Language` comparison and conversion errors  
**Solution**: Always use `.Name()` and `.Equal()` methods, never direct comparison

### Tree-sitter Integration
**Problem**: Parser creation failures for unsupported languages  
**Solution**: Always check parser availability, graceful degradation for unknown languages

### Memory Management
**Problem**: Parser instances consuming excessive memory in concurrent scenarios  
**Solution**: Parser pooling with configurable limits, explicit cleanup in error cases

### Context Preservation
**Problem**: Over-inclusion of context leading to massive chunks  
**Solution**: Configurable preservation levels, quality metrics to detect over-inclusion

---

## Future Considerations

### Extensibility Points
- **New Languages**: Add parser in `go-sitter-forest`, create language-specific chunker
- **New Strategies**: Implement `ChunkingStrategyType` interface, add to factory
- **Quality Metrics**: Extend `ChunkQualityMetrics` with new scoring algorithms

### Performance Optimization Opportunities
- **Caching**: Parse tree caching for repeated analysis
- **Streaming**: Process large repositories without loading everything into memory
- **Parallelization**: Language-specific chunking can be parallelized safely

### Integration Expansion
- **IDE Integration**: Chunking strategy could power code navigation features
- **Documentation**: Chunks could be used for automated documentation generation
- **Code Review**: Quality metrics could highlight problematic code patterns