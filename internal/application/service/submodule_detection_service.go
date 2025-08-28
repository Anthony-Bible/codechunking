package service

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// SubmoduleDetectionService provides submodule detection capabilities for the application layer.
// It acts as a facade over the submodule detection port, providing application-specific
// logic and observability features.
type SubmoduleDetectionService struct {
	detector outbound.SubmoduleDetector
	helper   *observabilityHelper
}

// NewSubmoduleDetectionService creates a new SubmoduleDetectionService instance.
func NewSubmoduleDetectionService(detector outbound.SubmoduleDetector) *SubmoduleDetectionService {
	if detector == nil {
		panic("detector cannot be nil")
	}

	tracer := otel.Tracer("submodule-detection-service")
	meter := otel.Meter("submodule-detection-service")

	// Initialize metrics
	detectionCounter, _ := meter.Int64Counter(
		"submodule_detection_total",
		metric.WithDescription("Total number of submodule detections performed"),
	)

	detectionDuration, _ := meter.Float64Histogram(
		"submodule_detection_duration_seconds",
		metric.WithDescription("Duration of submodule detection operations"),
	)

	detectionErrorCounter, _ := meter.Int64Counter(
		"submodule_detection_errors_total",
		metric.WithDescription("Total number of submodule detection errors"),
	)

	helper := &observabilityHelper{
		tracer:                tracer,
		detectionCounter:      detectionCounter,
		detectionDuration:     detectionDuration,
		detectionErrorCounter: detectionErrorCounter,
	}

	return &SubmoduleDetectionService{
		detector: detector,
		helper:   helper,
	}
}

// DetectSubmodules discovers all submodules in a repository.
func (s *SubmoduleDetectionService) DetectSubmodules(
	ctx context.Context,
	repositoryPath string,
) ([]valueobject.SubmoduleInfo, error) {
	op, cleanup := s.helper.startOperation(ctx, "DetectSubmodules", "submodule_detection",
		attribute.String("repository_path", repositoryPath))
	defer cleanup()

	slogger.Info(op.getContext(), "Detecting submodules in repository", slogger.Fields{
		"repository_path": repositoryPath,
	})

	submodules, err := s.detector.DetectSubmodules(op.getContext(), repositoryPath)
	if err != nil {
		op.finishWithError(err, "Failed to detect submodules", slogger.Fields{
			"repository_path": repositoryPath,
		})
		return nil, err
	}

	op.addAttributes(attribute.Int("submodules_found", len(submodules)))
	op.finishWithSuccess("Successfully detected submodules", slogger.Fields{
		"repository_path":  repositoryPath,
		"submodules_found": len(submodules),
	})

	return submodules, nil
}

// ParseGitmodulesFile parses a .gitmodules file and returns submodule configurations.
func (s *SubmoduleDetectionService) ParseGitmodulesFile(
	ctx context.Context,
	gitmodulesPath string,
) ([]valueobject.SubmoduleInfo, error) {
	op, cleanup := s.helper.startOperation(ctx, "ParseGitmodulesFile", "gitmodules_parsing",
		attribute.String("gitmodules_path", gitmodulesPath))
	defer cleanup()

	slogger.Info(op.getContext(), "Parsing .gitmodules file", slogger.Fields{
		"gitmodules_path": gitmodulesPath,
	})

	submodules, err := s.detector.ParseGitmodulesFile(op.getContext(), gitmodulesPath)
	if err != nil {
		op.finishWithError(err, "Failed to parse .gitmodules file", slogger.Fields{
			"gitmodules_path": gitmodulesPath,
		})
		return nil, err
	}

	op.addAttributes(attribute.Int("submodules_parsed", len(submodules)))
	op.finishWithSuccess("Successfully parsed .gitmodules file", slogger.Fields{
		"gitmodules_path":   gitmodulesPath,
		"submodules_parsed": len(submodules),
	})

	return submodules, nil
}

