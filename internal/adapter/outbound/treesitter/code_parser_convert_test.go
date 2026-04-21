package treesitter

// RED PHASE - Issue #58: Fix chunk line numbers
//
// These tests verify the CORRECT behavior of convertSemanticToCodeChunks.
// They will FAIL against the current implementation because:
//
// Bug 1: estimateLineNumber is called with semanticChunk.Content (just the chunk text)
//        and semanticChunk.StartByte (a FILE-WIDE byte offset). For any entity not at
//        file start, startByte >= len(content) which causes wrong line numbers.
//
// Bug 2: Go parsers (functions.go) never populate StartPosition/EndPosition on
//        SemanticCodeChunk even though tree-sitter provides accurate 0-indexed row/col.
//
// Bug 3: convertSemanticToCodeChunks is called without full file content, making
//        correct byte-scanning fallback impossible.
//
// The fix requires:
//   1. Adding a `fullContent string` parameter to convertSemanticToCodeChunks
//   2. Using StartPosition.Row+1 / EndPosition.Row+1 when positions are non-zero
//   3. Falling back to estimateLineNumber(fullContent, startByte) when positions are zero
//   4. Go parsers must populate StartPosition/EndPosition from node.StartPos/node.EndPos

import (
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"strings"
	"testing"
)

// buildFullContentWithFunctionAtLine builds a Go source string where a trivial function
// starts at the given 1-indexed line number by prepending (lineNumber-1) comment lines.
func buildFullContentWithFunctionAtLine(funcSrc string, lineNumber int) string {
	var sb strings.Builder
	for i := 1; i < lineNumber; i++ {
		sb.WriteString("// padding line\n")
	}
	sb.WriteString(funcSrc)
	return sb.String()
}

// TestConvertSemanticToCodeChunks_PositionPopulated_UsesRowPlusOne verifies that when
// StartPosition and EndPosition are populated (as tree-sitter provides), the resulting
// CodeChunk uses (Row + 1) for 1-indexed line numbers.
//
// FAILS because: current code ignores StartPosition/EndPosition entirely and calls
// estimateLineNumber(chunk.Content, startByte) which gives wrong results when
// startByte >= len(chunk.Content).
func TestConvertSemanticToCodeChunks_PositionPopulated_UsesRowPlusOne(t *testing.T) {
	t.Parallel()

	chunkContent := "func Hello() {\n\treturn\n}\n"
	// Place the function at line 10 (0-indexed row 9).
	fullContent := buildFullContentWithFunctionAtLine(chunkContent, 10)
	startByte := uint32(len(fullContent) - len(chunkContent))

	chunks := []outbound.SemanticCodeChunk{
		{
			ChunkID:  "test-position-chunk",
			Name:     "Hello",
			Type:     outbound.ConstructFunction,
			Language: valueobject.Go,
			Content:  chunkContent,
			// Tree-sitter gives 0-indexed rows. Row 9 = line 10 (1-indexed).
			StartPosition: valueobject.Position{Row: 9, Column: 0},
			// Function spans 3 lines: rows 9, 10, 11 -> lines 10, 11, 12.
			EndPosition: valueobject.Position{Row: 11, Column: 1},
			StartByte:   startByte,
			EndByte:     startByte + uint32(len(chunkContent)),
			Hash:        "deadbeef",
		},
	}

	p := &TreeSitterCodeParser{}
	// NOTE: When fullContent parameter is added, callers pass it here.
	// Until then the existing 3-arg signature is used and this test FAILS on assertions.
	result := p.convertSemanticToCodeChunks("example.go", chunks, "go")

	if len(result) != 1 {
		t.Fatalf("expected 1 CodeChunk, got %d", len(result))
	}

	got := result[0]

	// EXPECTED: StartPosition.Row (9) + 1 = 10
	// ACTUAL (buggy): estimateLineNumber(chunkContent, startByte) where startByte >= len(chunkContent)
	//                 returns strings.Count(chunkContent, "\n") + 1 = 4, which is WRONG.
	if got.StartLine != 10 {
		t.Errorf("StartLine: got %d, want 10 (StartPosition.Row=9 + 1); bug: startByte(%d) >= len(content)(%d) causes wrong fallback",
			got.StartLine, startByte, len(chunkContent))
	}

	// EXPECTED: EndPosition.Row (11) + 1 = 12
	// ACTUAL (buggy): same wrong byte-scan logic returns ~4 (end of chunk content).
	if got.EndLine != 12 {
		t.Errorf("EndLine: got %d, want 12 (EndPosition.Row=11 + 1)", got.EndLine)
	}
}

