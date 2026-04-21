package goparser

// RED PHASE - Issue #58: Go parser does not populate StartPosition/EndPosition
//
// These tests verify that every SemanticCodeChunk returned by the Go functions/methods
// extractor has StartPosition.Row and EndPosition.Row populated with values that reflect
// where the entity actually appears in the source file.
//
// FAILS because: buildMethodChunk and the function chunk builder (functions.go ~line 270)
// set StartByte/EndByte from node positions but never set StartPosition/EndPosition.
// The fix is to add:
//   StartPosition: valueobject.Position{Row: node.StartPos.Row, Column: node.StartPos.Column},
//   EndPosition:   valueobject.Position{Row: node.EndPos.Row, Column: node.EndPos.Column},
// mirroring the reference implementation in parsers/javascript/imports.go lines 46-47.

import (
	"codechunking/internal/adapter/outbound/treesitter"
	"codechunking/internal/port/outbound"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// buildSourceWithFunctionAtLine produces a minimal compilable Go source where the named
// function starts at the given 1-indexed line number, padded with blank comment lines.
func buildSourceWithFunctionAtLine(funcName string, startLine int) string {
	var sb strings.Builder
	sb.WriteString("package linetest\n")
	// We already wrote line 1 (package decl). Pad to (startLine - 1) with blank lines.
	for i := 2; i < startLine; i++ {
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "func %s() {\n\treturn\n}\n", funcName)
	return sb.String()
}

// buildSourceWithMethodAtLine produces a minimal compilable Go source with a struct and
// a method on it, where the method starts at the given 1-indexed line number.
func buildSourceWithMethodAtLine(methodName string, startLine int) string {
	var sb strings.Builder
	sb.WriteString("package linetest\n\ntype T struct{}\n")
	// Already wrote 3 lines. Pad to (startLine - 1).
	for i := 4; i < startLine; i++ {
		sb.WriteString("\n")
	}
	fmt.Fprintf(&sb, "func (t T) %s() {\n\treturn\n}\n", methodName)
	return sb.String()
}

// extractFunctionsFromSource is a test helper that parses source and returns all
// SemanticCodeChunks for functions/methods using the real Go tree-sitter parser.
func extractFunctionsFromSource(t *testing.T, source string) []outbound.SemanticCodeChunk {
	t.Helper()

	ctx := context.Background()
	parserIface, err := NewGoParser()
	require.NoError(t, err)

	parser := parserIface.(*ObservableGoParser)

	parseResult, err := parser.Parse(ctx, []byte(source))
	require.NoError(t, err)
	require.NotNil(t, parseResult)

	domainTree, err := treesitter.ConvertPortParseTreeToDomain(parseResult.ParseTree)
	require.NoError(t, err)

	opts := outbound.SemanticExtractionOptions{IncludePrivate: true}
	chunks, err := parser.ExtractFunctions(ctx, domainTree, opts)
	require.NoError(t, err)

	return chunks
}

// TestGoParser_ExtractFunctions_PopulatesStartPositionRow verifies that every extracted
// function chunk has a non-zero StartPosition.Row when the function does not start at
// the first line of the file.
//
// FAILS because: functions.go never sets StartPosition on the returned SemanticCodeChunk.
func TestGoParser_ExtractFunctions_PopulatesStartPositionRow(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		functionName string
		startLine    int // 1-indexed line where the function starts
		wantStartRow uint32
	}

	cases := []testCase{
		{
			name:         "function_at_line_10",
			functionName: "FuncAtTen",
			startLine:    10,
			wantStartRow: 9, // tree-sitter is 0-indexed
		},
		{
			name:         "function_at_line_25",
			functionName: "FuncAtTwentyFive",
			startLine:    25,
			wantStartRow: 24,
		},
		{
			name:         "function_at_line_50",
			functionName: "FuncAtFifty",
			startLine:    50,
			wantStartRow: 49,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := buildSourceWithFunctionAtLine(tc.functionName, tc.startLine)
			chunks := extractFunctionsFromSource(t, source)

			var found *outbound.SemanticCodeChunk
			for i := range chunks {
				if chunks[i].Name == tc.functionName {
					found = &chunks[i]
					break
				}
			}

			if found == nil {
				t.Fatalf("function %q not found in extracted chunks", tc.functionName)
			}

			// EXPECTED: StartPosition.Row == tc.wantStartRow (tree-sitter 0-indexed row)
			// ACTUAL (buggy): StartPosition == {Row: 0, Column: 0} because it is never set.
			if found.StartPosition.Row != tc.wantStartRow {
				t.Errorf("StartPosition.Row: got %d, want %d; "+
					"Go parser does not populate StartPosition from node.StartPos",
					found.StartPosition.Row, tc.wantStartRow)
			}
		})
	}
}

