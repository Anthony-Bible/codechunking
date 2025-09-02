package chunking

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"strings"
)

// FunctionAnalysisUtils provides utilities for analyzing function calls and relationships
// in code chunks. This supports Phase 4.3 Step 2 function-level chunking with context preservation.
type FunctionAnalysisUtils struct{}

// ExtractFunctionCallsFromContent extracts function calls from content based on language-specific patterns.
// Phase 4.3 Step 2: Enhanced called function detection for helper relationship analysis.
func (u *FunctionAnalysisUtils) ExtractFunctionCallsFromContent(
	content string,
	language valueobject.Language,
) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		// Simple pattern matching for function calls (basic implementation)
		switch language.Name() {
		case valueobject.LanguageGo:
			calls = append(calls, u.extractGoFunctionCalls(line, i)...)
		case valueobject.LanguagePython:
			calls = append(calls, u.extractPythonFunctionCalls(line, i)...)
		case valueobject.LanguageJavaScript, valueobject.LanguageTypeScript:
			calls = append(calls, u.extractJSFunctionCalls(line, i)...)
		}
	}

	return calls
}

// extractGoFunctionCalls extracts Go function calls from a line of code.
func (u *FunctionAnalysisUtils) extractGoFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Simple pattern: functionName( or variable.functionName(
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if u.IsValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: uint32(lineNum * 80),           // #nosec G115
					EndByte:   uint32(lineNum*80 + len(word)), // #nosec G115
				})
			}
		}
	}

	return calls
}

// extractPythonFunctionCalls extracts Python function calls from a line of code.
func (u *FunctionAnalysisUtils) extractPythonFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Similar logic for Python
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if u.IsValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: uint32(lineNum * 80),           // #nosec G115
					EndByte:   uint32(lineNum*80 + len(word)), // #nosec G115
				})
			}
		}
	}

	return calls
}

// extractJSFunctionCalls extracts JavaScript function calls from a line of code.
func (u *FunctionAnalysisUtils) extractJSFunctionCalls(line string, lineNum int) []outbound.FunctionCall {
	var calls []outbound.FunctionCall

	// Similar logic for JavaScript
	words := strings.Fields(line)
	for _, word := range words {
		if strings.Contains(word, "(") {
			funcName := strings.Split(word, "(")[0]
			if strings.Contains(funcName, ".") {
				funcName = strings.Split(funcName, ".")[1]
			}
			if u.IsValidFunctionName(funcName) {
				calls = append(calls, outbound.FunctionCall{
					Name:      funcName,
					StartByte: uint32(lineNum * 80),           // #nosec G115
					EndByte:   uint32(lineNum*80 + len(word)), // #nosec G115
				})
			}
		}
	}

	return calls
}

// IsValidFunctionName checks if a string looks like a valid function name.
func (u *FunctionAnalysisUtils) IsValidFunctionName(name string) bool {
	if len(name) < 1 || len(name) > 100 {
		return false
	}

	// Must start with letter or underscore
	if (name[0] < 'a' || name[0] > 'z') &&
		(name[0] < 'A' || name[0] > 'Z') &&
		name[0] != '_' {
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

// IsInternalFunction checks if a function call refers to an internal function/closure within the same chunk.
// Phase 4.3 Step 2: Internal closure and nested function detection.
func (u *FunctionAnalysisUtils) IsInternalFunction(funcName string, chunk outbound.SemanticCodeChunk) bool {
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

// HasSharedFunctionCalls checks if two chunks call the same functions, indicating a relationship.
func (u *FunctionAnalysisUtils) HasSharedFunctionCalls(chunk1, chunk2 outbound.SemanticCodeChunk) bool {
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

// HasSharedTypes checks if two chunks use the same types, indicating a relationship.
func (u *FunctionAnalysisUtils) HasSharedTypes(chunk1, chunk2 outbound.SemanticCodeChunk) bool {
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
