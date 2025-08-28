package service

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/domain/valueobject"
	"codechunking/internal/port/outbound"
	"context"
	"io"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// LanguageDetectorService provides language detection capabilities for the application layer.
// It acts as a facade over the language detection port, providing application-specific
// logic and observability features.
type LanguageDetectorService struct {
	detector outbound.LanguageDetector
	helper   *observabilityHelper
}

// NewLanguageDetectorService creates a new LanguageDetectorService instance.
func NewLanguageDetectorService(detector outbound.LanguageDetector) *LanguageDetectorService {
	if detector == nil {
		panic("detector cannot be nil")
	}

	tracer := otel.Tracer("language-detector-service")
	meter := otel.Meter("language-detector-service")

	// Initialize metrics
	detectionCounter, _ := meter.Int64Counter(
		"language_detection_total",
		metric.WithDescription("Total number of language detections performed"),
	)

	detectionDuration, _ := meter.Float64Histogram(
		"language_detection_duration_seconds",
		metric.WithDescription("Duration of language detection operations"),
	)

	detectionErrorCounter, _ := meter.Int64Counter(
		"language_detection_errors_total",
		metric.WithDescription("Total number of language detection errors"),
	)

	helper := &observabilityHelper{
		tracer:                tracer,
		detectionCounter:      detectionCounter,
		detectionDuration:     detectionDuration,
		detectionErrorCounter: detectionErrorCounter,
	}

	return &LanguageDetectorService{
		detector: detector,
		helper:   helper,
	}
}

// DetectFromFilePath detects language based on file path and extension.
func (s *LanguageDetectorService) DetectFromFilePath(
	ctx context.Context,
	filePath string,
) (valueobject.Language, error) {
	op, cleanup := s.helper.startOperation(ctx, "DetectFromFilePath", "filepath",
		attribute.String("file_path", filePath))
	defer cleanup()

	slogger.Info(op.getContext(), "Detecting language from file path", slogger.Fields{
		"file_path": filePath,
	})

	language, err := s.detector.DetectFromFilePath(op.getContext(), filePath)
	if err != nil {
		op.finishWithError(err, "Failed to detect language from file path", slogger.Fields{
			"file_path": filePath,
		})
		return valueobject.Language{}, err
	}

	op.addAttributes(attribute.String("language", language.Name()))
	op.finishWithSuccess("Successfully detected language from file path", slogger.Fields{
		"file_path": filePath,
		"language":  language.Name(),
	})

	return language, nil
}

// DetectFromContent detects language based on file content analysis.
func (s *LanguageDetectorService) DetectFromContent(
	ctx context.Context,
	content []byte,
	hint string,
) (valueobject.Language, error) {
	op, cleanup := s.helper.startOperation(ctx, "DetectFromContent", "content",
		attribute.String("hint", hint),
		attribute.Int("content_size", len(content)))
	defer cleanup()

	slogger.Info(op.getContext(), "Detecting language from content", slogger.Fields{
		"content_size": len(content),
		"hint":         hint,
	})

	language, err := s.detector.DetectFromContent(op.getContext(), content, hint)
	if err != nil {
		op.finishWithError(err, "Failed to detect language from content", slogger.Fields{
			"content_size": len(content),
			"hint":         hint,
		})
		return valueobject.Language{}, err
	}

	op.addAttributes(attribute.String("language", language.Name()))
	op.finishWithSuccess("Successfully detected language from content", slogger.Fields{
		"content_size": len(content),
		"hint":         hint,
		"language":     language.Name(),
	})

	return language, nil
}

// DetectFromReader detects language from a reader interface.
func (s *LanguageDetectorService) DetectFromReader(
	ctx context.Context,
	reader io.Reader,
	filename string,
) (valueobject.Language, error) {
	op, cleanup := s.helper.startOperation(ctx, "DetectFromReader", "reader",
		attribute.String("filename", filename))
	defer cleanup()

	slogger.Info(op.getContext(), "Detecting language from reader", slogger.Fields{
		"filename": filename,
	})

	language, err := s.detector.DetectFromReader(op.getContext(), reader, filename)
	if err != nil {
		op.finishWithError(err, "Failed to detect language from reader", slogger.Fields{
			"filename": filename,
		})
		return valueobject.Language{}, err
	}

	op.addAttributes(attribute.String("language", language.Name()))
	op.finishWithSuccess("Successfully detected language from reader", slogger.Fields{
		"filename": filename,
		"language": language.Name(),
	})

	return language, nil
}

// DetectMultipleLanguages detects all languages present in a file.
func (s *LanguageDetectorService) DetectMultipleLanguages(
	ctx context.Context,
	content []byte,
	filename string,
) ([]valueobject.Language, error) {
	op, cleanup := s.helper.startOperation(ctx, "DetectMultipleLanguages", "multiple",
		attribute.String("filename", filename),
		attribute.Int("content_size", len(content)))
	defer cleanup()

	slogger.Info(op.getContext(), "Detecting multiple languages", slogger.Fields{
		"filename":     filename,
		"content_size": len(content),
	})

	languages, err := s.detector.DetectMultipleLanguages(op.getContext(), content, filename)
	if err != nil {
		op.finishWithError(err, "Failed to detect multiple languages", slogger.Fields{
			"filename": filename,
		})
		return nil, err
	}

	var langNames []string
	for _, lang := range languages {
		langNames = append(langNames, lang.Name())
	}

	op.addAttributes(attribute.Int("languages_count", len(languages)))
	op.finishWithSuccess("Successfully detected multiple languages", slogger.Fields{
		"filename":  filename,
		"languages": langNames,
		"count":     len(languages),
	})

	return languages, nil
}

// DetectBatch performs batch language detection for multiple files efficiently.
func (s *LanguageDetectorService) DetectBatch(
	ctx context.Context,
	files []outbound.FileInfo,
) ([]outbound.DetectionResult, error) {
	op, cleanup := s.helper.startOperation(ctx, "DetectBatch", "batch",
		attribute.Int("files_count", len(files)))
	defer cleanup()

	slogger.Info(op.getContext(), "Starting batch language detection", slogger.Fields{
		"files_count": len(files),
	})

	results, err := s.detector.DetectBatch(op.getContext(), files)
	if err != nil {
		op.finishWithError(err, "Failed batch language detection", slogger.Fields{
			"files_count": len(files),
		})
		return nil, err
	}

	successCount := 0
	errorCount := 0
	for _, result := range results {
		if result.Error != nil {
			errorCount++
		} else {
			successCount++
		}
	}

	op.addAttributes(
		attribute.Int("results_count", len(results)),
		attribute.Int("success_count", successCount),
		attribute.Int("error_count", errorCount),
	)
	op.finishWithSuccess("Completed batch language detection", slogger.Fields{
		"files_count":   len(files),
		"results_count": len(results),
		"success_count": successCount,
		"error_count":   errorCount,
	})

	return results, nil
}
