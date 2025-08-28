package filefilter

import (
	"bufio"
	"codechunking/internal/port/outbound"
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GitignoreParser handles parsing of .gitignore files.
type GitignoreParser struct{}

// NewGitignoreParser creates a new gitignore parser instance.
func NewGitignoreParser() *GitignoreParser {
	return &GitignoreParser{}
}

// LoadPatterns loads gitignore patterns from a repository directory.
func (p *GitignoreParser) LoadPatterns(_ context.Context, repoPath string) ([]outbound.GitignorePattern, error) {
	var allPatterns []outbound.GitignorePattern

	// Look for .gitignore in the repository root
	gitignorePath := filepath.Join(repoPath, ".gitignore")
	patterns, err := p.parseGitignoreFile(gitignorePath)
	if err != nil {
		// If no .gitignore file exists, return empty patterns (not an error)
		if os.IsNotExist(err) {
			return allPatterns, nil
		}
		return nil, err
	}

	allPatterns = append(allPatterns, patterns...)

	return allPatterns, nil
}

// parseGitignoreFile parses a single .gitignore file and returns patterns.
func (p *GitignoreParser) parseGitignoreFile(filePath string) ([]outbound.GitignorePattern, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var patterns []outbound.GitignorePattern
	scanner := bufio.NewScanner(file)
	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse the pattern
		pattern := p.parsePattern(line, filePath, lineNumber)
		patterns = append(patterns, pattern)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return patterns, nil
}

// parsePattern parses a single gitignore pattern line.
func (p *GitignoreParser) parsePattern(line, sourceFile string, lineNumber int) outbound.GitignorePattern {
	pattern := outbound.GitignorePattern{
		SourceFile: sourceFile,
		LineNumber: lineNumber,
		ParsedAt:   time.Now(),
	}

	// Check for negation pattern
	if strings.HasPrefix(line, "!") {
		pattern.IsNegation = true
		pattern.Pattern = line[1:] // Remove the !
	} else {
		pattern.Pattern = line
	}

	// Check for directory pattern
	if strings.HasSuffix(pattern.Pattern, "/") {
		pattern.IsDirectory = true
	}

	return pattern
}
