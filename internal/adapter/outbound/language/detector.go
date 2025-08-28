package language

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const (
	// DetectionMethodExtension represents extension-based detection method.
	DetectionMethodExtension = "Extension"
	// DetectionMethodShebang represents shebang-based detection method.
	DetectionMethodShebang = "Shebang"
	// DetectionMethodContent represents content-based detection method.
	DetectionMethodContent = "Content"
	// DetectionMethodHeuristic represents heuristic-based detection method.
	DetectionMethodHeuristic = "Heuristic"
)

// Detector implements the outbound.LanguageDetector interface.
// It provides concrete implementation for language detection using various strategies.
type Detector struct {
	tracer          trace.Tracer
	meter           metric.Meter
	extensionMap    map[string]string
	shebangPatterns map[*regexp.Regexp]string
	contentPatterns map[*regexp.Regexp]string
	cache           *DetectionCache
	batchProcessor  *BatchProcessor
}

// NewDetector creates a new Detector instance.
func NewDetector() *Detector {
	detector := &Detector{
		tracer:          otel.Tracer("language-detector"),
		meter:           otel.Meter("language-detector"),
		extensionMap:    buildExtensionMap(),
		shebangPatterns: buildShebangPatterns(),
		contentPatterns: buildContentPatterns(),
		cache:           NewDetectionCache(1000, 5*time.Minute), // Default cache configuration
	}

	// Initialize batch processor with reference to this detector
	detector.batchProcessor = NewBatchProcessor(detector)
	return detector
}

// buildExtensionMap creates the extension to language mapping.
func buildExtensionMap() map[string]string {
	return map[string]string{
		".go":   valueobject.LanguageGo,
		".py":   valueobject.LanguagePython,
		".js":   valueobject.LanguageJavaScript,
		".ts":   valueobject.LanguageTypeScript,
		".java": valueobject.LanguageJava,
		".c":    valueobject.LanguageC,
		".cpp":  valueobject.LanguageCPlusPlus,
		".cxx":  valueobject.LanguageCPlusPlus,
		".cc":   valueobject.LanguageCPlusPlus,
		".rs":   valueobject.LanguageRust,
		".rb":   valueobject.LanguageRuby,
		".php":  valueobject.LanguagePHP,
		".html": valueobject.LanguageHTML,
		".css":  valueobject.LanguageCSS,
		".json": valueobject.LanguageJSON,
		".yaml": valueobject.LanguageYAML,
		".yml":  valueobject.LanguageYAML,
		".xml":  valueobject.LanguageXML,
		".md":   valueobject.LanguageMarkdown,
		".sql":  valueobject.LanguageSQL,
		".sh":   valueobject.LanguageShell,
	}
}

// buildShebangPatterns creates the shebang pattern to language mapping.
func buildShebangPatterns() map[*regexp.Regexp]string {
	return map[*regexp.Regexp]string{
		regexp.MustCompile(`^#!/usr/bin/env python`): valueobject.LanguagePython,
		regexp.MustCompile(`^#!/usr/bin/python`):     valueobject.LanguagePython,
		regexp.MustCompile(`^#!/usr/bin/env node`):   valueobject.LanguageJavaScript,
		regexp.MustCompile(`^#!/usr/bin/env ruby`):   valueobject.LanguageRuby,
		regexp.MustCompile(`^#!/usr/bin/ruby`):       valueobject.LanguageRuby,
		regexp.MustCompile(`^#!/usr/bin/env php`):    valueobject.LanguagePHP,
		regexp.MustCompile(`^#!/usr/bin/php`):        valueobject.LanguagePHP,
		regexp.MustCompile(`^#!/bin/bash`):           valueobject.LanguageShell,
		regexp.MustCompile(`^#!/bin/sh`):             valueobject.LanguageShell,
		regexp.MustCompile(`^#!/usr/bin/env bash`):   valueobject.LanguageShell,
	}
}

