package treesitter

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// TreeSitterCodeParser implements outbound.CodeParser using TreeSitter infrastructure.
type TreeSitterCodeParser struct {
	factory *ParserFactoryImpl
}

// NewTreeSitterCodeParser creates a new TreeSitter-based code parser.
func NewTreeSitterCodeParser(ctx context.Context) (*TreeSitterCodeParser, error) {
	factory, err := NewTreeSitterParserFactory(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser factory: %w", err)
	}

	return &TreeSitterCodeParser{
		factory: factory,
	}, nil
}

// ParseDirectory implements outbound.CodeParser interface.
func (p *TreeSitterCodeParser) ParseDirectory(
	ctx context.Context,
	dirPath string,
	config outbound.CodeParsingConfig,
) ([]outbound.CodeChunk, error) {
	slogger.Info(ctx, "Starting directory parsing with TreeSitter", slogger.Fields{
		"directory":        dirPath,
		"chunk_size_bytes": config.ChunkSizeBytes,
		"max_file_size":    config.MaxFileSizeBytes,
		"include_tests":    config.IncludeTests,
		"exclude_vendor":   config.ExcludeVendor,
	})

	var allChunks []outbound.CodeChunk
	var processedFiles int
	var skippedFiles int

	err := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			slogger.Warn(ctx, "Error walking directory", slogger.Fields{
				"path":  path,
				"error": err.Error(),
			})
			return err // Return the error to stop walking
		}

		// Skip directories
		if d.IsDir() {
			if p.shouldSkipDirectory(path, config) {
				slogger.Debug(ctx, "Skipping directory", slogger.Fields{"path": path})
				return filepath.SkipDir
			}
			return nil
		}

		// Process files
		if p.shouldProcessFile(path, d, config) {
			chunks, err := p.parseFile(ctx, path, config)
			if err != nil {
				slogger.Warn(ctx, "Failed to parse file", slogger.Fields{
					"file":  path,
					"error": err.Error(),
				})
				skippedFiles++
			} else {
				allChunks = append(allChunks, chunks...)
				processedFiles++
			}
		} else {
			skippedFiles++
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk directory %s: %w", dirPath, err)
	}

	slogger.Info(ctx, "Directory parsing completed", slogger.Fields{
		"total_chunks":    len(allChunks),
		"processed_files": processedFiles,
		"skipped_files":   skippedFiles,
		"directory":       dirPath,
	})

	return allChunks, nil
}

// parseFile parses a single file and returns code chunks.
func (p *TreeSitterCodeParser) parseFile(
	ctx context.Context,
	filePath string,
	config outbound.CodeParsingConfig,
) ([]outbound.CodeChunk, error) {
	// Check file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	if config.MaxFileSizeBytes > 0 && fileInfo.Size() > config.MaxFileSizeBytes {
		return nil, fmt.Errorf("file %s too large (%d bytes)", filePath, fileInfo.Size())
	}

	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	// Detect language
	language := p.detectLanguage(filePath)
	if language == nil {
		slogger.Debug(ctx, "Unsupported language, creating simple chunks", slogger.Fields{
			"file": filePath,
		})
		// For unsupported languages, create simple text chunks
		return p.createSimpleChunks(filePath, string(content), "text", config), nil
	}

	// Check if factory supports this language
	if !p.factory.IsLanguageSupported(ctx, *language) {
		slogger.Debug(ctx, "Language not supported by factory, creating simple chunks", slogger.Fields{
			"file":     filePath,
			"language": language.Name(),
		})
		// Fallback to simple text chunks
		return p.createSimpleChunks(filePath, string(content), language.Name(), config), nil
	}

	// Try to use TreeSitter parsing
	if chunks, err := p.parseWithTreeSitter(ctx, filePath, string(content), *language, config); err == nil {
		slogger.Debug(ctx, "File parsed successfully with TreeSitter", slogger.Fields{
			"file":      filePath,
			"language":  language.Name(),
			"chunks":    len(chunks),
			"file_size": fileInfo.Size(),
		})
		return chunks, nil
	} else {
		slogger.Warn(ctx, "TreeSitter parsing failed, falling back to simple chunks", slogger.Fields{
			"file":     filePath,
			"language": language.Name(),
			"error":    err.Error(),
		})
		// Fallback to simple text chunks - this is not an error, it's expected behavior
		return p.createSimpleChunks(filePath, string(content), language.Name(), config), nil
	}
}

