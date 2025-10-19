package filefilter

import (
	"codechunking/internal/port/outbound"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CompiledPattern represents a compiled gitignore pattern for efficient matching.
type CompiledPattern struct {
	Original    string
	Regex       *regexp.Regexp
	IsNegation  bool
	IsDirectory bool
	IsRootLevel bool
	Priority    int // Higher priority patterns are evaluated first
}

// CompiledPatternSet holds a set of compiled patterns with thread-safe access.
type CompiledPatternSet struct {
	Patterns []CompiledPattern
	CacheKey string
	LoadedAt time.Time
}

// Filter implements the outbound.FileFilter interface.
type Filter struct {
	binaryExtensions map[string]bool
	gitignoreParser  *GitignoreParser
	gitignoreMatcher *GitignoreMatcher
	// Pattern cache for performance optimization
	patternCache map[string]*CompiledPatternSet
	cacheMutex   sync.RWMutex
}

// NewFilter creates a new file filter instance.
func NewFilter() *Filter {
	// Get default binary extensions and convert to map for O(1) lookup
	extensions := outbound.GetDefaultBinaryExtensions()
	extMap := make(map[string]bool, len(extensions))
	for _, ext := range extensions {
		extMap[strings.ToLower(ext)] = true
	}

	return &Filter{
		binaryExtensions: extMap,
		gitignoreParser:  NewGitignoreParser(),
		gitignoreMatcher: NewGitignoreMatcher(),
		patternCache:     make(map[string]*CompiledPatternSet),
		cacheMutex:       sync.RWMutex{},
	}
}

// ShouldProcessFile determines if a file should be processed based on filtering rules.
func (f *Filter) ShouldProcessFile(
	ctx context.Context,
	filePath string,
	_ outbound.FileInfo,
) (outbound.FilterDecision, error) {
	start := time.Now()

	decision := outbound.FilterDecision{
		ProcessingTime: time.Since(start),
		Confidence:     1.0,
		Metadata:       make(map[string]interface{}),
	}

	// Check if binary by extension first (fastest)
	isBinary, err := f.DetectBinaryFromPath(ctx, filePath)
	if err != nil {
		return decision, err
	}

	decision.IsBinary = isBinary
	if isBinary {
		decision.ShouldProcess = false
		decision.FilterReason = "Binary file detected by extension"
		decision.DetectionMethod = "extension"
		return decision, nil
	}

	// Check gitignore patterns
	// For the minimal implementation, we'll assume repoPath is the directory containing the file
	repoPath := filepath.Dir(filePath)
	if filepath.Base(repoPath) == "." || repoPath == filePath {
		// If we can't determine repo path, use current directory
		repoPath = "."
	}

	isIgnored, err := f.MatchesGitignorePatterns(ctx, filePath, repoPath)
	if err != nil {
		// Don't fail on gitignore errors, just log and continue
		isIgnored = false
	}

	decision.IsGitIgnored = isIgnored
	if isIgnored {
		decision.ShouldProcess = false
		decision.FilterReason = "File matches gitignore patterns"
		decision.DetectionMethod = "gitignore"
	} else {
		decision.ShouldProcess = true
		decision.DetectionMethod = "combined"
	}

	decision.ProcessingTime = time.Since(start)
	return decision, nil
}

// DetectBinaryFile determines if a file is binary based on content analysis.
func (f *Filter) DetectBinaryFile(ctx context.Context, filePath string, content []byte) (bool, error) {
	// First check by extension
	if isBinary, err := f.DetectBinaryFromPath(ctx, filePath); err == nil && isBinary {
		return true, nil
	}

	// Check magic bytes
	if f.hasBinaryMagicBytes(content) {
		return true, nil
	}

	// Heuristic content analysis
	return f.isBinaryByHeuristics(content), nil
}

// hasBinaryMagicBytes checks for known binary file signatures.
func (f *Filter) hasBinaryMagicBytes(content []byte) bool {
	if len(content) < 4 {
		return false
	}

	// PNG signature
	if f.matchesPNGSignature(content) {
		return true
	}

	// JPEG signature
	if f.matchesJPEGSignature(content) {
		return true
	}

	// PDF signature
	if f.matchesPDFSignature(content) {
		return true
	}

	// ZIP signature
	if f.matchesZIPSignature(content) {
		return true
	}

	// ELF signature
	if f.matchesELFSignature(content) {
		return true
	}

	// Windows PE signature (MZ)
	if f.matchesPESignature(content) {
		return true
	}

	return false
}

// matchesPNGSignature checks for PNG file signature.
func (f *Filter) matchesPNGSignature(content []byte) bool {
	return len(content) >= 8 &&
		content[0] == 0x89 && content[1] == 0x50 && content[2] == 0x4E && content[3] == 0x47 &&
		content[4] == 0x0D && content[5] == 0x0A && content[6] == 0x1A && content[7] == 0x0A
}

// matchesJPEGSignature checks for JPEG file signature.
func (f *Filter) matchesJPEGSignature(content []byte) bool {
	return len(content) >= 3 && content[0] == 0xFF && content[1] == 0xD8 && content[2] == 0xFF
}

// matchesPDFSignature checks for PDF file signature.
func (f *Filter) matchesPDFSignature(content []byte) bool {
	return len(content) >= 5 && content[0] == 0x25 && content[1] == 0x50 &&
		content[2] == 0x44 && content[3] == 0x46 && content[4] == 0x2D
}

// matchesZIPSignature checks for ZIP file signature.
func (f *Filter) matchesZIPSignature(content []byte) bool {
	return len(content) >= 4 && content[0] == 0x50 && content[1] == 0x4B &&
		content[2] == 0x03 && content[3] == 0x04
}

// matchesELFSignature checks for ELF file signature.
func (f *Filter) matchesELFSignature(content []byte) bool {
	return len(content) >= 4 && content[0] == 0x7F && content[1] == 0x45 &&
		content[2] == 0x4C && content[3] == 0x46
}

// matchesPESignature checks for Windows PE file signature.
func (f *Filter) matchesPESignature(content []byte) bool {
	return len(content) >= 2 && content[0] == 0x4D && content[1] == 0x5A
}

// isBinaryByHeuristics performs heuristic analysis to determine if content is binary.
func (f *Filter) isBinaryByHeuristics(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// First check if content is valid UTF-8 - if so, it's likely text
	if f.isValidUTF8(content) {
		// Even valid UTF-8 can be binary if it has too many null bytes
		nullBytes := 0
		sampleSize := len(content)
		if sampleSize > 8192 {
			sampleSize = 8192
		}
		for i := range sampleSize {
			if content[i] == 0 {
				nullBytes++
			}
		}
		nullRatio := float64(nullBytes) / float64(sampleSize)
		return nullRatio > 0.3
	}

	// Not valid UTF-8, so use byte-level heuristics
	nullBytes := 0
	nonPrintable := 0
	sampleSize := len(content)
	if sampleSize > 8192 { // Limit sample size for performance
		sampleSize = 8192
	}

	for i := range sampleSize {
		b := content[i]
		if b == 0 {
			nullBytes++
		} else if (b < 32 && b != '\t' && b != '\n' && b != '\r') || b > 127 {
			nonPrintable++
		}
	}

	// Consider binary if >30% null bytes or >20% non-printable
	nullRatio := float64(nullBytes) / float64(sampleSize)
	nonPrintableRatio := float64(nonPrintable) / float64(sampleSize)

	return nullRatio > 0.3 || nonPrintableRatio > 0.2
}

// isValidUTF8 checks if the content is valid UTF-8 text.
// This helps distinguish UTF-8 text (with multibyte characters) from binary data.
func (f *Filter) isValidUTF8(content []byte) bool {
	// Check if at least 95% of the content is valid UTF-8
	// This allows for some corruption while still detecting valid text
	invalidBytes := 0
	i := 0
	for i < len(content) {
		r, size := f.decodeUTF8Rune(content[i:])
		if r == 0xFFFD && size == 1 { // Invalid UTF-8 sequence
			invalidBytes++
		}
		if size == 0 {
			break
		}
		i += size
	}

	invalidRatio := float64(invalidBytes) / float64(len(content))
	return invalidRatio < 0.05 // Less than 5% invalid bytes
}

// decodeUTF8Rune decodes a single UTF-8 rune from the input.
// Returns the rune and the number of bytes consumed.
func (f *Filter) decodeUTF8Rune(b []byte) (rune, int) {
	if len(b) == 0 {
		return 0, 0
	}

	// Single-byte ASCII (0xxxxxxx)
	if b[0] < 0x80 {
		return rune(b[0]), 1
	}

	// Two-byte sequence (110xxxxx 10xxxxxx)
	if len(b) >= 2 && b[0]&0xE0 == 0xC0 && b[1]&0xC0 == 0x80 {
		return rune((b[0]&0x1F)<<6 | (b[1] & 0x3F)), 2
	}

	// Three-byte sequence (1110xxxx 10xxxxxx 10xxxxxx)
	if len(b) >= 3 && b[0]&0xF0 == 0xE0 && b[1]&0xC0 == 0x80 && b[2]&0xC0 == 0x80 {
		return rune(uint32(b[0]&0x0F)<<12 | uint32(b[1]&0x3F)<<6 | uint32(b[2]&0x3F)), 3
	}

	// Four-byte sequence (11110xxx 10xxxxxx 10xxxxxx 10xxxxxx)
	if len(b) >= 4 && b[0]&0xF8 == 0xF0 && b[1]&0xC0 == 0x80 && b[2]&0xC0 == 0x80 && b[3]&0xC0 == 0x80 {
		return rune(uint32(b[0]&0x07)<<18 | uint32(b[1]&0x3F)<<12 | uint32(b[2]&0x3F)<<6 | uint32(b[3]&0x3F)), 4
	}

	// Invalid sequence
	return 0xFFFD, 1
}

// DetectBinaryFromPath determines if a file is binary based on extension only.
func (f *Filter) DetectBinaryFromPath(_ context.Context, filePath string) (bool, error) {
	if filePath == "" {
		return false, nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return false, nil
	}

	return f.binaryExtensions[ext], nil
}

// MatchesGitignorePatterns checks if a file matches any gitignore patterns.
func (f *Filter) MatchesGitignorePatterns(ctx context.Context, filePath string, repoPath string) (bool, error) {
	patterns, err := f.LoadGitignorePatterns(ctx, repoPath)
	if err != nil {
		// If we can't load patterns, assume not ignored but return the error for logging
		return false, err
	}

	// Normalize path for matching
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")
	normalizedPath = strings.TrimPrefix(normalizedPath, "./")

	// Track if any pattern matches (proper implementation with negation support)
	matched := false
	for _, pattern := range patterns {
		patternMatches := f.matchPattern(pattern.Pattern, normalizedPath)
		if patternMatches {
			if pattern.IsNegation {
				// Negation pattern - unmark as matched
				matched = false
			} else {
				// Regular pattern - mark as matched
				matched = true
			}
		}
	}

	return matched, nil
}

// LoadGitignorePatterns loads gitignore patterns from repository.
func (f *Filter) LoadGitignorePatterns(ctx context.Context, repoPath string) ([]outbound.GitignorePattern, error) {
	return f.gitignoreParser.LoadPatterns(ctx, repoPath)
}

// FilterFilesBatch performs batch file filtering for multiple files efficiently.
func (f *Filter) FilterFilesBatch(
	ctx context.Context,
	files []outbound.FileInfo,
	repoPath string,
) ([]outbound.FilterResult, error) {
	results := make([]outbound.FilterResult, 0, len(files))

	// Load gitignore patterns once for the whole batch - errors are handled per-file
	_, _ = f.LoadGitignorePatterns(context.Background(), repoPath)

	for _, file := range files {
		start := time.Now()

		// Build decision manually to use the correct repoPath
		decision := outbound.FilterDecision{
			ProcessingTime: 0,
			Confidence:     1.0,
			Metadata:       make(map[string]interface{}),
		}

		// Check if binary by extension first (fastest)
		isBinary, err := f.DetectBinaryFromPath(ctx, file.Path)
		if err != nil {
			result := outbound.FilterResult{
				FileInfo:       file,
				Decision:       decision,
				Error:          err,
				ProcessingTime: time.Since(start),
			}
			results = append(results, result)
			continue
		}

		decision.IsBinary = isBinary
		if isBinary {
			decision.ShouldProcess = false
			decision.FilterReason = "Binary file detected by extension"
			decision.DetectionMethod = "extension"
			decision.ProcessingTime = time.Since(start)

			result := outbound.FilterResult{
				FileInfo:       file,
				Decision:       decision,
				Error:          nil,
				ProcessingTime: time.Since(start),
			}
			results = append(results, result)
			continue
		}

		// Check gitignore patterns with the correct repoPath
		isIgnored, err := f.MatchesGitignorePatterns(ctx, file.Path, repoPath)
		if err != nil {
			// Don't fail on gitignore errors, just log and continue
			isIgnored = false
		}

		decision.IsGitIgnored = isIgnored
		if isIgnored {
			decision.ShouldProcess = false
			decision.FilterReason = "File matches gitignore patterns"
			decision.DetectionMethod = "gitignore"
		} else {
			decision.ShouldProcess = true
			decision.DetectionMethod = "combined"
		}

		decision.ProcessingTime = time.Since(start)

		result := outbound.FilterResult{
			FileInfo:       file,
			Decision:       decision,
			Error:          nil,
			ProcessingTime: time.Since(start),
		}

		results = append(results, result)
	}

	return results, nil
}

// matchPattern performs advanced pattern matching for gitignore patterns using the GitignoreMatcher.
func (f *Filter) matchPattern(pattern, path string) bool {
	return f.gitignoreMatcher.MatchPattern(pattern, path)
}