// buildContentPatterns creates the content pattern to language mapping.
func buildContentPatterns() map[*regexp.Regexp]string {
	patterns := make(map[*regexp.Regexp]string)

	// Go patterns
	patterns[regexp.MustCompile(`package\s+\w+`)] = valueobject.LanguageGo
	patterns[regexp.MustCompile(`func\s+\w+\s*\(`)] = valueobject.LanguageGo
	patterns[regexp.MustCompile(`import\s*\(\s*".*"\s*\)`)] = valueobject.LanguageGo

	// Python patterns
	patterns[regexp.MustCompile(`def\s+\w+\s*\(`)] = valueobject.LanguagePython
	patterns[regexp.MustCompile(`import\s+\w+`)] = valueobject.LanguagePython
	patterns[regexp.MustCompile(`from\s+\w+\s+import`)] = valueobject.LanguagePython
	patterns[regexp.MustCompile(`if\s+__name__\s*==\s*["']__main__["']`)] = valueobject.LanguagePython

	// JavaScript patterns
	patterns[regexp.MustCompile(`function\s+\w+\s*\(`)] = valueobject.LanguageJavaScript
	patterns[regexp.MustCompile(`const\s+\w+\s*=`)] = valueobject.LanguageJavaScript
	patterns[regexp.MustCompile(`let\s+\w+\s*=`)] = valueobject.LanguageJavaScript
	patterns[regexp.MustCompile(`var\s+\w+\s*=`)] = valueobject.LanguageJavaScript
	patterns[regexp.MustCompile(`=>\s*\{`)] = valueobject.LanguageJavaScript

	// TypeScript patterns
	patterns[regexp.MustCompile(`interface\s+\w+\s*\{`)] = valueobject.LanguageTypeScript
	patterns[regexp.MustCompile(`type\s+\w+\s*=`)] = valueobject.LanguageTypeScript
	patterns[regexp.MustCompile(`:\s*(string|number|boolean)`)] = valueobject.LanguageTypeScript

	// Other language patterns
	addOtherPatterns(patterns)

	return patterns
}

// addOtherPatterns adds remaining language patterns to reduce complexity.
func addOtherPatterns(patterns map[*regexp.Regexp]string) {
	// Java patterns
	patterns[regexp.MustCompile(`public\s+class\s+\w+`)] = valueobject.LanguageJava
	patterns[regexp.MustCompile(`public\s+static\s+void\s+main`)] = valueobject.LanguageJava
	patterns[regexp.MustCompile(`import\s+java\.\w+`)] = valueobject.LanguageJava

	// C patterns
	patterns[regexp.MustCompile(`#include\s*<\w+\.h>`)] = valueobject.LanguageC
	patterns[regexp.MustCompile(`int\s+main\s*\(`)] = valueobject.LanguageC
	patterns[regexp.MustCompile(`printf\s*\(`)] = valueobject.LanguageC

	// C++ patterns
	patterns[regexp.MustCompile(`#include\s*<iostream>`)] = valueobject.LanguageCPlusPlus
	patterns[regexp.MustCompile(`using\s+namespace\s+std`)] = valueobject.LanguageCPlusPlus
	patterns[regexp.MustCompile(`std::`)] = valueobject.LanguageCPlusPlus
	patterns[regexp.MustCompile(`class\s+\w+\s*\{`)] = valueobject.LanguageCPlusPlus

	// Rust patterns
	patterns[regexp.MustCompile(`fn\s+\w+\s*\(`)] = valueobject.LanguageRust
	patterns[regexp.MustCompile(`use\s+\w+::`)] = valueobject.LanguageRust
	patterns[regexp.MustCompile(`let\s+mut\s+\w+`)] = valueobject.LanguageRust

	// Other patterns
	addMarkupPatterns(patterns)
}