// TestGoParser_ExtractFunctions_PopulatesEndPositionRow verifies that every extracted
// function chunk has EndPosition.Row set to a value greater than StartPosition.Row for
// multi-line functions.
//
// FAILS because: functions.go never sets EndPosition on the returned SemanticCodeChunk.
func TestGoParser_ExtractFunctions_PopulatesEndPositionRow(t *testing.T) {
	t.Parallel()

	// A function at line 15 spanning 3 lines: rows 14, 15, 16 (0-indexed).
	// EndPosition.Row should be 16 (the closing brace line).
	source := buildSourceWithFunctionAtLine("MultiLineFunc", 15)
	chunks := extractFunctionsFromSource(t, source)

	var found *outbound.SemanticCodeChunk
	for i := range chunks {
		if chunks[i].Name == "MultiLineFunc" {
			found = &chunks[i]
			break
		}
	}

	require.NotNil(t, found, "MultiLineFunc not found in extracted chunks")

	// EXPECTED: EndPosition.Row > StartPosition.Row for a multi-line function.
	// ACTUAL (buggy): EndPosition == {Row: 0, Column: 0} because it is never set.
	if found.EndPosition.Row == 0 {
		t.Errorf("EndPosition.Row: got 0, want > 0; "+
			"Go parser does not populate EndPosition from node.EndPos; "+
			"StartByte=%d EndByte=%d",
			found.StartByte, found.EndByte)
	}

	if found.EndPosition.Row <= found.StartPosition.Row {
		t.Errorf("EndPosition.Row (%d) should be > StartPosition.Row (%d) for a multi-line function",
			found.EndPosition.Row, found.StartPosition.Row)
	}
}

// TestGoParser_ExtractMethods_PopulatesStartPositionRow verifies that method chunks
// (not just function chunks) also have StartPosition.Row populated correctly.
//
// FAILS because: buildMethodChunk in functions.go also never sets StartPosition/EndPosition.
func TestGoParser_ExtractMethods_PopulatesStartPositionRow(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		methodName   string
		startLine    int
		wantStartRow uint32
	}

	cases := []testCase{
		{
			name:         "method_at_line_10",
			methodName:   "MethodAtTen",
			startLine:    10,
			wantStartRow: 9,
		},
		{
			name:         "method_at_line_20",
			methodName:   "MethodAtTwenty",
			startLine:    20,
			wantStartRow: 19,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			source := buildSourceWithMethodAtLine(tc.methodName, tc.startLine)
			chunks := extractFunctionsFromSource(t, source)

			var found *outbound.SemanticCodeChunk
			for i := range chunks {
				if chunks[i].Name == tc.methodName {
					found = &chunks[i]
					break
				}
			}

			if found == nil {
				t.Fatalf("method %q not found in extracted chunks", tc.methodName)
			}

			// EXPECTED: StartPosition.Row == tc.wantStartRow
			// ACTUAL (buggy): StartPosition == {Row: 0, Column: 0}
			if found.StartPosition.Row != tc.wantStartRow {
				t.Errorf("StartPosition.Row: got %d, want %d; "+
					"buildMethodChunk does not populate StartPosition from node.StartPos",
					found.StartPosition.Row, tc.wantStartRow)
			}
		})
	}
}

// TestGoParser_AllExtractedChunks_HaveNonZeroPositionWhenNotAtFileStart verifies the
// contract that any chunk whose content does not start at byte 0 must have a non-zero
// StartPosition.Row. This is the high-level contract test covering both functions and methods.
//
// FAILS because: the Go parser never populates StartPosition/EndPosition on any chunk.
func TestGoParser_AllExtractedChunks_HaveNonZeroPositionWhenNotAtFileStart(t *testing.T) {
	t.Parallel()

	// Source with two functions: one at line 2, one at line 10.
	// The second function must have StartPosition.Row >= 9.
	source := "package linetest\n" +
		"func First() {}\n" + // line 2, row 1
		"\n" +
		"\n" +
		"\n" +
		"\n" +
		"\n" +
		"\n" +
		"\n" +
		"func Second() {\n" + // line 10, row 9
		"\treturn\n" +
		"}\n"

	chunks := extractFunctionsFromSource(t, source)

	var second *outbound.SemanticCodeChunk
	for i := range chunks {
		if chunks[i].Name == "Second" {
			second = &chunks[i]
			break
		}
	}

	require.NotNil(t, second, "Second function not found in extracted chunks")

	// Second starts at byte > 0 (after First and blank lines).
	// Its StartPosition.Row must be 9 (0-indexed line 10).
	if second.StartByte == 0 {
		t.Skip("StartByte unexpectedly 0; cannot test non-zero position invariant")
	}

	if second.StartPosition.Row == 0 {
		t.Errorf("StartPosition.Row: got 0 for function %q with StartByte=%d; "+
			"expected non-zero row because function does not start at file beginning; "+
			"Go parser must set StartPosition from node.StartPos like javascript/imports.go does",
			second.Name, second.StartByte)
	}
}