// parseWithTreeSitter attempts to parse using TreeSitter and create semantic chunks.
func (p *TreeSitterCodeParser) parseWithTreeSitter(
	ctx context.Context,
	filePath, content string,
	language valueobject.Language,
	config outbound.CodeParsingConfig,
) ([]outbound.CodeChunk, error) {
	// Create parser for the language
	parser, err := p.factory.CreateParser(ctx, language)
	if err != nil {
		return nil, fmt.Errorf("failed to create parser for %s: %w", language.Name(), err)
	}

	// Parse the content
	parseTree, err := parser.Parse(ctx, []byte(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse file %s: %w", filePath, err)
	}

	// Convert to domain parse tree
	converter := NewTreeSitterParseTreeConverter()
	domainTree, err := converter.ConvertToDomain(parseTree)
	if err != nil {
		return nil, fmt.Errorf("failed to convert parse tree for %s: %w", filePath, err)
	}

	// Try to extract semantic chunks
	extractor := NewSemanticCodeChunkExtractor()
	semanticChunks, err := extractor.Extract(ctx, domainTree)
	if err != nil {
		return nil, fmt.Errorf("failed to extract semantic chunks: %w", err)
	}

	// Convert semantic chunks to simple code chunks
	codeChunks := p.convertSemanticToCodeChunks(filePath, semanticChunks, language.Name())

	// If we got semantic chunks, use them; otherwise fall back to simple chunking
	if len(codeChunks) > 0 {
		return codeChunks, nil
	}

	// Fallback to simple chunking
	return p.createSimpleChunks(filePath, content, language.Name(), config), nil
}

// convertSemanticToCodeChunks converts SemanticCodeChunk to CodeChunk.
func (p *TreeSitterCodeParser) convertSemanticToCodeChunks(
	filePath string,
	semanticChunks []outbound.SemanticCodeChunk,
	languageName string,
) []outbound.CodeChunk {
	var codeChunks []outbound.CodeChunk

	for i, semanticChunk := range semanticChunks {
		// Estimate line numbers from byte positions (approximation)
		startLine := p.estimateLineNumber(semanticChunk.Content, int(semanticChunk.StartByte))
		endLine := p.estimateLineNumber(semanticChunk.Content, int(semanticChunk.EndByte))

		// Generate hash for the chunk
		hash := sha256.Sum256([]byte(semanticChunk.Content))
		hashStr := hex.EncodeToString(hash[:])

		// Create code chunk
		chunk := outbound.CodeChunk{
			ID:        fmt.Sprintf("%s:semantic:%d:%s", filePath, i, hashStr[:8]),
			FilePath:  filePath,
			StartLine: startLine,
			EndLine:   endLine,
			Content:   semanticChunk.Content,
			Language:  languageName,
			Size:      len(semanticChunk.Content),
			Hash:      hashStr,
			CreatedAt: time.Now(),
		}

		codeChunks = append(codeChunks, chunk)
	}

	return codeChunks
}

// createSimpleChunks creates simple text-based chunks when TreeSitter parsing isn't available.
func (p *TreeSitterCodeParser) createSimpleChunks(
	filePath, content, languageName string,
	config outbound.CodeParsingConfig,
) []outbound.CodeChunk {
	var chunks []outbound.CodeChunk

	// If the entire file is smaller than chunk size, return it as one chunk
	if len(content) <= config.ChunkSizeBytes {
		chunk := p.createChunk(filePath, content, 1, strings.Count(content, "\n")+1, languageName)
		chunks = append(chunks, chunk)
		return chunks
	}

	// Split content into chunks based on size
	lines := strings.Split(content, "\n")
	var currentChunk strings.Builder
	startLine := 1
	currentLine := 1

	for i, line := range lines {
		lineWithNewline := line + "\n"

		// If adding this line would exceed chunk size, finalize current chunk
		if currentChunk.Len()+len(lineWithNewline) > config.ChunkSizeBytes && currentChunk.Len() > 0 {
			chunk := p.createChunk(filePath, currentChunk.String(), startLine, currentLine-1, languageName)
			chunks = append(chunks, chunk)

			// Start new chunk
			currentChunk.Reset()
			startLine = currentLine
		}

		currentChunk.WriteString(lineWithNewline)
		currentLine++

		// Handle last line
		if i == len(lines)-1 && currentChunk.Len() > 0 {
			chunk := p.createChunk(filePath, currentChunk.String(), startLine, currentLine-1, languageName)
			chunks = append(chunks, chunk)
		}
	}

	return chunks
}

// createChunk creates a single code chunk.
func (p *TreeSitterCodeParser) createChunk(
	filePath, content string,
	startLine, endLine int,
	languageName string,
) outbound.CodeChunk {
	// Generate hash for the chunk
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	// Generate unique ID
	id := fmt.Sprintf("%s:%d-%d:%s", filePath, startLine, endLine, hashStr[:8])

	return outbound.CodeChunk{
		ID:        id,
		FilePath:  filePath,
		StartLine: startLine,
		EndLine:   endLine,
		Content:   content,
		Language:  languageName,
		Size:      len(content),
		Hash:      hashStr,
		CreatedAt: time.Now(),
	}
}

// estimateLineNumber estimates line number from byte position (simple approximation).
func (p *TreeSitterCodeParser) estimateLineNumber(content string, bytePos int) int {
	if bytePos <= 0 {
		return 1
	}
	if bytePos >= len(content) {
		return strings.Count(content, "\n") + 1
	}
	return strings.Count(content[:bytePos], "\n") + 1
}

// shouldProcessFile determines if a file should be processed.
func (p *TreeSitterCodeParser) shouldProcessFile(
	path string,
	d fs.DirEntry,
	config outbound.CodeParsingConfig,
) bool {
	// Skip if not a regular file
	if !d.Type().IsRegular() {
		return false
	}

	// Check file filters if specified
	if len(config.FileFilters) > 0 {
		filename := filepath.Base(path)
		for _, pattern := range config.FileFilters {
			if matched, _ := filepath.Match(pattern, filename); matched {
				return true
			}
		}
		return false
	}

	// Check if it's a code file we might want to handle
	ext := strings.ToLower(filepath.Ext(path))
	codeExtensions := []string{
		".go", ".py", ".js", ".ts", ".java", ".c", ".cpp", ".rs", ".rb", ".php",
		".scala", ".kt", ".swift", ".dart", ".cs", ".fs", ".ml", ".hs", ".pl",
		".sh", ".bash", ".zsh", ".fish", ".ps1", ".sql", ".html", ".css",
		".json", ".yaml", ".yml", ".xml", ".toml", ".ini", ".cfg", ".conf",
	}

	for _, codeExt := range codeExtensions {
		if ext == codeExt {
			// Skip test files if not included
			if !config.IncludeTests && p.isTestFile(path) {
				return false
			}
			return true
		}
	}

	return false
}

// shouldSkipDirectory determines if a directory should be skipped.
func (p *TreeSitterCodeParser) shouldSkipDirectory(path string, config outbound.CodeParsingConfig) bool {
	dirName := filepath.Base(path)

	// Skip vendor directories if excluded
	if config.ExcludeVendor {
		if dirName == "vendor" || dirName == "node_modules" {
			return true
		}
	}

	// Skip common non-code directories
	skipDirs := []string{
		".git", ".svn", ".hg", // Version control
		"node_modules", "vendor", // Dependencies
		"build", "dist", "target", "bin", "out", // Build outputs
		".idea", ".vscode", ".vs", // IDE files
		"__pycache__", ".pytest_cache", // Python cache
		"*.egg-info", "venv", ".env", // Python virtual environments
		"coverage", ".coverage", // Coverage reports
		"logs", "log", "tmp", "temp", // Temporary/log files
	}

	for _, skipDir := range skipDirs {
		if dirName == skipDir {
			return true
		}
	}

	return false
}

// detectLanguage detects the programming language from file path.
func (p *TreeSitterCodeParser) detectLanguage(path string) *valueobject.Language {
	ext := strings.ToLower(filepath.Ext(path))

	var langStr string
	switch ext {
	case ".go":
		langStr = valueobject.LanguageGo
	case ".py":
		langStr = valueobject.LanguagePython
	case ".js":
		langStr = valueobject.LanguageJavaScript
	case ".ts":
		langStr = valueobject.LanguageTypeScript
	case ".cpp", ".cc", ".cxx", ".c++":
		langStr = valueobject.LanguageCPlusPlus
	case ".rs":
		langStr = valueobject.LanguageRust
	case ".java":
		langStr = "java"
	case ".c":
		langStr = "c"
	default:
		return nil
	}

	language, err := valueobject.NewLanguage(langStr)
	if err != nil {
		return nil
	}

	return &language
}

// isTestFile determines if a file is a test file.
func (p *TreeSitterCodeParser) isTestFile(path string) bool {
	filename := strings.ToLower(filepath.Base(path))

	// Common test file patterns
	testPatterns := []string{
		"_test.", "test_", ".test.",
		"_spec.", "spec_", ".spec.",
		"_tests.", "tests_", ".tests.",
		"test.", "spec.", "tests.",
	}

	for _, pattern := range testPatterns {
		if strings.Contains(filename, pattern) {
			return true
		}
	}

	// Check directory names
	dir := strings.ToLower(filepath.Base(filepath.Dir(path)))
	testDirs := []string{"test", "tests", "spec", "specs", "__tests__"}
	for _, testDir := range testDirs {
		if dir == testDir {
			return true
		}
	}

	return false
}
