package valueobject

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The ParseResult, ParseError, ParseWarning and related types are now implemented
// in parse_result.go and imported by this test file.

// TestParseResult_NewParseResult tests creation of ParseResult value objects.
// This is a RED PHASE test that defines expected behavior for parse result creation.
func TestParseResult_NewParseResult(t *testing.T) {
	// Create test data
	language, _ := NewLanguage(LanguageGo)
	parseOptions := ParseOptions{
		Language:        language,
		RecoveryMode:    true,
		IncludeComments: false,
		MaxErrorCount:   10,
		TimeoutDuration: time.Second * 30,
	}

	// Create a simple parse tree for success cases
	rootNode := &ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   13,
		StartPos:  Position{Row: 0, Column: 0},
		EndPos:    Position{Row: 1, Column: 0},
		Children:  []*ParseNode{},
	}

	source := []byte("package main\n")
	metadata, _ := NewParseMetadata(time.Millisecond*10, "0.20.8", "1.0.0")
	parseTree, _ := NewParseTree(context.Background(), language, rootNode, source, metadata)

	tests := []struct {
		name      string
		success   bool
		parseTree *ParseTree
		errors    []ParseError
		warnings  []ParseWarning
		duration  time.Duration
		options   ParseOptions
		wantError bool
		errorMsg  string
	}{
		{
			name:      "successful parse result",
			success:   true,
			parseTree: parseTree,
			errors:    []ParseError{},
			warnings:  []ParseWarning{},
			duration:  time.Millisecond * 15,
			options:   parseOptions,
			wantError: false,
		},
		{
			name:      "failed parse result with errors",
			success:   false,
			parseTree: nil,
			errors: []ParseError{
				{
					Type:        "SyntaxError",
					Message:     "Unexpected token",
					Position:    Position{Row: 1, Column: 5},
					Severity:    "error",
					Recoverable: false,
					Timestamp:   time.Now(),
				},
			},
			warnings:  []ParseWarning{},
			duration:  time.Millisecond * 20,
			options:   parseOptions,
			wantError: false,
		},
		{
			name:      "successful parse result with warnings",
			success:   true,
			parseTree: parseTree,
			errors:    []ParseError{},
			warnings: []ParseWarning{
				{
					Type:      "StyleWarning",
					Message:   "Unused variable",
					Position:  Position{Row: 2, Column: 10},
					Code:      "W001",
					Timestamp: time.Now(),
				},
			},
			duration:  time.Millisecond * 12,
			options:   parseOptions,
			wantError: false,
		},
		{
			name:      "invalid: negative duration",
			success:   true,
			parseTree: parseTree,
			errors:    []ParseError{},
			warnings:  []ParseWarning{},
			duration:  time.Millisecond * -5,
			options:   parseOptions,
			wantError: true,
			errorMsg:  "parse duration cannot be negative",
		},
		{
			name:      "invalid: successful result without parse tree",
			success:   true,
			parseTree: nil,
			errors:    []ParseError{},
			warnings:  []ParseWarning{},
			duration:  time.Millisecond * 15,
			options:   parseOptions,
			wantError: true,
			errorMsg:  "successful parse result must have a parse tree",
		},
		{
			name:      "invalid: failed result without errors",
			success:   false,
			parseTree: nil,
			errors:    []ParseError{},
			warnings:  []ParseWarning{},
			duration:  time.Millisecond * 15,
			options:   parseOptions,
			wantError: true,
			errorMsg:  "failed parse result must have at least one error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewParseResult(
				context.Background(),
				tt.success,
				tt.parseTree,
				tt.errors,
				tt.warnings,
				tt.duration,
				tt.options,
			)

			if tt.wantError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.success, result.IsSuccessful())
				assert.Equal(t, tt.duration, result.Duration())
				assert.Equal(t, len(tt.errors), result.ErrorCount())
				assert.Equal(t, len(tt.warnings), result.WarningCount())

				if tt.success {
					assert.NotNil(t, result.ParseTree())
				} else {
					assert.Nil(t, result.ParseTree())
				}
			}
		})
	}
}

