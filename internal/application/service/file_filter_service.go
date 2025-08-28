package service

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/port/outbound"
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// FileFilterService provides file filtering capabilities for the application layer.
// It acts as a facade over the file filter port, providing application-specific
// logic and observability features.
type FileFilterService struct {
	filter outbound.FileFilter
	helper *observabilityHelper
}

// NewFileFilterService creates a new FileFilterService instance.
func NewFileFilterService(filter outbound.FileFilter) *FileFilterService {
	if filter == nil {
		panic("filter cannot be nil")
	}

	tracer := otel.Tracer("file-filter-service")
	meter := otel.Meter("file-filter-service")

	// Initialize metrics
	filterCounter, _ := meter.Int64Counter(
		"file_filter_total",
		metric.WithDescription("Total number of file filtering operations performed"),
	)

	filterDuration, _ := meter.Float64Histogram(
		"file_filter_duration_seconds",
		metric.WithDescription("Duration of file filtering operations"),
	)

	filterErrorCounter, _ := meter.Int64Counter(
		"file_filter_errors_total",
		metric.WithDescription("Total number of file filtering errors"),
	)

	helper := &observabilityHelper{
		tracer:                tracer,
		detectionCounter:      filterCounter,
		detectionDuration:     filterDuration,
		detectionErrorCounter: filterErrorCounter,
	}

	return &FileFilterService{
		filter: filter,
		helper: helper,
	}
}

// ShouldProcessFile determines if a file should be processed based on filtering rules.
func (s *FileFilterService) ShouldProcessFile(
	ctx context.Context,
	filePath string,
	fileInfo outbound.FileInfo,
) (outbound.FilterDecision, error) {
	op, cleanup := s.helper.startOperation(ctx, "file_filter_service.should_process_file", "should_process_file",
		attribute.String("file_path", filePath),
		attribute.Int64("file_size", fileInfo.Size),
	)
	defer cleanup()

	result, err := s.filter.ShouldProcessFile(op.getContext(), filePath, fileInfo)
	if err != nil {
		op.finishWithError(err, "Failed to determine if file should be processed", slogger.Fields{
			"file_path": filePath,
		})
		return outbound.FilterDecision{}, err
	}

	op.finishWithSuccess("File processing decision made", slogger.Fields{
		"file_path":      filePath,
		"should_process": result.ShouldProcess,
		"is_binary":      result.IsBinary,
		"is_git_ignored": result.IsGitIgnored,
	})

	return result, nil
}

// DetectBinaryFile determines if a file is binary based on content analysis.
func (s *FileFilterService) DetectBinaryFile(ctx context.Context, filePath string, content []byte) (bool, error) {
	op, cleanup := s.helper.startOperation(ctx, "file_filter_service.detect_binary_file", "detect_binary_file",
		attribute.String("file_path", filePath),
		attribute.Int("content_size", len(content)),
	)
	defer cleanup()

	result, err := s.filter.DetectBinaryFile(op.getContext(), filePath, content)
	if err != nil {
		op.finishWithError(err, "Failed to detect binary file from content", slogger.Fields{
			"file_path": filePath,
		})
		return false, err
	}

	op.finishWithSuccess("Binary file detection completed", slogger.Fields{
		"file_path": filePath,
		"is_binary": result,
	})

	return result, nil
}

// DetectBinaryFromPath determines if a file is binary based on extension.
func (s *FileFilterService) DetectBinaryFromPath(ctx context.Context, filePath string) (bool, error) {
	op, cleanup := s.helper.startOperation(
		ctx,
		"file_filter_service.detect_binary_from_path",
		"detect_binary_from_path",
		attribute.String("file_path", filePath),
	)
	defer cleanup()

	result, err := s.filter.DetectBinaryFromPath(op.getContext(), filePath)
	if err != nil {
		op.finishWithError(err, "Failed to detect binary file from path", slogger.Fields{
			"file_path": filePath,
		})
		return false, err
	}

	op.finishWithSuccess("Binary file detection from path completed", slogger.Fields{
		"file_path": filePath,
		"is_binary": result,
	})

	return result, nil
}

// MatchesGitignorePatterns checks if a file matches gitignore patterns.
func (s *FileFilterService) MatchesGitignorePatterns(
	ctx context.Context,
	filePath string,
	repoPath string,
) (bool, error) {
	op, cleanup := s.helper.startOperation(
		ctx,
		"file_filter_service.matches_gitignore_patterns",
		"matches_gitignore_patterns",
		attribute.String("file_path", filePath),
		attribute.String("repo_path", repoPath),
	)
	defer cleanup()

	result, err := s.filter.MatchesGitignorePatterns(op.getContext(), filePath, repoPath)
	if err != nil {
		op.finishWithError(err, "Failed to match gitignore patterns", slogger.Fields{
			"file_path": filePath,
			"repo_path": repoPath,
		})
		return false, err
	}

	op.finishWithSuccess("Gitignore pattern matching completed", slogger.Fields{
		"file_path": filePath,
		"repo_path": repoPath,
		"matches":   result,
	})

	return result, nil
}

// LoadGitignorePatterns loads gitignore patterns from repository.
func (s *FileFilterService) LoadGitignorePatterns(
	ctx context.Context,
	repoPath string,
) ([]outbound.GitignorePattern, error) {
	op, cleanup := s.helper.startOperation(
		ctx,
		"file_filter_service.load_gitignore_patterns",
		"load_gitignore_patterns",
		attribute.String("repo_path", repoPath),
	)
	defer cleanup()

	result, err := s.filter.LoadGitignorePatterns(op.getContext(), repoPath)
	if err != nil {
		op.finishWithError(err, "Failed to load gitignore patterns", slogger.Fields{
			"repo_path": repoPath,
		})
		return nil, err
	}

	op.finishWithSuccess("Gitignore patterns loaded successfully", slogger.Fields{
		"repo_path":     repoPath,
		"pattern_count": len(result),
	})

	return result, nil
}

// FilterFilesBatch performs batch filtering for multiple files.
func (s *FileFilterService) FilterFilesBatch(
	ctx context.Context,
	files []outbound.FileInfo,
	repoPath string,
) ([]outbound.FilterResult, error) {
	op, cleanup := s.helper.startOperation(ctx, "file_filter_service.filter_files_batch", "filter_files_batch",
		attribute.Int("file_count", len(files)),
		attribute.String("repo_path", repoPath),
	)
	defer cleanup()

	result, err := s.filter.FilterFilesBatch(op.getContext(), files, repoPath)
	if err != nil {
		op.finishWithError(err, "Failed to filter files in batch", slogger.Fields{
			"file_count": len(files),
			"repo_path":  repoPath,
		})
		return nil, err
	}

	op.finishWithSuccess("Batch file filtering completed", slogger.Fields{
		"file_count":   len(files),
		"repo_path":    repoPath,
		"result_count": len(result),
	})

	return result, nil
}