// TestConvertSemanticToCodeChunks_BothPositionsZero_ByteScanUsesFullContent verifies that
// when StartPosition/EndPosition are zero (not yet populated by parsers) the function
// falls back to byte-scanning with the FULL file content.
//
// FAILS because: current code calls estimateLineNumber(chunk.Content, startByte) where
// chunk.Content is only the function body and startByte is file-wide. For any function
// after the first ~N bytes of chunk content, bytePos >= len(content) and the scan is wrong.
//
// NOTE: This test documents expected behavior post-fix. Verifying the fallback requires
// the fullContent parameter. Once added, this test will pass when implemented correctly.
func TestConvertSemanticToCodeChunks_BothPositionsZero_ByteScanUsesFullContent(t *testing.T) {
	t.Parallel()

	chunkContent := "func Late() {\n\treturn\n}\n"
	// Place function at line 20 so startByte >> len(chunkContent).
	fullContent := buildFullContentWithFunctionAtLine(chunkContent, 20)
	startByte := uint32(len(fullContent) - len(chunkContent))

	chunks := []outbound.SemanticCodeChunk{
		{
			ChunkID:  "fallback-chunk",
			Name:     "Late",
			Type:     outbound.ConstructFunction,
			Language: valueobject.Go,
			Content:  chunkContent,
			// Positions zero: parser did not populate them (the current state of Go parsers).
			StartPosition: valueobject.Position{Row: 0, Column: 0},
			EndPosition:   valueobject.Position{Row: 0, Column: 0},
			StartByte:     startByte,
			EndByte:       startByte + uint32(len(chunkContent)),
			Hash:          "cafebabe",
		},
	}

	p := &TreeSitterCodeParser{}
	result := p.convertSemanticToCodeChunks("late.go", chunks, "go")

	if len(result) != 1 {
		t.Fatalf("expected 1 CodeChunk, got %d", len(result))
	}

	got := result[0]

	// Correct byte-scan against fullContent at startByte lands on line 20.
	// Current buggy code scans chunk.Content (~24 bytes) with startByte (~304 bytes) -> wrong.
	if got.StartLine != 20 {
		t.Errorf("StartLine (byte-scan fallback against full content): got %d, want 20; "+
			"current code scans chunk.Content (%d bytes) with file-wide startByte (%d) which is OOB",
			got.StartLine, len(chunkContent), startByte)
	}
}