// TestParseResult_AccessorMethods tests accessor methods for parse results.
// This is a RED PHASE test that defines expected accessor behavior.
func TestParseResult_AccessorMethods(t *testing.T) {
	// Create test parse result
	language, _ := NewLanguage(LanguagePython)
	parseOptions := ParseOptions{
		Language:        language,
		RecoveryMode:    true,
		MaxErrorCount:   5,
		TimeoutDuration: time.Second * 15,
	}

	rootNode := &ParseNode{
		Type:      "module",
		StartByte: 0,
		EndByte:   20,
		StartPos:  Position{Row: 0, Column: 0},
		EndPos:    Position{Row: 1, Column: 20},
		Children:  []*ParseNode{},
	}

	source := []byte("print('Hello World')")
	metadata, _ := NewParseMetadata(time.Millisecond*8, "0.20.8", "0.21.0")
	parseTree, _ := NewParseTree(context.Background(), language, rootNode, source, metadata)

	errors := []ParseError{
		{
			Type:        "WarningError",
			Message:     "Deprecated syntax",
			Position:    Position{Row: 0, Column: 5},
			Severity:    "warning",
			Recoverable: true,
			Timestamp:   time.Now(),
		},
	}

	warnings := []ParseWarning{
		{
			Type:      "StyleWarning",
			Message:   "Consider using f-strings",
			Position:  Position{Row: 0, Column: 0},
			Code:      "S001",
			Timestamp: time.Now(),
		},
	}

	duration := time.Millisecond * 25
	result, err := NewParseResult(context.Background(), true, parseTree, errors, warnings, duration, parseOptions)
	require.NoError(t, err)

	tests := []struct {
		name           string
		method         func() interface{}
		expectedResult interface{}
	}{
		{
			name: "IsSuccessful returns true for successful parse",
			method: func() interface{} {
				return result.IsSuccessful()
			},
			expectedResult: true,
		},
		{
			name: "HasErrors returns true when errors present",
			method: func() interface{} {
				return result.HasErrors()
			},
			expectedResult: true,
		},
		{
			name: "HasWarnings returns true when warnings present",
			method: func() interface{} {
				return result.HasWarnings()
			},
			expectedResult: true,
		},
		{
			name: "ErrorCount returns correct error count",
			method: func() interface{} {
				return result.ErrorCount()
			},
			expectedResult: 1,
		},
		{
			name: "WarningCount returns correct warning count",
			method: func() interface{} {
				return result.WarningCount()
			},
			expectedResult: 1,
		},
		{
			name: "Duration returns correct parsing duration",
			method: func() interface{} {
				return result.Duration()
			},
			expectedResult: duration,
		},
		{
			name: "ParseTree returns the parse tree",
			method: func() interface{} {
				tree := result.ParseTree()
				if tree == nil {
					return nil
				}
				return tree.Language().Name()
			},
			expectedResult: LanguagePython,
		},
		{
			name: "GetStatistics returns parsing statistics",
			method: func() interface{} {
				stats := result.GetStatistics()
				return stats.TotalNodes > 0
			},
			expectedResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.method()
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

// TestParseResult_ErrorHandlingMethods tests error handling methods.
// This is a RED PHASE test that defines expected error handling behavior.
func TestParseResult_ErrorHandlingMethods(t *testing.T) {
	language, _ := NewLanguage(LanguageJavaScript)
	parseOptions := ParseOptions{
		Language:      language,
		RecoveryMode:  false,
		MaxErrorCount: 3,
	}

	errors := []ParseError{
		{
			Type:        "SyntaxError",
			Message:     "Unexpected token ';'",
			Position:    Position{Row: 1, Column: 10},
			Severity:    "error",
			Recoverable: false,
			Code:        "E001",
			Timestamp:   time.Now(),
		},
		{
			Type:        "TypeError",
			Message:     "Cannot read property of undefined",
			Position:    Position{Row: 2, Column: 5},
			Severity:    "error",
			Recoverable: true,
			Code:        "E002",
			Timestamp:   time.Now(),
		},
	}

	result, err := NewParseResult(
		context.Background(),
		false,
		nil,
		errors,
		[]ParseWarning{},
		time.Millisecond*30,
		parseOptions,
	)
	require.NoError(t, err)

	tests := []struct {
		name     string
		testFunc func() (interface{}, error)
		validate func(interface{}) bool
	}{
		{
			name: "GetErrorsByType returns errors of specific type",
			testFunc: func() (interface{}, error) {
				return result.GetErrorsByType("SyntaxError"), nil
			},
			validate: func(result interface{}) bool {
				errs, ok := result.([]ParseError)
				return ok && len(errs) == 1 && errs[0].Type == "SyntaxError"
			},
		},
		{
			name: "GetErrorsByPosition returns errors at specific position",
			testFunc: func() (interface{}, error) {
				return result.GetErrorsByPosition(Position{Row: 1, Column: 10}), nil
			},
			validate: func(result interface{}) bool {
				errs, ok := result.([]ParseError)
				return ok && len(errs) == 1 && errs[0].Message == "Unexpected token ';'"
			},
		},
		{
			name: "GetRecoverableErrors returns only recoverable errors",
			testFunc: func() (interface{}, error) {
				return result.GetRecoverableErrors(), nil
			},
			validate: func(result interface{}) bool {
				errs, ok := result.([]ParseError)
				return ok && len(errs) == 1 && errs[0].Type == "TypeError"
			},
		},
		{
			name: "GetCriticalErrors returns only critical errors",
			testFunc: func() (interface{}, error) {
				return result.GetCriticalErrors(), nil
			},
			validate: func(result interface{}) bool {
				errs, ok := result.([]ParseError)
				return ok && len(errs) == 1 && errs[0].Type == "SyntaxError"
			},
		},
		{
			name: "FormatErrors returns formatted error summary",
			testFunc: func() (interface{}, error) {
				return result.FormatErrors(), nil
			},
			validate: func(result interface{}) bool {
				formatted, ok := result.(string)
				return ok && len(formatted) > 0 &&
					strings.Contains(formatted, "SyntaxError") &&
					strings.Contains(formatted, "TypeError")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.testFunc()
			require.NoError(t, err)
			assert.True(t, tt.validate(result), "Validation failed for %s", tt.name)
		})
	}
}

// TestParseResult_SerializationMethods tests serialization capabilities.
// This is a RED PHASE test that defines expected serialization behavior.
func TestParseResult_SerializationMethods(t *testing.T) {
	language, _ := NewLanguage(LanguageRust)
	parseOptions := ParseOptions{
		Language:          language,
		IncludeComments:   true,
		IncludeWhitespace: false,
		MaxErrorCount:     10,
	}

	rootNode := &ParseNode{
		Type:      "source_file",
		StartByte: 0,
		EndByte:   25,
		StartPos:  Position{Row: 0, Column: 0},
		EndPos:    Position{Row: 1, Column: 1},
		Children:  []*ParseNode{},
	}

	source := []byte("fn main() {\n    println!(\"Hello\");\n}")
	metadata, _ := NewParseMetadata(time.Millisecond*18, "0.20.8", "0.20.0")
	parseTree, _ := NewParseTree(context.Background(), language, rootNode, source, metadata)

	result, err := NewParseResult(
		context.Background(),
		true,
		parseTree,
		[]ParseError{},
		[]ParseWarning{},
		time.Millisecond*18,
		parseOptions,
	)
	require.NoError(t, err)

	tests := []struct {
		name     string
		testFunc func() (interface{}, error)
		validate func(interface{}) bool
	}{
		{
			name: "serialize to JSON",
			testFunc: func() (interface{}, error) {
				return result.ToJSON()
			},
			validate: func(result interface{}) bool {
				jsonStr, ok := result.(string)
				if !ok {
					return false
				}
				return strings.Contains(jsonStr, "\"success\"") &&
					strings.Contains(jsonStr, "\"statistics\"") &&
					strings.Contains(jsonStr, "Rust")
			},
		},
		{
			name: "export summary report",
			testFunc: func() (interface{}, error) {
				return result.ExportSummary()
			},
			validate: func(result interface{}) bool {
				summary, ok := result.(string)
				if !ok {
					return false
				}
				return strings.Contains(summary, "Parse Result Summary") &&
					strings.Contains(summary, "Language: Rust") &&
					strings.Contains(summary, "Status: Success")
			},
		},
		{
			name: "export detailed report",
			testFunc: func() (interface{}, error) {
				return result.ExportDetailedReport()
			},
			validate: func(result interface{}) bool {
				report, ok := result.(string)
				if !ok {
					return false
				}
				return strings.Contains(report, "Detailed Parse Report") &&
					strings.Contains(report, "Parse Tree Structure") &&
					strings.Contains(report, "Performance Statistics")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.testFunc()
			require.NoError(t, err)
			assert.True(t, tt.validate(result), "Validation failed for %s", tt.name)
		})
	}
}