// addMarkupPatterns adds markup and data format patterns.
func addMarkupPatterns(patterns map[*regexp.Regexp]string) {
	// Ruby patterns
	patterns[regexp.MustCompile(`def\s+\w+`)] = valueobject.LanguageRuby
	patterns[regexp.MustCompile(`require\s+['"].*['"]`)] = valueobject.LanguageRuby
	patterns[regexp.MustCompile(`class\s+\w+\s*<`)] = valueobject.LanguageRuby

	// PHP patterns
	patterns[regexp.MustCompile(`<\?php`)] = valueobject.LanguagePHP
	patterns[regexp.MustCompile(`function\s+\w+\s*\(`)] = valueobject.LanguagePHP
	patterns[regexp.MustCompile(`\$\w+\s*=`)] = valueobject.LanguagePHP

	// HTML patterns
	patterns[regexp.MustCompile(`<html\b`)] = valueobject.LanguageHTML
	patterns[regexp.MustCompile(`<!DOCTYPE\s+html`)] = valueobject.LanguageHTML
	patterns[regexp.MustCompile(`<div\b`)] = valueobject.LanguageHTML

	// CSS patterns
	patterns[regexp.MustCompile(`\w+\s*\{\s*[\w-]+:`)] = valueobject.LanguageCSS
	patterns[regexp.MustCompile(`@media\s+`)] = valueobject.LanguageCSS

	// JSON patterns
	patterns[regexp.MustCompile(`^\s*\{\s*"\w+"`)] = valueobject.LanguageJSON
	patterns[regexp.MustCompile(`^\s*\[\s*\{`)] = valueobject.LanguageJSON

	// YAML patterns
	patterns[regexp.MustCompile(`^\w+:\s*$`)] = valueobject.LanguageYAML
	patterns[regexp.MustCompile(`^\s*-\s+\w+:`)] = valueobject.LanguageYAML

	// XML patterns
	patterns[regexp.MustCompile(`<\?xml\s+version`)] = valueobject.LanguageXML
	patterns[regexp.MustCompile(`<\w+\s*xmlns`)] = valueobject.LanguageXML

	// Markdown patterns
	patterns[regexp.MustCompile(`^#\s+\w+`)] = valueobject.LanguageMarkdown
	patterns[regexp.MustCompile(`\*\*\w+\*\*`)] = valueobject.LanguageMarkdown

	// SQL patterns
	patterns[regexp.MustCompile(`SELECT\s+.*\s+FROM`)] = valueobject.LanguageSQL
	patterns[regexp.MustCompile(`CREATE\s+TABLE`)] = valueobject.LanguageSQL
}

// DetectFromFilePath detects language based on file path and extension.
func (d *Detector) DetectFromFilePath(ctx context.Context, filePath string) (valueobject.Language, error) {
	ctx, span := d.tracer.Start(ctx, "DetectFromFilePath")
	defer span.End()

	span.SetAttributes(attribute.String("file_path", filePath))

	// Check cache first
	cacheKey := fmt.Sprintf("filepath:%s", filePath)
	if cached, found := d.cache.Get(ctx, cacheKey); found {
		span.SetAttributes(attribute.Bool("cache_hit", true))
		slogger.Debug(ctx, "Language retrieved from cache", slogger.Fields{
			"file_path": filePath,
			"language":  cached.Name(),
		})
		return cached, nil
	}

	ext := strings.ToLower(filepath.Ext(filePath))
	if ext == "" {
		return valueobject.Language{}, &outbound.DetectionError{
			Type:      outbound.ErrorTypeInvalidFile,
			Message:   "file has no extension",
			FilePath:  filePath,
			Timestamp: time.Now(),
		}
	}

	if langName, exists := d.extensionMap[ext]; exists {
		language, err := valueobject.NewLanguage(langName)
		if err != nil {
			return valueobject.Language{}, &outbound.DetectionError{
				Type:      outbound.ErrorTypeInternal,
				Message:   fmt.Sprintf("failed to create language: %v", err),
				FilePath:  filePath,
				Timestamp: time.Now(),
				Cause:     err,
			}
		}

		// Cache the result
		d.cache.Put(ctx, cacheKey, language)
		span.SetAttributes(attribute.Bool("cache_hit", false))

		slogger.Debug(ctx, "Language detected by extension", slogger.Fields{
			"file_path": filePath,
			"extension": ext,
			"language":  langName,
		})

		return language, nil
	}

	return valueobject.Language{}, &outbound.DetectionError{
		Type:      outbound.ErrorTypeUnsupportedFormat,
		Message:   fmt.Sprintf("unsupported file extension: %s", ext),
		FilePath:  filePath,
		Timestamp: time.Now(),
	}
}

// DetectFromContent detects language based on file content analysis.
func (d *Detector) DetectFromContent(ctx context.Context, content []byte, hint string) (valueobject.Language, error) {
	ctx, span := d.tracer.Start(ctx, "DetectFromContent")
	defer span.End()

	span.SetAttributes(
		attribute.Int("content_size", len(content)),
		attribute.String("hint", hint),
	)

	if len(content) == 0 {
		return valueobject.Language{}, &outbound.DetectionError{
			Type:      outbound.ErrorTypeInvalidFile,
			Message:   "content is empty",
			Timestamp: time.Now(),
		}
	}

	// Check if binary content
	if isBinary(content) {
		return valueobject.Language{}, &outbound.DetectionError{
			Type:      outbound.ErrorTypeBinaryFile,
			Message:   "content appears to be binary",
			Timestamp: time.Now(),
		}
	}

	contentStr := string(content)

	// Try shebang detection first
	if lang, err := d.detectFromShebang(ctx, contentStr); err == nil {
		return lang, nil
	}

	// Try content pattern detection
	for pattern, langName := range d.contentPatterns {
		if pattern.MatchString(contentStr) {
			language, err := valueobject.NewLanguage(langName)
			if err != nil {
				continue
			}

			slogger.Debug(ctx, "Language detected by content pattern", slogger.Fields{
				"language": langName,
				"pattern":  pattern.String(),
			})

			return language, nil
		}
	}

	// Try hint-based detection
	if hint != "" {
		if langFromHint, err := d.detectFromHint(hint); err == nil {
			return langFromHint, nil
		}
	}

	// Return Unknown for ambiguous content
	return d.createUnknownLanguage()
}