// TestConvertSemanticToCodeChunks_EntityAtFileStart_Row0WithNonZeroEndRow verifies that
// an entity at the very beginning of a file (StartPosition.Row == 0) is reported as
// StartLine == 1 (not 0), and EndLine reflects the actual EndPosition.Row + 1.
//
// FAILS because: current code ignores StartPosition/EndPosition entirely.
// The byte-scan with startByte=0 accidentally gives the right StartLine=1, but
// EndLine is still wrong because EndByte is passed into estimateLineNumber(chunkContent, endByte)
// where endByte is likely >= len(chunkContent) for multi-line functions.
func TestConvertSemanticToCodeChunks_EntityAtFileStart_Row0WithNonZeroEndRow(t *testing.T) {
	t.Parallel()

	chunkContent := "func First() {\n\tvar a = 1\n\tvar b = 2\n\treturn\n}\n"

	chunks := []outbound.SemanticCodeChunk{
		{
			ChunkID:  "first-chunk",
			Name:     "First",
			Type:     outbound.ConstructFunction,
			Language: valueobject.Go,
			Content:  chunkContent,
			// Row 0 is valid (file start). EndRow 4 means 5 lines total (lines 1-5).
			StartPosition: valueobject.Position{Row: 0, Column: 0},
			EndPosition:   valueobject.Position{Row: 4, Column: 1},
			StartByte:     0,
			EndByte:       uint32(len(chunkContent)),
			Hash:          "firsthash",
		},
	}

	p := &TreeSitterCodeParser{}
	result := p.convertSemanticToCodeChunks("first.go", chunks, "go")

	if len(result) != 1 {
		t.Fatalf("expected 1 CodeChunk, got %d", len(result))
	}

	got := result[0]

	// Row 0 -> line 1 (1-indexed).
	if got.StartLine != 1 {
		t.Errorf("StartLine: got %d, want 1 (StartPosition.Row=0 -> 1-indexed line 1)", got.StartLine)
	}
	// Row 4 -> line 5 (1-indexed).
	if got.EndLine != 5 {
		t.Errorf("EndLine: got %d, want 5 (EndPosition.Row=4 -> 1-indexed line 5); "+
			"current code ignores EndPosition and may report wrong EndLine", got.EndLine)
	}
}

// TestConvertSemanticToCodeChunks_ZeroByteOffsets_ReturnsSafeLine1 verifies that when
// both byte offsets AND position fields are zero, the function does not panic and returns
// at least line 1 (never 0 or negative).
//
// FAILS when position-based logic is added: a zero StartPosition.Row must be disambiguated
// from "not populated" vs "actually at row 0". The fix needs a sentinel or separate check.
// For now, if positions are zero AND startByte is zero, line 1 is the correct answer.
func TestConvertSemanticToCodeChunks_ZeroByteOffsets_ReturnsSafeLine1(t *testing.T) {
	t.Parallel()

	chunkContent := "func Lonely() {}\n"
	fullContent := chunkContent

	chunks := []outbound.SemanticCodeChunk{
		{
			ChunkID:       "zero-chunk",
			Name:          "Lonely",
			Type:          outbound.ConstructFunction,
			Language:      valueobject.Go,
			Content:       chunkContent,
			StartPosition: valueobject.Position{Row: 0, Column: 0},
			EndPosition:   valueobject.Position{Row: 0, Column: 0},
			StartByte:     0,
			EndByte:       0,
			Hash:          "00000000",
		},
	}

	p := &TreeSitterCodeParser{}
	result := p.convertSemanticToCodeChunks("lonely.go", chunks, "go")

	if len(result) != 1 {
		t.Fatalf("expected 1 CodeChunk, got %d", len(result))
	}

	got := result[0]

	// Must not return 0 or negative - 1 is the floor.
	if got.StartLine < 1 {
		t.Errorf("StartLine: got %d, want >= 1 (line numbers must be 1-indexed)", got.StartLine)
	}
	if got.EndLine < 1 {
		t.Errorf("EndLine: got %d, want >= 1 (line numbers must be 1-indexed)", got.EndLine)
	}

	_ = fullContent // used to document intent; needed when fullContent param is added
}

