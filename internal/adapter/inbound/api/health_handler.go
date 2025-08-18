package api

import (
	"fmt"
	"net/http"
	"time"

	"codechunking/internal/application/dto"
	"codechunking/internal/port/inbound"
)

const (
	// Unit conversion constants.
	nanosecondsToMilliseconds = 1e6
)

// HealthHandler handles HTTP requests for health check operations.
type HealthHandler struct {
	healthService inbound.HealthService
	errorHandler  ErrorHandler
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(healthService inbound.HealthService, errorHandler ErrorHandler) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
		errorHandler:  errorHandler,
	}
}

// GetHealth handles GET /health.
func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	response, err := h.healthService.GetHealth(r.Context())
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	// Add performance and NATS-specific headers
	h.addHealthHeaders(w, response, time.Since(start))

	// Return 503 if status is unhealthy, 200 otherwise
	statusCode := http.StatusOK
	if response.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	if writeErr := WriteJSON(w, statusCode, response); writeErr != nil {
		// If JSON writing fails, set content-type and write a simple error response
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		if _, writeFailErr := w.Write([]byte("Health check response encoding failed")); writeFailErr != nil {
			// Unable to recover at this point, but avoid errcheck violation
			_ = writeFailErr
		}
	}
}

// addHealthHeaders adds performance and NATS-specific headers to the response.
func (h *HealthHandler) addHealthHeaders(w http.ResponseWriter, response *dto.HealthResponse, duration time.Duration) {
	// Add health check duration header
	w.Header().
		Set("X-Health-Check-Duration", fmt.Sprintf("%.2fms", float64(duration.Nanoseconds())/nanosecondsToMilliseconds))

	// Add NATS-specific headers if NATS dependency exists
	h.addNATSHeaders(w, response)
}

// addNATSHeaders adds NATS-specific headers to the response.
func (h *HealthHandler) addNATSHeaders(w http.ResponseWriter, response *dto.HealthResponse) {
	natsStatus, exists := response.Dependencies["nats"]
	if !exists {
		return
	}

	natsDetails, ok := natsStatus.Details["nats_health"]
	if !ok {
		return
	}

	natsHealth, detailsOk := natsDetails.(dto.NATSHealthDetails)
	if !detailsOk {
		return
	}

	// Add NATS connection status header
	connectionStatus := "disconnected"
	if natsHealth.Connected {
		connectionStatus = "connected"
	}
	w.Header().Set("X-Nats-Connection-Status", connectionStatus)

	// Add JetStream enabled header
	jetStreamStatus := "disabled"
	if natsHealth.JetStreamEnabled {
		jetStreamStatus = "enabled"
	}
	w.Header().Set("X-Jetstream-Enabled", jetStreamStatus)
}
