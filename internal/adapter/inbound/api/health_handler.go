package api

import (
	"encoding/json"
	"net/http"

	"codechunking/internal/port/inbound"
)

// HealthHandler handles HTTP requests for health check operations
type HealthHandler struct {
	healthService inbound.HealthService
	errorHandler  ErrorHandler
}

// NewHealthHandler creates a new HealthHandler
func NewHealthHandler(healthService inbound.HealthService, errorHandler ErrorHandler) *HealthHandler {
	return &HealthHandler{
		healthService: healthService,
		errorHandler:  errorHandler,
	}
}

// GetHealth handles GET /health
func (h *HealthHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	response, err := h.healthService.GetHealth(r.Context())
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Return 503 if status is unhealthy, 200 otherwise
	statusCode := http.StatusOK
	if response.Status == "unhealthy" {
		statusCode = http.StatusServiceUnavailable
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}