// IsSubmoduleDirectory determines if a directory is a Git submodule.
func (s *SubmoduleDetectionService) IsSubmoduleDirectory(
	ctx context.Context,
	directoryPath string,
	repositoryRoot string,
) (bool, *valueobject.SubmoduleInfo, error) {
	op, cleanup := s.helper.startOperation(ctx, "IsSubmoduleDirectory", "submodule_check",
		attribute.String("directory_path", directoryPath),
		attribute.String("repository_root", repositoryRoot))
	defer cleanup()

	slogger.Debug(op.getContext(), "Checking if directory is a submodule", slogger.Fields{
		"directory_path":  directoryPath,
		"repository_root": repositoryRoot,
	})

	isSubmodule, submoduleInfo, err := s.detector.IsSubmoduleDirectory(op.getContext(), directoryPath, repositoryRoot)
	if err != nil {
		op.finishWithError(err, "Failed to check submodule directory", slogger.Fields{
			"directory_path":  directoryPath,
			"repository_root": repositoryRoot,
		})
		return false, nil, err
	}

	op.addAttributes(attribute.Bool("is_submodule", isSubmodule))
	op.finishWithSuccess("Successfully checked submodule directory", slogger.Fields{
		"directory_path":  directoryPath,
		"repository_root": repositoryRoot,
		"is_submodule":    isSubmodule,
	})

	return isSubmodule, submoduleInfo, nil
}

// GetSubmoduleStatus retrieves the status of a specific submodule.
func (s *SubmoduleDetectionService) GetSubmoduleStatus(
	ctx context.Context,
	submodulePath string,
	repositoryRoot string,
) (valueobject.SubmoduleStatus, error) {
	op, cleanup := s.helper.startOperation(ctx, "GetSubmoduleStatus", "submodule_status",
		attribute.String("submodule_path", submodulePath),
		attribute.String("repository_root", repositoryRoot))
	defer cleanup()

	slogger.Debug(op.getContext(), "Getting submodule status", slogger.Fields{
		"submodule_path":  submodulePath,
		"repository_root": repositoryRoot,
	})

	status, err := s.detector.GetSubmoduleStatus(op.getContext(), submodulePath, repositoryRoot)
	if err != nil {
		op.finishWithError(err, "Failed to get submodule status", slogger.Fields{
			"submodule_path":  submodulePath,
			"repository_root": repositoryRoot,
		})
		return valueobject.SubmoduleStatusUnknown, err
	}

	op.addAttributes(attribute.String("status", status.String()))
	op.finishWithSuccess("Successfully retrieved submodule status", slogger.Fields{
		"submodule_path":  submodulePath,
		"repository_root": repositoryRoot,
		"status":          status.String(),
	})

	return status, nil
}

// ValidateSubmoduleConfiguration validates a submodule configuration.
func (s *SubmoduleDetectionService) ValidateSubmoduleConfiguration(
	ctx context.Context,
	submodule valueobject.SubmoduleInfo,
) error {
	op, cleanup := s.helper.startOperation(ctx, "ValidateSubmoduleConfiguration", "submodule_validation",
		attribute.String("submodule_path", submodule.Path()),
		attribute.String("submodule_url", submodule.URL()))
	defer cleanup()

	slogger.Debug(op.getContext(), "Validating submodule configuration", slogger.Fields{
		"submodule_path": submodule.Path(),
		"submodule_name": submodule.Name(),
		"submodule_url":  submodule.URL(),
	})

	err := s.detector.ValidateSubmoduleConfiguration(op.getContext(), submodule)
	if err != nil {
		op.finishWithError(err, "Failed to validate submodule configuration", slogger.Fields{
			"submodule_path": submodule.Path(),
			"submodule_name": submodule.Name(),
			"submodule_url":  submodule.URL(),
		})
		return err
	}

	op.finishWithSuccess("Successfully validated submodule configuration", slogger.Fields{
		"submodule_path": submodule.Path(),
		"submodule_name": submodule.Name(),
		"submodule_url":  submodule.URL(),
	})

	return nil
}
