# JavaScript Class Extraction - RED PHASE Test Specifications

## Overview

This document describes the comprehensive failing tests created for JavaScript class extraction in the RED phase of TDD. These tests define the expected behavior that needs to be implemented to make the tests pass.

## Test File Location

`/home/anthony/GolandProjects/codechunking/internal/adapter/outbound/treesitter/parsers/javascript/javascript_class_extraction_comprehensive_red_phase_test.go`

## Current Failure Analysis

### Root Cause Issues Identified

1. **Using StubParser Instead of Real Tree-sitter**
   - Log shows: "No registered parser found, using enhanced stub parser"
   - This means the system isn't using the real tree-sitter JavaScript parser
   - The `createRealParseTreeFromSource` function creates real parse trees, but the extraction logic falls back to stubs

2. **Nil Pointer Dereference in ParseTree.Language()**
   - Panic occurs at: `codechunking/internal/domain/valueobject.(*ParseTree).Language(...)`
   - Location: `/home/anthony/GolandProjects/codechunking/internal/domain/valueobject/parse_tree.go:159`
   - This causes the error handling test to fail with a panic instead of graceful error handling

3. **Zero Class Detection**
   - All tests consistently return 0 classes instead of expected counts
   - This suggests the class extraction logic isn't properly integrated with real tree-sitter AST nodes

## Test Specifications

### 1. TestJavaScriptClassExtraction_TreeSitterIntegration_RedPhase

**Purpose**: Verify tree-sitter AST node detection and integration
**Expected Behavior**:
- `parseTree.GetNodesByType("class_declaration")` should find class declaration nodes
- `parseTree.GetNodesByType("class")` should find class expression nodes  
- `parseTree.GetNodesByType("variable_declarator")` should find variable declarations
- `parseTree.Language()` should not cause nil pointer panic
- Language name should be "JavaScript"

**Current Status**: FAILS - Using stub parser, no AST nodes detected

### 2. TestJavaScriptClassExtraction_BasicClassDeclarations_RedPhase

**Purpose**: Test standard ES6 class declaration extraction
**Expected Behavior**:
- Should find 3 classes: Animal, Dog, Cat
- Animal class should have no inheritance dependencies
- Dog class should have Animal as inheritance dependency
- Cat class should have `has_private_members: true` metadata
- All classes should have `ConstructClass` type and correct qualified names

**Current Status**: FAILS - Returns 0 classes instead of 3

### 3. TestJavaScriptClassExtraction_ClassExpressions_RedPhase

**Purpose**: Test class expression detection (anonymous and named)
**Expected Behavior**:
- Should find 4 class expressions: MyClass, NamedClass, ExtendedClass, IIFEClass
- MyClass should be detected from `const MyClass = class {}`
- NamedClass should be detected from `class NamedClass {}`
- ExtendedClass should have inheritance dependency on MyClass
- IIFE class should be detected within immediately invoked function expression

**Current Status**: FAILS - Returns 0 classes instead of 4

### 4. TestJavaScriptClassExtraction_MixinFactories_RedPhase

**Purpose**: Test mixin pattern detection for functions returning classes
**Expected Behavior**:
- Should find Entity, User classes plus mixin factory functions
- User class should have `mixin_chain: ["Timestamped", "Serializable", "withValidation"]`
- Mixin factories should be detected with `returns_class: true` metadata
- Timestamped and Serializable should be identified as class-like constructs

**Current Status**: FAILS - Mixin factories not detected as class constructs

### 5. TestJavaScriptClassExtraction_StaticMemberMetadata_RedPhase

**Purpose**: Test static member extraction and metadata population
**Expected Behavior**:
- Should find 3 classes: MathUtils, DatabaseConnection, Singleton
- MathUtils metadata should include:
  - `static_properties: ["PI", "E", "VERSION"]` (never nil)
  - `static_methods: ["add", "multiply", "fetchConstant", "getSecret"]` (never nil)
  - `has_private_static_members: true`
- DatabaseConnection should have `has_static_init_block: true`
- Singleton should have `design_pattern: "singleton"`

**Current Status**: FAILS - No classes found, metadata not populated

### 6. TestJavaScriptClassExtraction_ErrorHandling_RedPhase

**Purpose**: Test robust error handling without panics
**Expected Behavior**:
- Nil parse tree should return error, not panic
- ParseTree with nil Language() should not panic
- Malformed JavaScript syntax should not panic
- Unsupported languages should return appropriate error
- Metadata arrays should never be nil (empty []string instead)

**Current Status**: FAILS - Nil pointer panic instead of graceful error handling

### 7. TestJavaScriptClassExtraction_ComplexIntegration_RedPhase

**Purpose**: Test end-to-end complex scenarios combining all features
**Expected Behavior**:
- Should find 6+ classes including Entity, User, ConfigManager, DatabaseManager, mixin factories
- Entity should have static properties and methods detected
- User should have complex mixin chain with private members
- ConfigManager should have static initialization block
- DatabaseManager should be detected as singleton pattern
- All mixin factories should have `returns_class: true`

**Current Status**: FAILS - Comprehensive integration not working

## Implementation Requirements

### Critical Fixes Needed

1. **Fix Tree-sitter Parser Registration**
   - Ensure JavaScript parser is properly registered in the parser factory
   - Remove fallback to StubParser for JavaScript language
   - Verify real tree-sitter JavaScript grammar is loaded

2. **Fix Nil Pointer Protection**
   - Add nil checks in `ParseTree.Language()` method
   - Add nil parse tree validation in `ExtractClasses` method
   - Ensure graceful error handling throughout the extraction chain

3. **Implement Class Detection Logic**
   - Parse `class_declaration` AST nodes correctly
   - Parse `class` expression AST nodes correctly  
   - Handle variable-assigned class expressions
   - Detect inheritance relationships from AST

4. **Implement Metadata Population**
   - Always initialize `static_properties` and `static_methods` as empty []string
   - Extract static members from AST nodes
   - Detect private static members (# prefix)
   - Detect static initialization blocks
   - Implement design pattern recognition (singleton, factory, mixin)

5. **Implement Mixin Pattern Detection**
   - Identify functions that return classes
   - Parse complex inheritance chains with mixins
   - Set `returns_class: true` metadata for mixin factories
   - Extract mixin chains from complex extends clauses

## AST Node Types to Handle

Based on tree-sitter JavaScript grammar:

- `class_declaration`: Standard class declarations
- `class`: Class expressions (anonymous or named)
- `class_heritage`: Extends clauses and inheritance
- `class_body`: Class body containing methods and properties
- `method_definition`: Class methods
- `field_definition`: Class properties
- `static_block`: Static initialization blocks
- `variable_declarator`: Variable assignments (for class expressions)
- `function_declaration`: Functions that might return classes
- `arrow_function`: Arrow functions that might return classes

## Success Criteria

All RED PHASE tests should pass when:

1. Real tree-sitter JavaScript parser is integrated
2. Nil pointer panics are eliminated
3. Class detection logic properly parses AST nodes
4. Metadata is correctly populated from AST analysis
5. Mixin patterns are detected and classified
6. Error handling is robust and graceful

## Next Phase

Once these RED PHASE tests pass, proceed to GREEN PHASE implementation following TDD methodology.