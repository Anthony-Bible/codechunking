package api

import (
	"codechunking/internal/application/common/slogger"
	"codechunking/internal/application/dto"
	"codechunking/internal/port/inbound"
	"net/http"
	"strconv"

	"github.com/google/uuid"
)

// ConnectorHandler handles HTTP requests for connector operations.
// The API is read-only; connectors are managed via config file reconciliation.
type ConnectorHandler struct {
	connectorService inbound.ConnectorService
	errorHandler     ErrorHandler
}

// NewConnectorHandler creates a new ConnectorHandler.
func NewConnectorHandler(connectorService inbound.ConnectorService, errorHandler ErrorHandler) *ConnectorHandler {
	return &ConnectorHandler{
		connectorService: connectorService,
		errorHandler:     errorHandler,
	}
}

// ListConnectors handles GET /connectors.
func (h *ConnectorHandler) ListConnectors(w http.ResponseWriter, r *http.Request) {
	query := dto.DefaultConnectorListQuery()

	if ct := r.URL.Query().Get("connector_type"); ct != "" {
		query.ConnectorType = ct
	}
	if status := r.URL.Query().Get("status"); status != "" {
		query.Status = status
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			query.Limit = v
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			query.Offset = v
		}
	}

	response, err := h.connectorService.ListConnectors(r.Context(), query)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	if err := WriteJSON(w, http.StatusOK, response); err != nil {
		slogger.Error(r.Context(), "failed to write connector response", slogger.Fields{"error": err})
	}
}

// GetConnector handles GET /connectors/{id}.
func (h *ConnectorHandler) GetConnector(w http.ResponseWriter, r *http.Request) {
	id, err := extractConnectorIDFromPath(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.connectorService.GetConnector(r.Context(), id)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	if err := WriteJSON(w, http.StatusOK, response); err != nil {
		slogger.Error(r.Context(), "failed to write connector response", slogger.Fields{"error": err})
	}
}

// SyncConnector handles POST /connectors/{id}/sync.
func (h *ConnectorHandler) SyncConnector(w http.ResponseWriter, r *http.Request) {
	id, err := extractConnectorIDFromPath(r)
	if err != nil {
		h.errorHandler.HandleValidationError(w, r, err)
		return
	}

	response, err := h.connectorService.SyncConnector(r.Context(), id)
	if err != nil {
		h.errorHandler.HandleServiceError(w, r, err)
		return
	}

	if err := WriteJSON(w, http.StatusAccepted, response); err != nil {
		slogger.Error(r.Context(), "failed to write connector response", slogger.Fields{"error": err})
	}
}

// extractConnectorIDFromPath extracts and validates the "id" path parameter as a UUID.
func extractConnectorIDFromPath(r *http.Request) (uuid.UUID, error) {
	return newPathParameterExtractor(r).extractUUIDPathValue("id", "connector")
}