// TestConvertSemanticToCodeChunks_MultipleChunks_EachGetsIndependentCorrectLines verifies
// that a file with multiple functions produces CodeChunks with correct independent line numbers
// for each function.
//
// FAILS because: current buggy byte-scan uses chunk.Content (not full file), causing all
// non-first functions to report the same wrong line numbers (content length boundary hit).
func TestConvertSemanticToCodeChunks_MultipleChunks_EachGetsIndependentCorrectLines(t *testing.T) {
	t.Parallel()

	// File layout (1-indexed lines):
	//   line 1: "package multi"
	//   line 2: ""
	//   line 3: "func Alpha() {}"   -> row 2
	//   line 4: ""
	//   line 5: "func Beta() {"     -> row 4
	//   line 6: "\treturn"
	//   line 7: "}"                 -> row 6
	//   line 8: ""
	//   line 9: "func Gamma() {"    -> row 8
	//   line 10: "\tx := 1"
	//   line 11: "\treturn"
	//   line 12: "}"                -> row 11

	type testCase struct {
		name          string
		chunk         outbound.SemanticCodeChunk
		wantStartLine int
		wantEndLine   int
	}

	cases := []testCase{
		{
			name: "Alpha_at_line_3",
			chunk: outbound.SemanticCodeChunk{
				ChunkID:       "alpha",
				Name:          "Alpha",
				Type:          outbound.ConstructFunction,
				Language:      valueobject.Go,
				Content:       "func Alpha() {}",
				StartPosition: valueobject.Position{Row: 2, Column: 0},
				EndPosition:   valueobject.Position{Row: 2, Column: 16},
				StartByte:     uint32(len("package multi\n\n")),
				EndByte:       uint32(len("package multi\n\n") + len("func Alpha() {}")),
				Hash:          "alpha-hash",
			},
			wantStartLine: 3,
			wantEndLine:   3,
		},
		{
			name: "Beta_at_lines_5_to_7",
			chunk: outbound.SemanticCodeChunk{
				ChunkID:       "beta",
				Name:          "Beta",
				Type:          outbound.ConstructFunction,
				Language:      valueobject.Go,
				Content:       "func Beta() {\n\treturn\n}",
				StartPosition: valueobject.Position{Row: 4, Column: 0},
				EndPosition:   valueobject.Position{Row: 6, Column: 1},
				StartByte:     uint32(len("package multi\n\nfunc Alpha() {}\n\n")),
				EndByte:       uint32(len("package multi\n\nfunc Alpha() {}\n\nfunc Beta() {\n\treturn\n}")),
				Hash:          "beta-hash",
			},
			wantStartLine: 5,
			wantEndLine:   7,
		},
		{
			name: "Gamma_at_lines_9_to_12",
			chunk: outbound.SemanticCodeChunk{
				ChunkID:       "gamma",
				Name:          "Gamma",
				Type:          outbound.ConstructFunction,
				Language:      valueobject.Go,
				Content:       "func Gamma() {\n\tx := 1\n\treturn\n}",
				StartPosition: valueobject.Position{Row: 8, Column: 0},
				EndPosition:   valueobject.Position{Row: 11, Column: 1},
				StartByte:     uint32(len("package multi\n\nfunc Alpha() {}\n\nfunc Beta() {\n\treturn\n}\n\n")),
				EndByte:       uint32(len("package multi\n\nfunc Alpha() {}\n\nfunc Beta() {\n\treturn\n}\n\nfunc Gamma() {\n\tx := 1\n\treturn\n}")),
				Hash:          "gamma-hash",
			},
			wantStartLine: 9,
			wantEndLine:   12,
		},
	}

	inputChunks := make([]outbound.SemanticCodeChunk, len(cases))
	for i, tc := range cases {
		inputChunks[i] = tc.chunk
	}

	p := &TreeSitterCodeParser{}
	result := p.convertSemanticToCodeChunks("multi.go", inputChunks, "go")

	if len(result) != len(cases) {
		t.Fatalf("expected %d CodeChunks, got %d", len(cases), len(result))
	}

	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := result[i]
			if got.StartLine != tc.wantStartLine {
				t.Errorf("StartLine: got %d, want %d (StartPosition.Row=%d + 1)",
					got.StartLine, tc.wantStartLine, tc.chunk.StartPosition.Row)
			}
			if got.EndLine != tc.wantEndLine {
				t.Errorf("EndLine: got %d, want %d (EndPosition.Row=%d + 1)",
					got.EndLine, tc.wantEndLine, tc.chunk.EndPosition.Row)
			}
		})
	}
}