// detectFromHint attempts detection based on filename hint.
func (d *Detector) detectFromHint(hint string) (valueobject.Language, error) {
	ext := filepath.Ext(hint)
	if ext == "" {
		return valueobject.Language{}, errors.New("no extension in hint")
	}

	langName, exists := d.extensionMap[strings.ToLower(ext)]
	if !exists {
		return valueobject.Language{}, errors.New("unsupported extension in hint")
	}

	return valueobject.NewLanguage(langName)
}

// createUnknownLanguage creates an Unknown language instance.
func (d *Detector) createUnknownLanguage() (valueobject.Language, error) {
	language, err := valueobject.NewLanguage(valueobject.LanguageUnknown)
	if err != nil {
		return valueobject.Language{}, &outbound.DetectionError{
			Type:      outbound.ErrorTypeInternal,
			Message:   fmt.Sprintf("failed to create unknown language: %v", err),
			Timestamp: time.Now(),
			Cause:     err,
		}
	}
	return language, nil
}

// DetectFromReader detects language from a reader interface.
func (d *Detector) DetectFromReader(
	ctx context.Context,
	reader io.Reader,
	filename string,
) (valueobject.Language, error) {
	ctx, span := d.tracer.Start(ctx, "DetectFromReader")
	defer span.End()

	span.SetAttributes(attribute.String("filename", filename))

	content, err := io.ReadAll(reader)
	if err != nil {
		return valueobject.Language{}, &outbound.DetectionError{
			Type:      outbound.ErrorTypeInternal,
			Message:   fmt.Sprintf("failed to read content: %v", err),
			Timestamp: time.Now(),
			Cause:     err,
		}
	}

	// Try extension-based detection first if filename is provided
	if filename != "" {
		if lang, err := d.DetectFromFilePath(ctx, filename); err == nil {
			return lang, nil
		}
	}

	// Fall back to content-based detection
	return d.DetectFromContent(ctx, content, filename)
}

// DetectMultipleLanguages detects all languages present in a file.
func (d *Detector) DetectMultipleLanguages(
	ctx context.Context,
	content []byte,
	filename string,
) ([]valueobject.Language, error) {
	ctx, span := d.tracer.Start(ctx, "DetectMultipleLanguages")
	defer span.End()

	var languages []valueobject.Language

	// Primary language detection
	primary, err := d.DetectFromContent(ctx, content, filename)
	if err == nil && primary.Name() != valueobject.LanguageUnknown {
		languages = append(languages, primary)
	}

	// Check for multi-language content
	if additional := d.detectAdditionalLanguages(content, filename); len(additional) > 0 {
		languages = append(languages, additional...)
	}

	if len(languages) == 0 {
		// Return Unknown if nothing detected
		if unknownLang, err := d.createUnknownLanguage(); err == nil {
			languages = append(languages, unknownLang)
		}
	}

	return languages, nil
}

// detectAdditionalLanguages detects additional languages in multi-language files.
func (d *Detector) detectAdditionalLanguages(content []byte, filename string) []valueobject.Language {
	var languages []valueobject.Language
	contentStr := string(content)

	// HTML with embedded languages
	if d.isHTMLContent(filename, contentStr) {
		languages = d.addHTMLLanguages(languages, contentStr)
	}

	return languages
}

// isHTMLContent checks if content is HTML.
func (d *Detector) isHTMLContent(filename, contentStr string) bool {
	return strings.Contains(filename, ".html") ||
		regexp.MustCompile(`<html\b`).MatchString(contentStr)
}

