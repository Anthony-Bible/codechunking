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

// SymlinkResolutionService provides symlink resolution capabilities for the application layer.
// It acts as a facade over the symlink resolution port, providing application-specific
// logic and observability features.
type SymlinkResolutionService struct {
	resolver outbound.SymlinkResolver
	helper   *observabilityHelper
}

// NewSymlinkResolutionService creates a new SymlinkResolutionService instance.
func NewSymlinkResolutionService(resolver outbound.SymlinkResolver) *SymlinkResolutionService {
	if resolver == nil {
		panic("resolver cannot be nil")
	}

	tracer := otel.Tracer("symlink-resolution-service")
	meter := otel.Meter("symlink-resolution-service")

	// Initialize metrics
	detectionCounter, _ := meter.Int64Counter(
		"symlink_resolution_total",
		metric.WithDescription("Total number of symlink resolutions performed"),
	)

	detectionDuration, _ := meter.Float64Histogram(
		"symlink_resolution_duration_seconds",
		metric.WithDescription("Duration of symlink resolution operations"),
	)

	detectionErrorCounter, _ := meter.Int64Counter(
		"symlink_resolution_errors_total",
		metric.WithDescription("Total number of symlink resolution errors"),
	)

	helper := &observabilityHelper{
		tracer:                tracer,
		detectionCounter:      detectionCounter,
		detectionDuration:     detectionDuration,
		detectionErrorCounter: detectionErrorCounter,
	}

	return &SymlinkResolutionService{
		resolver: resolver,
		helper:   helper,
	}
}

// DetectSymlinks discovers all symbolic links in a directory tree.
func (s *SymlinkResolutionService) DetectSymlinks(
	ctx context.Context,
	directoryPath string,
) ([]valueobject.SymlinkInfo, error) {
	op, cleanup := s.helper.startOperation(ctx, "DetectSymlinks", "symlink_detection",
		attribute.String("directory_path", directoryPath))
	defer cleanup()

	slogger.Info(op.getContext(), "Detecting symlinks in directory", slogger.Fields{
		"directory_path": directoryPath,
	})

	symlinks, err := s.resolver.DetectSymlinks(op.getContext(), directoryPath)
	if err != nil {
		op.finishWithError(err, "Failed to detect symlinks", slogger.Fields{
			"directory_path": directoryPath,
		})
		return nil, err
	}

	op.addAttributes(attribute.Int("symlinks_found", len(symlinks)))
	op.finishWithSuccess("Successfully detected symlinks", slogger.Fields{
		"directory_path": directoryPath,
		"symlinks_found": len(symlinks),
	})

	return symlinks, nil
}

// IsSymlink determines if a given path is a symbolic link.
func (s *SymlinkResolutionService) IsSymlink(
	ctx context.Context,
	filePath string,
) (bool, error) {
	op, cleanup := s.helper.startOperation(ctx, "IsSymlink", "symlink_check",
		attribute.String("file_path", filePath))
	defer cleanup()

	slogger.Debug(op.getContext(), "Checking if path is a symlink", slogger.Fields{
		"file_path": filePath,
	})

	isSymlink, err := s.resolver.IsSymlink(op.getContext(), filePath)
	if err != nil {
		op.finishWithError(err, "Failed to check symlink", slogger.Fields{
			"file_path": filePath,
		})
		return false, err
	}

	op.addAttributes(attribute.Bool("is_symlink", isSymlink))
	op.finishWithSuccess("Successfully checked symlink", slogger.Fields{
		"file_path":  filePath,
		"is_symlink": isSymlink,
	})

	return isSymlink, nil
}

// ResolveSymlink resolves a symbolic link to its target path.
func (s *SymlinkResolutionService) ResolveSymlink(
	ctx context.Context,
	symlinkPath string,
) (*valueobject.SymlinkInfo, error) {
	op, cleanup := s.helper.startOperation(ctx, "ResolveSymlink", "symlink_resolution",
		attribute.String("symlink_path", symlinkPath))
	defer cleanup()

	slogger.Debug(op.getContext(), "Resolving symlink", slogger.Fields{
		"symlink_path": symlinkPath,
	})

	symlinkInfo, err := s.resolver.ResolveSymlink(op.getContext(), symlinkPath)
	if err != nil {
		op.finishWithError(err, "Failed to resolve symlink", slogger.Fields{
			"symlink_path": symlinkPath,
		})
		return nil, err
	}

	if symlinkInfo != nil {
		op.addAttributes(attribute.String("target_path", symlinkInfo.TargetPath()))
	}
	op.finishWithSuccess("Successfully resolved symlink", slogger.Fields{
		"symlink_path": symlinkPath,
		"target_path":  symlinkInfo.TargetPath(),
	})

	return symlinkInfo, nil
}

// GetSymlinkTarget retrieves the target path of a symbolic link.
func (s *SymlinkResolutionService) GetSymlinkTarget(
	ctx context.Context,
	symlinkPath string,
) (string, error) {
	op, cleanup := s.helper.startOperation(ctx, "GetSymlinkTarget", "symlink_target",
		attribute.String("symlink_path", symlinkPath))
	defer cleanup()

	slogger.Debug(op.getContext(), "Getting symlink target", slogger.Fields{
		"symlink_path": symlinkPath,
	})

	targetPath, err := s.resolver.GetSymlinkTarget(op.getContext(), symlinkPath)
	if err != nil {
		op.finishWithError(err, "Failed to get symlink target", slogger.Fields{
			"symlink_path": symlinkPath,
		})
		return "", err
	}

	op.addAttributes(attribute.String("target_path", targetPath))
	op.finishWithSuccess("Successfully retrieved symlink target", slogger.Fields{
		"symlink_path": symlinkPath,
		"target_path":  targetPath,
	})

	return targetPath, nil
}

// ValidateSymlinkTarget checks if a symlink target exists and is accessible.
func (s *SymlinkResolutionService) ValidateSymlinkTarget(
	ctx context.Context,
	symlink valueobject.SymlinkInfo,
) (*outbound.SymlinkValidationResult, error) {
	op, cleanup := s.helper.startOperation(ctx, "ValidateSymlinkTarget", "symlink_validation",
		attribute.String("symlink_path", symlink.Path()),
		attribute.String("target_path", symlink.TargetPath()))
	defer cleanup()

	slogger.Debug(op.getContext(), "Validating symlink target", slogger.Fields{
		"symlink_path": symlink.Path(),
		"target_path":  symlink.TargetPath(),
	})

	result, err := s.resolver.ValidateSymlinkTarget(op.getContext(), symlink)
	if err != nil {
		op.finishWithError(err, "Failed to validate symlink target", slogger.Fields{
			"symlink_path": symlink.Path(),
			"target_path":  symlink.TargetPath(),
		})
		return nil, err
	}

	op.addAttributes(
		attribute.Bool("target_exists", result.TargetExists),
		attribute.Bool("is_accessible", result.IsAccessible),
	)
	op.finishWithSuccess("Successfully validated symlink target", slogger.Fields{
		"symlink_path":  symlink.Path(),
		"target_path":   symlink.TargetPath(),
		"target_exists": result.TargetExists,
		"is_accessible": result.IsAccessible,
	})

	return result, nil
}