// addHTMLLanguages adds HTML and embedded languages.
func (d *Detector) addHTMLLanguages(languages []valueobject.Language, contentStr string) []valueobject.Language {
	// Add HTML
	if htmlLang, err := valueobject.NewLanguage(valueobject.LanguageHTML); err == nil {
		if !containsLanguage(languages, htmlLang) {
			languages = append(languages, htmlLang)
		}
	}

	// Add embedded JavaScript
	if regexp.MustCompile(`<script\b`).MatchString(contentStr) {
		if jsLang, err := valueobject.NewLanguage(valueobject.LanguageJavaScript); err == nil {
			if !containsLanguage(languages, jsLang) {
				languages = append(languages, jsLang)
			}
		}
	}

	// Add embedded CSS
	if regexp.MustCompile(`<style\b`).MatchString(contentStr) {
		if cssLang, err := valueobject.NewLanguage(valueobject.LanguageCSS); err == nil {
			if !containsLanguage(languages, cssLang) {
				languages = append(languages, cssLang)
			}
		}
	}

	return languages
}

// DetectBatch performs batch language detection for multiple files efficiently.
func (d *Detector) DetectBatch(ctx context.Context, files []outbound.FileInfo) ([]outbound.DetectionResult, error) {
	ctx, span := d.tracer.Start(ctx, "DetectBatch")
	defer span.End()

	// Filter out directories and symlinks
	filteredFiles := make([]outbound.FileInfo, 0, len(files))
	for _, fileInfo := range files {
		if !fileInfo.IsDirectory && !fileInfo.IsSymlink {
			filteredFiles = append(filteredFiles, fileInfo)
		}
	}

	span.SetAttributes(
		attribute.Int("total_files", len(files)),
		attribute.Int("filtered_files", len(filteredFiles)),
	)

	// Use concurrent batch processor for improved performance
	concurrency := 0 // Use default concurrency
	if len(filteredFiles) > 100 {
		concurrency = d.batchProcessor.maxWorkers
	}

	return d.batchProcessor.ProcessConcurrently(ctx, filteredFiles, concurrency)
}

// createDetectionResult creates a detection result for a single file.
func (d *Detector) createDetectionResult(
	ctx context.Context,
	fileInfo outbound.FileInfo,
	start time.Time,
) outbound.DetectionResult {
	var result outbound.DetectionResult
	result.FileInfo = fileInfo

	language, err := d.DetectFromFilePath(ctx, fileInfo.Path)
	result.DetectionTime = time.Since(start)

	if err != nil {
		result.Error = err
		result.Language = valueobject.Language{}
		result.Confidence = 0.0
		result.Method = DetectionMethodExtension
	} else {
		result.Language = language
		result.Confidence = d.getConfidenceForMethod(DetectionMethodExtension)
		result.Method = DetectionMethodExtension
	}

	return result
}

// Helper functions

// detectFromShebang detects language from shebang lines.
func (d *Detector) detectFromShebang(ctx context.Context, content string) (valueobject.Language, error) {
	if !strings.HasPrefix(content, "#!") {
		return valueobject.Language{}, errors.New("no shebang found")
	}

	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return valueobject.Language{}, errors.New("no shebang found")
	}

	firstLine := lines[0]

	for pattern, langName := range d.shebangPatterns {
		if pattern.MatchString(firstLine) {
			language, err := valueobject.NewLanguage(langName)
			if err != nil {
				continue
			}

			slogger.Debug(ctx, "Language detected by shebang", slogger.Fields{
				"language": langName,
				"shebang":  firstLine,
			})

			return language, nil
		}
	}

	return valueobject.Language{}, fmt.Errorf("unknown shebang: %s", firstLine)
}

// getConfidenceForMethod returns confidence score for detection method.
func (d *Detector) getConfidenceForMethod(method string) float64 {
	switch method {
	case DetectionMethodExtension:
		return 0.95
	case DetectionMethodShebang:
		return 0.90
	case DetectionMethodContent:
		return 0.80
	case DetectionMethodHeuristic:
		return 0.70
	default:
		return 0.50
	}
}

// isBinary checks if content appears to be binary.
func isBinary(content []byte) bool {
	if len(content) == 0 {
		return false
	}

	// Check for null bytes in first 512 bytes (or entire content if smaller)
	checkSize := 512
	if len(content) < checkSize {
		checkSize = len(content)
	}

	for _, b := range content[:checkSize] {
		if b == 0 {
			return true
		}
	}
	return false
}

// containsLanguage checks if a language is already in the list.
func containsLanguage(languages []valueobject.Language, target valueobject.Language) bool {
	for _, lang := range languages {
		if lang.Name() == target.Name() {
			return true
		}
	}
	return false
}
